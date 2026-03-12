# qgo Feature Matrix

> Comprehensive comparison of qgo against major quantum computing SDKs.
> Last updated: 2026-03-12

**Legend:** ✅ Full support | ⚠️ Partial / limited | ❌ Not supported

> Competitor feature sets are approximated from public documentation as of early 2026 and may have changed.

---

## Overview

| Property | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil |
|---|---|---|---|---|---|---|---|
| **Language** | Go 1.24 | Python | Python | Python | Python | Q# / Python | Python |
| **Organization** | Open source | IBM | Google | Xanadu | Amazon | Microsoft | Rigetti |
| **License** | MIT | Apache 2.0 | Apache 2.0 | Apache 2.0 | Apache 2.0 | MIT | Apache 2.0 |
| **Core external deps** | 0 (stdlib only) | ~50+ | ~30+ | ~40+ | ~20+ | N/A (compiled) | ~15+ |
| **Install** | `go get` | `pip install` | `pip install` | `pip install` | `pip install` | VS Code ext | `pip install` |
| **Primary focus** | Production infra | Full-stack | Research | QML / autodiff | Multi-provider | QEC / resource est. | Rigetti hardware |

---

## Gate Library

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| 1Q fixed gates | ✅ 10 | ✅ 15+ | ✅ 20+ | ✅ 15+ | ✅ 10+ | ✅ 10+ | ✅ 6 | I, H, X, Y, Z, S, Sdg, T, Tdg, SX — zero-alloc singletons |
| 2Q fixed gates | ✅ 4 | ✅ 10+ | ✅ 10+ | ✅ 8+ | ✅ 5+ | ✅ 5+ | ✅ 2 | CNOT, CZ, SWAP, CY |
| 3Q fixed gates | ✅ 2 | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | CCX (Toffoli), CSWAP (Fredkin) |
| 1Q rotation gates | ✅ 5 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | RX, RY, RZ, Phase, U3 |
| 2Q parameterized gates | ✅ 7 | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | CP, CRX, CRY, CRZ, RXX, RYY, RZZ |
| Multi-controlled gates | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ❌ | `Controlled(g, n)`, MCX, MCZ, MCP — arbitrary control count |
| Custom unitary | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | Unitarity validated (1e-10 tolerance) |
| State preparation | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | Mottonen decomposition |
| IonQ native gates | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ❌ | GPI, GPI2, MS — first-class, not via plugin |
| Gate adjoint (inverse) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Every gate has `.Inverse()` |
| Gate decomposition | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | `.Decompose(qubits)` on any gate |
| Pulse-level control | ⚠️ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | ✅ | OpenPulse (Braket) + IonQ custom pulse shapes |
| **Total gate types** | **31** | **50+** | **50+** | **40+** | **25+** | **20+** | **~15** | 16 fixed + 15 parameterized |

---

## Circuit Representation

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Immutable IR | ✅ | ❌ | ⚠️ | ❌ | ❌ | ✅ | ❌ | Defensive copies on all accessors |
| Fluent builder API | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ | Eager validation, error short-circuits |
| Classical bits | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | Separate clbit allocation |
| Circuit metadata | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | ❌ | Key-value string pairs, preserved through transpile |
| Circuit composition | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | `Compose` + `ComposeInverse` with qubit remapping |
| Mid-circuit measurement | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | Auto-detected via `IsDynamic()` |
| Classical conditioning | ✅ | ✅ | ✅ | ❌ | ⚠️ | ✅ | ✅ | `If(clbit, value, gate)` / `IfBlock(...)` |
| Reset | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | Deterministic qubit-to-\|0> |
| Barrier | ✅ | ✅ | ⚠️ | ❌ | ❌ | ❌ | ❌ | Transpilation boundary marker |
| Circuit statistics | ✅ | ✅ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ❌ | Depth, gate count, 2Q count, param count |
| Type safety | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | Compile-time gate/qubit validation |

---

## Symbolic / Parameterized Circuits

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Symbolic parameters | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | `param.New("theta")` with expression tree |
| Symbolic expressions | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | ❌ | Add, Sub, Mul, Div, Neg, Literal, ParamRef |
| Symbolic gate types | ✅ 9 | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | SymRX/RY/RZ/Phase/U3/CP/RXX/RYY/RZZ |
| Parameter binding | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ | `ir.Bind(circuit, map)` returns new circuit |
| Free parameter query | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | `ir.FreeParameters(circuit)` |
| Parameter sweeps | ✅ | ⚠️ | ✅ | ✅ | ✅ | ❌ | ⚠️ | Linspace, Points, Product, Zip combinators |
| Parallel sweep execution | ✅ | ⚠️ | ✅ | ✅ | ✅ | ❌ | ❌ | `RunSim` / `RunDensitySim` with goroutine workers |
| Automatic differentiation | ❌ | ❌ | ❌ | ✅ | ⚠️ | ❌ | ❌ | PennyLane's core differentiator |

---

## Simulation

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Statevector simulator | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Up to 28 qubits, stride-based |
| Density matrix simulator | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | Up to 14 qubits, two-pass U*rho*U† |
| Clifford / stabilizer sim | ✅ | ✅ | ✅ | ⚠️ | ❌ | ✅ | ❌ | Aaronson-Gottesman tableau, 1000+ qubits |
| Tensor network sim | ❌ | ⚠️ | ❌ | ✅ | ✅ | ❌ | ❌ | |
| GPU acceleration | ❌ | ✅ | ✅ | ✅ | N/A | ❌ | N/A | |
| Goroutine parallelism | ✅ | N/A | N/A | N/A | N/A | N/A | N/A | Auto at 17+q (SV), 9+q (DM) |
| Dynamic circuit sim | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | Per-shot collapse with feed-forward |
| Noise simulation | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ⚠️ | Via density matrix + noise model |
| Purity | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | `dm.Purity()` = Tr(rho^2) |
| State fidelity | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | `dm.Fidelity(target)` |

---

## Noise Modeling

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Depolarizing (1Q / 2Q) | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | sqrt(1-p)I convention; maximally mixed at p=3/4 |
| Amplitude damping | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | T1 energy decay |
| Phase damping | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | T2 dephasing |
| Bit flip | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | |
| Phase flip | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | |
| Generalized amp. damping | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | Thermal population parameter |
| Thermal relaxation | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | Combined T1/T2 with gate time |
| Readout error | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ⚠️ | Per-qubit P(0\|1), P(1\|0) |
| Noise model resolution | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | gate+qubit > gate > default priority |
| Custom Kraus channels | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | ❌ | `Custom()` with TP validation |
| Kraus/SuperOp/Choi/PTM | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | Full 4-way conversion in `sim/operator` |
| Channel composition | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | Sequential compose + tensor product |
| Process / avg gate fidelity | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | `ProcessFidelity`, `AverageGateFidelity` |

---

## Pauli Algebra and Observables

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Pauli strings | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ⚠️ | Symplectic uint8 encoding |
| Pauli sum (Hamiltonian) | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | Linear combination of PauliStrings |
| Pauli algebra (mul, commute) | ✅ | ✅ | ✅ | ⚠️ | ❌ | ❌ | ❌ | Phase table, anticommutator |
| Simplify | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | Combine like terms, drop zero |
| Expect from statevector | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | `pauli.Expect(sv, ps)` |
| Expect from density matrix | ✅ | ✅ | ✅ | ⚠️ | ❌ | ❌ | ❌ | `pauli.ExpectDM(rho, dim, ps)` |
| Expect from counts | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | Z-basis estimates from shot data |

---

## Transpilation

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Pass framework | ✅ | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ⚠️ | `func(Circuit, Target) (Circuit, error)` |
| Decompose to target basis | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | Rule-based + Euler + KAK fallback chain |
| Gate direction fixing | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠️ | ✅ | H-conjugation for CX reversal |
| Cancel adjacent gates | ✅ | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | H-H, X-X, CNOT-CNOT, S-Sdg, T-Tdg, etc. |
| Merge rotations | ✅ | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | Same-axis rotation angle summation |
| Commute through CNOT | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | Z-commutes-through-control optimization |
| Parallelize ops | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | Reorder for minimum circuit depth |
| Remove barriers | ✅ | ✅ | N/A | N/A | N/A | N/A | N/A | |
| Validate target | ✅ | ✅ | ⚠️ | ❌ | ❌ | ⚠️ | ⚠️ | Post-transpile correctness check |
| SABRE routing | ✅ | ✅ | ⚠️ | ❌ | ❌ | ⚠️ | ✅ | 20-trial parallel multi-start |
| KAK decomposition | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | Arbitrary 2Q gate into CNOT + 1Q |
| Euler decomposition | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ZYZ / ZXZ adaptive to target |
| Multi-controlled decomp. | ✅ | ✅ | ✅ | ⚠️ | ❌ | ✅ | ❌ | Barenco et al. V-gate recursion |
| Optimization levels | ✅ 4 | ✅ 4 | ⚠️ 2 | ❌ | ❌ | ❌ | ❌ | None / Basic / Full / Parallel |
| Parallel strategy selection | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | Run strategies concurrently, pick lowest cost |
| Predefined targets | ✅ 7 | ✅ many | ⚠️ | ❌ | ⚠️ | ⚠️ | ✅ 1 | IonQ(2), IBM(3), Quantinuum, Simulator |
| Noise-adaptive transpile | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | |
| Dynamic decoupling | ❌ | ✅ | ⚠️ | ❌ | ❌ | ❌ | ❌ | |

---

## Hardware Backends

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| IBM Quantum | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | ❌ | Qiskit Runtime V2, IAM auth, stdlib-only |
| IonQ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | v0.4 REST, native gateset auto-detect |
| AWS Braket | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | Separate module (AWS SDK v2 dep) |
| Google Quantum | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | |
| Rigetti | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | |
| Quantinuum | ⚠️ | ⚠️ | ❌ | ✅ | ❌ | ✅ | ❌ | Target defined, client not yet |
| Local simulator backend | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Wraps statevector sim, result caching |
| Mock backend | ✅ | ⚠️ | ⚠️ | ❌ | ❌ | ❌ | ❌ | Configurable latency, error injection |
| HTTP retry + backoff | ✅ | ⚠️ | ⚠️ | ❌ | ✅ | ⚠️ | ⚠️ | Exponential backoff for 429/5xx |
| Stdlib-only HTTP clients | ✅ | N/A | N/A | N/A | N/A | N/A | N/A | IonQ + IBM use only net/http |

---

## Job Orchestration

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Sync submit + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | `manager.Submit()` |
| Async submit | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ⚠️ | `SubmitAsync()` returns Go channel |
| Batch submission | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ❌ | `SubmitBatch()` to multiple backends |
| Watch channels | ✅ | ⚠️ | ❌ | ❌ | ⚠️ | ❌ | ❌ | Streaming status updates via Go channel |
| Sweep submission | ✅ | ⚠️ | ✅ | ✅ | ✅ | ❌ | ⚠️ | Concurrent workers per parameter point |
| Concurrency limiter | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ❌ | Semaphore-based `MaxConcurrent` |
| Retry with backoff | ✅ | ⚠️ | ❌ | ❌ | ✅ | ❌ | ❌ | Configurable policy + exponential delay |
| Circuit breaker | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | Threshold-based with half-open probe |
| Job cancellation | ✅ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ | `backend.Cancel(ctx, jobID)` |

---

## Observability and Logging

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Structured logging | ✅ | ⚠️ | ⚠️ | ❌ | ⚠️ | ❌ | ⚠️ | `log/slog` with DI, trace correlation |
| Distributed tracing (OTel) | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ❌ | `otelbridge` creates nested spans |
| Prometheus metrics | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ❌ | `prombridge` with 9 metric types |
| Zero-dep hooks | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `observe.Hooks` wrap pattern (httptrace-inspired) |
| Context propagation | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ❌ | `WithHooks(ctx)` / `FromContext(ctx)` |
| Per-pass timing | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | WrapTranspile + WrapPass hooks |
| Job lifecycle hooks | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | WrapJob + OnJobPoll |
| HTTP request tracing | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | WrapHTTP with method / path / backend |
| Sweep execution hooks | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | WrapSweep for parameter sweep tracking |

---

## QASM Support

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| OpenQASM 2.0 parse | ✅ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ❌ | `qreg`/`creg`, measure `q -> c` syntax |
| OpenQASM 3.0 parse | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | ❌ | `qubit`/`bit`, `c = measure q` syntax |
| OpenQASM 3.0 emit | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | ❌ | Pi-fraction formatting |
| Lossless round-trip | ✅ | ✅ | ⚠️ | ❌ | ❌ | ❌ | ❌ | Fuzz-tested parse-emit-reparse |
| Gate modifiers (ctrl @) | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | `ctrl @`, `inv @` applied |
| Gate modifiers (pow, negctrl) | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | Integer pow; negctrl via X sandwich |
| Quil support | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | |
| Hand-written parser | ✅ | ❌ | ❌ | N/A | N/A | N/A | N/A | Recursive descent, zero deps |

---

## Visualization

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| ASCII circuit diagram | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ⚠️ | Cirq-style: @, X, x, M symbols |
| SVG circuit diagram | ✅ | ⚠️ | ❌ | ❌ | ❌ | ❌ | ❌ | Zero-dep SVG generation |
| Interactive (Jupyter) | ❌ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | N/A for Go |
| LaTeX export | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | |
| Matplotlib plots | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ⚠️ | N/A for Go |

---

## Algorithms and Applications

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Built-in VQE | ❌ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | Primitives available (sweeps + expectation values) |
| Built-in QAOA | ❌ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | |
| Grover's search | ❌ | ✅ | ⚠️ | ⚠️ | ⚠️ | ✅ | ❌ | |
| QFT | ❌ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ❌ | |
| QPE | ❌ | ✅ | ⚠️ | ⚠️ | ⚠️ | ✅ | ❌ | |
| Error correction codes | ❌ | ⚠️ | ⚠️ | ❌ | ❌ | ✅ | ❌ | |
| Resource estimation | ❌ | ⚠️ | ❌ | ❌ | ❌ | ✅ | ❌ | |
| Classical optimizers | ❌ | ✅ | ❌ | ✅ | ⚠️ | ❌ | ❌ | Use Go optimizers (e.g. gonum) externally |
| Automatic differentiation | ❌ | ❌ | ❌ | ✅ | ⚠️ | ❌ | ❌ | |

---

## Testing and Quality

| Feature | qgo | Qiskit | Cirq | PennyLane | Braket SDK | Q#/QDK | PyQuil | qgo Notes |
|---|---|---|---|---|---|---|---|---|
| Unit tests | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | All 27+ packages covered |
| Fuzz tests | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 16 fuzz targets across 4 modules |
| Integration tests | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Verified against real IonQ API |
| Example tests (verified) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | 9 `Example` functions run by `go test` |
| Race detector safe | ✅ | N/A | N/A | N/A | N/A | N/A | N/A | Tested with `-race` flag |
| Benchmarks | ✅ | ✅ | ⚠️ | ⚠️ | ❌ | ⚠️ | ❌ | 16-qubit GHZ ~2.8ms |

---

## Unique Strengths

### Zero External Dependencies

All 32 core packages use only the Go standard library. The IonQ and IBM backend clients use `net/http` and `encoding/json` from stdlib — no third-party HTTP or JSON libraries. Only three separate Go modules have external dependencies: `backend/braket` (AWS SDK v2), `observe/otelbridge` (OpenTelemetry), and `observe/prombridge` (Prometheus). This means `go build` from scratch requires zero network fetches for core functionality, and dependency audits are trivial.

### Go Language Advantages

Compile-time type safety catches gate-qubit arity mismatches that Python SDKs only discover at runtime. Goroutine-based parallelism for statevector (17+ qubits) and density matrix (9+ qubits) operations requires no GIL workarounds. Single-binary deployment produces trivially small container images. The standard library `context.Context` provides natural cancellation and timeout propagation through the entire stack, from job submission to HTTP requests.

### Production Observability

qgo is the only quantum SDK with built-in distributed tracing, metrics, and structured logging. The wrap-pattern hook design (inspired by `net/http/httptrace`) enables zero-overhead instrumentation when no hooks are attached. OpenTelemetry spans nest naturally: `qgo.transpile > qgo.transpile.{pass} > qgo.http`. Prometheus metrics cover 9 dimensions across transpilation, simulation, job management, and HTTP latency. The `log/slog` integration uses dependency injection per the Go team's recommendation, with trace context correlation for connecting logs to spans.

### Immutable Circuit IR

Unlike Qiskit's mutable `QuantumCircuit`, qgo's `ir.Circuit` is immutable after construction. All accessor methods return defensive copies. This prevents accidental mutation bugs in transpilation pipelines and enables safe concurrent access without locks.

### Parallel Transpilation Strategy Selection

At `LevelParallel`, qgo runs Basic and Full optimization pipelines concurrently and selects the result with the lowest cost function (weighted: 10x two-qubit gates + 1x depth + 0.1x total gates). No other quantum SDK runs competing optimization strategies in parallel.

### Comprehensive Noise Tooling

Full Kraus / SuperOp / Choi / PTM interconversion in a zero-dependency package, plus process fidelity and average gate fidelity computations. Eight built-in Kraus channels cover all standard noise models, and custom channels can be constructed from arbitrary Kraus operators.

---

## Known Gaps and Intentional Omissions

| Feature | Available In | Reason for Omission |
|---|---|---|
| GPU acceleration | Qiskit (cuAer), Cirq (qsim), PennyLane (cuQuantum) | Go lacks a mature CUDA ecosystem; goroutine parallelism provides good scaling for typical workloads |
| Tensor network simulator | PennyLane (lightning.tensor), Braket (TN1) | Requires complex numerical libraries; users can submit to TN1 via the Braket backend |
| Automatic differentiation | PennyLane (core feature) | Fundamentally tied to Python/JAX/PyTorch autograd; parameter sweeps provide the primitives for external optimizers |
| Pulse-level control (defcal, simulation) | Qiskit (pulse), PyQuil (Quilt) | Gate-to-pulse calibration mappings and pulse simulation deferred; OpenPulse programs (Braket) and IonQ custom pulse shapes supported |
| Built-in VQE / QAOA | Qiskit, Cirq, PennyLane, Braket | qgo provides the primitives (parameterized circuits, sweeps, expectation values); algorithm implementations belong in user code or a separate package |
| Quantum error correction | Q#/QDK (stabilizer codes) | QEC is Microsoft's core differentiator; beyond scope of an infrastructure SDK |
| Resource estimation | Q#/QDK | Tightly coupled to Q#'s compiler infrastructure |
| Interactive notebooks | Qiskit, Cirq, PennyLane | Go is not a notebook language; SVG output can be embedded in web UIs |
| Classical optimizer suite | Qiskit (SciPy), PennyLane (gradient descent) | Go has mature optimization libraries (e.g. gonum/optimize) that integrate directly |
| QASM fractional pow | Qiskit, Q# | Integer pow supported; fractional pow (e.g. pow(0.5)) requires matrix eigendecomposition |

---

## Package Summary

| Package | Description | External Deps |
|---|---|---|
| `circuit/gate` | 16 fixed + 15 parameterized + custom unitary + symbolic gates | None |
| `circuit/builder` | Fluent circuit construction with eager validation | None |
| `circuit/ir` | Immutable circuit IR with stats and metadata | None |
| `circuit/draw` | ASCII + SVG circuit visualization | None |
| `circuit/param` | Symbolic parameters and expression trees | None |
| `sim/statevector` | Statevector simulator (up to 28 qubits) | None |
| `sim/densitymatrix` | Density matrix simulator (up to 14 qubits) | None |
| `sim/clifford` | Stabilizer simulator (Aaronson-Gottesman tableau) | None |
| `sim/noise` | 8 Kraus channels + readout error + Custom (TP-validated) + NoiseModel | None |
| `sim/pauli` | Pauli algebra + expectation values | None |
| `sim/operator` | Kraus / SuperOp / Choi / PTM + fidelity metrics | None |
| `qasm/parser` | OpenQASM 2/3 recursive descent parser | None |
| `qasm/emitter` | OpenQASM 3.0 emitter with pi-fraction formatting | None |
| `qasm/token` | QASM token types | None |
| `transpile` | Pass type definition | None |
| `transpile/pass` | 8 optimization passes | None |
| `transpile/pipeline` | 4-level optimization pipelines | None |
| `transpile/routing` | SABRE routing algorithm | None |
| `transpile/target` | 7 predefined hardware targets | None |
| `transpile/decompose` | KAK + Euler + multi-controlled decomposition | None |
| `transpile/analysis` | Qubit timeline analysis | None |
| `transpile/verify` | Post-transpile circuit equivalence checking | None |
| `pulse` | Port, Frame, Waveform, 8 instruction types, immutable Program, fluent Builder | None |
| `pulse/waveform` | Constant, Gaussian, DRAG, GaussianSquare, Arbitrary waveforms | None |
| `sweep` | Parameter sweeps (Linspace, Points, Product, Zip) | None |
| `backend` | Backend interface + Job / Result types | None |
| `backend/local` | Local simulator backend with result caching | None |
| `backend/ionq` | IonQ v0.4 REST client | None |
| `backend/ibm` | IBM Qiskit Runtime V2 client | None |
| `backend/mock` | Configurable mock for testing | None |
| `job/manager` | Concurrent job orchestration + Watch channels | None |
| `job/retry` | Retry policy + circuit breaker | None |
| `observe` | Zero-dep observability hooks (7 hook types) | None |
| `backend/braket` | AWS Braket client | aws-sdk-go-v2 |
| `observe/otelbridge` | OpenTelemetry span bridge | go.opentelemetry.io/otel |
| `observe/prombridge` | Prometheus metrics bridge | prometheus/client_golang |
