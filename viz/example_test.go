package viz_test

import (
	"fmt"
	"math"
	"strings"

	"github.com/splch/goqu/viz"
)

func Example_histogram() {
	counts := map[string]int{"00": 512, "11": 488}
	svg := viz.Histogram(counts)
	fmt.Println(strings.Contains(svg, "<svg"))
	// Output:
	// true
}

func Example_histogramProb() {
	probs := map[string]float64{"00": 0.5, "11": 0.5}
	svg := viz.HistogramProb(probs)
	fmt.Println(strings.Contains(svg, "<svg"))
	// Output:
	// true
}

func Example_bloch() {
	// |+> state
	s := 1 / math.Sqrt(2)
	state := []complex128{complex(s, 0), complex(s, 0)}
	svg := viz.Bloch(state)
	fmt.Println(strings.Contains(svg, "<svg"))
	// Output:
	// true
}

func Example_stateCity() {
	// Single qubit |0><0| density matrix
	rho := []complex128{1, 0, 0, 0}
	svg := viz.StateCity(rho, 2)
	fmt.Println(strings.Contains(svg, "<svg"))
	// Output:
	// true
}
