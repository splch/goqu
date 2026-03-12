package gate

import "fmt"

// Pow returns g raised to the integer power k.
// k=0 returns an identity gate, k=1 returns g unchanged,
// k>1 multiplies the matrix repeatedly, k<0 uses the inverse.
func Pow(g Gate, k int) Gate {
	if k == 1 {
		return g
	}
	n := g.Qubits()
	dim := 1 << n
	if k == 0 {
		m := make([]complex128, dim*dim)
		for i := range dim {
			m[i*dim+i] = 1
		}
		return &unitary{name: fmt.Sprintf("pow(0) @ %s", g.Name()), n: n, matrix: m}
	}
	base := g
	exp := k
	if k < 0 {
		base = g.Inverse()
		exp = -k
	}
	bm := base.Matrix()
	result := make([]complex128, len(bm))
	copy(result, bm)
	for i := 1; i < exp; i++ {
		result = matMulFlat(result, bm, dim)
	}
	return &unitary{
		name:   fmt.Sprintf("pow(%d) @ %s", k, g.Name()),
		n:      n,
		matrix: result,
	}
}

// matMulFlat multiplies two dim x dim flat row-major complex matrices.
func matMulFlat(a, b []complex128, dim int) []complex128 {
	result := make([]complex128, dim*dim)
	for i := range dim {
		for k := range dim {
			aik := a[i*dim+k]
			if aik == 0 {
				continue
			}
			for j := range dim {
				result[i*dim+j] += aik * b[k*dim+j]
			}
		}
	}
	return result
}
