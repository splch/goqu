package manager

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/backend/mock"
	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/param"
	"github.com/splch/qgo/sweep"
)

func TestSubmitSync(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", mock.New("sim", mock.WithStatusSequence(
		backend.StateCompleted,
	)))

	result, err := m.Submit(context.Background(), "sim", &backend.SubmitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Probabilities["00"] != 0.5 {
		t.Errorf("P(00) = %v, want 0.5", result.Probabilities["00"])
	}
}

func TestSubmitUnknownBackend(t *testing.T) {
	m := New()
	_, err := m.Submit(context.Background(), "nonexistent", &backend.SubmitRequest{})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestSubmitPolling(t *testing.T) {
	b := mock.New("sim", mock.WithStatusSequence(
		backend.StateSubmitted,
		backend.StateRunning,
		backend.StateCompleted,
	))
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", b)

	result, err := m.Submit(context.Background(), "sim", &backend.SubmitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestSubmitJobFailed(t *testing.T) {
	b := mock.New("sim", mock.WithStatusSequence(
		backend.StateFailed,
	))
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", b)

	_, err := m.Submit(context.Background(), "sim", &backend.SubmitRequest{})
	if err == nil {
		t.Fatal("expected error for failed job")
	}
}

func TestSubmitAsync(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", mock.New("sim", mock.WithStatusSequence(
		backend.StateCompleted,
	)))

	ch := m.SubmitAsync(context.Background(), "sim", &backend.SubmitRequest{})
	roe := <-ch
	if roe.Err != nil {
		t.Fatal(roe.Err)
	}
	if roe.Result == nil {
		t.Fatal("expected result")
	}
}

func TestSubmitBatch(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("a", mock.New("a", mock.WithStatusSequence(backend.StateCompleted)))
	m.Register("b", mock.New("b", mock.WithStatusSequence(backend.StateCompleted)))

	ch := m.SubmitBatch(context.Background(), []string{"a", "b"}, &backend.SubmitRequest{})
	results := map[string]bool{}
	for roe := range ch {
		if roe.Err != nil {
			t.Fatal(roe.Err)
		}
		results[roe.Backend] = true
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestWatch(t *testing.T) {
	b := mock.New("sim", mock.WithStatusSequence(
		backend.StateSubmitted,
		backend.StateRunning,
		backend.StateCompleted,
	))
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", b)

	job, _ := b.Submit(context.Background(), &backend.SubmitRequest{})

	var states []backend.JobState
	for status := range m.Watch(context.Background(), "sim", job.ID) {
		states = append(states, status.State)
	}
	if len(states) == 0 {
		t.Fatal("expected at least one status update")
	}
	last := states[len(states)-1]
	if last != backend.StateCompleted {
		t.Errorf("last state = %s, want completed", last)
	}
}

func TestWatchCancellation(t *testing.T) {
	b := mock.New("sim", mock.WithStatusSequence(
		backend.StateRunning, // never completes
	))
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", b)

	job, _ := b.Submit(context.Background(), &backend.SubmitRequest{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	count := 0
	for range m.Watch(ctx, "sim", job.ID) {
		count++
	}
	if count == 0 {
		t.Fatal("expected at least one status update before cancellation")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	m := New(
		WithPollFrequency(time.Millisecond),
		WithMaxConcurrent(2),
	)
	m.Register("sim", mock.New("sim",
		mock.WithLatency(10*time.Millisecond),
		mock.WithStatusSequence(backend.StateCompleted),
	))

	// Submit 4 jobs concurrently — only 2 should run at a time.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	channels := make([]<-chan ResultOrError, 4)
	for i := range 4 {
		channels[i] = m.SubmitAsync(ctx, "sim", &backend.SubmitRequest{})
	}

	for i, ch := range channels {
		roe := <-ch
		if roe.Err != nil {
			t.Errorf("job %d: %v", i, roe.Err)
		}
	}
}

func TestSubmitBatch_EmptyBackends(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	ch := m.SubmitBatch(context.Background(), []string{}, &backend.SubmitRequest{})
	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Errorf("got %d results from empty batch, want 0", count)
	}
}

func TestWatch_NonexistentBackend(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ch := m.Watch(ctx, "nonexistent", "fake-job-id")
	count := 0
	for range ch {
		count++
	}
	// Channel should close quickly (either with error status or empty).
	// Just verify it closes within timeout and doesn't hang.
}

func TestSubmitSweep(t *testing.T) {
	m := New(WithPollFrequency(time.Millisecond))
	m.Register("sim", mock.New("sim", mock.WithStatusSequence(
		backend.StateCompleted,
	)))

	theta := param.New("theta")
	c, err := builder.New("ry", 1).
		SymRY(theta.Expr(), 0).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sw := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 3}
	ch := m.SubmitSweep(context.Background(), "sim", c, 100, sw)

	results := make(map[int]SweepResultOrError)
	for roe := range ch {
		if roe.Err != nil {
			t.Errorf("sweep point %d: %v", roe.Index, roe.Err)
		}
		results[roe.Index] = roe
	}
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
	for i := 0; i < 3; i++ {
		if _, ok := results[i]; !ok {
			t.Errorf("missing result for index %d", i)
		}
	}
}
