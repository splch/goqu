package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultPolicy(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	p := Policy{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	calls := 0
	err := Do(context.Background(), p, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDoExhausted(t *testing.T) {
	p := Policy{
		MaxAttempts:   2,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Millisecond,
		BackoffFactor: 1.0,
	}
	calls := 0
	err := Do(context.Background(), p, func() error {
		calls++
		return errors.New("always fails")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestDoNonRetryable(t *testing.T) {
	p := Policy{
		MaxAttempts:   5,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Millisecond,
		BackoffFactor: 1.0,
		IsRetryable:   func(err error) bool { return false },
	}
	calls := 0
	err := Do(context.Background(), p, func() error {
		calls++
		return errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry)", calls)
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := Policy{
		MaxAttempts:   10,
		InitialDelay:  time.Second, // long delay
		MaxDelay:      time.Second,
		BackoffFactor: 1.0,
	}
	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := Do(ctx, p, func() error {
		calls++
		return errors.New("retry me")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	if !cb.Allow() {
		t.Error("closed breaker should allow")
	}
	if cb.State() != "closed" {
		t.Errorf("state = %q, want closed", cb.State())
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second)
	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("should still allow after 1 failure")
	}
	cb.RecordFailure()
	if cb.Allow() {
		t.Error("should not allow after threshold failures")
	}
	if cb.State() != "open" {
		t.Errorf("state = %q, want open", cb.State())
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second)
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("should allow after success reset")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	if cb.Allow() {
		t.Error("should not allow when open")
	}

	// Wait for reset period.
	time.Sleep(20 * time.Millisecond)
	if !cb.Allow() {
		t.Error("should allow in half-open state")
	}
	if cb.State() != "half-open" {
		t.Errorf("state = %q, want half-open", cb.State())
	}

	// Second call in half-open should be blocked.
	if cb.Allow() {
		t.Error("should block second call in half-open")
	}

	// Success resets to closed.
	cb.RecordSuccess()
	if cb.State() != "closed" {
		t.Errorf("state = %q, want closed after success", cb.State())
	}
}
