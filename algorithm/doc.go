// Package algorithm provides quantum algorithms and classical optimization.
//
// The package is organized into focused subpackages:
//
// Classical optimization and circuit primitives:
//
//   - [algorithm/optim] - classical optimizers (Nelder-Mead, SPSA, Adam, L-BFGS)
//   - [algorithm/gradient] - quantum gradient computation (parameter-shift, finite difference)
//   - [algorithm/ansatz] - parameterized circuit templates (RealAmplitudes, EfficientSU2)
//
// Variational algorithms:
//
//   - [algorithm/vqe] - Variational Quantum Eigensolver
//   - [algorithm/vqd] - Variational Quantum Deflation (excited states)
//   - [algorithm/qaoa] - Quantum Approximate Optimization Algorithm
//   - [algorithm/vqc] - Variational Quantum Classifier and Quantum Kernel
//
// Search and estimation:
//
//   - [algorithm/grover] - Grover's search algorithm
//   - [algorithm/ampest] - Amplitude Estimation (standard and iterative)
//   - [algorithm/counting] - Quantum Approximate Counting
//
// Phase estimation and simulation:
//
//   - [algorithm/qpe] - Quantum Phase Estimation and QFT
//   - [algorithm/trotter] - Trotter-Suzuki Hamiltonian simulation
//
// Textbook and advanced algorithms:
//
//   - [algorithm/textbook] - Bernstein-Vazirani, Deutsch-Jozsa, Simon's
//   - [algorithm/shor] - Shor's factoring algorithm
//   - [algorithm/hhl] - HHL linear systems solver
//
// Error mitigation:
//
//   - [algorithm/mitigation] - error mitigation (ZNE, readout, Pauli twirling, DD, PEC, CDR, TREX)
package algorithm
