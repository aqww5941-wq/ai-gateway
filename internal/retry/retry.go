// Package retry provides bounded exponential-backoff retry with full jitter,
// tuned for upstream LLM API calls.
//
// Design notes:
//
//   - We only retry on errors the caller explicitly marks as retryable. For
//     LLM providers that's typically 429/5xx and network errors — never on
//     400-class client errors or on context cancellation.
//   - We honor Retry-After when the upstream sends one. Without this we risk
//     hammering a rate-limited provider and getting permanently banned.
//   - Full jitter (random(0, backoff)) rather than equal jitter — this gives
//     better convergence under herd conditions, per the AWS Architecture Blog
//     analysis on backoff strategies.
//   - Streaming calls should NOT be retried once the first byte has been sent
//     to the client. The retry helper here is for unary calls; the streaming
//     path retries only before opening the upstream stream.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net"
	"time"

	"ai-gateway/internal/provider"
)

// Policy controls how aggressively we retry.
type Policy struct {
	// MaxAttempts includes the first try. 1 means "no retries".
	// Default 3 (so up to 2 retries after the first failure).
	MaxAttempts int

	// BaseDelay is the unit step for exponential backoff: attempt N waits
	// random(0, BaseDelay * 2^(N-1)), capped at MaxDelay.
	// Default 200ms.
	BaseDelay time.Duration

	// MaxDelay caps the per-attempt wait so a high MaxAttempts can't compound
	// into multi-minute pauses. Default 5 s.
	MaxDelay time.Duration

	// Now and Sleep are injectable for tests.
	Now   func() time.Time
	Sleep func(context.Context, time.Duration) error
}

func (p *Policy) applyDefaults() {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.Sleep == nil {
		p.Sleep = ctxSleep
	}
}

// IsRetryable inspects err and decides whether the next attempt makes sense.
// It is exported so callers can use the same classification when populating
// breaker outcomes or metrics.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Never retry on caller cancellation — that's a deliberate stop.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// Typed upstream HTTP error with a known retryable status code.
	if ue := provider.AsUpstream(err); ue != nil {
		return ue.IsRetryable()
	}
	// Anything that looks like a transient network failure.
	var nerr net.Error
	if errors.As(err, &nerr) {
		return true
	}
	return false
}

// Do runs fn under the retry policy, returning the final result and error.
// fn is called at least once. Between attempts we wait min(Retry-After hint,
// jittered exponential backoff). The function returns immediately if ctx is
// cancelled.
//
// If fn returns a non-retryable error on attempt N, we return that error
// without further attempts — there is no point retrying a 400.
func Do[T any](ctx context.Context, p Policy, fn func(ctx context.Context, attempt int) (T, error)) (T, error) {
	p.applyDefaults()
	var (
		zero T
		last error
	)
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		out, err := fn(ctx, attempt)
		if err == nil {
			return out, nil
		}
		last = err
		if !IsRetryable(err) {
			return zero, err
		}
		if attempt == p.MaxAttempts {
			break
		}
		wait := backoffDelay(p, attempt)
		// Upstream-supplied Retry-After wins if it's longer — never undercut the upstream.
		if ue := provider.AsUpstream(err); ue != nil && ue.RetryAfter > wait {
			wait = ue.RetryAfter
		}
		if wait > p.MaxDelay {
			wait = p.MaxDelay
		}
		if err := p.Sleep(ctx, wait); err != nil {
			return zero, err
		}
	}
	return zero, last
}

// backoffDelay returns a jittered exponential delay for the given attempt
// (1-indexed). It never exceeds Policy.MaxDelay.
func backoffDelay(p Policy, attempt int) time.Duration {
	// 2^(attempt-1), guarded against overflow for absurd attempt counts.
	exp := math.Pow(2, float64(attempt-1))
	if exp > 1<<20 {
		exp = 1 << 20
	}
	cap := time.Duration(float64(p.BaseDelay) * exp)
	if cap > p.MaxDelay {
		cap = p.MaxDelay
	}
	// Full jitter: random in [0, cap].
	return time.Duration(rand.Int64N(int64(cap) + 1))
}

// ctxSleep waits d or until ctx fires, whichever is first.
func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
