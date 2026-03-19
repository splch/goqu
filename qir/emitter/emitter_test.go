package emitter

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/param"
	"github.com/splch/goqu/qir"
)

func TestEmitBellBase(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"%Result = type opaque",
		"%Qubit = type opaque",
		"define i64 @bell() #0",
		"entry:",
		"call void @__quantum__rt__initialize(ptr null)",
		"br label %body",
		"body:",
		"call void @__quantum__qis__h__body(ptr null)",
		"call void @__quantum__qis__cnot__body(ptr null, ptr inttoptr (i64 1 to ptr))",
		"br label %measurements",
		"measurements:",
		"call void @__quantum__qis__mz__body(",
		"br label %output",
		"output:",
		"call void @__quantum__rt__result_record_output(",
		"ret i64 0",
		`"entry_point"`,
		`"qir_profiles"="base_profile"`,
		`"required_num_qubits"="2"`,
		`"required_num_results"="2"`,
		"!0 = !{i32 1, !\"qir_major_version\", i32 2}",
	}

	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitSingleQubitGates(t *testing.T) {
	c, err := builder.New("single", 1).
		WithClbits(1).
		H(0).X(0).Y(0).Z(0).
		S(0).T(0).Apply(gate.I, 0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	gates := []string{
		"__quantum__qis__h__body",
		"__quantum__qis__x__body",
		"__quantum__qis__y__body",
		"__quantum__qis__z__body",
		"__quantum__qis__s__body",
		"__quantum__qis__t__body",
		// I gate is a no-op and correctly skipped in QIR output.
	}

	for _, g := range gates {
		if !strings.Contains(s, g) {
			t.Errorf("output missing %q\nFull output:\n%s", g, s)
		}
	}
}

func TestEmitDaggerGates(t *testing.T) {
	c, err := builder.New("dagger", 1).
		WithClbits(1).
		Apply(gate.Sdg, 0).
		Apply(gate.Tdg, 0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"__quantum__qis__s__adj",
		"__quantum__qis__t__adj",
	}

	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitParameterizedGates(t *testing.T) {
	c, err := builder.New("rotations", 2).
		RX(math.Pi/4, 0).
		RY(math.Pi/2, 1).
		RZ(math.Pi, 0).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"__quantum__qis__rx__body",
		"__quantum__qis__ry__body",
		"__quantum__qis__rz__body",
		"double",
	}

	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitTwoQubitGates(t *testing.T) {
	c, err := builder.New("two_qubit", 2).
		CNOT(0, 1).
		CZ(0, 1).
		SWAP(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"__quantum__qis__cnot__body",
		"__quantum__qis__cz__body",
		"__quantum__qis__swap__body",
	}

	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitThreeQubitGates(t *testing.T) {
	c, err := builder.New("three_qubit", 3).
		CCX(0, 1, 2).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "__quantum__qis__ccx__body") {
		t.Errorf("output missing ccx\nFull output:\n%s", s)
	}
	if !strings.Contains(s, `"required_num_qubits"="3"`) {
		t.Errorf("output missing required_num_qubits=3\nFull output:\n%s", s)
	}
}

func TestEmitDecomposition(t *testing.T) {
	// SX gate should be decomposed since it's not in the QIR basis.
	c, err := builder.New("decompose", 1).
		WithClbits(1).
		Apply(gate.SX, 0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	// SX should NOT appear as a QIR intrinsic (it was decomposed).
	if strings.Contains(s, "__quantum__qis__sx__body") {
		t.Errorf("output should not contain sx intrinsic (should be decomposed)\nFull output:\n%s", s)
	}

	// The output should still be valid QIR with gate calls.
	if !strings.Contains(s, "__quantum__qis__") {
		t.Errorf("output missing gate intrinsics\nFull output:\n%s", s)
	}
}

func TestEmitWithComments(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c, WithComments(true))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "; QIR Base Profile") {
		t.Errorf("output missing comment header\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "; H q[0]") {
		t.Errorf("output missing gate comment\nFull output:\n%s", s)
	}
}

func TestEmitWithEntryPoint(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).CNOT(0, 1).MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c, WithEntryPoint("my_circuit"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "define i64 @my_circuit()") {
		t.Errorf("output missing custom entry point\nFull output:\n%s", s)
	}
}

func TestEmitProfileOverride(t *testing.T) {
	// Static circuit with profile override to adaptive.
	c, err := builder.New("static", 1).
		WithClbits(1).
		H(0).Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c, WithProfile(qir.AdaptiveProfile))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `"qir_profiles"="adaptive_profile"`) {
		t.Errorf("output missing adaptive profile attribute\nFull output:\n%s", s)
	}
}

func TestEmitRejectsUnboundParams(t *testing.T) {
	theta := param.New("theta")
	c, err := builder.New("variational", 1).
		WithClbits(1).
		SymRY(theta.Expr(), 0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_, err = EmitString(c)
	if err == nil {
		t.Fatal("expected error for unbound parameters")
	}
	if !strings.Contains(err.Error(), "unbound symbolic parameters") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEmitEmptyCircuit(t *testing.T) {
	c, err := builder.New("empty", 0).Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "define i64 @empty()") {
		t.Errorf("output missing entry point\nFull output:\n%s", s)
	}
	if !strings.Contains(s, `"required_num_qubits"="0"`) {
		t.Errorf("output missing required_num_qubits=0\nFull output:\n%s", s)
	}
}

func TestEmitNoMeasurements(t *testing.T) {
	c, err := builder.New("no_meas", 1).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `"required_num_results"="0"`) {
		t.Errorf("output missing required_num_results=0\nFull output:\n%s", s)
	}
}

func TestEmitBaseProfileDetection(t *testing.T) {
	// Static circuit: measurements only at end.
	c, err := builder.New("static", 2).
		H(0).CNOT(0, 1).MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `"qir_profiles"="base_profile"`) {
		t.Errorf("expected base_profile for static circuit\nFull output:\n%s", s)
	}
}

func TestEmitAdaptiveProfileDetection(t *testing.T) {
	// Dynamic circuit: mid-circuit measurement + conditioned gate.
	c, err := builder.New("teleport", 3).
		WithClbits(3).
		H(0).CNOT(0, 1).
		Measure(0, 0).
		IfBlock(0, 1, func(b *builder.Builder) { b.X(1) }).
		Measure(1, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `"qir_profiles"="adaptive_profile"`) {
		t.Errorf("expected adaptive_profile for dynamic circuit\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "__quantum__rt__read_result") {
		t.Errorf("adaptive profile should use read_result\nFull output:\n%s", s)
	}
}

func TestEmitQubitRefs(t *testing.T) {
	c, err := builder.New("refs", 3).
		H(0).H(1).H(2).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	// Qubit 0 should use "ptr null".
	if !strings.Contains(s, "call void @__quantum__qis__h__body(ptr null)") {
		t.Errorf("qubit 0 should use ptr null\nFull output:\n%s", s)
	}
	// Qubit 1 should use inttoptr.
	if !strings.Contains(s, "ptr inttoptr (i64 1 to ptr)") {
		t.Errorf("qubit 1 should use inttoptr\nFull output:\n%s", s)
	}
	// Qubit 2 should use inttoptr.
	if !strings.Contains(s, "ptr inttoptr (i64 2 to ptr)") {
		t.Errorf("qubit 2 should use inttoptr\nFull output:\n%s", s)
	}
}

func TestEmitModuleFlags(t *testing.T) {
	c, err := builder.New("flags", 1).WithClbits(1).H(0).Measure(0, 0).Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"!llvm.module.flags = !{!0, !1, !2, !3}",
		`!"qir_major_version", i32 2`,
		`!"qir_minor_version", i32 0`,
		`!"dynamic_qubit_management", i1 false`,
		`!"dynamic_result_management", i1 false`,
	}

	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitAttributeGroups(t *testing.T) {
	c, err := builder.New("attrs", 1).WithClbits(1).H(0).Measure(0, 0).Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `attributes #0 = {`) {
		t.Errorf("output missing attribute group #0\nFull output:\n%s", s)
	}
	if !strings.Contains(s, `attributes #1 = { "irreversible" }`) {
		t.Errorf("output missing attribute group #1\nFull output:\n%s", s)
	}
}

func TestEmitReset(t *testing.T) {
	c, err := builder.New("reset_test", 1).
		WithClbits(1).
		H(0).Reset(0).H(0).Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "__quantum__qis__reset__body") {
		t.Errorf("output missing reset intrinsic\nFull output:\n%s", s)
	}
	// Reset makes it dynamic, so should be adaptive profile.
	if !strings.Contains(s, `"qir_profiles"="adaptive_profile"`) {
		t.Errorf("reset circuit should use adaptive profile\nFull output:\n%s", s)
	}
}

func TestEmitControlledRotations(t *testing.T) {
	c, err := builder.New("ctrl_rot", 2).
		Apply(gate.CRZ(math.Pi/4), 0, 1).
		Apply(gate.CRX(math.Pi/2), 0, 1).
		Apply(gate.CRY(math.Pi), 0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"__quantum__qis__crz__body",
		"__quantum__qis__crx__body",
		"__quantum__qis__cry__body",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestProfileString(t *testing.T) {
	if qir.BaseProfile.String() != "base_profile" {
		t.Errorf("expected base_profile, got %s", qir.BaseProfile.String())
	}
	if qir.AdaptiveProfile.String() != "adaptive_profile" {
		t.Errorf("expected adaptive_profile, got %s", qir.AdaptiveProfile.String())
	}
}

func TestDetectProfile(t *testing.T) {
	// Static circuit.
	static, _ := builder.New("s", 1).WithClbits(1).H(0).Measure(0, 0).Build()
	if qir.DetectProfile(static) != qir.BaseProfile {
		t.Error("expected BaseProfile for static circuit")
	}

	// Dynamic circuit.
	dynamic, _ := builder.New("d", 2).
		WithClbits(2).
		H(0).Measure(0, 0).
		IfBlock(0, 1, func(b *builder.Builder) { b.X(1) }).
		Measure(1, 1).
		Build()
	if qir.DetectProfile(dynamic) != qir.AdaptiveProfile {
		t.Error("expected AdaptiveProfile for dynamic circuit")
	}
}

// TestEmitDeclarationsComplete verifies that all used intrinsics are declared.
func TestEmitDeclarationsComplete(t *testing.T) {
	c, err := builder.New("all_gates", 3).
		H(0).X(1).Y(2).Z(0).S(1).T(2).Apply(gate.I, 0).
		RX(0.5, 0).RY(0.5, 1).RZ(0.5, 2).
		CNOT(0, 1).CZ(1, 2).SWAP(0, 2).
		CCX(0, 1, 2).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	// Every "call void @func(" in the body should have a matching "declare" or "define".
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "call void @") {
			continue
		}
		// Extract function name.
		start := strings.Index(line, "@")
		end := strings.Index(line[start:], "(")
		if start < 0 || end < 0 {
			continue
		}
		fnName := line[start : start+end]
		// Check it's declared.
		declPattern := fnName + "("
		if !strings.Contains(s, "declare void "+declPattern) &&
			!strings.Contains(s, "declare i1 "+declPattern) &&
			!strings.Contains(s, "define i64 "+declPattern) {
			t.Errorf("function %s is called but not declared\nFull output:\n%s", fnName, s)
		}
	}
}

// TestEmitLargeCircuit ensures the emitter handles larger circuits.
func TestEmitLargeCircuit(t *testing.T) {
	b := builder.New("large", 10)
	for i := range 10 {
		b = b.H(i)
	}
	for i := range 9 {
		b = b.CNOT(i, i+1)
	}
	b = b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, `"required_num_qubits"="10"`) {
		t.Errorf("output missing required_num_qubits=10\nFull output:\n%s", s)
	}
	if !strings.Contains(s, `"required_num_results"="10"`) {
		t.Errorf("output missing required_num_results=10\nFull output:\n%s", s)
	}
}

func TestEmitIfElseBlock(t *testing.T) {
	c, err := builder.New("if_else", 2).
		WithClbits(2).
		H(0).Measure(0, 0).
		IfElseBlock(0, 1,
			func(b *builder.Builder) { b.X(1) },
			func(b *builder.Builder) { b.Z(1) },
		).
		Measure(1, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"__quantum__rt__read_result",
		"br i1",
		"__quantum__qis__x__body",
		"__quantum__qis__z__body",
		`"adaptive_profile"`,
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

// Suppress unused import warnings.
var _ = param.New
