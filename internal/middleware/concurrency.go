package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// ConcurrencyLimiter enforces a max-in-flight limit with an optional waiting
// queue. Requests that exceed the concurrent limit wait in the queue up to
// queueTimeout; if the queue is full or the timeout fires, they get 503.
type ConcurrencyLimiter struct {
	sem     chan struct{}
	queue   chan struct{}
	timeout time.Duration
	logger  *slog.Logger
}

// NewConcurrencyLimiter creates a limiter. maxConcurrency is the hard limit on
// in-flight requests. queueSize is the number of waiters before immediate
// rejection. queueTimeout is how long a waiter stays before giving up.
func NewConcurrencyLimiter(maxConcurrency, queueSize int, queueTimeout time.Duration, logger *slog.Logger) *ConcurrencyLimiter {
	if maxConcurrency <= 0 {
		return nil
	}
	cl := &ConcurrencyLimiter{
		sem:     make(chan struct{}, maxConcurrency),
		timeout: queueTimeout,
		logger:  logger,
	}
	if queueSize > 0 {
		cl.queue = make(chan struct{}, queueSize)
	}
	return cl
}

// Wrap returns middleware that enforces the concurrency limit.
func (cl *ConcurrencyLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fast path: acquire a slot immediately.
		select {
		case cl.sem <- struct{}{}:
			defer func() { <-cl.sem }()
			next.ServeHTTP(w, r)
			return
		default:
		}

		// Slow path: queue or reject.
		if cl.queue == nil {
			cl.logger.Warn("request rejected: at concurrency limit", "limit", cap(cl.sem))
			http.Error(w, "server overloaded: too many requests", http.StatusServiceUnavailable)
			return
		}

		// Enter waiting queue.
		select {
		case cl.queue <- struct{}{}:
			// Got a queue slot. Wait for our turn with a timeout.
			timer := time.NewTimer(cl.timeout)
			select {
			case cl.sem <- struct{}{}:
				timer.Stop()
				<-cl.queue // leave queue
				defer func() { <-cl.sem }()
				next.ServeHTTP(w, r)
			case <-timer.C:
				<-cl.queue // leave queue
				cl.logger.Warn("request rejected: queue timeout", "timeout", cl.timeout)
				http.Error(w, "server overloaded: queue timeout", http.StatusServiceUnavailable)
			}
		default:
			cl.logger.Warn("request rejected: queue full", "queue_size", cap(cl.queue), "inflight", cap(cl.sem))
			http.Error(w, "server overloaded: too many requests", http.StatusServiceUnavailable)
		}
	})
}
