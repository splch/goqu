package gate_test

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/gate"
)

func TestControlledSingletonReturns(t *testing.T) {
	tests := []struct {
		inner gate.Gate
		nCtrl int
		want  gate.Gate
	}{
		{gate.X, 1, gate.CNOT},
		{gate.Z, 1, gate.CZ},
		{gate.Y, 1, gate.CY},
		{gate.X, 2, gate.CCX},
	}
	for _, tt := range tests {
		got := gate.Controlled(tt.inner, tt.nCtrl)
		if got != tt.want {
			t.Errorf("Controlled(%s, %d) = %v, want %v", tt.inner.Name(), tt.nCtrl, got.Name(), tt.want.Name())
		}
	}
}

func TestControlledInterface(t *testing.T) {
	g := gate.Controlled(gate.H, 2)
	cg, ok := g.(gate.ControlledGate)
	if !ok {
		t.Fatal("Controlled(H, 2) does not implement ControlledGate")
	}
	if cg.Inner() != gate.H {
		t.Error("Inner() != H")
	}
	if cg.NumControls() != 2 {
		t.Errorf("NumControls() = %d, want 2", cg.NumControls())
	}
	if g.Qubits() != 3 {
		t.Errorf("Qubits() = %d, want 3", g.Qubits())
	}
	if g.Name() != "C2-H" {
		t.Errorf("Name() = %q, want %q", g.Name(), "C2-H")
	}
}

func TestControlledMatrixUnitarity(t *testing.T) {
	gates := []gate.Gate{
		gate.Controlled(gate.H, 1),
		gate.Controlled(gate.H, 2),
		gate.Controlled(gate.X, 3),
		gate.Controlled(gate.S, 2),
		gate.MCZ(3),
		gate.MCP(math.Pi/4, 2),
	}
	for _, g := range gates {
		m := g.Matrix()
		dim := 1 << g.Qubits()
		if len(m) != dim*dim {
			t.Errorf("%s: Matrix() length = %d, want %d", g.Name(), len(m), dim*dim)
			continue
		}
		// Check U†U = I.
		for i := range dim {
			for j := range dim {
				var sum complex128
				for k := range dim {
					sum += cmplx.Conj(m[k*dim+i]) * m[k*dim+j]
				}
				want := complex(0, 0)
				if i == j {
					want = 1
				}
				if cmplx.Abs(sum-want) > 1e-10 {
					t.Errorf("%s: U†U[%d][%d] = %v, want %v", g.Name(), i, j, sum, want)
				}
			}
		}
	}
}

func TestControlledMatrixCorrectness(t *testing.T) {
	// C1-H: identity on top-left 2x2, H on bottom-right 2x2.
	g := gate.Controlled(gate.H, 1)
	m := g.Matrix()
	s2 := 1.0 / math.Sqrt2

	// |00> -> |00>
	if cmplx.Abs(m[0]-1) > 1e-10 {
		t.Errorf("m[0,0] = %v, want 1", m[0])
	}
	// |01> -> |01>
	if cmplx.Abs(m[5]-1) > 1e-10 {
		t.Errorf("m[1,1] = %v, want 1", m[5])
	}
	// |10> -> s2*|10> + s2*|11>  (H applied)
	if cmplx.Abs(m[10]-complex(s2, 0)) > 1e-10 {
		t.Errorf("m[2,2] = %v, want %v", m[10], s2)
	}
	if cmplx.Abs(m[11]-complex(s2, 0)) > 1e-10 {
		t.Errorf("m[2,3] = %v, want %v", m[11], s2)
	}
}

func TestControlledInverse(t *testing.T) {
	g := gate.Controlled(gate.S, 2)
	inv := g.Inverse()
	if inv.Qubits() != g.Qubits() {
		t.Errorf("Inverse Qubits() = %d, want %d", inv.Qubits(), g.Qubits())
	}

	// g * inv should be identity.
	m := g.Matrix()
	mi := inv.Matrix()
	dim := 1 << g.Qubits()
	for i := range dim {
		for j := range dim {
			var sum complex128
			for k := range dim {
				sum += m[i*dim+k] * mi[k*dim+j]
			}
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			if cmplx.Abs(sum-want) > 1e-10 {
				t.Errorf("g*inv [%d][%d] = %v, want %v", i, j, sum, want)
			}
		}
	}
}

func TestMCXNaming(t *testing.T) {
	g := gate.MCX(3)
	if g.Name() != "C3-X" {
		t.Errorf("MCX(3).Name() = %q, want %q", g.Name(), "C3-X")
	}
	if g.Qubits() != 4 {
		t.Errorf("MCX(3).Qubits() = %d, want 4", g.Qubits())
	}
}

func TestMCZMatrix(t *testing.T) {
	// MCZ(2) = C2-Z: 8x8 identity except m[7,7] = -1.
	g := gate.MCZ(2)
	m := g.Matrix()
	for i := range 8 {
		for j := range 8 {
			want := complex(0, 0)
			if i == j {
				want = 1
				if i == 7 {
					want = -1
				}
			}
			if cmplx.Abs(m[i*8+j]-want) > 1e-10 {
				t.Errorf("MCZ(2) m[%d,%d] = %v, want %v", i, j, m[i*8+j], want)
			}
		}
	}
}

func TestMCPParams(t *testing.T) {
	g := gate.MCP(math.Pi/4, 2)
	params := g.Params()
	if len(params) != 1 || math.Abs(params[0]-math.Pi/4) > 1e-10 {
		t.Errorf("MCP(pi/4, 2).Params() = %v, want [pi/4]", params)
	}
}

func TestControlledPanicsLowControls(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Controlled(X, 0) did not panic")
		}
	}()
	gate.Controlled(gate.X, 0)
}

func TestControlledMatrixPanicsLargeGate(t *testing.T) {
	// 10 controls + 1 target = 11 qubits, should panic on Matrix().
	g := gate.Controlled(gate.X, 10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Matrix() on 11-qubit gate did not panic")
		}
	}()
	g.Matrix()
}

func TestControlled2QInner(t *testing.T) {
	// Controlled(SWAP, 1) = CSWAP: verify matrix matches.
	g := gate.Controlled(gate.SWAP, 1)
	m := g.Matrix()
	cswapM := gate.CSWAP.Matrix()
	for i, v := range m {
		if cmplx.Abs(v-cswapM[i]) > 1e-10 {
			t.Errorf("Controlled(SWAP,1) m[%d] = %v, want %v", i, v, cswapM[i])
		}
	}
}
