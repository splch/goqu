//go:build integration

package ionq

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/builder"
)

func TestIonQSimulatorIntegration(t *testing.T) {
	apiKey := os.Getenv("IONQ_API_KEY")
	if apiKey == "" {
		t.Skip("IONQ_API_KEY not set")
	}

	b := New(apiKey, WithDevice("simulator"))

	// Build a Bell circuit.
	c, err := builder.New("bell-integration", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Submit.
	job, err := b.Submit(ctx, &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
		Name:    "qgo-integration-test",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("Job submitted: %s", job.ID)

	// Poll until done.
	for {
		status, err := b.Status(ctx, job.ID)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		t.Logf("Status: %s", status.State)
		if status.State == backend.StateCompleted {
			break
		}
		if status.State == backend.StateFailed {
			t.Fatalf("Job failed: %s", status.Error)
		}
		time.Sleep(time.Second)
	}

	// Retrieve results.
	result, err := b.Result(ctx, job.ID)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	t.Logf("Probabilities: %v", result.Probabilities)

	// Verify Bell state: only |00⟩ and |11⟩ should appear.
	for bs, p := range result.Probabilities {
		if bs != "00" && bs != "11" {
			t.Errorf("unexpected bitstring %q with probability %v", bs, p)
		}
	}

	p00 := result.Probabilities["00"]
	p11 := result.Probabilities["11"]
	if p00 < 0.3 || p00 > 0.7 {
		t.Errorf("P(00) = %v, expected ~0.5", p00)
	}
	if p11 < 0.3 || p11 > 0.7 {
		t.Errorf("P(11) = %v, expected ~0.5", p11)
	}

	// Test ToCounts conversion.
	counts := result.ToCounts()
	fmt.Printf("Counts (from probabilities): %v\n", counts)
}
