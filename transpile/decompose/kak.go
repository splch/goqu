package decompose

import (
	"math"
	"math/cmplx"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/mathutil"
)

// KAK decomposes an arbitrary 2-qubit unitary into at most 3 CNOTs + single-qubit rotations.
// It is equivalent to KAKForBasis with BasisZYZ.
func KAK(m []complex128, q0, q1 int) []ir.Operation {
	return KAKForBasis(m, q0, q1, BasisZYZ)
}

// KAKForBasis decomposes an arbitrary 2-qubit unitary using the KAK (Cartan)
// decomposition with the specified Euler convention for single-qubit rotations.
// Reference: Tucci, arXiv:quant-ph/0507171; Vatan-Williams, arXiv:quant-ph/0308006.
func KAKForBasis(m []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	if isGlobalPhaseOf(m, Eye(4), 1e-9) {
		return nil
	}
	if isGlobalPhaseOf(m, gate.CNOT.Matrix(), 1e-9) {
		return []ir.Operation{{Gate: gate.CNOT, Qubits: []int{q0, q1}}}
	}
	if isGlobalPhaseOf(m, gate.SWAP.Matrix(), 1e-9) {
		return []ir.Operation{
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.CNOT, Qubits: []int{q1, q0}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	}
	if isGlobalPhaseOf(m, gate.CZ.Matrix(), 1e-9) {
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.H, Qubits: []int{q1}},
		}
	}
	if isGlobalPhaseOf(m, gate.CY.Matrix(), 1e-9) {
		return []ir.Operation{
			{Gate: gate.Sdg, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.S, Qubits: []int{q1}},
		}
	}
	if ops := tryLocalDecomposeForBasis(m, q0, q1, basis); ops != nil {
		return ops
	}
	return kakGeneralForBasis(m, q0, q1, basis)
}

// KakParams extracts the KAK decomposition parameters from a 4×4 unitary:
//
//	U = (K1l⊗K1r) · exp(i(x·XX + y·YY + z·ZZ)) · (K2l⊗K2r)
//
// Returns the four 2×2 K-matrices, the Weyl parameters (x,y,z), and the
// count of nonzero Weyl parameters.
//
// Algorithm roadmap (Vatan-Williams, arXiv:quant-ph/0308006):
//  1. Normalize the 4x4 unitary to SU(4) by dividing out the determinant phase.
//  2. Transform to the magic basis: Up = Q† · U · Q. In this basis, local
//     unitaries (A⊗B) become real orthogonal matrices, separating local from
//     non-local content.
//  3. Compute M2 = Up^T · Up (transpose, not conjugate transpose). This matrix
//     encodes the non-local part and is symmetric-unitary.
//  4. Diagonalize M2 with a real orthogonal matrix P to extract eigenvalues.
//  5. From the eigenvalue phases, compute the Weyl chamber coordinates (x, y, z)
//     that parameterize the non-local content of the two-qubit gate.
//  6. Try all 16 sign combinations for the diagonal square root D^{1/2}
//     (each eigenvalue has two square roots), selecting the combination that
//     minimizes the number of nonzero Weyl parameters (fewer parameters mean
//     fewer CNOTs in the final circuit).
//  7. Reconstruct the local unitaries K1l, K1r (before the interaction) and
//     K2l, K2r (after the interaction) by factoring the Kronecker products
//     back into 2x2 matrices.
//
// The magic basis Q = (1/sqrt(2)) * [[1,0,0,i],[0,i,1,0],[0,i,-1,0],[1,0,0,-i]]
// diagonalizes the CNOT interaction: in this basis, the non-local part of any
// two-qubit unitary becomes a diagonal matrix. This is why transforming to the
// magic basis cleanly separates local rotations from entangling content.
//
// The Weyl chamber is a geometric region that parameterizes the non-local
// equivalence classes of two-qubit unitaries. Every two-qubit gate, up to
// local single-qubit rotations, corresponds to a unique point (x, y, z)
// in the Weyl chamber. The number of nonzero coordinates determines the
// minimum CNOT count: 0 CNOTs for the identity class, 1 for CNOT-class
// gates (single nonzero coordinate equal to pi/4), 2 for one nonzero
// coordinate at a non-CNOT value, and 3 for the general case.
func KakParams(m []complex128) (k1l, k1r, k2l, k2r []complex128, x, y, z float64, nNonzero int) {
	// 1. Normalize to SU(4).
	det := det4x4(m)
	detPhase := cmplx.Phase(det) / 4
	su4 := MatScale(m, cmplx.Exp(complex(0, -detPhase)))

	// 2. Transform to magic basis: Up = Q† · U · Q.
	up := MatMul(magicQAdj, MatMul(su4, magicQ, 4), 4)

	// 3. Compute M2 = Up^T · Up (transpose, NOT conjugate transpose).
	upT := transposeMatrix(up, 4)
	m2 := MatMul(upT, up, 4)

	// 4. Diagonalize M2.
	p := diagonalizeSymmetricUnitary(m2)

	// 5. Compute D = P^T · M2 · P (diagonal).
	pC := realToComplex(p)
	pTC := realToComplex(transposeReal4(p))
	d := MatMul(pTC, MatMul(m2, pC, 4), 4)

	// 6. Try all 16 sign combinations for D^{1/2}.
	var m2Phases [4]float64
	for k := range 4 {
		m2Phases[k] = cmplx.Phase(d[k*4+k])
	}

	bestMask := 0
	bestNonzero := 4
	bestErr := math.Inf(1)
	for mask := range 16 {
		var dHalf [4]complex128
		for k := range 4 {
			hp := m2Phases[k] / 2
			if mask&(1<<k) != 0 {
				hp += math.Pi
			}
			dHalf[k] = cmplx.Exp(complex(0, hp))
		}
		dHalfInv := make([]complex128, 16)
		for k := range 4 {
			dHalfInv[k*4+k] = cmplx.Conj(dHalf[k])
		}
		k1mb := MatMul(up, MatMul(pC, dHalfInv, 4), 4)
		k1 := MatMul(magicQ, MatMul(k1mb, magicQAdj, 4), 4)
		al, ar := factorKronecker(k1)
		prod := Tensor(al, 2, ar, 2)
		if _, ok := GlobalPhase(prod, k1, 0.5); !ok {
			continue
		}
		e := kronError(k1)
		if e > 0.1 {
			continue
		}
		dHalfMat := make([]complex128, 16)
		for k := range 4 {
			dHalfMat[k*4+k] = dHalf[k]
		}
		uc := MatMul(magicQ, MatMul(dHalfMat, magicQAdj, 4), 4)
		u0 := uc[0*4+0] + uc[0*4+3]
		u1 := uc[1*4+1] + uc[1*4+2]
		u3 := uc[0*4+0] - uc[0*4+3]
		mx := (cmplx.Phase(u0) + cmplx.Phase(u1)) / 2
		my := (cmplx.Phase(u1) + cmplx.Phase(u3)) / 2
		mz := (cmplx.Phase(u0) + cmplx.Phase(u3)) / 2
		nz := 0
		if math.Abs(mx) > 1e-8 {
			nz++
		}
		if math.Abs(my) > 1e-8 {
			nz++
		}
		if math.Abs(mz) > 1e-8 {
			nz++
		}
		if nz < bestNonzero || (nz == bestNonzero && e < bestErr) {
			bestErr = e
			bestMask = mask
			bestNonzero = nz
		}
	}

	// 7. Use the best mask to compute the decomposition.
	var dHalf [4]complex128
	for k := range 4 {
		hp := m2Phases[k] / 2
		if bestMask&(1<<k) != 0 {
			hp += math.Pi
		}
		dHalf[k] = cmplx.Exp(complex(0, hp))
	}

	dHalfInv := make([]complex128, 16)
	dHalfMat := make([]complex128, 16)
	for k := range 4 {
		dHalfInv[k*4+k] = cmplx.Conj(dHalf[k])
		dHalfMat[k*4+k] = dHalf[k]
	}

	k1mb := MatMul(up, MatMul(pC, dHalfInv, 4), 4)
	k2mb := pTC

	k1Full := MatMul(magicQ, MatMul(k1mb, magicQAdj, 4), 4)
	k2Full := MatMul(magicQ, MatMul(k2mb, magicQAdj, 4), 4)

	k1l, k1r = factorKronecker(k1Full)
	k2l, k2r = factorKronecker(k2Full)

	// Extract Weyl parameters.
	udComp := MatMul(magicQ, MatMul(dHalfMat, magicQAdj, 4), 4)
	ud0 := udComp[0*4+0] + udComp[0*4+3]
	ud1 := udComp[1*4+1] + udComp[1*4+2]
	ud3 := udComp[0*4+0] - udComp[0*4+3]
	phi0 := cmplx.Phase(ud0)
	phi1 := cmplx.Phase(ud1)
	phi3 := cmplx.Phase(ud3)
	x = (phi0 + phi1) / 2
	y = (phi1 + phi3) / 2
	z = (phi0 + phi3) / 2

	const tol = 1e-8
	nNonzero = 0
	if math.Abs(x) > tol {
		nNonzero++
	}
	if math.Abs(y) > tol {
		nNonzero++
	}
	if math.Abs(z) > tol {
		nNonzero++
	}
	return
}

// isCNOTEquiv checks if Weyl parameters correspond to a CNOT-class gate
// (exactly one parameter = pi/4).
func isCNOTEquiv(x, y, z float64) bool {
	const tol = 1e-6
	check := func(v float64) bool {
		return math.Abs(math.Abs(v)-math.Pi/4) < tol
	}
	ax, ay, az := math.Abs(x) > 1e-8, math.Abs(y) > 1e-8, math.Abs(z) > 1e-8
	if ax && !ay && !az {
		return check(x)
	}
	if !ax && ay && !az {
		return check(y)
	}
	if !ax && !ay && az {
		return check(z)
	}
	return false
}

// Pre-computed 2x2 matrices for the 3-CNOT decomposition template.
// Combines analytical K-matrices from Vatan-Williams (quant-ph/0308006)
// with CNOT's own KAK K-matrices, computed once in init().
var (
	_u0l, _u0r                             []complex128
	_u1l                                   []complex128
	_u1ra, _u1rb                           []complex128
	_u2la, _u2lb                           []complex128
	_u2ra, _u2rb                           []complex128
	_u3l, _u3r                             []complex128
	_bK1lAdj, _bK1rAdj, _bK2lAdj, _bK2rAdj []complex128
)

// init pre-computes the magic basis matrices and the Vatan-Williams
// K-matrices used in the 3-CNOT decomposition template.
//
// The mapping between variable names and the Vatan-Williams paper
// (arXiv:quant-ph/0308006) is as follows:
//
//	K11l, K11r  -- K-matrices for the first CNOT decomposition (gate 1, left/right)
//	K12l, K12r  -- K-matrices for the first CNOT decomposition (gate 2, left/right)
//	K21r        -- K-matrix for the second CNOT decomposition (gate 1, right)
//	K22l, K22r  -- K-matrices for the second CNOT decomposition (gate 2, left/right)
//	K31l, K31r  -- K-matrices for the third CNOT decomposition (gate 1, left/right)
//	K32lK21l    -- Combined K32l · K21l product
//	K32r        -- K-matrix for the third CNOT decomposition (gate 2, right)
//
// These analytical K-matrices, combined with the CNOT's own KAK K-matrices
// (bK1l, bK1r, bK2l, bK2r), produce the 11 pre-computed u-matrices (_u0l
// through _u3r) that form the 3-CNOT circuit template. Each Weyl parameter
// (a, b, c) appears as exactly one Rz rotation sandwiched between fixed
// u-matrices, enabling independent control of all three non-local coordinates.
func init() {
	inv := complex(1.0/math.Sqrt2, 0)
	magicQ = []complex128{
		inv, 0, 0, inv * 1i,
		0, inv * 1i, inv, 0,
		0, inv * 1i, -inv, 0,
		inv, 0, 0, -inv * 1i,
	}
	magicQAdj = MatAdj(magicQ, 4)

	// Decompose CNOT to get basis K-matrices (used by oneCNOTCircuit
	// and as building blocks for the 3-CNOT u-matrices).
	bK1l, bK1r, bK2l, bK2r, _, _, _, _ := KakParams(gate.CNOT.Matrix())
	_bK1lAdj = matAdj2(bK1l)
	_bK1rAdj = matAdj2(bK1r)
	_bK2lAdj = matAdj2(bK2l)
	_bK2rAdj = matAdj2(bK2r)

	// Analytical K-matrices from Vatan-Williams (quant-ph/0308006),
	// specialized for CNOT as basis gate (b=0).
	// These describe three different equivalent decompositions of the
	// basis gate, providing enough degrees of freedom to parametrize
	// all three Weyl coordinates independently in a 3-CNOT circuit.
	invP := complex(0.5, -0.5) // 1/(1+i) = (1-i)/2
	invM := complex(0.5, 0.5)  // 1/(1-i) = (1+i)/2

	K11l := []complex128{invP * (-1i), invP, invP * (-1i), -invP}
	K11r := []complex128{inv * 1i, -inv, inv, inv * (-1i)}
	K12l := []complex128{invP * 1i, invP * 1i, -invP, invP}
	K12r := []complex128{inv * 1i, inv, -inv, inv * (-1i)}
	K32lK21l := []complex128{inv * complex(1, 1), 0, 0, inv * complex(1, -1)}
	K21r := []complex128{invM * (-1i), invM, invM * 1i, invM}
	K22l := []complex128{inv, -inv, inv, inv}
	K22r := []complex128{0, 1, -1, 0}
	K31l := []complex128{inv, inv, -inv, inv}
	K31r := []complex128{1i, 0, 0, -1i}
	K32r := []complex128{invM, -invM, invM * (-1i), invM * (-1i)}

	// Pre-compute the 11 u-matrices for the 3-CNOT template:
	//   q0: [U3l]--*--[U2l]--*--[U1l]--*--[U0l]
	//              |         |         |
	//   q1: [U3r]--X--[U2r]--X--[U1r]--X--[U0r]
	_u0l = matMul2(K31l, _bK1lAdj)
	_u0r = matMul2(K31r, _bK1rAdj)
	_u1l = matMul2(_bK2lAdj, matMul2(K32lK21l, _bK1lAdj))
	_u1ra = matMul2(_bK2rAdj, K32r)
	_u1rb = matMul2(K21r, _bK1rAdj)
	_u2la = matMul2(_bK2lAdj, K22l)
	_u2lb = matMul2(K11l, _bK1lAdj)
	_u2ra = matMul2(_bK2rAdj, K22r)
	_u2rb = matMul2(K11r, _bK1rAdj)
	_u3l = matMul2(_bK2lAdj, K12l)
	_u3r = matMul2(_bK2rAdj, K12r)
}

// weylUnitary computes exp(i*(x·XX + y·YY + z·ZZ)) as a 4x4 matrix.
func weylUnitary(x, y, z float64) []complex128 {
	// Diagonal in magic basis: d0=e^{i(x-y+z)}, d1=e^{i(x+y-z)},
	// d2=e^{i(-x-y-z)}, d3=e^{i(-x+y+z)}.
	dHalfMat := make([]complex128, 16)
	dHalfMat[0] = cmplx.Exp(complex(0, x-y+z))
	dHalfMat[5] = cmplx.Exp(complex(0, x+y-z))
	dHalfMat[10] = cmplx.Exp(complex(0, -x-y-z))
	dHalfMat[15] = cmplx.Exp(complex(0, -x+y+z))
	return MatMul(magicQ, MatMul(dHalfMat, magicQAdj, 4), 4)
}

// kronError measures how far a 4x4 matrix is from a tensor product A⊗B.
func kronError(m []complex128) float64 {
	a, b := factorKronecker(m)
	prod := Tensor(a, 2, b, 2)
	ph, ok := GlobalPhase(prod, m, 1.0)
	if !ok {
		return 10.0
	}
	factor := cmplx.Exp(complex(0, ph))
	e := 0.0
	for i := range m {
		e += cmplx.Abs(prod[i] - factor*m[i])
	}
	return e
}

// zzCircuit: exp(i·c·ZZ) = CX · (I⊗Rz(-2c)) · CX. Uses 2 CNOTs.
func zzCircuit(c float64, q0, q1 int) []ir.Operation {
	return []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		{Gate: gate.RZ(-2 * c), Qubits: []int{q1}},
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
	}
}

// xxCircuit: exp(i·a·XX) = (H⊗H)·CX·(I⊗Rz(-2a))·CX·(H⊗H). Uses 2 CNOTs.
func xxCircuit(a float64, q0, q1 int) []ir.Operation {
	return []ir.Operation{
		{Gate: gate.H, Qubits: []int{q0}},
		{Gate: gate.H, Qubits: []int{q1}},
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		{Gate: gate.RZ(-2 * a), Qubits: []int{q1}},
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		{Gate: gate.H, Qubits: []int{q0}},
		{Gate: gate.H, Qubits: []int{q1}},
	}
}

// yyCircuit: exp(i·b·YY) = (Rx(-π/2)⊗Rx(-π/2))·CX·(I⊗Rz(-2b))·CX·(Rx(π/2)⊗Rx(π/2)).
func yyCircuit(b float64, q0, q1 int) []ir.Operation {
	return []ir.Operation{
		{Gate: gate.RX(math.Pi / 2), Qubits: []int{q0}},
		{Gate: gate.RX(math.Pi / 2), Qubits: []int{q1}},
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		{Gate: gate.RZ(-2 * b), Qubits: []int{q1}},
		{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		{Gate: gate.RX(-math.Pi / 2), Qubits: []int{q0}},
		{Gate: gate.RX(-math.Pi / 2), Qubits: []int{q1}},
	}
}

// OpsToUnitary4 computes the 4x4 unitary from operations on q0, q1.
func OpsToUnitary4(ops []ir.Operation, q0, q1 int) []complex128 {
	if len(ops) == 0 {
		return Eye(4)
	}
	result := Eye(4)
	for _, op := range ops {
		var opMat []complex128
		if op.Gate.Qubits() == 1 {
			gMat := op.Gate.Matrix()
			if op.Qubits[0] == q0 {
				opMat = Tensor(gMat, 2, Eye(2), 2)
			} else {
				opMat = Tensor(Eye(2), 2, gMat, 2)
			}
		} else {
			if op.Qubits[0] == q0 && op.Qubits[1] == q1 {
				opMat = op.Gate.Matrix()
			} else {
				sw := gate.SWAP.Matrix()
				opMat = MatMul(sw, MatMul(op.Gate.Matrix(), sw, 4), 4)
			}
		}
		result = MatMul(opMat, result, 4)
	}
	return result
}

// factorKronecker factors a 4x4 unitary (approximately A⊗B) into 2x2 matrices.
func factorKronecker(m []complex128) (a, b []complex128) {
	bestR, bestC := 0, 0
	bestAbs := 0.0
	for r := range 4 {
		for c := range 4 {
			if cmplx.Abs(m[r*4+c]) > bestAbs {
				bestAbs = cmplx.Abs(m[r*4+c])
				bestR, bestC = r, c
			}
		}
	}
	if bestAbs < 1e-15 {
		return Eye(2), Eye(2)
	}

	ar, br := bestR/2, bestR%2
	ac, bc := bestC/2, bestC%2
	pivot := m[bestR*4+bestC]

	b = make([]complex128, 4)
	for r := range 2 {
		for c := range 2 {
			b[r*2+c] = m[(ar*2+r)*4+(ac*2+c)] / pivot
		}
	}

	bPivot := b[br*2+bc]
	if cmplx.Abs(bPivot) < 1e-15 {
		bPivot = 1
	}
	a = make([]complex128, 4)
	for r := range 2 {
		for c := range 2 {
			a[r*2+c] = m[(r*2+br)*4+(c*2+bc)] / bPivot
		}
	}

	a = ToSU2(a)
	b = ToSU2(b)
	return
}

// eulerFromMatrix decomposes a 2×2 unitary matrix into RZ·RY·RZ operations.
func eulerFromMatrix(m []complex128, q int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerZYZ(m)
	var ops []ir.Operation
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(mathutil.NormalizeAngle(gamma)), Qubits: []int{q}})
	}
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(mathutil.NormalizeAngle(beta)), Qubits: []int{q}})
	}
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(mathutil.NormalizeAngle(alpha)), Qubits: []int{q}})
	}
	return ops
}

// diagonalizeSymmetricUnitary finds a real orthogonal matrix P such that
// P^T · M · P is diagonal, where M is a symmetric unitary matrix (M^T = M).
//
// A symmetric unitary has the property that its eigenvectors can be chosen
// to be real (forming an orthogonal matrix), but finding them requires care.
// The standard eigendecomposition of a complex matrix does not guarantee real
// eigenvectors. Instead, this function tries multiple linear combinations of
// the real and imaginary parts of M (e.g., a·Re(M) + b·Im(M)), each of which
// is a real symmetric matrix and thus has a real orthogonal eigendecomposition.
// Since Re(M) and Im(M) commute (they share eigenvectors), the correct linear
// combination will simultaneously diagonalize both, and hence diagonalize M
// itself. Multiple coefficients are tried because numerical conditioning varies.
func diagonalizeSymmetricUnitary(m []complex128) []float64 {
	re := make([]float64, 16)
	im := make([]float64, 16)
	for i := range 16 {
		re[i] = real(m[i])
		im[i] = imag(m[i])
	}

	coeffs := [][2]float64{
		{1, 0}, {0, 1}, {1, 1}, {1, -1},
		{2, 1}, {1, 2}, {3, 1}, {1, 3},
		{0.7, 0.3}, {0.3, 0.7},
	}

	bestP := eyeReal4()
	bestOffDiag := math.Inf(1)

	for _, c := range coeffs {
		combo := make([]float64, 16)
		for i := range 16 {
			combo[i] = c[0]*re[i] + c[1]*im[i]
		}
		for i := range 4 {
			for j := i + 1; j < 4; j++ {
				avg := (combo[i*4+j] + combo[j*4+i]) / 2
				combo[i*4+j] = avg
				combo[j*4+i] = avg
			}
		}

		p, _ := jacobi4(combo)

		pC := realToComplex(p)
		pTC := realToComplex(transposeReal4(p))
		d := MatMul(pTC, MatMul(m, pC, 4), 4)
		offDiag := 0.0
		for i := range 4 {
			for j := range 4 {
				if i != j {
					offDiag += cmplx.Abs(d[i*4+j])
				}
			}
		}

		if offDiag < bestOffDiag {
			bestOffDiag = offDiag
			bestP = make([]float64, 16)
			copy(bestP, p)
		}
		if bestOffDiag < 1e-10 {
			break
		}
	}

	if detReal4(bestP) < 0 {
		for i := range 4 {
			bestP[i*4] = -bestP[i*4]
		}
	}

	return bestP
}

func eyeReal4() []float64 {
	m := make([]float64, 16)
	for i := range 4 {
		m[i*4+i] = 1
	}
	return m
}

// Real matrix helpers

func transposeReal4(m []float64) []float64 {
	t := make([]float64, 16)
	for i := range 4 {
		for j := range 4 {
			t[i*4+j] = m[j*4+i]
		}
	}
	return t
}

func realToComplex(m []float64) []complex128 {
	c := make([]complex128, len(m))
	for i, v := range m {
		c[i] = complex(v, 0)
	}
	return c
}

func detReal4(m []float64) float64 {
	c := make([]complex128, 16)
	for i, v := range m {
		c[i] = complex(v, 0)
	}
	return real(det4x4(c))
}

// jacobi4 computes the eigendecomposition of a 4x4 real symmetric matrix
// using the classical Jacobi eigenvalue algorithm. It iteratively applies
// Givens rotations to zero out the largest off-diagonal element until the
// matrix is diagonal (within tolerance). Returns the orthogonal eigenvector
// matrix V and the eigenvalues sorted in descending order.
func jacobi4(m []float64) ([]float64, [4]float64) {
	a := make([]float64, 16)
	copy(a, m)
	v := make([]float64, 16)
	for i := range 4 {
		v[i*4+i] = 1
	}

	for range 200 {
		maxVal := 0.0
		p, q := 0, 1
		for i := range 4 {
			for j := i + 1; j < 4; j++ {
				if math.Abs(a[i*4+j]) > maxVal {
					maxVal = math.Abs(a[i*4+j])
					p, q = i, j
				}
			}
		}
		if maxVal < 1e-15 {
			break
		}

		app, aqq, apq := a[p*4+p], a[q*4+q], a[p*4+q]
		var c, s float64
		if math.Abs(app-aqq) < 1e-30 {
			c = math.Sqrt2 / 2
			s = math.Sqrt2 / 2
		} else {
			tau := (aqq - app) / (2 * apq)
			var t float64
			if tau >= 0 {
				t = 1.0 / (tau + math.Sqrt(1+tau*tau))
			} else {
				t = -1.0 / (-tau + math.Sqrt(1+tau*tau))
			}
			c = 1.0 / math.Sqrt(1+t*t)
			s = t * c
		}

		for i := range 4 {
			if i == p || i == q {
				continue
			}
			aip, aiq := a[i*4+p], a[i*4+q]
			a[i*4+p] = c*aip - s*aiq
			a[p*4+i] = a[i*4+p]
			a[i*4+q] = s*aip + c*aiq
			a[q*4+i] = a[i*4+q]
		}
		a[p*4+p] = c*c*app - 2*s*c*apq + s*s*aqq
		a[q*4+q] = s*s*app + 2*s*c*apq + c*c*aqq
		a[p*4+q] = 0
		a[q*4+p] = 0

		for i := range 4 {
			vip, viq := v[i*4+p], v[i*4+q]
			v[i*4+p] = c*vip - s*viq
			v[i*4+q] = s*vip + c*viq
		}
	}

	var eigvals [4]float64
	for i := range 4 {
		eigvals[i] = a[i*4+i]
	}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 4; j++ {
			if eigvals[j] > eigvals[i] {
				eigvals[i], eigvals[j] = eigvals[j], eigvals[i]
				for k := range 4 {
					v[k*4+i], v[k*4+j] = v[k*4+j], v[k*4+i]
				}
			}
		}
	}
	return v, eigvals
}

// Magic basis change matrix Q and its adjoint.
var (
	magicQ    []complex128
	magicQAdj []complex128
)

func isGlobalPhaseOf(a, b []complex128, tol float64) bool {
	_, ok := GlobalPhase(a, b, tol)
	return ok
}

func transposeMatrix(m []complex128, n int) []complex128 {
	t := make([]complex128, n*n)
	for r := range n {
		for c := range n {
			t[r*n+c] = m[c*n+r]
		}
	}
	return t
}

func det4x4(m []complex128) complex128 {
	var det complex128
	for j := range 4 {
		minor := minor4x4(m, 0, j)
		sign := complex(1, 0)
		if j%2 == 1 {
			sign = -1
		}
		det += sign * m[j] * det3x3(minor)
	}
	return det
}

func minor4x4(m []complex128, r, c int) []complex128 {
	minor := make([]complex128, 0, 9)
	for i := range 4 {
		if i == r {
			continue
		}
		for j := range 4 {
			if j == c {
				continue
			}
			minor = append(minor, m[i*4+j])
		}
	}
	return minor
}

func det3x3(m []complex128) complex128 {
	return m[0]*(m[4]*m[8]-m[5]*m[7]) -
		m[1]*(m[3]*m[8]-m[5]*m[6]) +
		m[2]*(m[3]*m[7]-m[4]*m[6])
}

// eulerFromMatrixForBasis decomposes a 2×2 unitary using the specified Euler convention.
func eulerFromMatrixForBasis(m []complex128, q int, basis EulerBasis) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	switch basis {
	case BasisZSX:
		return eulerZSX(m, q)
	case BasisZXZ:
		return eulerZXZ(m, q)
	default:
		return eulerFromMatrix(m, q)
	}
}

// tryLocalDecomposeForBasis checks if m ≈ A⊗B and decomposes with the given basis.
func tryLocalDecomposeForBasis(m []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	a, b := factorKronecker(m)
	prod := Tensor(a, 2, b, 2)
	if isGlobalPhaseOf(prod, m, 1e-9) {
		ops := make([]ir.Operation, 0, 6)
		ops = append(ops, eulerFromMatrixForBasis(a, q0, basis)...)
		ops = append(ops, eulerFromMatrixForBasis(b, q1, basis)...)
		if len(ops) == 0 {
			return nil
		}
		return ops
	}
	return nil
}

// kakGeneralForBasis implements the full KAK decomposition using the specified Euler convention.
func kakGeneralForBasis(m []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	k1l, k1r, k2l, k2r, x, y, z, nNonzero := KakParams(m)

	if nNonzero == 0 {
		ops := make([]ir.Operation, 0, 6)
		k := matMul2(k1l, k2l)
		ops = append(ops, eulerFromMatrixForBasis(k, q0, basis)...)
		k = matMul2(k1r, k2r)
		ops = append(ops, eulerFromMatrixForBasis(k, q1, basis)...)
		return ops
	}
	if nNonzero == 1 && isCNOTEquiv(x, y, z) {
		return oneCNOTCircuitForBasis(k1l, k1r, k2l, k2r, q0, q1, basis)
	}
	if nNonzero == 1 {
		return twoCNOTCircuitForBasis(x, y, z, k1l, k1r, k2l, k2r, q0, q1, basis)
	}
	return threeCNOTCircuitForBasis(x, y, z, k1l, k1r, k2l, k2r, q0, q1, basis)
}

func oneCNOTCircuitForBasis(k1l, k1r, k2l, k2r []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	Al := matMul2(k1l, _bK1lAdj)
	Ar := matMul2(k1r, _bK1rAdj)
	Bl := matMul2(_bK2lAdj, k2l)
	Br := matMul2(_bK2rAdj, k2r)

	ops := make([]ir.Operation, 0, 13)
	ops = append(ops, eulerFromMatrixForBasis(Bl, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(Br, q1, basis)...)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	ops = append(ops, eulerFromMatrixForBasis(Al, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(Ar, q1, basis)...)
	return ops
}

func twoCNOTCircuitForBasis(x, y, z float64, k1l, k1r, k2l, k2r []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	const tol = 1e-8
	xz := math.Abs(x) > tol
	yz := math.Abs(y) > tol
	zz := math.Abs(z) > tol

	var udOps []ir.Operation
	switch {
	case zz:
		udOps = zzCircuit(z, q0, q1)
	case xz:
		udOps = xxCircuit(x, q0, q1)
	case yz:
		udOps = yyCircuit(y, q0, q1)
	}

	udMat := OpsToUnitary4(udOps, q0, q1)
	udTarget := weylUnitary(x, y, z)
	correction := MatMul(udTarget, MatAdj(udMat, 4), 4)
	k1Full := Tensor(k1l, 2, k1r, 2)
	afterMat := MatMul(k1Full, correction, 4)
	al, ar := factorKronecker(afterMat)

	ops := make([]ir.Operation, 0, 4*3+len(udOps))
	ops = append(ops, eulerFromMatrixForBasis(k2l, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(k2r, q1, basis)...)
	ops = append(ops, udOps...)
	ops = append(ops, eulerFromMatrixForBasis(al, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(ar, q1, basis)...)
	return ops
}

func threeCNOTCircuitForBasis(a, b, c float64, k1l, k1r, k2l, k2r []complex128, q0, q1 int, basis EulerBasis) []ir.Operation {
	U0l := matMul2(k1l, _u0l)
	U0r := matMul2(k1r, _u0r)
	U1l := _u1l
	U1r := matMul2(_u1ra, matMul2(rzMat(-2*c), _u1rb))
	U2l := matMul2(_u2la, matMul2(rzMat(-2*a), _u2lb))
	U2r := matMul2(_u2ra, matMul2(rzMat(2*b), _u2rb))
	U3l := matMul2(_u3l, k2l)
	U3r := matMul2(_u3r, k2r)

	ops := make([]ir.Operation, 0, 27)
	ops = append(ops, eulerFromMatrixForBasis(U3l, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(U3r, q1, basis)...)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	ops = append(ops, eulerFromMatrixForBasis(U2l, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(U2r, q1, basis)...)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	ops = append(ops, eulerFromMatrixForBasis(U1l, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(U1r, q1, basis)...)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	ops = append(ops, eulerFromMatrixForBasis(U0l, q0, basis)...)
	ops = append(ops, eulerFromMatrixForBasis(U0r, q1, basis)...)
	return ops
}
