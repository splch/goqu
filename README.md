# Goqu

[![Go Reference](https://pkg.go.dev/badge/github.com/splch/goqu.svg)](https://pkg.go.dev/github.com/splch/goqu)
[![CI](https://github.com/splch/goqu/actions/workflows/ci.yml/badge.svg)](https://github.com/splch/goqu/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/splch/goqu/branch/main/graph/badge.svg)](https://codecov.io/gh/splch/goqu)
[![Go Report Card](https://goreportcard.com/badge/github.com/splch/goqu)](https://goreportcard.com/report/github.com/splch/goqu)

A quantum computing SDK in pure Go — build, simulate, transpile, and run quantum circuits with zero external dependencies.

```go
package main

import (
    "fmt"

    "github.com/splch/goqu/circuit/builder"
    "github.com/splch/goqu/sim/statevector"
)

func main() {
    c, _ := builder.New("bell", 2).
        H(0).
        CNOT(0, 1).
        MeasureAll().
        Build()

    sim := statevector.New(2)
    counts, _ := sim.Run(c, 1024)
    fmt.Println(counts) // map[00:~512 11:~512]
}
```

## Install

```
go get github.com/splch/goqu@latest
```

Requires Go 1.24+.

## Features

| Area | Details |
|---|---|
| Circuit Construction | Fluent builder, 40+ gates, immutable IR, symbolic parameters, dynamic circuits |
| Simulation | Statevector (28 qubits, auto-parallel), density matrix, Clifford (1000s of qubits), CUDA, Metal |
| Algorithms | Grover, Shor, VQE, QAOA, QPE, HHL, amplitude estimation, Trotter, VQC, and more |
| Error Mitigation | ZNE, readout correction, Pauli twirling, dynamical decoupling, PEC, CDR, TREX |
| Hardware Backends | IBM, IonQ, Google, Amazon Braket, Quantinuum, Rigetti |
| Transpilation | SABRE routing, 4 optimization levels, decomposition, gate cancellation, verification |
| Interop | OpenQASM 2/3 parser + emitter, Quil emitter |
| Visualization | Text, SVG, LaTeX circuits; histograms; Bloch spheres |
| Pulse Programming | OpenPulse model, waveforms, defcal |
| Noise Modeling | Kraus operators, device noise models, depolarizing/amplitude-damping channels |
| Observability | Zero-dep hooks + OpenTelemetry and Prometheus bridges |
| Education | 20 Jupyter notebooks from qubits to Shor's algorithm (Go kernel via [gonb](https://github.com/janpfeifer/gonb)) |

```
q0: ─H───@──
         │
q1: ─────X──
```

## Documentation

- [API Reference](https://pkg.go.dev/github.com/splch/goqu)
- [Notebooks](notebooks/)
- [Benchmarks](https://splch.github.io/goqu/dev/bench/)

## Contributing

Contributions welcome — open an issue or submit a PR. Run `make test` before submitting.

## License

[MIT](LICENSE)
