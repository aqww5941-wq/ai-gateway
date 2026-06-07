package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// TestCoalescer_SingleFlightDeduplication — N concurrent callers with the
// same key should produce exactly 1 upstream call.
func TestCoalescer_SingleFlightDeduplication(t *testing.T) {
	c := NewCoalescer()
	var calls atomic.Int32
	start := make(chan struct{})

	fn := func(ctx context.Context) (*provider.ChatResponse, error) {
		calls.Add(1)
		// Pause so concurrent callers all land in the in-flight window.
		time.Sleep(50 * time.Millisecond)
		return &provider.ChatResponse{ID: "shared"}, nil
	}

	const n = 50
	var wg sync.WaitGroup
	var sharedCnt atomic.Int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, shared, err := c.Do(context.Background(), "k", fn)
			if err != nil {
				t.Errorf("Do returned err: %v", err)
				return
			}
			if shared {
				sharedCnt.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream invoked %d times, want exactly 1", got)
	}
	// At least N-1 callers should have piggybacked (the leader sees shared=false).
	if got := sharedCnt.Load(); got < n-1 {
		t.Fatalf("only %d/%d callers reported shared, want >= %d", got, n, n-1)
	}
}

// TestCoalescer_DifferentKeysDoNotShare — coalescing must be key-scoped.
func TestCoalescer_DifferentKeysDoNotShare(t *testing.T) {
	c := NewCoalescer()
	var calls atomic.Int32
	fn := func(ctx context.Context) (*provider.ChatResponse, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &provider.ChatResponse{}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = c.Do(context.Background(), string(rune('a'+i)), fn)
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != 5 {
		t.Fatalf("upstream invoked %d times, want 5 (one per key)", got)
	}
}

// TestCoalescer_PropagatesError — errors from the leader are returned to
// every shared caller, so a flaky upstream doesn't get hidden.
func TestCoalescer_PropagatesError(t *testing.T) {
	c := NewCoalescer()
	want := &provider.UpstreamError{StatusCode: 503}
	fn := func(ctx context.Context) (*provider.ChatResponse, error) {
		time.Sleep(20 * time.Millisecond)
		return nil, want
	}

	var wg sync.WaitGroup
	var errs atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := c.Do(context.Background(), "k", fn)
			if err != nil {
				errs.Add(1)
			}
		}()
	}
	wg.Wait()
	if errs.Load() != 10 {
		t.Fatalf("only %d/10 callers got err", errs.Load())
	}
}
