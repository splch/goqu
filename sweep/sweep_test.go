package sweep

import (
	"math"
	"testing"
)

func TestLinspace_CountZero(t *testing.T) {
	l := Linspace{Key: "x", Start: 0, Stop: 1, Count: 0}
	if l.Len() != 0 {
		t.Errorf("Len() = %d, want 0", l.Len())
	}
	if r := l.Resolve(); r != nil {
		t.Errorf("Resolve() = %v, want nil", r)
	}
}

func TestLinspace_CountNegative(t *testing.T) {
	l := Linspace{Key: "x", Start: 0, Stop: 1, Count: -3}
	if l.Len() != 0 {
		t.Errorf("Len() = %d, want 0", l.Len())
	}
}

func TestLinspace_CountOne(t *testing.T) {
	l := Linspace{Key: "x", Start: 3.14, Stop: 6.28, Count: 1}
	if l.Len() != 1 {
		t.Errorf("Len() = %d, want 1", l.Len())
	}
	r := l.Resolve()
	if len(r) != 1 {
		t.Fatalf("len(Resolve()) = %d, want 1", len(r))
	}
	if r[0]["x"] != 3.14 {
		t.Errorf("value = %v, want 3.14", r[0]["x"])
	}
}

func TestLinspace_Five(t *testing.T) {
	l := Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 5}
	if l.Len() != 5 {
		t.Errorf("Len() = %d, want 5", l.Len())
	}
	if p := l.Params(); len(p) != 1 || p[0] != "theta" {
		t.Errorf("Params() = %v, want [theta]", p)
	}
	r := l.Resolve()
	if len(r) != 5 {
		t.Fatalf("len(Resolve()) = %d, want 5", len(r))
	}
	// Check endpoints.
	if math.Abs(r[0]["theta"]-0) > 1e-15 {
		t.Errorf("r[0] = %v, want 0", r[0]["theta"])
	}
	if math.Abs(r[4]["theta"]-math.Pi) > 1e-12 {
		t.Errorf("r[4] = %v, want pi", r[4]["theta"])
	}
	// Check middle value.
	want := math.Pi / 2
	if math.Abs(r[2]["theta"]-want) > 1e-12 {
		t.Errorf("r[2] = %v, want %v", r[2]["theta"], want)
	}
}

func TestPoints_Empty(t *testing.T) {
	p := NewPoints("x", nil)
	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
	if r := p.Resolve(); r != nil {
		t.Errorf("Resolve() = %v, want nil", r)
	}
}

func TestPoints_Multi(t *testing.T) {
	p := NewPoints("x", []float64{1.0, 2.0, 3.0})
	if p.Len() != 3 {
		t.Errorf("Len() = %d, want 3", p.Len())
	}
	if params := p.Params(); len(params) != 1 || params[0] != "x" {
		t.Errorf("Params() = %v, want [x]", params)
	}
	r := p.Resolve()
	for i, want := range []float64{1.0, 2.0, 3.0} {
		if r[i]["x"] != want {
			t.Errorf("r[%d] = %v, want %v", i, r[i]["x"], want)
		}
	}
}

func TestPoints_DefensiveCopy(t *testing.T) {
	vals := []float64{1.0, 2.0}
	p := NewPoints("x", vals)
	vals[0] = 999
	r := p.Resolve()
	if r[0]["x"] != 1.0 {
		t.Errorf("defensive copy failed: got %v, want 1.0", r[0]["x"])
	}
}

func TestSingle(t *testing.T) {
	s := Single(map[string]float64{"a": 1.0, "b": 2.0})
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}
	params := s.Params()
	if len(params) != 2 {
		t.Errorf("len(Params()) = %d, want 2", len(params))
	}
	r := s.Resolve()
	if len(r) != 1 {
		t.Fatalf("len(Resolve()) = %d, want 1", len(r))
	}
	if r[0]["a"] != 1.0 || r[0]["b"] != 2.0 {
		t.Errorf("Resolve() = %v, want {a:1 b:2}", r[0])
	}
}

func TestSingle_DefensiveCopy(t *testing.T) {
	m := map[string]float64{"a": 1.0}
	s := Single(m)
	m["a"] = 999
	r := s.Resolve()
	if r[0]["a"] != 1.0 {
		t.Errorf("defensive copy failed: got %v, want 1.0", r[0]["a"])
	}
}

func TestProduct_TwoByThree(t *testing.T) {
	a := Linspace{Key: "x", Start: 0, Stop: 1, Count: 2}
	b := Linspace{Key: "y", Start: 10, Stop: 20, Count: 3}
	p := Product(a, b)
	if p.Len() != 6 {
		t.Errorf("Len() = %d, want 6", p.Len())
	}
	r := p.Resolve()
	if len(r) != 6 {
		t.Fatalf("len(Resolve()) = %d, want 6", len(r))
	}
	// First point: x=0, y=10.
	if r[0]["x"] != 0 || r[0]["y"] != 10 {
		t.Errorf("r[0] = %v, want {x:0 y:10}", r[0])
	}
	// Last point: x=1, y=20.
	if r[5]["x"] != 1 || r[5]["y"] != 20 {
		t.Errorf("r[5] = %v, want {x:1 y:20}", r[5])
	}
}

func TestProduct_EmptyFactor(t *testing.T) {
	a := Linspace{Key: "x", Start: 0, Stop: 1, Count: 3}
	b := Linspace{Key: "y", Start: 0, Stop: 1, Count: 0}
	p := Product(a, b)
	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
}

func TestProduct_SingleFactor(t *testing.T) {
	a := NewPoints("x", []float64{1, 2, 3})
	p := Product(a)
	if p.Len() != 3 {
		t.Errorf("Len() = %d, want 3", p.Len())
	}
}

func TestProduct_ThreeFactors(t *testing.T) {
	a := Linspace{Key: "x", Start: 0, Stop: 1, Count: 2}
	b := Linspace{Key: "y", Start: 0, Stop: 1, Count: 2}
	c := Linspace{Key: "z", Start: 0, Stop: 1, Count: 2}
	p := Product(a, b, c)
	if p.Len() != 8 {
		t.Errorf("Len() = %d, want 8", p.Len())
	}
}

func TestProduct_OverlappingParams(t *testing.T) {
	// When params overlap, later sweep values overwrite earlier ones per point.
	a := NewPoints("x", []float64{1, 2})
	b := NewPoints("x", []float64{10, 20})
	p := Product(a, b)
	r := p.Resolve()
	// 2×2 = 4 points, last sweep value for "x" wins in each merged map.
	if len(r) != 4 {
		t.Fatalf("len(Resolve()) = %d, want 4", len(r))
	}
}

func TestProduct_Empty(t *testing.T) {
	p := Product()
	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
}

func TestZip_MatchingLengths(t *testing.T) {
	a := NewPoints("x", []float64{1, 2, 3})
	b := NewPoints("y", []float64{10, 20, 30})
	z, err := Zip(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if z.Len() != 3 {
		t.Errorf("Len() = %d, want 3", z.Len())
	}
	r := z.Resolve()
	if r[0]["x"] != 1 || r[0]["y"] != 10 {
		t.Errorf("r[0] = %v, want {x:1 y:10}", r[0])
	}
	if r[2]["x"] != 3 || r[2]["y"] != 30 {
		t.Errorf("r[2] = %v, want {x:3 y:30}", r[2])
	}
}

func TestZip_MismatchedLengths(t *testing.T) {
	a := NewPoints("x", []float64{1, 2})
	b := NewPoints("y", []float64{10, 20, 30})
	_, err := Zip(a, b)
	if err == nil {
		t.Fatal("expected error for mismatched lengths")
	}
}

func TestMustZip_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustZip with mismatched lengths")
		}
	}()
	a := NewPoints("x", []float64{1})
	b := NewPoints("y", []float64{1, 2})
	MustZip(a, b)
}

func TestZip_Empty(t *testing.T) {
	z, err := Zip()
	if err != nil {
		t.Fatal(err)
	}
	if z.Len() != 0 {
		t.Errorf("Len() = %d, want 0", z.Len())
	}
}

func TestZip_OverlappingParams(t *testing.T) {
	a := NewPoints("x", []float64{1, 2})
	b := NewPoints("x", []float64{10, 20})
	z, err := Zip(a, b)
	if err != nil {
		t.Fatal(err)
	}
	r := z.Resolve()
	// Later sweep overwrites earlier for same key.
	if r[0]["x"] != 10 {
		t.Errorf("r[0][x] = %v, want 10", r[0]["x"])
	}
}
