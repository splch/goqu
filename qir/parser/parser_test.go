package parser

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/qir/emitter"
)

func TestParseBellCircuit(t *testing.T) {
	qirSource := `
%Result = type opaque
%Qubit = type opaque

@0 = internal constant [3 x i8] c"r0\00"
@1 = internal constant [3 x i8] c"r1\00"
@2 = internal constant [7 x i8] c"output\00"

define i64 @bell() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  call void @__quantum__qis__h__body(ptr null)
  call void @__quantum__qis__cnot__body(ptr null, ptr inttoptr (i64 1 to ptr))
  br label %measurements
measurements:
  call void @__quantum__qis__mz__body(ptr null, ptr writeonly null)
  call void @__quantum__qis__mz__body(ptr inttoptr (i64 1 to ptr), ptr writeonly inttoptr (i64 1 to ptr))
  br label %output
output:
  call void @__quantum__rt__array_record_output(i64 2, ptr @2)
  call void @__quantum__rt__result_record_output(ptr null, ptr @0)
  call void @__quantum__rt__result_record_output(ptr inttoptr (i64 1 to ptr), ptr @1)
  ret i64 0
}

declare void @__quantum__qis__h__body(ptr)
declare void @__quantum__qis__cnot__body(ptr, ptr)
declare void @__quantum__qis__mz__body(ptr, ptr writeonly) #1
declare void @__quantum__rt__initialize(ptr)
declare void @__quantum__rt__array_record_output(i64, ptr)
declare void @__quantum__rt__result_record_output(ptr, ptr)

attributes #0 = { "entry_point" "qir_profiles"="base_profile" "output_labeling_schema"="schema_id" "required_num_qubits"="2" "required_num_results"="2" }
attributes #1 = { "irreversible" }

!llvm.module.flags = !{!0, !1, !2, !3}
!0 = !{i32 1, !"qir_major_version", i32 2}
!1 = !{i32 7, !"qir_minor_version", i32 0}
!2 = !{i32 1, !"dynamic_qubit_management", i1 false}
!3 = !{i32 1, !"dynamic_result_management", i1 false}
`

	c, err := ParseString(qirSource)
	if err != nil {
		t.Fatal(err)
	}

	if c.Name() != "bell" {
		t.Errorf("expected name 'bell', got %q", c.Name())
	}
	if c.NumQubits() != 2 {
		t.Errorf("expected 2 qubits, got %d", c.NumQubits())
	}
	if c.NumClbits() != 2 {
		t.Errorf("expected 2 clbits, got %d", c.NumClbits())
	}

	ops := c.Ops()

	// Should have: H, CNOT, Meas(0), Meas(1) = 4 ops.
	if len(ops) != 4 {
		t.Fatalf("expected 4 ops, got %d", len(ops))
	}

	// First op: H on qubit 0.
	if ops[0].Gate == nil || ops[0].Gate.Name() != "H" {
		t.Errorf("op 0: expected H, got %v", ops[0].Gate)
	}
	if len(ops[0].Qubits) != 1 || ops[0].Qubits[0] != 0 {
		t.Errorf("op 0: expected qubit [0], got %v", ops[0].Qubits)
	}

	// Second op: CNOT on qubits 0, 1.
	if ops[1].Gate == nil || ops[1].Gate.Name() != "CNOT" {
		t.Errorf("op 1: expected CNOT, got %v", ops[1].Gate)
	}
	if len(ops[1].Qubits) != 2 || ops[1].Qubits[0] != 0 || ops[1].Qubits[1] != 1 {
		t.Errorf("op 1: expected qubits [0,1], got %v", ops[1].Qubits)
	}

	// Third op: measurement on qubit 0 -> clbit 0.
	if ops[2].Gate != nil {
		t.Errorf("op 2: expected measurement (nil gate), got %v", ops[2].Gate)
	}
	if len(ops[2].Clbits) != 1 || ops[2].Clbits[0] != 0 {
		t.Errorf("op 2: expected clbit [0], got %v", ops[2].Clbits)
	}

	// Fourth op: measurement on qubit 1 -> clbit 1.
	if ops[3].Gate != nil {
		t.Errorf("op 3: expected measurement (nil gate), got %v", ops[3].Gate)
	}
	if len(ops[3].Clbits) != 1 || ops[3].Clbits[0] != 1 {
		t.Errorf("op 3: expected clbit [1], got %v", ops[3].Clbits)
	}
}

func TestParseAllFixedGates(t *testing.T) {
	qirSource := `
define i64 @gates() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  call void @__quantum__qis__h__body(ptr null)
  call void @__quantum__qis__x__body(ptr null)
  call void @__quantum__qis__y__body(ptr null)
  call void @__quantum__qis__z__body(ptr null)
  call void @__quantum__qis__s__body(ptr null)
  call void @__quantum__qis__s__adj(ptr null)
  call void @__quantum__qis__t__body(ptr null)
  call void @__quantum__qis__t__adj(ptr null)
  call void @__quantum__qis__id__body(ptr null)
  call void @__quantum__qis__cnot__body(ptr null, ptr inttoptr (i64 1 to ptr))
  call void @__quantum__qis__cz__body(ptr null, ptr inttoptr (i64 1 to ptr))
  call void @__quantum__qis__swap__body(ptr null, ptr inttoptr (i64 1 to ptr))
  call void @__quantum__qis__ccnot__body(ptr null, ptr inttoptr (i64 1 to ptr), ptr inttoptr (i64 2 to ptr))
  br label %measurements
measurements:
  br label %output
output:
  ret i64 0
}

attributes #0 = { "entry_point" "required_num_qubits"="3" "required_num_results"="0" }
`

	c, err := ParseString(qirSource)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	expectedGates := []string{
		"H", "X", "Y", "Z", "S", "S\u2020", "T", "T\u2020", "I",
		"CNOT", "CZ", "SWAP", "CCX",
	}

	if len(ops) != len(expectedGates) {
		t.Fatalf("expected %d ops, got %d", len(expectedGates), len(ops))
	}

	for i, expected := range expectedGates {
		if ops[i].Gate == nil {
			t.Errorf("op %d: expected gate %q, got nil", i, expected)
			continue
		}
		if ops[i].Gate.Name() != expected {
			t.Errorf("op %d: expected %q, got %q", i, expected, ops[i].Gate.Name())
		}
	}
}

func TestParseParameterizedGates(t *testing.T) {
	qirSource := `
define i64 @param() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  call void @__quantum__qis__rx__body(double 7.853982e-01, ptr null)
  call void @__quantum__qis__ry__body(double 1.570796e+00, ptr inttoptr (i64 1 to ptr))
  call void @__quantum__qis__rz__body(double 3.141593e+00, ptr null)
  br label %measurements
measurements:
  br label %output
output:
  ret i64 0
}

attributes #0 = { "entry_point" "required_num_qubits"="2" "required_num_results"="0" }
`

	c, err := ParseString(qirSource)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ops))
	}

	// RX gate.
	if ops[0].Gate == nil {
		t.Fatal("op 0: expected gate, got nil")
	}
	params := ops[0].Gate.Params()
	if len(params) != 1 || math.Abs(params[0]-math.Pi/4) > 0.001 {
		t.Errorf("op 0: expected RX(pi/4), got params %v", params)
	}
	if ops[0].Qubits[0] != 0 {
		t.Errorf("op 0: expected qubit 0, got %d", ops[0].Qubits[0])
	}

	// RY gate on qubit 1.
	if ops[1].Qubits[0] != 1 {
		t.Errorf("op 1: expected qubit 1, got %d", ops[1].Qubits[0])
	}
}

func TestParseReset(t *testing.T) {
	qirSource := `
define i64 @reset_test() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  call void @__quantum__qis__h__body(ptr null)
  call void @__quantum__qis__reset__body(ptr null)
  call void @__quantum__qis__h__body(ptr null)
  br label %measurements
measurements:
  br label %output
output:
  ret i64 0
}

attributes #0 = { "entry_point" "required_num_qubits"="1" "required_num_results"="0" }
`

	c, err := ParseString(qirSource)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ops))
	}

	if ops[1].Gate == nil || ops[1].Gate.Name() != "reset" {
		t.Errorf("op 1: expected reset, got %v", ops[1].Gate)
	}
}

func TestParseWithNameOverride(t *testing.T) {
	qirSource := `
define i64 @original() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  br label %measurements
measurements:
  br label %output
output:
  ret i64 0
}
attributes #0 = { "entry_point" "required_num_qubits"="0" "required_num_results"="0" }
`

	c, err := ParseString(qirSource, WithName("custom_name"))
	if err != nil {
		t.Fatal(err)
	}

	if c.Name() != "custom_name" {
		t.Errorf("expected name 'custom_name', got %q", c.Name())
	}
}

// TestRoundTrip builds a circuit, emits to QIR, parses back, and verifies.
func TestRoundTrip(t *testing.T) {
	// Build a Bell circuit.
	original, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Emit to QIR.
	qirStr, err := emitter.EmitString(original)
	if err != nil {
		t.Fatal(err)
	}

	// Parse back.
	parsed, err := ParseString(qirStr)
	if err != nil {
		t.Fatalf("parse failed: %v\nQIR:\n%s", err, qirStr)
	}

	// Verify structural equivalence.
	if parsed.NumQubits() != original.NumQubits() {
		t.Errorf("qubit count: got %d, want %d", parsed.NumQubits(), original.NumQubits())
	}
	if parsed.NumClbits() != original.NumClbits() {
		t.Errorf("clbit count: got %d, want %d", parsed.NumClbits(), original.NumClbits())
	}

	parsedOps := parsed.Ops()
	originalOps := original.Ops()
	if len(parsedOps) != len(originalOps) {
		t.Fatalf("op count: got %d, want %d", len(parsedOps), len(originalOps))
	}

	for i := range originalOps {
		origOp := originalOps[i]
		parsOp := parsedOps[i]

		// Compare gate names.
		if origOp.Gate == nil && parsOp.Gate == nil {
			// Both measurements.
			if len(origOp.Clbits) != len(parsOp.Clbits) {
				t.Errorf("op %d: clbit count mismatch", i)
			}
		} else if origOp.Gate != nil && parsOp.Gate != nil {
			if origOp.Gate.Name() != parsOp.Gate.Name() {
				t.Errorf("op %d: gate name got %q, want %q", i, parsOp.Gate.Name(), origOp.Gate.Name())
			}
		} else {
			t.Errorf("op %d: gate nil mismatch", i)
		}

		// Compare qubit indices.
		if len(origOp.Qubits) != len(parsOp.Qubits) {
			t.Errorf("op %d: qubit count mismatch", i)
		} else {
			for j := range origOp.Qubits {
				if origOp.Qubits[j] != parsOp.Qubits[j] {
					t.Errorf("op %d qubit %d: got %d, want %d", i, j, parsOp.Qubits[j], origOp.Qubits[j])
				}
			}
		}
	}
}

func TestRoundTripParameterized(t *testing.T) {
	original, err := builder.New("rotations", 2).
		RX(math.Pi/4, 0).
		RY(math.Pi/2, 1).
		RZ(math.Pi, 0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	qirStr, err := emitter.EmitString(original)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseString(qirStr)
	if err != nil {
		t.Fatalf("parse failed: %v\nQIR:\n%s", err, qirStr)
	}

	parsedOps := parsed.Ops()
	originalOps := original.Ops()

	if len(parsedOps) != len(originalOps) {
		t.Fatalf("op count: got %d, want %d", len(parsedOps), len(originalOps))
	}

	// Verify parameterized gates have approximately correct parameters.
	for i := range originalOps {
		if originalOps[i].Gate == nil || parsedOps[i].Gate == nil {
			continue
		}
		origParams := originalOps[i].Gate.Params()
		parsParams := parsedOps[i].Gate.Params()
		if len(origParams) != len(parsParams) {
			t.Errorf("op %d: param count got %d, want %d", i, len(parsParams), len(origParams))
			continue
		}
		for j := range origParams {
			if math.Abs(origParams[j]-parsParams[j]) > 1e-4 {
				t.Errorf("op %d param %d: got %f, want %f", i, j, parsParams[j], origParams[j])
			}
		}
	}
}

func TestRoundTripLargeCircuit(t *testing.T) {
	b := builder.New("ghz", 5)
	b = b.H(0)
	for i := range 4 {
		b = b.CNOT(i, i+1)
	}
	b = b.MeasureAll()
	original, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	qirStr, err := emitter.EmitString(original)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseString(qirStr)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.NumQubits() != 5 {
		t.Errorf("expected 5 qubits, got %d", parsed.NumQubits())
	}

	// H + 4 CNOT + 5 measurements = 10 ops.
	if parsed.NumOps() != 10 {
		t.Errorf("expected 10 ops, got %d", parsed.NumOps())
	}
}

func TestParseTailCall(t *testing.T) {
	// Some QIR emitters (Q#, Quantinuum) use "tail call" syntax.
	qirSource := `
define i64 @main() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  tail call void @__quantum__qis__h__body(ptr null)
  tail call void @__quantum__qis__cnot__body(ptr null, ptr nonnull inttoptr (i64 1 to ptr))
  br label %measurements
measurements:
  call void @__quantum__qis__mz__body(ptr null, ptr writeonly null)
  br label %output
output:
  ret i64 0
}

attributes #0 = { "entry_point" "required_num_qubits"="2" "required_num_results"="1" }
`

	c, err := ParseString(qirSource)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ops))
	}
	if ops[0].Gate.Name() != "H" {
		t.Errorf("expected H, got %q", ops[0].Gate.Name())
	}
	if ops[1].Gate.Name() != "CNOT" {
		t.Errorf("expected CNOT, got %q", ops[1].Gate.Name())
	}
}

func TestParseReaderInterface(t *testing.T) {
	qirSource := `
define i64 @reader_test() #0 {
entry:
  call void @__quantum__rt__initialize(ptr null)
  br label %body
body:
  call void @__quantum__qis__h__body(ptr null)
  br label %measurements
measurements:
  br label %output
output:
  ret i64 0
}
attributes #0 = { "entry_point" "required_num_qubits"="1" "required_num_results"="0" }
`

	c, err := Parse(strings.NewReader(qirSource))
	if err != nil {
		t.Fatal(err)
	}

	if c.NumQubits() != 1 {
		t.Errorf("expected 1 qubit, got %d", c.NumQubits())
	}
}
