// Package retry provides retry policies and a circuit breaker for job submission.
package retry

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Policy configures retry behavior for transient failures.
type Policy struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	IsRetryable   func(error) bool // nil means retry all errors
}

// DefaultPolicy returns a sensible default: 3 attempts, 1s initial, 30s max, 2x backoff.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// Do executes fn with retries according to the policy.
func Do(ctx context.Context, p Policy, fn func() error) error {
	var lastErr error
	delay := p.InitialDelay

	for attempt := range p.MaxAttempts {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if p.IsRetryable != nil && !p.IsRetryable(lastErr) {
			return lastErr
		}
		if attempt == p.MaxAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
		delay = time.Duration(float64(delay) * p.BackoffFactor)
		if delay > p.MaxDelay {
			delay = p.MaxDelay
		}
	}
	return fmt.Errorf("retry: exhausted %d attempts: %w", p.MaxAttempts, lastErr)
}

// CircuitBreaker prevents repeated calls to a failing backend.
type CircuitBreaker struct {
	threshold  int
	resetAfter time.Duration

	mu       sync.Mutex
	failures int
	state    cbState
	openedAt time.Time
}

type cbState int

const (
	cbClosed   cbState = iota // normal operation
	cbOpen                    // rejecting calls
	cbHalfOpen                // allowing one probe call
)

// NewCircuitBreaker creates a breaker that opens after threshold consecutive
// failures and resets after the given duration.
func NewCircuitBreaker(threshold int, resetAfter time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:  threshold,
		resetAfter: resetAfter,
	}
}

// Allow reports whether a call should proceed. Returns false when the
// breaker is open (unless the reset period has elapsed).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbClosed:
		return true
	case cbOpen:
		if time.Since(cb.openedAt) >= cb.resetAfter {
			cb.state = cbHalfOpen
			return true
		}
		return false
	case cbHalfOpen:
		return false // only one probe at a time
	}
	return true
}

// RecordSuccess records a successful call, resetting the breaker.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = cbClosed
}

// RecordFailure records a failed call. If the failure count reaches the
// threshold, the breaker opens.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures >= cb.threshold {
		cb.state = cbOpen
		cb.openedAt = time.Now()
	}
}

// State returns the current breaker state as a string.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbClosed:
		return "closed"
	case cbOpen:
		return "open"
	case cbHalfOpen:
		return "half-open"
	}
	return "unknown"
}
