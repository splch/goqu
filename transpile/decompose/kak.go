package decompose

import (
	"math"
	"math/cmplx"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// KAK decomposes an arbitrary 2-qubit unitary into at most 3 CNOTs + single-qubit rotations.
func KAK(m []complex128, q0, q1 int) []ir.Operation {
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
	if ops := tryLocalDecompose(m, q0, q1); ops != nil {
		return ops
	}
	return kakGeneral(m, q0, q1)
}

// tryLocalDecompose checks if m ≈ A⊗B up to global phase.
func tryLocalDecompose(m []complex128, q0, q1 int) []ir.Operation {
	a, b := factorKronecker(m)
	prod := Tensor(a, 2, b, 2)
	if isGlobalPhaseOf(prod, m, 1e-9) {
		var ops []ir.Operation
		ops = append(ops, eulerFromMatrix(a, q0)...)
		ops = append(ops, eulerFromMatrix(b, q1)...)
		if len(ops) == 0 {
			return nil
		}
		return ops
	}
	return nil
}

// kakGeneral implements the full KAK decomposition using M2 = Up^T · Up.
func kakGeneral(m []complex128, q0, q1 int) []ir.Operation {
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
	// D[k,k] = e^{2i·φ_k}, and D^{1/2}[k] = e^{i·φ_k} or -e^{i·φ_k}.
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
		// Build D^{-1/2} matrix.
		dHalfInv := make([]complex128, 16)
		for k := range 4 {
			dHalfInv[k*4+k] = cmplx.Conj(dHalf[k])
		}
		// K1_mb = Up · P · D^{-1/2}
		k1mb := MatMul(up, MatMul(pC, dHalfInv, 4), 4)
		// K1 = Q · K1_mb · Q†
		k1 := MatMul(magicQ, MatMul(k1mb, magicQAdj, 4), 4)
		// Check if K1 is a tensor product.
		al, ar := factorKronecker(k1)
		prod := Tensor(al, 2, ar, 2)
		if _, ok := GlobalPhase(prod, k1, 0.5); !ok {
			continue
		}
		e := kronError(k1)
		if e > 0.1 {
			continue
		}
		// Compute interaction parameters to count non-zero terms.
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
		// Prefer fewer non-zero interaction parameters (fewer CNOTs), then smaller kronError.
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

	// K2_mb = P^T, K1_mb = Up · P · D^{-1/2}
	k1mb := MatMul(up, MatMul(pC, dHalfInv, 4), 4)
	k2mb := pTC

	// Convert to computational basis.
	k1 := MatMul(magicQ, MatMul(k1mb, magicQAdj, 4), 4)
	k2 := MatMul(magicQ, MatMul(k2mb, magicQAdj, 4), 4)

	k1l, k1r := factorKronecker(k1)
	k2l, k2r := factorKronecker(k2)

	// 8. Compute Ud = Q · diag(D^{1/2}) · Q†.
	udComp := MatMul(magicQ, MatMul(dHalfMat, magicQAdj, 4), 4)

	// 9. Extract Weyl parameters directly from Ud_comp matrix structure.
	// Ud = exp(i*(x·XX + y·YY + z·ZZ)) has the form:
	//   [0,0] = (d0+d3)/2,  [0,3] = (d0-d3)/2
	//   [1,1] = (d1+d2)/2,  [1,2] = (d1-d2)/2
	// where d0=e^{i(x-y+z)}, d1=e^{i(x+y-z)}, d2=e^{i(-x-y-z)}, d3=e^{i(-x+y+z)}.
	ud0 := udComp[0*4+0] + udComp[0*4+3] // d0
	ud1 := udComp[1*4+1] + udComp[1*4+2] // d1
	ud3 := udComp[0*4+0] - udComp[0*4+3] // d3
	phi0 := cmplx.Phase(ud0) // x - y + z
	phi1 := cmplx.Phase(ud1) // x + y - z
	phi3 := cmplx.Phase(ud3) // -x + y + z
	x := (phi0 + phi1) / 2
	y := (phi1 + phi3) / 2
	z := (phi0 + phi3) / 2

	// 10. Build the CNOT circuit for Ud.
	udOps := udCircuit(x, y, z, q0, q1)
	udMat := opsToUnitary4(udOps, q0, q1)

	// 11. Compute correction: udComp may differ from udMat by numerical error.
	correction := MatMul(udComp, MatAdj(udMat, 4), 4)
	k1Full := Tensor(k1l, 2, k1r, 2)
	afterMat := MatMul(k1Full, correction, 4)
	al, ar := factorKronecker(afterMat)

	var ops []ir.Operation
	ops = append(ops, eulerFromMatrix(k2l, q0)...)
	ops = append(ops, eulerFromMatrix(k2r, q1)...)
	ops = append(ops, udOps...)
	ops = append(ops, eulerFromMatrix(al, q0)...)
	ops = append(ops, eulerFromMatrix(ar, q1)...)
	return ops
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

// udCircuit builds a CNOT circuit implementing exp(i*(x·XX + y·YY + z·ZZ)).
func udCircuit(x, y, z float64, q0, q1 int) []ir.Operation {
	const tol = 1e-8
	xz := math.Abs(x) > tol
	yz := math.Abs(y) > tol
	zz := math.Abs(z) > tol

	nNonzero := 0
	if xz {
		nNonzero++
	}
	if yz {
		nNonzero++
	}
	if zz {
		nNonzero++
	}

	if nNonzero == 0 {
		return nil
	}

	// Single-parameter cases: 2 CNOTs each.
	if nNonzero == 1 {
		if zz {
			return zzCircuit(z, q0, q1)
		}
		if xz {
			return xxCircuit(x, q0, q1)
		}
		return yyCircuit(y, q0, q1)
	}

	// Multi-parameter: concatenate individual circuits.
	// Since [XX, YY] = [YY, ZZ] = [XX, ZZ] = 0,
	// exp(i*(x·XX+y·YY+z·ZZ)) = exp(i·x·XX)·exp(i·y·YY)·exp(i·z·ZZ).
	var ops []ir.Operation
	if zz {
		ops = append(ops, zzCircuit(z, q0, q1)...)
	}
	if yz {
		ops = append(ops, yyCircuit(y, q0, q1)...)
	}
	if xz {
		ops = append(ops, xxCircuit(x, q0, q1)...)
	}
	return ops
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
// Uses Rx(π/2) basis change since Rx(-π/2)·Z·Rx(π/2) = Y.
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

// opsToUnitary4 computes the 4x4 unitary from operations on q0, q1.
func opsToUnitary4(ops []ir.Operation, q0, q1 int) []complex128 {
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
	if !nearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(normalizeAngle(gamma)), Qubits: []int{q}})
	}
	if !nearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(normalizeAngle(beta)), Qubits: []int{q}})
	}
	if !nearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(normalizeAngle(alpha)), Qubits: []int{q}})
	}
	return ops
}

// diagonalizeSymmetricUnitary finds orthogonal P such that P^T·M·P is diagonal.
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

// --- Real matrix helpers ---

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

// jacobi4 eigendecomposes a 4x4 real symmetric matrix.
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

func init() {
	inv := complex(1.0/math.Sqrt2, 0)
	magicQ = []complex128{
		inv, 0, 0, inv * 1i,
		0, inv * 1i, inv, 0,
		0, inv * 1i, -inv, 0,
		inv, 0, 0, -inv * 1i,
	}
	magicQAdj = MatAdj(magicQ, 4)
}

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
