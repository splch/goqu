# Goqu

<img height="256" alt="image" src="https://github.com/user-attachments/assets/42cf36fb-33d3-43d9-9176-c28dd5909958" />

A zero-dependency quantum computing SDK for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/splch/goqu.svg)](https://pkg.go.dev/github.com/splch/goqu)
![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

- **Zero external dependencies** &mdash; stdlib-only core, single binary deployment
- **Statevector + density matrix simulators** with goroutine parallelism
- **Cloud backends** &mdash; IBM Quantum, IonQ, Quantinuum, Rigetti, Google Quantum, AWS Braket
- **Transpilation pipeline** &mdash; SABRE routing, KAK/Euler decomposition, 7 optimization passes
- **Production observability** &mdash; OpenTelemetry, Prometheus, and slog hooks

## Install

```
go get github.com/splch/goqu
```

Requires Go 1.24 or later.

## Quick Start

Build a Bell state, simulate it, and draw the circuit:

```go
package main

import (
	"fmt"
	"math"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/sim/statevector"
)

func main() {
	// Build a Bell circuit.
	c, _ := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()

	// Draw it.
	fmt.Println(draw.String(c))
	// q0: -H---@---M--
	//          |
	// q1: -----X---M--

	// Simulate 1000 shots.
	sim := statevector.New(2)
	counts, _ := sim.Run(c, 1000)
	for state, n := range counts {
		fmt.Printf("|%s>: %d\n", state, n)
	}

	// Inspect the statevector.
	sim2 := statevector.New(2)
	sim2.Evolve(c)
	for i, amp := range sim2.StateVector() {
		if p := real(amp)*real(amp) + imag(amp)*imag(amp); p > 1e-10 {
			fmt.Printf("|%02b> probability: %.1f\n", i, math.Round(p*10)/10)
		}
	}
	// |00> probability: 0.5
	// |11> probability: 0.5
}
```

## Examples

### Noisy Simulation

```go
c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()

nm := noise.New()
nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
nm.AddDefaultError(2, noise.Depolarizing2Q(0.01))

dm := densitymatrix.New(2)
dm.WithNoise(nm)
dm.Evolve(c)

fmt.Printf("Purity: %.4f\n", dm.Purity()) // < 1.0 for mixed state
```

### Transpile for Hardware

```go
c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()

out, _ := pipeline.Run(ctx, c, target.IBMBrisbane, pipeline.LevelFull)
fmt.Printf("depth=%d\n", out.Stats().Depth)
```

### Parameterized Circuits

```go
theta := param.New("theta")

c, _ := builder.New("vqe", 2).
	SymRX(theta.Expr(), 0).
	CNOT(0, 1).
	Build()

bound, _ := ir.Bind(c, map[string]float64{"theta": math.Pi / 4})
```

## Packages

| Package | Description |
|---|---|
| `circuit/gate` | 28 quantum gates + custom unitaries |
| `circuit/builder` | Fluent circuit construction API |
| `circuit/ir` | Immutable circuit representation |
| `circuit/draw` | ASCII circuit visualization |
| `circuit/param` | Symbolic parameters and expressions |
| `sim/statevector` | Statevector simulator (parallel at 17+ qubits) |
| `sim/densitymatrix` | Density matrix simulator with noise |
| `sim/noise` | 7 noise channels and noise models |
| `qasm/parser` | OpenQASM 2/3 parser |
| `qasm/emitter` | OpenQASM 3.0 code generation |
| `transpile/pass` | 7 optimization passes |
| `transpile/pipeline` | Pre-built pipelines (4 optimization levels) |
| `transpile/routing` | SABRE routing with 20 trials |
| `transpile/target` | 7 hardware targets (IBM, IonQ, Quantinuum) |
| `transpile/decompose` | KAK and Euler decomposition |
| `backend/local` | Local simulator backend |
| `backend/ionq` | IonQ v0.4 REST client (stdlib-only) |
| `backend/ibm` | IBM Qiskit Runtime V2 (stdlib-only) |
| `backend/braket` | AWS Braket (separate module) |
| `job/manager` | Concurrent job submission, polling, Watch channels |
| `job/retry` | Retry policies with circuit breaker |
| `observe` | Zero-dep hooks for instrumentation |
| `observe/otelbridge` | OpenTelemetry span bridge (separate module) |
| `observe/prombridge` | Prometheus metrics bridge (separate module) |

## Backends

| Backend | Transport | Dependencies | Features |
|---|---|---|---|
| Local | In-process | None | Statevector sim, result caching |
| IonQ | REST v0.4 | None | Native gate support, radians-to-turns |
| IBM Quantum | REST (Qiskit Runtime V2) | None | IAM auth, OpenQASM 3.0 submission |
| AWS Braket | AWS SDK | `aws-sdk-go-v2` | S3 results, multi-device ARN mapping |
| Mock | In-process | None | Configurable latency, error injection |

## Performance

| Benchmark | Result |
|---|---|
| 16-qubit GHZ state | ~2.8 ms |
| Parallel threshold (statevector) | 17+ qubits |
| Parallel threshold (density matrix) | 9+ qubits |
| Max qubits (statevector) | 28 |

Run benchmarks locally:

```
go test -bench=. ./sim/statevector/
```

## Documentation

[API Reference (pkg.go.dev)](https://pkg.go.dev/github.com/splch/goqu)

## License

[MIT](LICENSE)
