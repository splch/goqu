package builder_test

import (
	"fmt"

	"github.com/splch/qgo/circuit/builder"
)

func ExampleBuilder_bell() {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}
	stats := c.Stats()
	fmt.Printf("name=%s qubits=%d depth=%d gates=%d two_qubit=%d\n",
		c.Name(), c.NumQubits(), stats.Depth, stats.GateCount, stats.TwoQubitGates)
	// Output: name=bell qubits=2 depth=2 gates=2 two_qubit=1
}

func ExampleBuilder_ghz() {
	b := builder.New("ghz-4", 4).H(0)
	for i := 0; i < 3; i++ {
		b.CNOT(i, i+1)
	}
	c, err := b.Build()
	if err != nil {
		panic(err)
	}
	stats := c.Stats()
	fmt.Printf("qubits=%d depth=%d two_qubit=%d\n",
		c.NumQubits(), stats.Depth, stats.TwoQubitGates)
	// Output: qubits=4 depth=4 two_qubit=3
}
