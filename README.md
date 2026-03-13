<div align="center">

<img width="256" src="https://github.com/user-attachments/assets/42cf36fb-33d3-43d9-9176-c28dd5909958" />

# Goqu

**Kamehame-Hadamard!**

[![Go Reference](https://pkg.go.dev/badge/github.com/splch/goqu.svg)](https://pkg.go.dev/github.com/splch/goqu)
![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

Goqu lets you build, simulate, transpile, and run quantum circuits in pure Go. No C bindings, no Python interop, no external dependencies in the core — just `go get` and go.

## Quick Start

```bash
go get github.com/splch/goqu@latest
```

Create a Bell state in five lines:

```go
c, _ := builder.New("bell", 2).
    H(0).
    CNOT(0, 1).
    MeasureAll().
    Build()

sim := statevector.New(2)
counts, _ := sim.Run(c, 1024)
fmt.Println(counts) // map[00:~512 11:~512]
```

## Features

| | |
|---|---|
| **28+ gates** | H, CNOT, Toffoli, RX/RY/RZ, U3, IonQ native gates, custom unitaries, and more |
| **7 simulators** | Statevector (up to 28 qubits), density matrix, Clifford, pulse-level, Pauli, noise channels, operator theory |
| **7 hardware backends** | IonQ, IBM Quantum, Google, AWS Braket, Rigetti, Quantinuum, and local |
| **Transpilation** | SABRE routing, gate decomposition (Euler, KAK, Barenco), 4 optimization levels, equivalence verification |
| **Algorithms** | Grover, Shor, VQE, QAOA, QPE, HHL, amplitude estimation, Trotterization, VQC, textbook algorithms |
| **Pulse programming** | OpenPulse model with 5 waveform types, gate-to-pulse calibration, OpenQASM 3.0 parsing |
| **Serialization** | OpenQASM 2.0 & 3.0 round-trip, Quil emission |
| **Visualization** | ASCII, SVG, and LaTeX circuit diagrams |
| **Observability** | OpenTelemetry and Prometheus bridges (optional separate modules) |
| **Job management** | Concurrent submission, polling, retry with circuit breaker |

## Run on Real Hardware

```go
// Transpile for IonQ's native gate set
compiled, _ := pipeline.Run(circuit, pipeline.OptLevel2, target.IonQ())

// Submit to IonQ
backend := ionq.New(apiKey)
result, _ := backend.Run(ctx, compiled, 1000)
```

## Interactive Notebooks

Four Jupyter notebooks (powered by [gonb](https://github.com/janpfeifer/gonb)) walk through the SDK:

1. **Getting Started** — circuits, visualization, simulation
2. **Simulation & Noise** — density matrix, noise channels, Pauli expectations
3. **Variational Circuits** — symbolic parameters, sweeps, QASM round-trip
4. **Transpilation** — hardware targeting, optimization levels, before/after comparison

## Development

```bash
make test       # Run tests
make test-race  # Race detector
make lint       # golangci-lint
make bench      # Statevector & density matrix benchmarks
make fuzz       # Fuzz parsers and transpiler passes
make coverage   # HTML coverage report
```

## License

[MIT](LICENSE) — Spencer Churchill
