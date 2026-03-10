package pipeline_test

import (
	"context"
	"fmt"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/transpile/pipeline"
	"github.com/splch/qgo/transpile/target"
)

func ExampleRun() {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}

	// Transpile for the simulator target (accepts all gates).
	out, err := pipeline.Run(context.Background(), c, target.Simulator, pipeline.LevelBasic)
	if err != nil {
		panic(err)
	}
	fmt.Printf("transpiled: qubits=%d depth=%d\n", out.NumQubits(), out.Stats().Depth)
}
