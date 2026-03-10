package parser_test

import (
	"fmt"

	"github.com/splch/qgo/qasm/parser"
)

func ExampleParseString() {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
bit[2] c;
h q[0];
cx q[0], q[1];
c = measure q;`

	c, err := parser.ParseString(src)
	if err != nil {
		panic(err)
	}
	stats := c.Stats()
	fmt.Printf("qubits=%d clbits=%d ops=%d depth=%d\n",
		c.NumQubits(), c.NumClbits(), stats.GateCount, stats.Depth)
	// Output: qubits=2 clbits=2 ops=4 depth=3
}
