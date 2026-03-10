package local

import (
	"context"
	"testing"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/builder"
)

func TestSubmitBellState(t *testing.T) {
	b := New()
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.State != backend.StateCompleted {
		t.Fatalf("expected completed, got %s", job.State)
	}

	result, err := b.Result(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Bell state should only have "00" and "11" outcomes.
	for bs := range result.Counts {
		if bs != "00" && bs != "11" {
			t.Errorf("unexpected bitstring %q in Bell state", bs)
		}
	}

	total := 0
	for _, c := range result.Counts {
		total += c
	}
	if total != 1000 {
		t.Errorf("total shots = %d, want 1000", total)
	}
}

func TestSubmitNilCircuit(t *testing.T) {
	b := New()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: nil,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestSubmitTooManyQubits(t *testing.T) {
	b := New(WithMaxQubits(2))
	c, _ := builder.New("big", 4).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected error for too many qubits")
	}
}

func TestSubmitZeroShots(t *testing.T) {
	b := New()
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   0,
	})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestStatusUnknownJob(t *testing.T) {
	b := New()
	_, err := b.Status(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestResultUnknownJob(t *testing.T) {
	b := New()
	_, err := b.Result(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestCancelNoOp(t *testing.T) {
	b := New()
	if err := b.Cancel(context.Background(), "any"); err != nil {
		t.Fatal(err)
	}
}

func TestName(t *testing.T) {
	b := New()
	if b.Name() != "local.simulator" {
		t.Errorf("Name() = %q, want %q", b.Name(), "local.simulator")
	}
}

func TestCancelledContext(t *testing.T) {
	b := New()
	c, _ := builder.New("test", 1).H(0).Build()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Submit(ctx, &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
