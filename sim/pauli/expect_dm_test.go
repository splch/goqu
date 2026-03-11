package pauli

import (
	"math"
	"testing"
)

// pureStateDM constructs |psi><psi| as a flat density matrix.
func pureStateDM(psi []complex128) ([]complex128, int) {
	dim := len(psi)
	rho := make([]complex128, dim*dim)
	for i := range dim {
		for j := range dim {
			rho[i*dim+j] = psi[i] * complex(real(psi[j]), -imag(psi[j]))
		}
	}
	return rho, dim
}

// maxMixed1Q returns the maximally mixed 1-qubit state I/2.
func maxMixed1Q() ([]complex128, int) {
	return []complex128{0.5, 0, 0, 0.5}, 2
}

func TestExpectDM_PureStates(t *testing.T) {
	// Pure state density matrix expectations should match statevector expectations.
	tests := []struct {
		name  string
		state []complex128
		op    string
		want  float64
	}{
		{"|0> Z", ket0, "Z", 1},
		{"|1> Z", ket1, "Z", -1},
		{"|+> X", ketPlus, "X", 1},
		{"|-> X", ketMinus, "X", -1},
		{"|+i> Y", ketPlusI, "Y", 1},
		{"|-i> Y", ketMinusI, "Y", -1},
		{"|0> X", ket0, "X", 0},
		{"|+> Z", ketPlus, "Z", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rho, dim := pureStateDM(tt.state)
			ps, _ := Parse(tt.op)
			got := ExpectDM(rho, dim, ps)
			if math.Abs(real(got)-tt.want) > eps || math.Abs(imag(got)) > eps {
				t.Errorf("ExpectDM = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpectDM_Bell(t *testing.T) {
	bell := bellPhi()
	rho, dim := pureStateDM(bell)

	tests := []struct {
		op   string
		want float64
	}{
		{"ZZ", 1},
		{"XX", 1},
		{"YY", -1},
		{"II", 1},
		{"ZI", 0},
		{"IZ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			ps, _ := Parse(tt.op)
			got := ExpectDM(rho, dim, ps)
			if math.Abs(real(got)-tt.want) > eps || math.Abs(imag(got)) > eps {
				t.Errorf("ExpectDM<%s> = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}

func TestExpectDM_MaxMixed(t *testing.T) {
	rho, dim := maxMixed1Q()

	// Maximally mixed: <X> = <Y> = <Z> = 0, <I> = 1
	for _, op := range []string{"X", "Y", "Z"} {
		ps, _ := Parse(op)
		got := ExpectDM(rho, dim, ps)
		if math.Abs(real(got)) > eps || math.Abs(imag(got)) > eps {
			t.Errorf("mixed <%s> = %v, want 0", op, got)
		}
	}
	iOp, _ := Parse("I")
	got := ExpectDM(rho, dim, iOp)
	if math.Abs(real(got)-1) > eps {
		t.Errorf("mixed <I> = %v, want 1", got)
	}
}

func TestExpectDM_CrossValidate(t *testing.T) {
	// For pure states, ExpectDM should match Expect.
	states := []struct {
		name string
		psi  []complex128
	}{
		{"|0>", ket0},
		{"|1>", ket1},
		{"|+>", ketPlus},
		{"|->", ketMinus},
		{"|+i>", ketPlusI},
		{"|-i>", ketMinusI},
	}
	ops := []string{"X", "Y", "Z", "I"}

	for _, st := range states {
		for _, op := range ops {
			ps, _ := Parse(op)
			svResult := Expect(st.psi, ps)
			rho, dim := pureStateDM(st.psi)
			dmResult := ExpectDM(rho, dim, ps)

			if math.Abs(real(svResult)-real(dmResult)) > eps || math.Abs(imag(svResult)-imag(dmResult)) > eps {
				t.Errorf("%s <%s>: sv=%v dm=%v", st.name, op, svResult, dmResult)
			}
		}
	}
}

func TestExpectSumDM(t *testing.T) {
	ket00 := []complex128{1, 0, 0, 0}
	rho, dim := pureStateDM(ket00)

	zz := NewPauliString(0.5, map[int]Pauli{0: Z, 1: Z}, 2)
	xi := NewPauliString(0.3, map[int]Pauli{0: X}, 2)
	sum, _ := NewPauliSum([]PauliString{zz, xi})

	got := ExpectSumDM(rho, dim, sum)
	if math.Abs(real(got)-0.5) > eps {
		t.Errorf("ExpectSumDM = %v, want 0.5", got)
	}
}
