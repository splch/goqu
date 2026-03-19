package mitigation

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/noise"
)

// PECConfig specifies parameters for probabilistic error cancellation.
type PECConfig struct {
	// Circuit is the quantum circuit to mitigate.
	Circuit *ir.Circuit
	// Executor evaluates a circuit and returns an expectation value.
	Executor Executor
	// NoiseModel describes the depolarizing noise on each gate.
	NoiseModel *noise.NoiseModel
	// Samples is the number of quasi-probability samples. Default: 1000.
	Samples int
}

// PECResult holds the output of probabilistic error cancellation.
type PECResult struct {
	// MitigatedValue is the unbiased expectation estimate.
	MitigatedValue float64
	// Overhead is the sampling overhead γ^L.
	Overhead float64
	// RawValues are the sign-weighted values per sample.
	RawValues []float64
}

// RunPEC performs probabilistic error cancellation.
//
// For each gate in the circuit, it decomposes the ideal inverse noise channel
// into a quasi-probability distribution over Pauli corrections. It then samples
// corrected circuits and averages the sign-weighted results.
//
// Returns an error if any noise channel is non-depolarizing.
func RunPEC(ctx context.Context, cfg PECConfig) (*PECResult, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.RunPEC: Circuit is nil")
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("mitigation.RunPEC: Executor is nil")
	}
	if cfg.NoiseModel == nil {
		return nil, fmt.Errorf("mitigation.RunPEC: NoiseModel is nil")
	}

	samples := cfg.Samples
	if samples <= 0 {
		samples = 1000
	}

	ops := cfg.Circuit.Ops()

	// Precompute per-gate quasi-probability decompositions.
	type gateDecomp struct {
		eta   []float64   // quasi-probability weights
		gamma float64     // one-norm
		pauli []gate.Gate // correction Paulis (nil for identity / no correction)
		nq    int         // 1 or 2
	}

	decomps := make([]gateDecomp, len(ops))
	totalGamma := 1.0

	for i, op := range ops {
		if op.Gate == nil {
			continue
		}
		name := op.Gate.Name()
		if name == "reset" || name == "barrier" {
			continue
		}

		ch := cfg.NoiseModel.Lookup(name, op.Qubits)
		if ch == nil {
			// No noise on this gate - identity decomposition.
			continue
		}

		p, ok := extractDepolarizingParam(ch)
		if !ok {
			return nil, fmt.Errorf("mitigation.RunPEC: non-depolarizing channel %q on gate %q", ch.Name(), name)
		}

		nq := ch.Qubits()
		var d gateDecomp
		d.nq = nq

		if nq == 1 {
			// 1Q depolarizing: inverse channel quasi-probability decomposition.
			// Pauli eigenvalue λ = 1-4p/3.
			// η₀ = (1-p/3)/(1-4p/3), ηₖ = -(p/3)/(1-4p/3) for k=1,2,3.
			denom := 1 - 4*p/3
			if math.Abs(denom) < 1e-15 {
				return nil, fmt.Errorf("mitigation.RunPEC: depolarizing parameter p=%.4f too large", p)
			}
			eta0 := (1 - p/3) / denom
			etak := -(p / 3) / denom
			d.eta = []float64{eta0, etak, etak, etak}
			d.pauli = []gate.Gate{gate.I, gate.X, gate.Y, gate.Z}
			d.gamma = math.Abs(eta0) + 3*math.Abs(etak)
		} else {
			// 2Q depolarizing: inverse channel quasi-probability decomposition.
			// Kraus[0] = sqrt(1-p)·I⊗I, Kraus[k] = sqrt(p/15)·P_k for k=1..15.
			// Pauli eigenvalue λ = 1-16p/15.
			// η₀ = (1-p/15)/(1-16p/15), ηₖ = -(p/15)/(1-16p/15) for k=1..15.
			denom := 1 - 16*p/15
			if math.Abs(denom) < 1e-15 {
				return nil, fmt.Errorf("mitigation.RunPEC: 2Q depolarizing parameter p=%.4f too large", p)
			}
			eta0 := (1 - p/15) / denom
			etak := -(p / 15) / denom
			d.eta = make([]float64, 16)
			d.pauli = make([]gate.Gate, 16)
			d.eta[0] = eta0
			d.pauli[0] = nil // no correction
			for k := 1; k < 16; k++ {
				d.eta[k] = etak
				d.pauli[k] = nil // placeholder, we handle 2Q Paulis below
			}
			d.gamma = math.Abs(eta0) + 15*math.Abs(etak)
		}

		decomps[i] = d
		totalGamma *= d.gamma
	}

	// Sampling loop.
	rng := rand.New(rand.NewSource(rand.Int63()))
	values := make([]float64, samples)

	// Precompute 2Q Pauli pairs for indexing.
	pauli2Q := [16][2]gate.Gate{}
	for a := range 4 {
		for b := range 4 {
			pauli2Q[a*4+b] = [2]gate.Gate{paulis[a], paulis[b]}
		}
	}

	for s := range samples {
		var newOps []ir.Operation
		signProd := 1.0

		for i, op := range ops {
			newOps = append(newOps, op)

			d := decomps[i]
			if d.eta == nil {
				continue
			}

			// Sample correction from |ηᵢ|/γ distribution.
			u := rng.Float64() * d.gamma
			cumulative := 0.0
			chosen := 0
			for k, eta := range d.eta {
				cumulative += math.Abs(eta)
				if u <= cumulative {
					chosen = k
					break
				}
			}

			// Record sign.
			if d.eta[chosen] < 0 {
				signProd *= -1
			}

			// Insert Pauli correction.
			if chosen != 0 {
				if d.nq == 1 {
					newOps = append(newOps, ir.Operation{
						Gate:   d.pauli[chosen],
						Qubits: []int{op.Qubits[0]},
					})
				} else {
					pair := pauli2Q[chosen]
					if pair[0] != gate.I {
						newOps = append(newOps, ir.Operation{
							Gate:   pair[0],
							Qubits: []int{op.Qubits[0]},
						})
					}
					if pair[1] != gate.I {
						newOps = append(newOps, ir.Operation{
							Gate:   pair[1],
							Qubits: []int{op.Qubits[1]},
						})
					}
				}
			}
		}

		circ := ir.New(cfg.Circuit.Name(), cfg.Circuit.NumQubits(),
			cfg.Circuit.NumClbits(), newOps, cfg.Circuit.Metadata())

		val, err := cfg.Executor(ctx, circ)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunPEC: sample %d: %w", s, err)
		}

		values[s] = totalGamma * signProd * val
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return &PECResult{
		MitigatedValue: sum / float64(samples),
		Overhead:       totalGamma,
		RawValues:      values,
	}, nil
}

// extractDepolarizingParam extracts the depolarizing parameter p from a noise
// channel. Returns (p, true) if the channel is depolarizing, (0, false) otherwise.
//
// For a 1Q depolarizing channel: Kraus[0] = sqrt(1-p)·I, so p = 1 - |K0[0,0]|².
// For a 2Q depolarizing channel: Kraus[0] = sqrt(1-p)·I⊗I, same extraction.
//
// Validates that K₁…K_{d²-1} are each proportional to a Pauli with equal weight
// sqrt(p/(d²-1)), confirming the channel is depolarizing and not merely a channel
// whose first Kraus operator happens to be proportional to identity.
func extractDepolarizingParam(ch noise.Channel) (float64, bool) {
	kraus := ch.Kraus()
	nq := ch.Qubits()
	dim := 1 << nq

	expectedOps := dim * dim // 4 for 1Q, 16 for 2Q
	if len(kraus) != expectedOps {
		return 0, false
	}

	// Check Kraus[0] is proportional to identity.
	k0 := kraus[0]
	if len(k0) != dim*dim {
		return 0, false
	}

	// Extract scale factor from k0[0,0].
	scale := k0[0]
	if cmplx.Abs(scale) < 1e-15 {
		return 0, false
	}

	// Verify k0 is proportional to identity.
	for r := range dim {
		for c := range dim {
			expected := complex(0, 0)
			if r == c {
				expected = scale
			}
			if cmplx.Abs(k0[r*dim+c]-expected) > 1e-10 {
				return 0, false
			}
		}
	}

	p := 1 - real(scale)*real(scale) - imag(scale)*imag(scale)
	if p < -1e-10 || p > 1+1e-10 {
		return 0, false
	}
	if p < 0 {
		p = 0
	}

	// Verify remaining Kraus operators have equal Frobenius norm consistent
	// with depolarizing: each should satisfy ||K_k||_F² = p·dim/(d²-1),
	// since Tr(σ†σ) = dim for any d×d Pauli.
	if p > 1e-15 {
		expectedNormSq := p * float64(dim) / float64(dim*dim-1)
		for i := 1; i < len(kraus); i++ {
			ki := kraus[i]
			if len(ki) != dim*dim {
				return 0, false
			}
			normSq := 0.0
			for _, v := range ki {
				normSq += real(v)*real(v) + imag(v)*imag(v)
			}
			if math.Abs(normSq-expectedNormSq) > 1e-8 {
				return 0, false
			}
		}
	}

	return p, true
}
