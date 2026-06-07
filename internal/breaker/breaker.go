// Package breaker provides a per-provider circuit breaker.
//
// A breaker has three states:
//
//	closed     — calls pass through; consecutive failures are counted
//	open       — calls fail fast with ErrOpen; after CoolDown elapses we go half-open
//	half-open  — a limited number of probe calls is admitted; if HalfOpenSuccess of
//	             them succeed in a row we close again, otherwise we re-open
//
// Design notes:
//
//   - The hot path (Allow) is lock-free: state, failure counter, open-since timestamp
//     and probe counter all live in sync/atomic fields. Only a transition (open→half-open,
//     half-open→closed, etc.) takes the mutex, and only briefly.
//   - A single Breaker is intended per (provider, model) or per provider. The Manager
//     type at the bottom keeps a sync.Map of them keyed by name for cheap lookup.
//   - The breaker classifies errors via the IsFailure callback. Callers should pass
//     a function that returns true for genuine upstream failures (5xx, network) and
//     false for client-side errors (400, 401, ctx cancellation). Without this, a flood
//     of "bad request" responses would needlessly trip the breaker.
package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrOpen is returned by Allow / Call when the breaker is open. Callers should
// treat this as "skip this target" and fall back to another, not as a user-facing error.
var ErrOpen = errors.New("circuit breaker open")

// State is the breaker state. Use the String() method for logging; the int value
// itself is exposed so callers can compare atomically with Load.
type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config controls when the breaker trips and how it recovers.
//
// Defaults are picked to be conservative for an LLM gateway, where occasional
// upstream blips are normal and tripping too aggressively makes the gateway
// look less reliable than the providers it fronts.
type Config struct {
	// FailureThreshold: number of consecutive failures in StateClosed before tripping.
	// Default 5.
	FailureThreshold uint32

	// CoolDown: how long the breaker stays open before admitting half-open probes.
	// Default 10 s.
	CoolDown time.Duration

	// HalfOpenSuccess: number of consecutive successful probes in StateHalfOpen
	// required to fully close the breaker. Default 2.
	HalfOpenSuccess uint32

	// HalfOpenMaxInFlight: at most this many probe calls are admitted concurrently
	// in StateHalfOpen. Default 1 — we want a single canary, not a thundering herd.
	HalfOpenMaxInFlight uint32

	// Now is injectable for tests. Production callers leave it nil and time.Now is used.
	Now func() time.Time
}

func (c *Config) applyDefaults() {
	if c.FailureThreshold == 0 {
		c.FailureThreshold = 5
	}
	if c.CoolDown == 0 {
		c.CoolDown = 10 * time.Second
	}
	if c.HalfOpenSuccess == 0 {
		c.HalfOpenSuccess = 2
	}
	if c.HalfOpenMaxInFlight == 0 {
		c.HalfOpenMaxInFlight = 1
	}
	if c.Now == nil {
		c.Now = time.Now
	}
}

// Breaker is the per-target state machine. Zero value is NOT usable — call New.
type Breaker struct {
	name string
	cfg  Config

	state         atomic.Int32 // State
	failures      atomic.Uint32
	openSinceUnix atomic.Int64 // unix nanos when we transitioned to Open
	probesInFlight atomic.Uint32
	probesOK      atomic.Uint32

	// trMu serializes state transitions only. Hot-path Allow/onSuccess/onFailure
	// stay lock-free; transitions are rare so the contention is negligible.
	trMu sync.Mutex
}

// New builds a Breaker with the given name (used in logs/metrics) and config.
func New(name string, cfg Config) *Breaker {
	cfg.applyDefaults()
	b := &Breaker{name: name, cfg: cfg}
	b.state.Store(int32(StateClosed))
	return b
}

func (b *Breaker) Name() string { return b.name }

func (b *Breaker) State() State { return State(b.state.Load()) }

// Allow reports whether a new call may proceed. If it returns nil the caller
// MUST eventually report the outcome via OnSuccess or OnFailure. If it returns
// ErrOpen the caller should fall back without making the upstream call.
func (b *Breaker) Allow() error {
	switch State(b.state.Load()) {
	case StateClosed:
		return nil
	case StateOpen:
		// Maybe enough time has passed to allow a probe.
		opened := time.Unix(0, b.openSinceUnix.Load())
		if b.cfg.Now().Sub(opened) < b.cfg.CoolDown {
			return ErrOpen
		}
		b.tryTransitionToHalfOpen()
		// Re-check — we may now be half-open or another goroutine may already have
		// snapped us closed.
		return b.allowFromHalfOpenOrAfterTransition()
	case StateHalfOpen:
		return b.allowFromHalfOpenOrAfterTransition()
	}
	return nil
}

// allowFromHalfOpenOrAfterTransition admits probes up to HalfOpenMaxInFlight.
// It's separated so both the Open→HalfOpen transition path and the
// already-HalfOpen path share the same admission logic.
func (b *Breaker) allowFromHalfOpenOrAfterTransition() error {
	if State(b.state.Load()) != StateHalfOpen {
		// Another goroutine may have already closed/opened us; defer to the new state.
		return b.Allow()
	}
	// Reserve a probe slot. CompareAndSwap loop avoids the read-then-write race.
	for {
		cur := b.probesInFlight.Load()
		if cur >= b.cfg.HalfOpenMaxInFlight {
			return ErrOpen
		}
		if b.probesInFlight.CompareAndSwap(cur, cur+1) {
			return nil
		}
	}
}

func (b *Breaker) tryTransitionToHalfOpen() {
	b.trMu.Lock()
	defer b.trMu.Unlock()
	// Re-check under the mutex; another goroutine may have already transitioned.
	if State(b.state.Load()) != StateOpen {
		return
	}
	opened := time.Unix(0, b.openSinceUnix.Load())
	if b.cfg.Now().Sub(opened) < b.cfg.CoolDown {
		return
	}
	b.probesInFlight.Store(0)
	b.probesOK.Store(0)
	b.state.Store(int32(StateHalfOpen))
}

// OnSuccess records a successful call.
func (b *Breaker) OnSuccess() {
	switch State(b.state.Load()) {
	case StateClosed:
		// Reset the failure counter — only consecutive failures trip us.
		b.failures.Store(0)
	case StateHalfOpen:
		// Free the probe slot, then count the success.
		b.probesInFlight.Add(^uint32(0)) // atomic decrement
		ok := b.probesOK.Add(1)
		if ok >= b.cfg.HalfOpenSuccess {
			b.transitionToClosed()
		}
	case StateOpen:
		// Late success arriving after we re-opened — ignore.
	}
}

// OnFailure records a failed call.
func (b *Breaker) OnFailure() {
	switch State(b.state.Load()) {
	case StateClosed:
		if b.failures.Add(1) >= b.cfg.FailureThreshold {
			b.transitionToOpen()
		}
	case StateHalfOpen:
		// Free the probe slot, then re-open immediately — any probe failure
		// during recovery is treated as proof we're still broken.
		b.probesInFlight.Add(^uint32(0))
		b.transitionToOpen()
	case StateOpen:
		// Already open, nothing to do.
	}
}

func (b *Breaker) transitionToOpen() {
	b.trMu.Lock()
	defer b.trMu.Unlock()
	b.state.Store(int32(StateOpen))
	b.openSinceUnix.Store(b.cfg.Now().UnixNano())
	b.failures.Store(0)
	b.probesOK.Store(0)
}

func (b *Breaker) transitionToClosed() {
	b.trMu.Lock()
	defer b.trMu.Unlock()
	b.state.Store(int32(StateClosed))
	b.failures.Store(0)
	b.probesOK.Store(0)
	b.probesInFlight.Store(0)
}

// Manager is a thread-safe registry of breakers keyed by an arbitrary string
// (typically provider name). Lookup is lock-free in the common case via sync.Map.
type Manager struct {
	cfg     Config
	entries sync.Map // map[string]*Breaker
}

// NewManager builds a Manager. The supplied config is used as the default for
// every breaker it lazily creates.
func NewManager(cfg Config) *Manager {
	cfg.applyDefaults()
	return &Manager{cfg: cfg}
}

// Get returns the breaker for the given name, creating it if absent.
func (m *Manager) Get(name string) *Breaker {
	if v, ok := m.entries.Load(name); ok {
		return v.(*Breaker)
	}
	nb := New(name, m.cfg)
	actual, _ := m.entries.LoadOrStore(name, nb)
	return actual.(*Breaker)
}

// Snapshot is the diagnostic view of a single breaker.
type Snapshot struct {
	Name     string
	State    string
	Failures uint32
	ProbesOK uint32
}

// Snapshots returns the current state of every registered breaker. Useful for
// /admin/status endpoints.
func (m *Manager) Snapshots() []Snapshot {
	var out []Snapshot
	m.entries.Range(func(k, v any) bool {
		b := v.(*Breaker)
		out = append(out, Snapshot{
			Name:     b.name,
			State:    b.State().String(),
			Failures: b.failures.Load(),
			ProbesOK: b.probesOK.Load(),
		})
		return true
	})
	return out
}
