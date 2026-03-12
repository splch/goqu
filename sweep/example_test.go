package sweep_test

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/param"
	"github.com/splch/qgo/sweep"
)

func ExampleLinspace() {
	l := sweep.Linspace{Key: "theta", Start: 0, Stop: 1, Count: 3}
	fmt.Println("Len:", l.Len())
	for _, m := range l.Resolve() {
		fmt.Printf("theta=%.1f\n", m["theta"])
	}
	// Output:
	// Len: 3
	// theta=0.0
	// theta=0.5
	// theta=1.0
}

func ExampleProduct() {
	x := sweep.NewPoints("x", []float64{1, 2})
	y := sweep.NewPoints("y", []float64{10, 20})
	p := sweep.Product(x, y)
	fmt.Println("Len:", p.Len())
	for _, m := range p.Resolve() {
		fmt.Printf("x=%.0f y=%.0f\n", m["x"], m["y"])
	}
	// Output:
	// Len: 4
	// x=1 y=10
	// x=1 y=20
	// x=2 y=10
	// x=2 y=20
}

func ExampleRunSim() {
	theta := param.New("theta")
	c, err := builder.New("ry", 1).
		SymRY(theta.Expr(), 0).
		MeasureAll().
		Build()
	if err != nil {
		panic(err)
	}

	sw := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 3}
	results, err := sweep.RunSim(context.Background(), c, 1000, sw)
	if err != nil {
		panic(err)
	}

	for _, r := range results {
		ones := r.Counts["1"]
		fmt.Printf("theta=%.2f: P(1)≈%.1f\n", r.Bindings["theta"], float64(ones)/1000)
	}
	// Output:
	// theta=0.00: P(1)≈0.0
	// theta=1.57: P(1)≈0.5
	// theta=3.14: P(1)≈1.0
}
