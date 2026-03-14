package ionq

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestMarshalBellCircuit(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if input.Gateset != "qis" {
		t.Errorf("gateset = %q, want %q", input.Gateset, "qis")
	}
	if input.Qubits != 2 {
		t.Errorf("qubits = %d, want 2", input.Qubits)
	}
	if len(input.Circuit) != 2 { // H + CNOT (measurements skipped)
		t.Errorf("gates = %d, want 2", len(input.Circuit))
	}

	// Verify H gate.
	h := input.Circuit[0]
	if h.Gate != "h" {
		t.Errorf("gate[0] = %q, want %q", h.Gate, "h")
	}
	if h.Target == nil || *h.Target != 0 {
		t.Errorf("gate[0].target = %v, want 0", h.Target)
	}

	// Verify CNOT gate.
	cx := input.Circuit[1]
	if cx.Gate != "cnot" {
		t.Errorf("gate[1] = %q, want %q", cx.Gate, "cnot")
	}
	if cx.Control == nil || *cx.Control != 0 {
		t.Errorf("gate[1].control = %v, want 0", cx.Control)
	}
	if cx.Target == nil || *cx.Target != 1 {
		t.Errorf("gate[1].target = %v, want 1", cx.Target)
	}
}

func TestMarshalRotations(t *testing.T) {
	c, _ := builder.New("rot", 1).
		RX(math.Pi, 0).
		RY(math.Pi/2, 0).
		RZ(math.Pi/4, 0).
		Build()

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if len(input.Circuit) != 3 {
		t.Fatalf("gates = %d, want 3", len(input.Circuit))
	}

	names := []string{"rx", "ry", "rz"}
	rots := []float64{math.Pi, math.Pi / 2, math.Pi / 4}
	for i, g := range input.Circuit {
		if g.Gate != names[i] {
			t.Errorf("gate[%d] = %q, want %q", i, g.Gate, names[i])
		}
		if g.Rotation == nil || math.Abs(*g.Rotation-rots[i]) > 1e-10 {
			t.Errorf("gate[%d].rotation = %v, want %v", i, g.Rotation, rots[i])
		}
	}
}

func TestMarshalSWAP(t *testing.T) {
	c, _ := builder.New("swap", 2).SWAP(0, 1).Build()
	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Circuit) != 1 {
		t.Fatalf("gates = %d, want 1", len(input.Circuit))
	}
	g := input.Circuit[0]
	if g.Gate != "swap" {
		t.Errorf("gate = %q, want %q", g.Gate, "swap")
	}
	if len(g.Targets) != 2 || g.Targets[0] != 0 || g.Targets[1] != 1 {
		t.Errorf("targets = %v, want [0 1]", g.Targets)
	}
}

func TestMarshalNativeGates(t *testing.T) {
	c := ir.New("native", 2, 0, []ir.Operation{
		{Gate: gate.GPI(math.Pi), Qubits: []int{0}},
		{Gate: gate.GPI2(math.Pi / 2), Qubits: []int{1}},
		{Gate: gate.MS(0, math.Pi), Qubits: []int{0, 1}},
	}, nil)

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if input.Gateset != "native" {
		t.Errorf("gateset = %q, want %q", input.Gateset, "native")
	}
	if len(input.Circuit) != 3 {
		t.Fatalf("gates = %d, want 3", len(input.Circuit))
	}

	// GPI: phase should be in turns.
	gpi := input.Circuit[0]
	if gpi.Gate != "gpi" {
		t.Errorf("gate[0] = %q, want %q", gpi.Gate, "gpi")
	}
	wantPhase := 0.5 // π radians = 0.5 turns
	if gpi.Phase == nil || math.Abs(*gpi.Phase-wantPhase) > 1e-10 {
		t.Errorf("gpi.phase = %v, want %v", gpi.Phase, wantPhase)
	}

	// MS: phases in turns, angle = 0.25.
	ms := input.Circuit[2]
	if ms.Gate != "ms" {
		t.Errorf("gate[2] = %q, want %q", ms.Gate, "ms")
	}
	if ms.Angle == nil || *ms.Angle != 0.25 {
		t.Errorf("ms.angle = %v, want 0.25", ms.Angle)
	}
	if len(ms.Phases) != 2 {
		t.Fatalf("ms.phases length = %d, want 2", len(ms.Phases))
	}
	// phi0 = 0 → 0 turns, phi1 = π → 0.5 turns
	if math.Abs(ms.Phases[0]) > 1e-10 {
		t.Errorf("ms.phases[0] = %v, want 0", ms.Phases[0])
	}
	if math.Abs(ms.Phases[1]-0.5) > 1e-10 {
		t.Errorf("ms.phases[1] = %v, want 0.5", ms.Phases[1])
	}
}

func TestMarshalNOPGate(t *testing.T) {
	c := ir.New("nop-test", 2, 0, []ir.Operation{
		{Gate: gate.GPI(0), Qubits: []int{0}},
		{Gate: gate.NOP(1.5), Qubits: nil},
		{Gate: gate.GPI(0), Qubits: []int{1}},
	}, nil)

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if input.Gateset != "native" {
		t.Errorf("gateset = %q, want %q", input.Gateset, "native")
	}
	if len(input.Circuit) != 3 {
		t.Fatalf("gates = %d, want 3", len(input.Circuit))
	}

	nop := input.Circuit[1]
	if nop.Gate != "nop" {
		t.Errorf("gate = %q, want %q", nop.Gate, "nop")
	}
	if nop.Time == nil || *nop.Time != 1.5 {
		t.Errorf("nop.time = %v, want 1.5", nop.Time)
	}
	if nop.Target != nil {
		t.Errorf("nop.target = %v, want nil", nop.Target)
	}
}

func TestMarshalMixedGatesError(t *testing.T) {
	c := ir.New("mixed", 2, 0, []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.GPI(0.5), Qubits: []int{1}},
	}, nil)

	_, err := marshalCircuit(c)
	if err == nil {
		t.Fatal("expected error for mixed gatesets")
	}
}

func TestMarshalJSON(t *testing.T) {
	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var roundtrip ionqInput
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.Qubits != 2 {
		t.Errorf("roundtrip qubits = %d, want 2", roundtrip.Qubits)
	}
	if roundtrip.Gateset != "qis" {
		t.Errorf("roundtrip gateset = %q, want %q", roundtrip.Gateset, "qis")
	}
}

func TestDetectGatesetEmpty(t *testing.T) {
	c := ir.New("empty", 1, 0, nil, nil)
	gs, err := detectGateset(c)
	if err != nil {
		t.Fatal(err)
	}
	if gs != "qis" {
		t.Errorf("empty circuit gateset = %q, want %q", gs, "qis")
	}
}

func TestBitstring(t *testing.T) {
	tests := []struct {
		key       int
		numQubits int
		want      string
	}{
		{0, 2, "00"},
		{1, 2, "01"},
		{2, 2, "10"},
		{3, 2, "11"},
		{0, 3, "000"},
		{7, 3, "111"},
		{5, 3, "101"},
	}
	for _, tt := range tests {
		got := bitstring(tt.key, tt.numQubits)
		if got != tt.want {
			t.Errorf("bitstring(%d, %d) = %q, want %q", tt.key, tt.numQubits, got, tt.want)
		}
	}
}

func TestRadiansToTurns(t *testing.T) {
	tests := []struct {
		rad  float64
		want float64
	}{
		{0, 0},
		{math.Pi, 0.5},
		{2 * math.Pi, 1.0},
		{math.Pi / 2, 0.25},
	}
	for _, tt := range tests {
		got := radiansToTurns(tt.rad)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("radiansToTurns(%v) = %v, want %v", tt.rad, got, tt.want)
		}
	}
}

func TestDelayDroppedInQIS(t *testing.T) {
	// NOP is native-only; delays in QIS circuits should be silently dropped.
	c := ir.New("delay-qis", 1, 0, []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.Delay(500, gate.UnitUs), Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
	}, nil)

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if input.Gateset != "qis" {
		t.Errorf("gateset = %q, want %q", input.Gateset, "qis")
	}
	// Delay should be dropped, leaving only H and X.
	if len(input.Circuit) != 2 {
		t.Fatalf("gates = %d, want 2 (delay should be dropped in QIS)", len(input.Circuit))
	}
	if input.Circuit[0].Gate != "h" {
		t.Errorf("gate[0] = %q, want h", input.Circuit[0].Gate)
	}
	if input.Circuit[1].Gate != "x" {
		t.Errorf("gate[1] = %q, want x", input.Circuit[1].Gate)
	}
}

func TestDelayToNOPNativeGateset(t *testing.T) {
	// Delay should also work in native gateset circuits.
	c := ir.New("delay-native", 1, 0, []ir.Operation{
		{Gate: gate.GPI(0), Qubits: []int{0}},
		{Gate: gate.Delay(100, gate.UnitNs), Qubits: []int{0}},
		{Gate: gate.GPI(0), Qubits: []int{0}},
	}, nil)

	input, err := marshalCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if input.Gateset != "native" {
		t.Errorf("gateset = %q, want %q", input.Gateset, "native")
	}
	nop := input.Circuit[1]
	if nop.Gate != "nop" {
		t.Errorf("gate = %q, want %q", nop.Gate, "nop")
	}
	// 100 ns = 0.1 microseconds.
	if nop.Time == nil || math.Abs(*nop.Time-0.1) > 1e-10 {
		t.Errorf("nop.time = %v, want 0.1", nop.Time)
	}
}
