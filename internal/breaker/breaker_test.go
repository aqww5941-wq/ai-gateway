package breaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock advances only when we tell it to — required because the breaker's
// cool-down comparison must be deterministic in tests.
type fakeClock struct{ now atomic.Int64 }

func (c *fakeClock) Now() time.Time          { return time.Unix(0, c.now.Load()) }
func (c *fakeClock) Advance(d time.Duration) { c.now.Add(int64(d)) }

func newTestBreaker(cfg Config) (*Breaker, *fakeClock) {
	clk := &fakeClock{}
	clk.now.Store(time.Now().UnixNano())
	cfg.Now = clk.Now
	return New("t", cfg), clk
}

// TestClosedToOpenOnThreshold verifies the basic trip behavior.
func TestClosedToOpenOnThreshold(t *testing.T) {
	b, _ := newTestBreaker(Config{FailureThreshold: 3, CoolDown: time.Second})

	for i := 0; i < 2; i++ {
		if err := b.Allow(); err != nil {
			t.Fatalf("Allow returned %v at i=%d, expected nil", err, i)
		}
		b.OnFailure()
	}
	if b.State() != StateClosed {
		t.Fatalf("state after 2 failures = %s, want closed", b.State())
	}
	// Third failure trips.
	if err := b.Allow(); err != nil {
		t.Fatalf("Allow returned %v on 3rd call, expected nil", err)
	}
	b.OnFailure()
	if b.State() != StateOpen {
		t.Fatalf("state after 3 failures = %s, want open", b.State())
	}
	if err := b.Allow(); err != ErrOpen {
		t.Fatalf("Allow when open = %v, want ErrOpen", err)
	}
}

// TestSuccessResetsFailureCounter — only *consecutive* failures should trip.
func TestSuccessResetsFailureCounter(t *testing.T) {
	b, _ := newTestBreaker(Config{FailureThreshold: 3})

	b.OnFailure()
	b.OnFailure()
	b.OnSuccess() // resets counter
	b.OnFailure()
	b.OnFailure()
	if b.State() != StateClosed {
		t.Fatalf("state = %s, want closed (success should have reset counter)", b.State())
	}
}

// TestOpenToHalfOpenAfterCooldown — once the cool-down passes, Allow should
// admit probes, with at most HalfOpenMaxInFlight in flight.
func TestOpenToHalfOpenAfterCooldown(t *testing.T) {
	b, clk := newTestBreaker(Config{
		FailureThreshold:    1,
		CoolDown:            100 * time.Millisecond,
		HalfOpenSuccess:     2,
		HalfOpenMaxInFlight: 1,
	})
	b.OnFailure() // open

	if err := b.Allow(); err != ErrOpen {
		t.Fatalf("Allow immediately after open = %v, want ErrOpen", err)
	}

	clk.Advance(200 * time.Millisecond)
	if err := b.Allow(); err != nil {
		t.Fatalf("Allow after cool-down = %v, want nil (probe admitted)", err)
	}
	if b.State() != StateHalfOpen {
		t.Fatalf("state = %s, want half-open", b.State())
	}
	// A second concurrent probe must be denied — only one canary at a time.
	if err := b.Allow(); err != ErrOpen {
		t.Fatalf("second concurrent probe = %v, want ErrOpen", err)
	}
}

// TestHalfOpenRecoversToClosed — N consecutive successful probes close the breaker.
func TestHalfOpenRecoversToClosed(t *testing.T) {
	b, clk := newTestBreaker(Config{
		FailureThreshold:    1,
		CoolDown:            10 * time.Millisecond,
		HalfOpenSuccess:     2,
		HalfOpenMaxInFlight: 1,
	})
	b.OnFailure()
	clk.Advance(20 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatal("probe 1 admit failed")
	}
	b.OnSuccess()
	if b.State() != StateHalfOpen {
		t.Fatalf("after 1 probe success state = %s, want half-open", b.State())
	}
	if err := b.Allow(); err != nil {
		t.Fatal("probe 2 admit failed")
	}
	b.OnSuccess()
	if b.State() != StateClosed {
		t.Fatalf("after 2 probe successes state = %s, want closed", b.State())
	}
}

// TestHalfOpenReopensOnFailure — even one probe failure during recovery
// is treated as the upstream still being broken.
func TestHalfOpenReopensOnFailure(t *testing.T) {
	b, clk := newTestBreaker(Config{
		FailureThreshold:    1,
		CoolDown:            10 * time.Millisecond,
		HalfOpenSuccess:     2,
		HalfOpenMaxInFlight: 1,
	})
	b.OnFailure()
	clk.Advance(20 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatal("probe admit failed")
	}
	b.OnFailure()
	if b.State() != StateOpen {
		t.Fatalf("after probe failure state = %s, want open", b.State())
	}
}

// TestConcurrentAllowDoesNotOverbook — under heavy concurrency, the breaker
// must not admit more than HalfOpenMaxInFlight probes simultaneously.
func TestConcurrentAllowDoesNotOverbook(t *testing.T) {
	b, clk := newTestBreaker(Config{
		FailureThreshold:    1,
		CoolDown:            time.Millisecond,
		HalfOpenSuccess:     1000, // keep it half-open during the burst
		HalfOpenMaxInFlight: 3,
	})
	b.OnFailure()
	clk.Advance(10 * time.Millisecond)

	var admitted atomic.Uint32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Allow(); err == nil {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := admitted.Load(); got != 3 {
		t.Fatalf("admitted %d concurrent probes, want exactly 3", got)
	}
}

// TestManagerSharedBreaker — Get is idempotent: two calls for the same key
// return the same instance.
func TestManagerSharedBreaker(t *testing.T) {
	m := NewManager(Config{})
	a := m.Get("openai")
	b := m.Get("openai")
	if a != b {
		t.Fatal("Manager.Get returned distinct instances for same key")
	}
	c := m.Get("claude")
	if a == c {
		t.Fatal("Manager.Get returned same instance for different keys")
	}
}
