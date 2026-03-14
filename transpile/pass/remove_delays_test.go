package pass

import (
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/transpile/target"
)

func TestRemoveDelays(t *testing.T) {
	c, err := builder.New("test", 2).
		H(0).
		Delay(0, 100, gate.UnitNs).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveDelays(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveDelays: %v", err)
	}

	ops := result.Ops()
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops after delay removal, got %d", len(ops))
	}
	if ops[0].Gate != gate.H {
		t.Errorf("op 0: expected H, got %s", ops[0].Gate.Name())
	}
	if ops[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", ops[1].Gate.Name())
	}
}

func TestRemoveDelaysNoDelays(t *testing.T) {
	c, err := builder.New("test", 1).
		H(0).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveDelays(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveDelays: %v", err)
	}

	if len(result.Ops()) != 1 {
		t.Errorf("expected 1 op, got %d", len(result.Ops()))
	}
}
