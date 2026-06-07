package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// TestRetriesUntilSuccess — first 2 attempts fail with retryable errors,
// 3rd succeeds. We should see all 3 attempts.
func TestRetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	out, err := Do(context.Background(), Policy{
		MaxAttempts: 5,
		BaseDelay:   time.Microsecond,
		MaxDelay:    time.Microsecond,
	}, func(ctx context.Context, attempt int) (string, error) {
		n := attempts.Add(1)
		if n < 3 {
			return "", &provider.UpstreamError{StatusCode: 503}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("Do returned %v, want nil", err)
	}
	if out != "ok" {
		t.Fatalf("out = %q, want ok", out)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

// TestStopsOnNonRetryable — a 400 should NOT trigger another attempt.
func TestStopsOnNonRetryable(t *testing.T) {
	var attempts atomic.Int32
	_, err := Do(context.Background(), Policy{MaxAttempts: 5}, func(ctx context.Context, attempt int) (string, error) {
		attempts.Add(1)
		return "", &provider.UpstreamError{StatusCode: 400}
	})
	if err == nil {
		t.Fatal("Do returned nil, expected non-retryable error to surface")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no retries on 400)", got)
	}
}

// TestHonorsRetryAfter — when the upstream sends Retry-After: 2s, we must
// wait at least that long, even if our jittered backoff would have been shorter.
func TestHonorsRetryAfter(t *testing.T) {
	var slept []time.Duration
	policy := Policy{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Second,
		Sleep: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	}
	_, _ = Do(context.Background(), policy, func(ctx context.Context, attempt int) (string, error) {
		return "", &provider.UpstreamError{StatusCode: 429, RetryAfter: 2 * time.Second}
	})
	if len(slept) != 1 {
		t.Fatalf("slept %d times, want 1", len(slept))
	}
	if slept[0] < 2*time.Second {
		t.Fatalf("slept %s, want >= 2s (Retry-After honor)", slept[0])
	}
}

// TestRespectsContextCancellation — once ctx is cancelled mid-wait, Do should
// return promptly with the ctx error, not the upstream error.
func TestRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Do(ctx, Policy{MaxAttempts: 5}, func(ctx context.Context, attempt int) (string, error) {
		return "", &provider.UpstreamError{StatusCode: 503}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// fakeNetErr satisfies net.Error so IsRetryable treats it as transient.
type fakeNetErr struct{}

func (fakeNetErr) Error() string   { return "fake-net" }
func (fakeNetErr) Timeout() bool   { return true }
func (fakeNetErr) Temporary() bool { return true }

func TestRetriesOnNetworkError(t *testing.T) {
	var attempts atomic.Int32
	_, err := Do(context.Background(), Policy{
		MaxAttempts: 3,
		BaseDelay:   time.Microsecond,
	}, func(ctx context.Context, attempt int) (string, error) {
		attempts.Add(1)
		var _ net.Error = fakeNetErr{}
		return "", fmt.Errorf("dial failed: %w", fakeNetErr{})
	})
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3 (all attempts used)", got)
	}
}

// TestMaxAttemptsOne — MaxAttempts:1 means "no retries"; one call total.
func TestMaxAttemptsOne(t *testing.T) {
	var attempts atomic.Int32
	_, _ = Do(context.Background(), Policy{MaxAttempts: 1}, func(ctx context.Context, attempt int) (string, error) {
		attempts.Add(1)
		return "", &provider.UpstreamError{StatusCode: 503}
	})
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}
