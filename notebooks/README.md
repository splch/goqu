# Goqu Notebooks

A comprehensive quantum computing curriculum using [Goqu](https://github.com/splch/goqu) with [gonb](https://github.com/janpfeifer/gonb), a Go Jupyter kernel. The 16 notebooks progress from qubits to advanced algorithms, exercising every Goqu feature along the way.

## Setup

```bash
pip install jupyter
go install github.com/janpfeifer/gonb@latest && gonb --install
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/gopls@latest
```

## Running

```bash
cd notebooks
jupyter notebook
```

Open any `.ipynb` file and run cells sequentially.

## Curriculum

| # | Notebook | Topics |
|---|---|---|
| 01 | [The Qubit](01-the-qubit.ipynb) | Qubits, superposition, Bloch sphere, Born rule, statevector simulation |
| 02 | [Single-Qubit Gates](02-single-qubit-gates.ipynb) | Pauli gates, phase gates, rotations, gate matrices, inverses, universality |
| 03 | [Measurement](03-measurement.ipynb) | Computational basis measurement, collapse, partial measurement, no-cloning, mid-circuit measurement |
| 04 | [Multi-Qubit Gates](04-multi-qubit-gates.ipynb) | CNOT, CZ, Toffoli, Fredkin, controlled-U, multi-controlled gates, custom unitaries |
| 05 | [Entanglement](05-entanglement.ipynb) | Bell states, GHZ states, tensor products, density matrices, purity |
| 06 | [Teleportation](06-teleportation.ipynb) | Quantum teleportation, superdense coding, dynamic circuits, classical control flow |
| 07 | [Quantum Fourier Transform](07-quantum-fourier-transform.ipynb) | QFT circuit, inverse QFT, phase kickback, circuit composition |
| 08 | [Phase Estimation](08-phase-estimation.ipynb) | QPE, Deutsch-Jozsa, Bernstein-Vazirani, Simon's algorithm, state preparation |
| 09 | [Grover's Search](09-grovers-search.ipynb) | Amplitude amplification, phase/boolean oracles, optimal iterations, over-rotation |
| 10 | [Shor's Algorithm](10-shors-algorithm.ipynb) | Integer factoring, period finding, quantum counting, amplitude estimation |
| 11 | [Noise and Decoherence](11-noise-and-decoherence.ipynb) | Noise channels, density matrix simulation, T1/T2, custom channels, readout errors |
| 12 | [Transpilation](12-transpilation.ipynb) | Basis gates, hardware targets, Euler/KAK decomposition, SABRE routing, optimization levels |
| 13 | [Variational Algorithms](13-variational-algorithms.ipynb) | VQE, QAOA, ansatz templates, optimizers, gradients, parameter sweeps |
| 14 | [Quantum Machine Learning](14-quantum-machine-learning.ipynb) | Feature maps, variational quantum classifier, quantum kernels |
| 15 | [Error Mitigation](15-error-mitigation.ipynb) | ZNE, Pauli twirling, dynamical decoupling, PEC, CDR, TREX, readout mitigation |
| 16 | [Advanced Topics](16-advanced-topics.ipynb) | Trotter simulation, HHL, Clifford sim, Pauli algebra, pulse programming, QASM/Quil, backends |

## Pedagogy

Each notebook follows a consistent structure:
1. **Concept introduction** — what and why, with intuition
2. **Code demonstration** — build it in Goqu, visualize it
3. **Predict-then-verify** — "what will this output?" then run and compare
4. **Exercises** — progressively harder challenges with hints
5. **Key takeaways** — summary and common misconceptions addressed

## How It Works

Each notebook renders circuit diagrams and visualizations inline:

```go
// Circuit diagrams
fmt.Println(draw.String(circuit))

// SVG visualizations (Bloch sphere, histogram, state city)
gonbui.DisplayHTML(viz.Histogram(counts))
```

The `go.mod` in this directory uses a `replace` directive to reference the local Goqu source. gonb resolves this via the `go.work` file.

## Notes

- Variables shared across cells are declared at the package level using `var` blocks
- Imports are placed in their own declaration cells (no `%%` prefix)
- Executable cells start with `%%`
