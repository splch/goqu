package defcal

import (
	"math"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

// test hardware configuration
var (
	testPort  = pulse.MustPort("d0", 1e-9)
	testFrame = pulse.MustFrame("q0_drive", testPort, 0, 0)
)

func makeHCalibration() ProgramFunc {
	return func(_ []float64) (*pulse.Program, error) {
		T := 50e-9
		amp := math.Pi / (2 * T) // pi/2 rotation
		wf := waveform.MustConstant(complex(amp, 0), T)
		return pulse.NewBuilder("H_cal").
			AddPort(testPort).
			AddFrame(testFrame).
			Play(testFrame, wf).
			ShiftPhase(testFrame, math.Pi).
			Build()
	}
}

func makeRXCalibration() ProgramFunc {
	return func(params []float64) (*pulse.Program, error) {
		theta := params[0]
		T := 50e-9
		amp := theta / T
		wf := waveform.MustConstant(complex(amp, 0), T)
		return pulse.NewBuilder("RX_cal").
			AddPort(testPort).
			AddFrame(testFrame).
			Play(testFrame, wf).
			Build()
	}
}

func TestNewTable(t *testing.T) {
	tbl := NewTable()
	_, ok := tbl.Lookup("H", []int{0})
	if ok {
		t.Error("empty table should return false")
	}
}

func TestAddAndLookup(t *testing.T) {
	tbl := NewTable()
	tbl.Add("H", []int{0}, makeHCalibration())

	fn, ok := tbl.Lookup("H", []int{0})
	if !ok {
		t.Fatal("expected to find H on qubit 0")
	}
	prog, err := fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Stats().NumInstructions != 2 {
		t.Errorf("instructions = %d, want 2", prog.Stats().NumInstructions)
	}
}

func TestLookupDefault(t *testing.T) {
	tbl := NewTable()
	tbl.Add("H", nil, makeHCalibration()) // gate-level default

	fn, ok := tbl.Lookup("H", []int{3})
	if !ok {
		t.Fatal("expected gate-level default for H")
	}
	prog, err := fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Stats().NumInstructions == 0 {
		t.Error("expected non-empty program from default calibration")
	}
}

func TestLookupPriority(t *testing.T) {
	tbl := NewTable()

	// Default: returns program with ShiftPhase
	tbl.Add("H", nil, makeHCalibration()) // 2 instructions

	// Qubit-specific override: returns just a Play
	tbl.Add("H", []int{0}, func(_ []float64) (*pulse.Program, error) {
		T := 50e-9
		wf := waveform.MustConstant(complex(math.Pi/(2*T), 0), T)
		return pulse.NewBuilder("H_q0_override").
			AddPort(testPort).
			AddFrame(testFrame).
			Play(testFrame, wf).
			Build()
	})

	fn, ok := tbl.Lookup("H", []int{0})
	if !ok {
		t.Fatal("expected qubit-specific H")
	}
	prog, err := fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Override has 1 instruction, default has 2.
	if prog.Stats().NumInstructions != 1 {
		t.Errorf("instructions = %d, want 1 (override)", prog.Stats().NumInstructions)
	}
}

func TestCompileSimple(t *testing.T) {
	tbl := NewTable()
	tbl.Add("H", nil, makeHCalibration())

	c := ir.New("test", 1, 0, []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}, nil)

	prog, err := Compile(c, tbl, []pulse.Port{testPort}, map[int]pulse.Frame{0: testFrame})
	if err != nil {
		t.Fatal(err)
	}
	// H calibration produces 2 instructions (Play + ShiftPhase).
	if n := prog.Stats().NumInstructions; n != 2 {
		t.Errorf("instructions = %d, want 2", n)
	}
}

func TestCompileParameterized(t *testing.T) {
	tbl := NewTable()
	tbl.Add("RX", nil, makeRXCalibration())

	theta := 1.23
	c := ir.New("test", 1, 0, []ir.Operation{
		{Gate: gate.RX(theta), Qubits: []int{0}},
	}, nil)

	prog, err := Compile(c, tbl, []pulse.Port{testPort}, map[int]pulse.Frame{0: testFrame})
	if err != nil {
		t.Fatal(err)
	}
	if n := prog.Stats().NumInstructions; n != 1 {
		t.Errorf("instructions = %d, want 1", n)
	}
}

func TestCompileMissingCalibration(t *testing.T) {
	tbl := NewTable() // empty

	c := ir.New("test", 1, 0, []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}, nil)

	_, err := Compile(c, tbl, []pulse.Port{testPort}, map[int]pulse.Frame{0: testFrame})
	if err == nil {
		t.Error("expected error for missing calibration")
	}
}

func TestCompileBarrierAndMeasurement(t *testing.T) {
	tbl := NewTable()
	tbl.Add("X", nil, func(_ []float64) (*pulse.Program, error) {
		T := 50e-9
		wf := waveform.MustConstant(complex(math.Pi/T, 0), T)
		return pulse.NewBuilder("X_cal").
			AddPort(testPort).
			AddFrame(testFrame).
			Play(testFrame, wf).
			Build()
	})

	// Use builder to get proper barrier gate type.
	c, err := builder.New("test", 1).
		X(0).
		Barrier(0).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	prog, err2 := Compile(c, tbl, []pulse.Port{testPort}, map[int]pulse.Frame{0: testFrame})
	if err2 != nil {
		t.Fatal(err2)
	}

	// X → Play(1), Barrier(1), Measurement → Capture(1) = 3 instructions.
	instrs := prog.Instructions()
	if len(instrs) != 3 {
		t.Fatalf("instructions = %d, want 3", len(instrs))
	}
	if _, ok := instrs[0].(pulse.Play); !ok {
		t.Errorf("instrs[0] type = %T, want Play", instrs[0])
	}
	if _, ok := instrs[1].(pulse.Barrier); !ok {
		t.Errorf("instrs[1] type = %T, want Barrier", instrs[1])
	}
	if _, ok := instrs[2].(pulse.Capture); !ok {
		t.Errorf("instrs[2] type = %T, want Capture", instrs[2])
	}
}
