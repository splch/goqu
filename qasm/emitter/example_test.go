package emitter_test

import (
	"fmt"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/qasm/emitter"
)

func ExampleEmitString() {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}
	qasm, err := emitter.EmitString(c)
	if err != nil {
		panic(err)
	}
	fmt.Print(qasm)
	// Output:
	// OPENQASM 3.0;
	// include "stdgates.inc";
	// qubit[2] q;
	//
	// h q[0];
	// cx q[0], q[1];
}
