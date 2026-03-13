// Package pulsesim simulates pulse programs on a statevector.
//
// The simulator models single-qubit drive Hamiltonians in the rotating frame:
//
//	H = Omega(t)/2 * (cos(phi)*X + sin(phi)*Y)
//
// where Omega is the waveform envelope and phi is the frame phase.
// Each time step produces an analytical unitary (no ODE integration required).
//
// Two-qubit interactions are supported via functional options:
//   - [WithCoupling] enables static ZZ coupling between qubit pairs
//   - [WithCRFrames] declares cross-resonance drive frames for 2Q entangling gates
package pulsesim

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand/v2"

	"github.com/splch/qgo/pulse"
)

// FrameMap associates pulse frame names with qubit indices.
type FrameMap map[string]int

// frameState tracks the runtime state of a single frame during simulation.
type frameState struct {
	phase     float64
	frequency float64
	qubit     int
	dt        float64
	crControl int // -1 if not a CR frame
	crTarget  int // -1 if not a CR frame
}

// Sim simulates pulse programs via full statevector evolution.
type Sim struct {
	numQubits int
	state     []complex128
	mapping   FrameMap
	coupling  CouplingMap
	crFrames  CRFrameMap
}

// New creates a pulse simulator initialized to |0...0>.
// Optional [Option] values configure two-qubit coupling and CR frames.
func New(numQubits int, fm FrameMap, opts ...Option) *Sim {
	if numQubits < 1 || numQubits > 28 {
		panic(fmt.Sprintf("pulsesim: numQubits %d out of range [1, 28]", numQubits))
	}
	n := 1 << numQubits
	state := make([]complex128, n)
	state[0] = 1

	fm2 := make(FrameMap, len(fm))
	for k, v := range fm {
		fm2[k] = v
	}
	s := &Sim{numQubits: numQubits, state: state, mapping: fm2}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Evolve applies all pulse instructions, resetting the state to |0...0> first.
func (s *Sim) Evolve(prog *pulse.Program) error {
	for i := range s.state {
		s.state[i] = 0
	}
	s.state[0] = 1

	// Validate coupling map qubit indices.
	for pair := range s.coupling {
		if pair[0] < 0 || pair[0] >= s.numQubits || pair[1] < 0 || pair[1] >= s.numQubits {
			return fmt.Errorf("pulsesim: coupling pair %v out of range [0, %d)", pair, s.numQubits)
		}
	}

	frames := make(map[string]*frameState)
	for _, f := range prog.Frames() {
		qubit, ok := s.mapping[f.Name()]
		if !ok {
			return fmt.Errorf("pulsesim: frame %q not in FrameMap", f.Name())
		}
		if qubit < 0 || qubit >= s.numQubits {
			return fmt.Errorf("pulsesim: frame %q maps to qubit %d, out of range [0, %d)",
				f.Name(), qubit, s.numQubits)
		}
		fs := &frameState{
			phase:     f.Phase(),
			frequency: f.Frequency(),
			qubit:     qubit,
			dt:        f.Port().Dt(),
			crControl: -1,
			crTarget:  -1,
		}
		if cr, ok := s.crFrames[f.Name()]; ok {
			if cr[0] < 0 || cr[0] >= s.numQubits {
				return fmt.Errorf("pulsesim: CR frame %q control qubit %d out of range [0, %d)",
					f.Name(), cr[0], s.numQubits)
			}
			if cr[1] < 0 || cr[1] >= s.numQubits {
				return fmt.Errorf("pulsesim: CR frame %q target qubit %d out of range [0, %d)",
					f.Name(), cr[1], s.numQubits)
			}
			if cr[0] == cr[1] {
				return fmt.Errorf("pulsesim: CR frame %q has control == target (%d)",
					f.Name(), cr[0])
			}
			fs.crControl = cr[0]
			fs.crTarget = cr[1]
		}
		frames[f.Name()] = fs
	}

	for _, inst := range prog.Instructions() {
		switch v := inst.(type) {
		case pulse.Play:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			samples := v.Waveform.Sample(fs.dt)
			if fs.crControl >= 0 {
				// Cross-resonance drive: apply 2Q Hamiltonian.
				for _, sample := range samples {
					amp := cmplx.Abs(sample)
					if amp > 1e-15 {
						theta := amp * fs.dt
						phi := fs.phase + cmplx.Phase(sample)
						s.applyCRStep(fs.crControl, fs.crTarget, theta, phi)
					}
					fs.phase += 2 * math.Pi * fs.frequency * fs.dt
				}
			} else {
				// Standard 1Q drive.
				for _, sample := range samples {
					amp := cmplx.Abs(sample)
					if amp > 1e-15 {
						theta := amp * fs.dt
						phi := fs.phase + cmplx.Phase(sample)
						s.applyDriveStep(fs.qubit, theta, phi)
					}
					fs.phase += 2 * math.Pi * fs.frequency * fs.dt
				}
			}
			// Apply static ZZ coupling for the total play duration.
			totalDuration := float64(len(samples)) * fs.dt
			s.applyAllZZ(totalDuration)

		case pulse.Delay:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			fs.phase += 2 * math.Pi * fs.frequency * v.Duration
			s.applyAllZZ(v.Duration)

		case pulse.SetPhase:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			fs.phase = v.Phase

		case pulse.ShiftPhase:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			fs.phase += v.Delta

		case pulse.SetFrequency:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			fs.frequency = v.Frequency

		case pulse.ShiftFrequency:
			fs, ok := frames[v.Frame.Name()]
			if !ok {
				return fmt.Errorf("pulsesim: unknown frame %q", v.Frame.Name())
			}
			fs.frequency += v.Delta

		case pulse.Barrier:
			// No-op: synchronization marker only.

		case pulse.Capture:
			// No-op in Evolve; measurement is handled in Run.
		}
	}
	return nil
}

// applyDriveStep applies a single-qubit drive unitary for one time step.
//
//	U = cos(theta/2)*I - i*sin(theta/2)*(cos(phi)*X + sin(phi)*Y)
func (s *Sim) applyDriveStep(qubit int, theta, phi float64) {
	cosH := math.Cos(theta / 2)
	sinH := math.Sin(theta / 2)
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	m00 := complex(cosH, 0)
	m01 := complex(-sinH*sinPhi, -sinH*cosPhi)
	m10 := complex(sinH*sinPhi, -sinH*cosPhi)
	m11 := complex(cosH, 0)

	halfBlock := 1 << qubit
	block := halfBlock << 1
	n := len(s.state)
	for b0 := 0; b0 < n; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset
			i1 := i0 + halfBlock
			a0, a1 := s.state[i0], s.state[i1]
			s.state[i0] = m00*a0 + m01*a1
			s.state[i1] = m10*a0 + m11*a1
		}
	}
}

// Run evolves the state and samples measurement counts.
func (s *Sim) Run(prog *pulse.Program, shots int) (map[string]int, error) {
	if shots <= 0 {
		return nil, fmt.Errorf("pulsesim: shots must be positive")
	}
	if err := s.Evolve(prog); err != nil {
		return nil, err
	}

	probs := s.probabilities()
	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for range shots {
		idx := sampleIndex(probs, rng)
		bs := formatBitstring(idx, s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

// StateVector returns a copy of the current statevector.
func (s *Sim) StateVector() []complex128 {
	out := make([]complex128, len(s.state))
	copy(out, s.state)
	return out
}

func (s *Sim) probabilities() []float64 {
	probs := make([]float64, len(s.state))
	for i, amp := range s.state {
		probs[i] = real(amp)*real(amp) + imag(amp)*imag(amp)
	}
	return probs
}

func sampleIndex(probs []float64, rng *rand.Rand) int {
	r := rng.Float64()
	cum := 0.0
	for i, p := range probs {
		cum += p
		if r < cum {
			return i
		}
	}
	return len(probs) - 1
}

func formatBitstring(idx, n int) string {
	bs := make([]byte, n)
	for i := range n {
		if idx&(1<<i) != 0 {
			bs[n-1-i] = '1'
		} else {
			bs[n-1-i] = '0'
		}
	}
	return string(bs)
}
