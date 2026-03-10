package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/splch/qgo/backend"
)

func TestDefaultBehavior(t *testing.T) {
	b := New("test")
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if job.Backend != "test" {
		t.Errorf("Backend = %q, want %q", job.Backend, "test")
	}
}

func TestStatusSequence(t *testing.T) {
	b := New("test", WithStatusSequence(
		backend.StateSubmitted,
		backend.StateRunning,
		backend.StateCompleted,
	))
	job, _ := b.Submit(context.Background(), &backend.SubmitRequest{})

	for _, want := range []backend.JobState{
		backend.StateSubmitted,
		backend.StateRunning,
		backend.StateCompleted,
		backend.StateCompleted, // repeats last
	} {
		status, err := b.Status(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != want {
			t.Errorf("got %s, want %s", status.State, want)
		}
	}
}

func TestFixedResult(t *testing.T) {
	r := &backend.Result{
		Probabilities: map[string]float64{"000": 1.0},
		Shots:         100,
	}
	b := New("test", WithFixedResult(r))
	result, err := b.Result(context.Background(), "any")
	if err != nil {
		t.Fatal(err)
	}
	if result.Probabilities["000"] != 1.0 {
		t.Errorf("unexpected result: %v", result.Probabilities)
	}
}

func TestSubmitError(t *testing.T) {
	want := errors.New("auth failed")
	b := New("test", WithSubmitError(want))
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{})
	if err != want {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestResultError(t *testing.T) {
	want := errors.New("not found")
	b := New("test", WithResultError(want))
	_, err := b.Result(context.Background(), "any")
	if err != want {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestStatusUnknownJob(t *testing.T) {
	b := New("test")
	_, err := b.Status(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestStatusCallCount(t *testing.T) {
	b := New("test")
	job, _ := b.Submit(context.Background(), &backend.SubmitRequest{})
	for range 5 {
		b.Status(context.Background(), job.ID)
	}
	if got := b.StatusCallCount(job.ID); got != 5 {
		t.Errorf("StatusCallCount = %d, want 5", got)
	}
}
