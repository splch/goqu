package decompose

import (
	"math"
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// apply1qOp builds the 4x4 matrix for a single-qubit operation acting on
// a 2-qubit system. qubit indicates which qubit (0 or 1) the gate targets.
// Convention: qubit 0 is the MSB (left tensor factor), qubit 1 is the LSB.
func apply1qOp(g gate.Gate, qubit int) []complex128 {
	if qubit == 0 {
		return Tensor(g.Matrix(), 2, Eye(2), 2)
	}
	return Tensor(Eye(2), 2, g.Matrix(), 2)
}

// apply1qOp3q builds the 8x8 matrix for a single-qubit operation on a
// 3-qubit system. qubit is 0, 1, or 2 (MSB to LSB).
func apply1qOp3q(g gate.Gate, qubit int) []complex128 {
	switch qubit {
	case 0:
		return Tensor(g.Matrix(), 2, Eye(4), 4)
	case 1:
		top := Tensor(Eye(2), 2, g.Matrix(), 2)
		return Tensor(top, 4, Eye(2), 2)
	case 2:
		return Tensor(Eye(4), 4, g.Matrix(), 2)
	}
	return nil
}

// circuitUnitary2q computes the 4x4 unitary for a sequence of operations
// on a 2-qubit system (qubits 0 and 1).
func circuitUnitary2q(ops []ir.Operation) []complex128 {
	u := Eye(4)
	for _, op := range ops {
		var opMat []complex128
		switch op.Gate.Qubits() {
		case 1:
			opMat = apply1qOp(op.Gate, op.Qubits[0])
		case 2:
			q0, q1 := op.Qubits[0], op.Qubits[1]
			if q0 == 0 && q1 == 1 {
				opMat = op.Gate.Matrix()
			} else if q0 == 1 && q1 == 0 {
				// CNOT(1,0): swap, apply CNOT(0,1), swap
				sw := gate.SWAP.Matrix()
				opMat = MatMul(sw, MatMul(op.Gate.Matrix(), sw, 4), 4)
			} else {
				opMat = op.Gate.Matrix()
			}
		default:
			opMat = op.Gate.Matrix()
		}
		u = MatMul(opMat, u, 4)
	}
	return u
}

// circuitUnitary3q computes the 8x8 unitary for a sequence of operations
// on a 3-qubit system (qubits 0, 1, 2).
func circuitUnitary3q(ops []ir.Operation) []complex128 {
	u := Eye(8)
	for _, op := range ops {
		var opMat []complex128
		switch op.Gate.Qubits() {
		case 1:
			opMat = apply1qOp3q(op.Gate, op.Qubits[0])
		case 2:
			opMat = embed2qIn3q(op.Gate.Matrix(), op.Qubits[0], op.Qubits[1])
		default:
			opMat = op.Gate.Matrix()
		}
		u = MatMul(opMat, u, 8)
	}
	return u
}

// embed2qIn3q embeds a 2-qubit gate matrix into a 3-qubit (8x8) system.
// q0 and q1 are the qubit indices (0, 1, or 2) the gate acts on.
func embed2qIn3q(mat []complex128, q0, q1 int) []complex128 {
	dim := 8

	// Build the full matrix by iterating over all 3-qubit basis states.
	result := make([]complex128, dim*dim)
	for inBits := range dim {
		for outBits := range dim {
			// Extract the bits for q0 and q1 from inBits and outBits.
			inQ0 := (inBits >> (2 - q0)) & 1
			inQ1 := (inBits >> (2 - q1)) & 1
			outQ0 := (outBits >> (2 - q0)) & 1
			outQ1 := (outBits >> (2 - q1)) & 1

			// The "other" qubit must be the same.
			otherQ := 3 - q0 - q1 // the third qubit index
			inOther := (inBits >> (2 - otherQ)) & 1
			outOther := (outBits >> (2 - otherQ)) & 1
			if inOther != outOther {
				continue
			}

			// Gate matrix element: row = (outQ0, outQ1), col = (inQ0, inQ1)
			gRow := outQ0*2 + outQ1
			gCol := inQ0*2 + inQ1
			result[outBits*dim+inBits] = mat[gRow*4+gCol]
		}
	}
	return result
}

func TestDecomposeByRule_SWAP_To3CX(t *testing.T) {
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.SWAP, []int{0, 1}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(SWAP): returned nil")
	}

	// Count CNOTs.
	cxCount := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			cxCount++
		}
	}
	if cxCount != 3 {
		t.Errorf("DecomposeByRule(SWAP): expected 3 CNOTs, got %d", cxCount)
	}

	// Verify matrix equivalence.
	got := circuitUnitary2q(ops)
	want := gate.SWAP.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(SWAP): decomposed circuit does not match SWAP matrix up to global phase")
	}
}

func TestDecomposeByRule_CZ_ToHCXH(t *testing.T) {
	basis := []string{"CX", "CNOT", "H", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.CZ, []int{0, 1}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CZ): returned nil")
	}

	// Should have H, CX, H pattern.
	cxCount := 0
	hCount := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			cxCount++
		}
		if op.Gate == gate.H {
			hCount++
		}
	}
	if cxCount != 1 {
		t.Errorf("DecomposeByRule(CZ): expected 1 CNOT, got %d", cxCount)
	}
	if hCount != 2 {
		t.Errorf("DecomposeByRule(CZ): expected 2 H gates, got %d", hCount)
	}

	got := circuitUnitary2q(ops)
	want := gate.CZ.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CZ): decomposed circuit does not match CZ matrix up to global phase")
	}
}

func TestDecomposeByRule_CY(t *testing.T) {
	basis := []string{"CX", "CNOT", "RZ", "SX", "X", "S", "Sdg"}
	ops := DecomposeByRule(gate.CY, []int{0, 1}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CY): returned nil")
	}

	got := circuitUnitary2q(ops)
	want := gate.CY.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CY): decomposed circuit does not match CY matrix up to global phase")
	}
}

func TestDecomposeByRule_CP(t *testing.T) {
	phi := math.Pi / 4
	cpGate := gate.CP(phi)
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	ops := DecomposeByRule(cpGate, []int{0, 1}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CP(pi/4)): returned nil")
	}

	// Count CNOTs -- CP decomposes to 2 CNOTs.
	cxCount := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			cxCount++
		}
	}
	if cxCount != 2 {
		t.Errorf("DecomposeByRule(CP): expected 2 CNOTs, got %d", cxCount)
	}

	got := circuitUnitary2q(ops)
	want := cpGate.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CP(pi/4)): decomposed circuit does not match CP matrix up to global phase")
	}
}

func TestDecomposeByRule_CRZ(t *testing.T) {
	theta := math.Pi / 3
	crzGate := gate.CRZ(theta)
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	ops := DecomposeByRule(crzGate, []int{0, 1}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CRZ(pi/3)): returned nil")
	}

	// CRZ decomposes to 2 CNOTs.
	cxCount := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			cxCount++
		}
	}
	if cxCount != 2 {
		t.Errorf("DecomposeByRule(CRZ): expected 2 CNOTs, got %d", cxCount)
	}

	got := circuitUnitary2q(ops)
	want := crzGate.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CRZ(pi/3)): decomposed circuit does not match CRZ matrix up to global phase")
	}
}

func TestDecomposeByRule_CCX(t *testing.T) {
	basis := []string{"CX", "CNOT", "H", "T", "Tdg", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.CCX, []int{0, 1, 2}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CCX): returned nil")
	}

	// CCX decomposes to 6 CNOTs + single-qubit gates.
	cxCount := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			cxCount++
		}
	}
	if cxCount != 6 {
		t.Errorf("DecomposeByRule(CCX): expected 6 CNOTs, got %d", cxCount)
	}

	// All gates should be 1-qubit or CNOT (basis gates).
	for i, op := range ops {
		nq := op.Gate.Qubits()
		if nq > 2 {
			t.Errorf("DecomposeByRule(CCX): op[%d] is %d-qubit gate %s, expected at most 2-qubit",
				i, nq, op.Gate.Name())
		}
	}

	// Verify via matrix product.
	got := circuitUnitary3q(ops)
	want := gate.CCX.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CCX): decomposed circuit does not match CCX matrix up to global phase")
	}
}

func TestDecomposeByRule_CSWAP(t *testing.T) {
	basis := []string{"CX", "CNOT", "H", "T", "Tdg", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.CSWAP, []int{0, 1, 2}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(CSWAP): returned nil")
	}

	// All gates should be at most 2-qubit.
	for i, op := range ops {
		nq := op.Gate.Qubits()
		if nq > 2 {
			t.Errorf("DecomposeByRule(CSWAP): op[%d] is %d-qubit gate %s", i, nq, op.Gate.Name())
		}
	}

	got := circuitUnitary3q(ops)
	want := gate.CSWAP.Matrix()
	if _, ok := GlobalPhase(got, want, 1e-10); !ok {
		t.Errorf("DecomposeByRule(CSWAP): decomposed circuit does not match CSWAP matrix up to global phase")
	}
}

func TestDecomposeByRule_Identity(t *testing.T) {
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.I, []int{0}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(I): returned nil, expected empty slice")
	}
	if len(ops) != 0 {
		t.Errorf("DecomposeByRule(I): expected 0 ops, got %d", len(ops))
	}
}

func TestDecomposeByRule_H(t *testing.T) {
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	ops := DecomposeByRule(gate.H, []int{0}, basis)
	if ops == nil {
		t.Fatal("DecomposeByRule(H): returned nil")
	}
	// H = RZ(pi/2) * SX * RZ(pi/2) => 3 ops.
	if len(ops) != 3 {
		t.Errorf("DecomposeByRule(H): expected 3 ops, got %d", len(ops))
	}
}

func TestDecomposeByRule_NilForUnknownBasis(t *testing.T) {
	// With a basis that has no CX or MS, DecomposeByRule should return nil.
	ops := DecomposeByRule(gate.SWAP, []int{0, 1}, []string{"RY", "RZ"})
	if ops != nil {
		t.Errorf("DecomposeByRule with no CX/MS basis: expected nil, got %d ops", len(ops))
	}
}

func TestDecomposeByRule_CP_VariousAngles(t *testing.T) {
	angles := []float64{math.Pi / 6, math.Pi / 2, math.Pi, 2.5}
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	for _, phi := range angles {
		cpGate := gate.CP(phi)
		ops := DecomposeByRule(cpGate, []int{0, 1}, basis)
		if ops == nil {
			t.Errorf("DecomposeByRule(CP(%.4f)): returned nil", phi)
			continue
		}
		got := circuitUnitary2q(ops)
		want := cpGate.Matrix()
		if _, ok := GlobalPhase(got, want, 1e-10); !ok {
			t.Errorf("DecomposeByRule(CP(%.4f)): decomposed circuit does not match", phi)
		}
	}
}

func TestDecomposeByRule_CRZ_VariousAngles(t *testing.T) {
	angles := []float64{math.Pi / 5, math.Pi / 2, math.Pi, 3.0}
	basis := []string{"CX", "CNOT", "RZ", "SX", "X"}
	for _, theta := range angles {
		crzGate := gate.CRZ(theta)
		ops := DecomposeByRule(crzGate, []int{0, 1}, basis)
		if ops == nil {
			t.Errorf("DecomposeByRule(CRZ(%.4f)): returned nil", theta)
			continue
		}
		got := circuitUnitary2q(ops)
		want := crzGate.Matrix()
		if _, ok := GlobalPhase(got, want, 1e-10); !ok {
			t.Errorf("DecomposeByRule(CRZ(%.4f)): decomposed circuit does not match", theta)
		}
	}
}

func TestSWAP_MatrixProduct_3CNOT(t *testing.T) {
	// Directly verify: CNOT(0,1) * CNOT(1,0) * CNOT(0,1) = SWAP.
	cx01 := gate.CNOT.Matrix()

	// CNOT(1,0): swap target/control using SWAP conjugation.
	sw := gate.SWAP.Matrix()
	cx10 := MatMul(sw, MatMul(cx01, sw, 4), 4)

	// Product: CNOT(0,1) * CNOT(1,0) * CNOT(0,1)
	tmp := MatMul(cx10, cx01, 4)
	product := MatMul(cx01, tmp, 4)

	want := gate.SWAP.Matrix()
	if _, ok := GlobalPhase(product, want, 1e-10); !ok {
		t.Error("CNOT(0,1)*CNOT(1,0)*CNOT(0,1) does not equal SWAP up to global phase")
	}
}

func TestCZ_MatrixProduct_HCXH(t *testing.T) {
	// CZ = (I tensor H) * CNOT * (I tensor H)
	ih := Tensor(Eye(2), 2, gate.H.Matrix(), 2)
	cx := gate.CNOT.Matrix()

	tmp := MatMul(cx, ih, 4)
	product := MatMul(ih, tmp, 4)

	want := gate.CZ.Matrix()
	if _, ok := GlobalPhase(product, want, 1e-10); !ok {
		t.Error("(I*H)*CX*(I*H) does not equal CZ up to global phase")
	}
}
