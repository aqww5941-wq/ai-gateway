package retry

import (
	"context"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// BenchmarkDo_SuccessFirstAttempt — the common case. Every call succeeds on
// attempt 1, so Do must NOT pay for backoff, jitter, or extra allocations.
func BenchmarkDo_SuccessFirstAttempt(b *testing.B) {
	resp := &provider.ChatResponse{}
	p := Policy{MaxAttempts: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Do(context.Background(), p, func(ctx context.Context, attempt int) (*provider.ChatResponse, error) {
			return resp, nil
		})
	}
}

// BenchmarkDo_RetriesTwice — the unhappy path. Two retryable failures, third
// attempt succeeds. Includes the backoff sleep, so a small jitter helps.
func BenchmarkDo_RetriesTwice(b *testing.B) {
	resp := &provider.ChatResponse{}
	p := Policy{MaxAttempts: 3, BaseDelay: time.Microsecond, MaxDelay: time.Microsecond}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		_, _ = Do(context.Background(), p, func(ctx context.Context, n int) (*provider.ChatResponse, error) {
			attempt++
			if attempt < 3 {
				return nil, &provider.UpstreamError{StatusCode: 503}
			}
			return resp, nil
		})
	}
}
