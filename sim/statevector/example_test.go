package statevector_test

import (
	"fmt"
	"math"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/sim/statevector"
)

func ExampleSim_Evolve() {
	// Build a Bell circuit and inspect the statevector.
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}

	sim := statevector.New(2)
	if err := sim.Evolve(c); err != nil {
		panic(err)
	}

	// The Bell state has non-zero amplitudes only for |00> and |11>.
	sv := sim.StateVector()
	for i, amp := range sv {
		p := real(amp)*real(amp) + imag(amp)*imag(amp)
		if p > 1e-10 {
			fmt.Printf("|%02b|^2 = %.1f\n", i, math.Round(p*10)/10)
		}
	}
	// Output:
	// |00|^2 = 0.5
	// |11|^2 = 0.5
}

func ExampleSim_Run() {
	// Build a circuit that always produces |11>.
	c, err := builder.New("x2", 2).
		X(0).
		X(1).
		MeasureAll().
		Build()
	if err != nil {
		panic(err)
	}

	sim := statevector.New(2)
	counts, err := sim.Run(c, 100)
	if err != nil {
		panic(err)
	}
	fmt.Printf("11: %d shots\n", counts["11"])
	// Output: 11: 100 shots
}
