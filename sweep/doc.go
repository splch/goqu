// Package sweep provides parameter sweep types for evaluating variational
// quantum circuits across ranges of parameter values.
//
// A [Sweep] resolves to a list of parameter binding maps. Primitive sweeps
// ([Linspace], [Points], [Single]) cover a single parameter or fixed bindings.
// Combinators ([Product], [Zip]) compose sweeps into higher-dimensional spaces.
//
// [RunSim] and [RunDensitySim] execute a parameterized circuit across all
// sweep points in parallel using statevector or density matrix simulation.
//
//	theta := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 10}
//	results, err := sweep.RunSim(ctx, circuit, 1024, theta)
package sweep
