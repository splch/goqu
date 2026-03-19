// Package mitigation provides quantum error mitigation techniques for NISQ devices.
//
// Error mitigation improves expectation value estimates from noisy quantum
// hardware without the overhead of full quantum error correction.
//
// # Zero-Noise Extrapolation (ZNE)
//
// ZNE intentionally amplifies circuit noise at several scale factors, then
// extrapolates back to the zero-noise limit. The workflow is:
//
//  1. Fold the circuit at each scale factor ([FoldCircuit])
//  2. Execute each folded circuit via an [Executor]
//  3. Extrapolate the results to zero noise ([Extrapolate])
//
// [RunZNE] orchestrates these steps:
//
//	result, err := mitigation.RunZNE(ctx, mitigation.ZNEConfig{
//	    Circuit:      circ,
//	    Executor:     mitigation.DensityMatrixExecutor(hamiltonian, noiseModel),
//	    ScaleFactors: []float64{1, 3, 5},
//	    Extrapolator: mitigation.LinearExtrapolator,
//	})
//
// # Measurement Error Mitigation
//
// Measurement (readout) error mitigation calibrates the classical confusion
// matrix of the measurement apparatus and inverts it to correct raw counts
// or probabilities:
//
//	cal, err := mitigation.CalibrateReadout(ctx, numQubits, shots, basisExec)
//	corrected := cal.CorrectCounts(rawCounts)
//
// # Pauli Twirling
//
// Pauli twirling converts coherent errors into stochastic Pauli errors by
// inserting random Pauli gates around 2-qubit gates (CNOT and CZ). This is
// a prerequisite for PEC and improves ZNE accuracy:
//
//	result, err := mitigation.RunTwirl(ctx, mitigation.TwirlConfig{
//	    Circuit:  circ,
//	    Executor: noisyExec,
//	    Samples:  100,
//	})
//
// # Digital Dynamical Decoupling (DD)
//
// DD inserts identity-equivalent pulse sequences (XX or XY4) into idle qubit
// periods. This is a pure circuit transform - no executor or noise model needed:
//
//	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
//	    Circuit:  circ,
//	    Sequence: mitigation.DDXX,
//	})
//
// # Probabilistic Error Cancellation (PEC)
//
// PEC provides unbiased estimation via quasi-probability sampling over Pauli
// corrections. Requires a depolarizing noise model:
//
//	result, err := mitigation.RunPEC(ctx, mitigation.PECConfig{
//	    Circuit:    circ,
//	    Executor:   noisyExec,
//	    NoiseModel: nm,
//	    Samples:    1000,
//	})
//
// # Clifford Data Regression (CDR)
//
// CDR generates near-Clifford training circuits, runs noisy and ideal
// simulations, fits an affine correction model, and applies it to the
// original noisy result. No noise model is needed:
//
//	result, err := mitigation.RunCDR(ctx, mitigation.CDRConfig{
//	    Circuit:     circ,
//	    Executor:    noisyExec,
//	    Hamiltonian: hamiltonian,
//	    NumTraining: 20,
//	})
//
// # Twirled Readout Error eXtinction (TREX)
//
// TREX mitigates readout errors with O(n) overhead by inserting random X
// gates before measurements and classically undoing the bit flips:
//
//	result, err := mitigation.RunTREX(ctx, mitigation.TREXConfig{
//	    Circuit: circ,
//	    Runner:  shotRunner,
//	    Shots:   1000,
//	    Samples: 10,
//	})
package mitigation
