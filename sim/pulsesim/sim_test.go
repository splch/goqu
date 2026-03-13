package pulsesim

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

const tol = 1e-10

func prob(amp complex128) float64 {
	return real(amp)*real(amp) + imag(amp)*imag(amp)
}

func TestNewSim(t *testing.T) {
	sim := New(2, FrameMap{"f0": 0, "f1": 1})
	sv := sim.StateVector()
	if len(sv) != 4 {
		t.Fatalf("len(sv) = %d, want 4", len(sv))
	}
	if sv[0] != 1 {
		t.Errorf("sv[0] = %v, want 1", sv[0])
	}
	for i := 1; i < 4; i++ {
		if sv[i] != 0 {
			t.Errorf("sv[%d] = %v, want 0", i, sv[i])
		}
	}
}

func makePiPulseProgram(phaseBefore float64) *pulse.Program {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0) // freq=0 for clean rotation

	T := 100e-9
	amp := math.Pi / T
	wf := waveform.MustConstant(complex(amp, 0), T)

	instrs := []pulse.Instruction{}
	if phaseBefore != 0 {
		instrs = append(instrs, pulse.SetPhase{Frame: frame, Phase: phaseBefore})
	}
	instrs = append(instrs, pulse.Play{Frame: frame, Waveform: wf})

	return pulse.NewProgram("pi-pulse",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		instrs,
		nil,
	)
}

func TestPiPulse(t *testing.T) {
	sim := New(1, FrameMap{"q0_drive": 0})
	if err := sim.Evolve(makePiPulseProgram(0)); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// |0> -> -i|1>: P(|0>) ~ 0, P(|1>) ~ 1
	if p := prob(sv[0]); p > tol {
		t.Errorf("|0> probability = %g, want ~0", p)
	}
	if p := prob(sv[1]); math.Abs(p-1) > tol {
		t.Errorf("|1> probability = %g, want ~1", p)
	}
}

func TestHalfPiPulse(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)

	T := 100e-9
	amp := math.Pi / (2 * T) // pi/2 rotation
	wf := waveform.MustConstant(complex(amp, 0), T)

	prog := pulse.NewProgram("half-pi",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{pulse.Play{Frame: frame, Waveform: wf}},
		nil,
	)

	sim := New(1, FrameMap{"q0_drive": 0})
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// |0> -> (|0> - i|1>)/sqrt(2): both probabilities ~ 0.5
	p0 := prob(sv[0])
	p1 := prob(sv[1])
	if math.Abs(p0-0.5) > tol {
		t.Errorf("|0> probability = %g, want ~0.5", p0)
	}
	if math.Abs(p1-0.5) > tol {
		t.Errorf("|1> probability = %g, want ~0.5", p1)
	}
}

func TestPhaseShiftThenPlay(t *testing.T) {
	// Without phase: pi-pulse about X axis -> |0> -> -i|1>
	sim0 := New(1, FrameMap{"q0_drive": 0})
	if err := sim0.Evolve(makePiPulseProgram(0)); err != nil {
		t.Fatal(err)
	}
	sv0 := sim0.StateVector()

	// With phase pi/2: pi-pulse about Y axis -> |0> -> |1>
	sim1 := New(1, FrameMap{"q0_drive": 0})
	if err := sim1.Evolve(makePiPulseProgram(math.Pi / 2)); err != nil {
		t.Fatal(err)
	}
	sv1 := sim1.StateVector()

	// Both should have |1> with unit probability, but different phases.
	if math.Abs(prob(sv0[1])-1) > tol {
		t.Errorf("X-pulse |1> prob = %g, want ~1", prob(sv0[1]))
	}
	if math.Abs(prob(sv1[1])-1) > tol {
		t.Errorf("Y-pulse |1> prob = %g, want ~1", prob(sv1[1]))
	}

	// X-rotation: sv[1] ~ -i (imag ~ -1)
	if math.Abs(imag(sv0[1])+1) > tol {
		t.Errorf("X-pulse sv[1] = %v, want ~-i", sv0[1])
	}
	// Y-rotation: sv[1] ~ +1 (real ~ 1)
	if math.Abs(real(sv1[1])-1) > tol {
		t.Errorf("Y-pulse sv[1] = %v, want ~1", sv1[1])
	}
}

func TestDelayPhaseAdvance(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	// freq=1 GHz so delay of 0.25 ns gives phase advance of pi/2.
	frame := pulse.MustFrame("q0_drive", port, 1e9, 0)

	T := 100e-9
	amp := math.Pi / T
	wf := waveform.MustConstant(complex(amp, 0), T)

	prog := pulse.NewProgram("delay-test",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{
			pulse.Delay{Frame: frame, Duration: 0.25e-9},   // phase += pi/2
			pulse.SetFrequency{Frame: frame, Frequency: 0}, // stop accumulation
			pulse.Play{Frame: frame, Waveform: wf},         // rotate at phase pi/2
		},
		nil,
	)

	sim := New(1, FrameMap{"q0_drive": 0})
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// Phase pi/2 → Y-axis rotation → |0> → |1> with real amplitude.
	if math.Abs(prob(sv[1])-1) > tol {
		t.Errorf("|1> probability = %g, want ~1", prob(sv[1]))
	}
	if math.Abs(real(sv[1])-1) > tol {
		t.Errorf("sv[1] = %v, want ~1+0i (Y-rotation)", sv[1])
	}
}

func TestBarrierNoop(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)

	prog := pulse.NewProgram("barrier-only",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{
			pulse.Barrier{Frames: []pulse.Frame{frame}},
		},
		nil,
	)

	sim := New(1, FrameMap{"q0_drive": 0})
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	if sv[0] != 1 {
		t.Errorf("sv[0] = %v, want 1 (unchanged)", sv[0])
	}
	if sv[1] != 0 {
		t.Errorf("sv[1] = %v, want 0 (unchanged)", sv[1])
	}
}

func TestRunCapture(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)

	// Pi-pulse → should measure |1> with near certainty.
	T := 100e-9
	amp := math.Pi / T
	wf := waveform.MustConstant(complex(amp, 0), T)

	prog := pulse.NewProgram("run-test",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{
			pulse.Play{Frame: frame, Waveform: wf},
			pulse.Capture{Frame: frame, Duration: 1e-6},
		},
		nil,
	)

	sim := New(1, FrameMap{"q0_drive": 0})
	counts, err := sim.Run(prog, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if counts["1"] < 990 {
		t.Errorf("expected ~1000 counts for |1>, got %d", counts["1"])
	}
}

func TestMultiQubit(t *testing.T) {
	port0 := pulse.MustPort("d0", 1e-9)
	port1 := pulse.MustPort("d1", 1e-9)
	frame0 := pulse.MustFrame("q0_drive", port0, 0, 0)
	frame1 := pulse.MustFrame("q1_drive", port1, 0, 0)

	T := 100e-9
	amp := math.Pi / T
	wf := waveform.MustConstant(complex(amp, 0), T)

	// Pi-pulse on qubit 0 only.
	prog := pulse.NewProgram("multi",
		[]pulse.Port{port0, port1},
		[]pulse.Frame{frame0, frame1},
		[]pulse.Instruction{
			pulse.Play{Frame: frame0, Waveform: wf},
		},
		nil,
	)

	sim := New(2, FrameMap{"q0_drive": 0, "q1_drive": 1})
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// State should be |01> (qubit 0 = 1, qubit 1 = 0) → index 1.
	if p := prob(sv[1]); math.Abs(p-1) > tol {
		t.Errorf("|01> probability = %g, want ~1", p)
	}
	for i, amp := range sv {
		if i != 1 && prob(amp) > tol {
			t.Errorf("|%02b> probability = %g, want ~0", i, prob(amp))
		}
	}
}

func TestMissingFrameMapping(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)
	wf := waveform.MustConstant(0.5, 1e-7)

	prog := pulse.NewProgram("missing",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{pulse.Play{Frame: frame, Waveform: wf}},
		nil,
	)

	sim := New(1, FrameMap{}) // no mapping
	if err := sim.Evolve(prog); err == nil {
		t.Error("expected error for missing frame mapping")
	}
}

// --- Two-qubit coupling tests ---

func TestStaticZZ(t *testing.T) {
	// ZZ coupling is diagonal: it adds phases but doesn't change probabilities.
	// Compare statevectors with and without ZZ to verify the phases changed.
	port0 := pulse.MustPort("d0", 1e-9)
	port1 := pulse.MustPort("d1", 1e-9)
	f0 := pulse.MustFrame("f0", port0, 0, 0)
	f1 := pulse.MustFrame("f1", port1, 0, 0)

	T := 100e-9
	halfPiAmp := math.Pi / (2 * T)
	halfPi := waveform.MustConstant(complex(halfPiAmp, 0), T)

	zzStrength := 1e6 // rad/s

	prog := pulse.NewProgram("zz-test",
		[]pulse.Port{port0, port1},
		[]pulse.Frame{f0, f1},
		[]pulse.Instruction{
			pulse.Play{Frame: f0, Waveform: halfPi},
			pulse.Play{Frame: f1, Waveform: halfPi},
			pulse.Delay{Frame: f0, Duration: 0.5e-6},
		},
		nil,
	)

	fm := FrameMap{"f0": 0, "f1": 1}

	// With coupling.
	cm := CouplingMap{orderedPair(0, 1): {ZZ: zzStrength}}
	simZZ := New(2, fm, WithCoupling(cm))
	if err := simZZ.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	svZZ := simZZ.StateVector()

	// Without coupling.
	simRef := New(2, fm)
	if err := simRef.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	svRef := simRef.StateVector()

	// Both should have unit norm.
	for _, sv := range [][]complex128{svZZ, svRef} {
		var norm float64
		for _, a := range sv {
			norm += real(a)*real(a) + imag(a)*imag(a)
		}
		if math.Abs(norm-1) > tol {
			t.Errorf("norm = %g, want 1", norm)
		}
	}

	// The statevectors should differ (ZZ added relative phases).
	diff := 0.0
	for i := range svZZ {
		d := svZZ[i] - svRef[i]
		diff += real(d)*real(d) + imag(d)*imag(d)
	}
	if diff < 1e-10 {
		t.Error("ZZ coupling had no effect on statevector")
	}
}

func TestCRDrive_TargetFlip(t *testing.T) {
	// |10> (control=1, target=0) with CR pi-pulse should flip target to |11>.
	port0 := pulse.MustPort("d0", 1e-9)
	portCR := pulse.MustPort("dcr", 1e-9)
	f0 := pulse.MustFrame("f0", port0, 0, 0)
	fCR := pulse.MustFrame("cr01", portCR, 0, 0)

	T := 100e-9
	piAmp := math.Pi / T
	piPulse := waveform.MustConstant(complex(piAmp, 0), T)

	// First flip qubit 0 to |1> (control).
	prog := pulse.NewProgram("cr-flip",
		[]pulse.Port{port0, portCR},
		[]pulse.Frame{f0, fCR},
		[]pulse.Instruction{
			pulse.Play{Frame: f0, Waveform: piPulse},  // |00> → |10>
			pulse.Play{Frame: fCR, Waveform: piPulse}, // CR pi on target
		},
		nil,
	)

	fm := FrameMap{"f0": 0, "cr01": 0}     // cr01 maps to qubit 0 for FrameMap
	crFrames := CRFrameMap{"cr01": {0, 1}} // control=0, target=1
	sim := New(2, fm, WithCRFrames(crFrames))
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// State should be |11> (index 3).
	if p := prob(sv[3]); math.Abs(p-1) > 0.01 {
		t.Errorf("|11> probability = %g, want ~1", p)
	}
}

func TestCRDrive_ControlZero(t *testing.T) {
	// |00> with CR pi-pulse: control=|0> means target also rotates.
	// With our model, control=|0> applies R(+theta) to target.
	port0 := pulse.MustPort("d0", 1e-9)
	portCR := pulse.MustPort("dcr", 1e-9)
	f0 := pulse.MustFrame("f0", port0, 0, 0)
	fCR := pulse.MustFrame("cr01", portCR, 0, 0)

	T := 100e-9
	piAmp := math.Pi / T
	piPulse := waveform.MustConstant(complex(piAmp, 0), T)

	prog := pulse.NewProgram("cr-ctrl0",
		[]pulse.Port{port0, portCR},
		[]pulse.Frame{f0, fCR},
		[]pulse.Instruction{
			// No flip on control, so control=|0>.
			pulse.Play{Frame: fCR, Waveform: piPulse}, // CR pi on target
		},
		nil,
	)

	fm := FrameMap{"f0": 0, "cr01": 0}
	crFrames := CRFrameMap{"cr01": {0, 1}}
	sim := New(2, fm, WithCRFrames(crFrames))
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// Control=|0>: target sees R(+pi, 0), so |00> → |01>.
	if p := prob(sv[2]); math.Abs(p-1) > 0.01 {
		t.Errorf("|01> (idx 2) probability = %g, want ~1", p)
	}
}

func TestCRDrive_BellState(t *testing.T) {
	// |+0>: superposition on control, then CR pi/2 → entangled.
	port0 := pulse.MustPort("d0", 1e-9)
	portCR := pulse.MustPort("dcr", 1e-9)
	f0 := pulse.MustFrame("f0", port0, 0, 0)
	fCR := pulse.MustFrame("cr01", portCR, 0, 0)

	T := 100e-9
	halfPiAmp := math.Pi / (2 * T)
	halfPi := waveform.MustConstant(complex(halfPiAmp, 0), T)

	prog := pulse.NewProgram("cr-bell",
		[]pulse.Port{port0, portCR},
		[]pulse.Frame{f0, fCR},
		[]pulse.Instruction{
			pulse.Play{Frame: f0, Waveform: halfPi},  // |00> → |+0>
			pulse.Play{Frame: fCR, Waveform: halfPi}, // CR pi/2
		},
		nil,
	)

	fm := FrameMap{"f0": 0, "cr01": 0}
	crFrames := CRFrameMap{"cr01": {0, 1}}
	sim := New(2, fm, WithCRFrames(crFrames))
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// Entangled state: no single basis state should have probability 1.
	maxP := 0.0
	for _, a := range sv {
		p := prob(a)
		if p > maxP {
			maxP = p
		}
	}
	if maxP > 0.9 {
		t.Errorf("max probability = %g, expected entanglement (< 0.9)", maxP)
	}

	// Verify norm.
	var norm float64
	for _, a := range sv {
		norm += prob(a)
	}
	if math.Abs(norm-1) > tol {
		t.Errorf("norm = %g, want 1", norm)
	}
}

func TestZZDuringDelay(t *testing.T) {
	// ZZ accumulates during Delay instructions.
	port := pulse.MustPort("d0", 1e-9)
	f0 := pulse.MustFrame("f0", port, 0, 0)

	// Start in |+> on q0, |+> on q1 (need 1Q drives first).
	T := 100e-9
	halfPiAmp := math.Pi / (2 * T)
	halfPi := waveform.MustConstant(complex(halfPiAmp, 0), T)

	port1 := pulse.MustPort("d1", 1e-9)
	f1 := pulse.MustFrame("f1", port1, 0, 0)

	prog := pulse.NewProgram("zz-delay",
		[]pulse.Port{port, port1},
		[]pulse.Frame{f0, f1},
		[]pulse.Instruction{
			pulse.Play{Frame: f0, Waveform: halfPi},
			pulse.Play{Frame: f1, Waveform: halfPi},
			pulse.Delay{Frame: f0, Duration: 1e-6},
		},
		nil,
	)

	zzStrength := 1e7 // strong coupling
	fm := FrameMap{"f0": 0, "f1": 1}
	cm := CouplingMap{orderedPair(0, 1): {ZZ: zzStrength}}
	sim := New(2, fm, WithCoupling(cm))
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	var norm float64
	for _, a := range sv {
		norm += prob(a)
	}
	if math.Abs(norm-1) > tol {
		t.Errorf("norm = %g, want 1", norm)
	}
}

func TestZZPlusCRDrive(t *testing.T) {
	// Combined ZZ + CR in same program.
	port0 := pulse.MustPort("d0", 1e-9)
	portCR := pulse.MustPort("dcr", 1e-9)
	f0 := pulse.MustFrame("f0", port0, 0, 0)
	fCR := pulse.MustFrame("cr01", portCR, 0, 0)

	T := 100e-9
	halfPiAmp := math.Pi / (2 * T)
	halfPi := waveform.MustConstant(complex(halfPiAmp, 0), T)

	prog := pulse.NewProgram("zz-cr",
		[]pulse.Port{port0, portCR},
		[]pulse.Frame{f0, fCR},
		[]pulse.Instruction{
			pulse.Play{Frame: f0, Waveform: halfPi},
			pulse.Play{Frame: fCR, Waveform: halfPi},
		},
		nil,
	)

	fm := FrameMap{"f0": 0, "cr01": 0}
	crFrames := CRFrameMap{"cr01": {0, 1}}
	cm := CouplingMap{orderedPair(0, 1): {ZZ: 1e5}}
	sim := New(2, fm, WithCoupling(cm), WithCRFrames(crFrames))
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	var norm float64
	for _, a := range sv {
		norm += prob(a)
	}
	if math.Abs(norm-1) > tol {
		t.Errorf("norm = %g, want 1", norm)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Existing 1Q New(n, fm) calls work unchanged.
	sim := New(1, FrameMap{"q0_drive": 0})
	if sim.numQubits != 1 {
		t.Errorf("numQubits = %d, want 1", sim.numQubits)
	}
	if sim.coupling != nil {
		t.Error("coupling should be nil without WithCoupling")
	}
	if sim.crFrames != nil {
		t.Error("crFrames should be nil without WithCRFrames")
	}
}

func TestCRFrameValidation(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	f := pulse.MustFrame("cr01", port, 0, 0)

	prog := pulse.NewProgram("bad-cr",
		[]pulse.Port{port},
		[]pulse.Frame{f},
		[]pulse.Instruction{
			pulse.Delay{Frame: f, Duration: 1e-8},
		},
		nil,
	)

	tests := []struct {
		name   string
		cr     CRFrameMap
		nq     int
		errSub string
	}{
		{
			"control out of range",
			CRFrameMap{"cr01": {5, 0}},
			2,
			"control qubit 5 out of range",
		},
		{
			"target out of range",
			CRFrameMap{"cr01": {0, 5}},
			2,
			"target qubit 5 out of range",
		},
		{
			"control equals target",
			CRFrameMap{"cr01": {0, 0}},
			2,
			"control == target",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := FrameMap{"cr01": 0}
			sim := New(tt.nq, fm, WithCRFrames(tt.cr))
			err := sim.Evolve(prog)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestGaussianPulse(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)

	// Large-amplitude Gaussian to ensure state change.
	wf := waveform.MustGaussian(math.Pi/50e-9, 100e-9, 20e-9)

	prog := pulse.NewProgram("gaussian",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{pulse.Play{Frame: frame, Waveform: wf}},
		nil,
	)

	sim := New(1, FrameMap{"q0_drive": 0})
	if err := sim.Evolve(prog); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()

	// After a Gaussian pulse, the state should differ from |0>.
	p0 := prob(sv[0])
	if math.Abs(p0-1) < tol {
		t.Error("Gaussian pulse had no effect on state")
	}

	// Verify unitarity.
	norm := prob(sv[0]) + prob(sv[1])
	if math.Abs(norm-1) > tol {
		t.Errorf("norm = %g, want 1", norm)
	}
}
