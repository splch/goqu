package emitter

import (
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestEmitWhile(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Qubits: []int{0}, Clbits: []int{0}}, // measure
		{ControlFlow: &ir.ControlFlow{
			Type:      ir.ControlFlowWhile,
			Condition: ir.Condition{Clbit: 0, Value: 1},
			Bodies: [][]ir.Operation{{
				{Gate: gate.X, Qubits: []int{0}},
				{Qubits: []int{0}, Clbits: []int{0}},
			}},
		}},
	}
	c := ir.New("test", 1, 1, ops, nil)
	got, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "while (c[0] == 1)") {
		t.Errorf("missing while statement in:\n%s", got)
	}
	if !strings.Contains(got, "x q[0];") {
		t.Errorf("missing x gate in while body:\n%s", got)
	}
}

func TestEmitFor(t *testing.T) {
	ops := []ir.Operation{
		{ControlFlow: &ir.ControlFlow{
			Type:     ir.ControlFlowFor,
			Bodies:   [][]ir.Operation{{{Gate: gate.X, Qubits: []int{0}}}},
			ForRange: &ir.ForRange{Start: 0, End: 5, Step: 1},
		}},
	}
	c := ir.New("test", 1, 0, ops, nil)
	got, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "for int i in [0:5]") {
		t.Errorf("missing for statement in:\n%s", got)
	}
}

func TestEmitForWithStep(t *testing.T) {
	ops := []ir.Operation{
		{ControlFlow: &ir.ControlFlow{
			Type:     ir.ControlFlowFor,
			Bodies:   [][]ir.Operation{{{Gate: gate.X, Qubits: []int{0}}}},
			ForRange: &ir.ForRange{Start: 0, End: 10, Step: 2},
		}},
	}
	c := ir.New("test", 1, 0, ops, nil)
	got, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "for int i in [0:10:2]") {
		t.Errorf("missing for statement with step in:\n%s", got)
	}
}

func TestEmitIfElse(t *testing.T) {
	ops := []ir.Operation{
		{ControlFlow: &ir.ControlFlow{
			Type:      ir.ControlFlowIfElse,
			Condition: ir.Condition{Clbit: 0, Value: 1},
			Bodies: [][]ir.Operation{
				{{Gate: gate.X, Qubits: []int{0}}},
				{{Gate: gate.Z, Qubits: []int{0}}},
			},
		}},
	}
	c := ir.New("test", 1, 1, ops, nil)
	got, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "if (c[0] == 1)") {
		t.Errorf("missing if statement in:\n%s", got)
	}
	if !strings.Contains(got, "} else {") {
		t.Errorf("missing else in:\n%s", got)
	}
}

func TestEmitSwitch(t *testing.T) {
	ops := []ir.Operation{
		{ControlFlow: &ir.ControlFlow{
			Type: ir.ControlFlowSwitch,
			Bodies: [][]ir.Operation{
				{{Gate: gate.X, Qubits: []int{0}}},
				{{Gate: gate.Y, Qubits: []int{0}}},
				{{Gate: gate.Z, Qubits: []int{0}}}, // default
			},
			SwitchArg: &ir.SwitchArg{
				Clbits:   []int{0, 1},
				Cases:    []int{0, 1},
				Register: "c",
			},
		}},
	}
	c := ir.New("test", 1, 2, ops, nil)
	got, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "switch (c)") {
		t.Errorf("missing switch statement in:\n%s", got)
	}
	if !strings.Contains(got, "case 0:") {
		t.Errorf("missing case 0 in:\n%s", got)
	}
	if !strings.Contains(got, "case 1:") {
		t.Errorf("missing case 1 in:\n%s", got)
	}
	if !strings.Contains(got, "default:") {
		t.Errorf("missing default in:\n%s", got)
	}
}
