package pulsesim

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

// FuzzEvolve builds random 1-2 qubit pulse programs, evolves them, and
// verifies that the simulator never panics and the statevector retains unit norm.
func FuzzEvolve(f *testing.F) {
	f.Add(uint8(1), uint8(5), uint32(42))
	f.Add(uint8(2), uint8(10), uint32(0))
	f.Add(uint8(1), uint8(3), uint32(123))
	f.Add(uint8(2), uint8(15), uint32(9999))

	f.Fuzz(func(t *testing.T, nQubits uint8, nInstrs uint8, seed uint32) {
		if nQubits == 0 || nQubits > 2 {
			return
		}
		if nInstrs == 0 || nInstrs > 20 {
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))
		nq := int(nQubits)

		// Create ports and frames.
		ports := make([]pulse.Port, nq)
		frames := make([]pulse.Frame, nq)
		fm := make(FrameMap, nq)
		for i := range nq {
			ports[i] = pulse.MustPort(fmt.Sprintf("d%d", i), 1e-9)
			frames[i] = pulse.MustFrame(
				fmt.Sprintf("q%d_drive", i),
				ports[i],
				0, // freq=0 for clean rotations
				0,
			)
			fm[fmt.Sprintf("q%d_drive", i)] = i
		}

		b := pulse.NewBuilder("fuzz")
		for _, p := range ports {
			b.AddPort(p)
		}
		for _, fr := range frames {
			b.AddFrame(fr)
		}

		// Use small amplitudes to keep rotations bounded.
		wf := waveform.MustConstant(complex(0.1, 0), 10e-9)

		for range int(nInstrs) {
			fr := frames[rng.Intn(nq)]
			switch rng.Intn(7) {
			case 0:
				b.Play(fr, wf)
			case 1:
				b.Delay(fr, 1e-8+rng.Float64()*1e-7)
			case 2:
				b.SetPhase(fr, rng.Float64()*2*math.Pi)
			case 3:
				b.ShiftPhase(fr, rng.Float64()*math.Pi)
			case 4:
				b.SetFrequency(fr, rng.Float64()*1e9)
			case 5:
				b.Barrier(fr)
			case 6:
				b.Capture(fr, 1e-7)
			}
		}

		prog, err := b.Build()
		if err != nil {
			return
		}

		sim := New(nq, fm)
		if err := sim.Evolve(prog); err != nil {
			return // frame mapping errors are fine
		}

		sv := sim.StateVector()
		var norm float64
		for _, a := range sv {
			norm += real(a)*real(a) + imag(a)*imag(a)
		}
		if math.Abs(norm-1) > 1e-10 {
			t.Errorf("statevector norm = %g, want 1", norm)
		}
	})
}

// FuzzEvolve2Q builds random 2-3 qubit pulse programs with optional coupling/CR
// and verifies norm conservation.
func FuzzEvolve2Q(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(42), true, true)
	f.Add(uint8(3), uint8(10), uint32(0), false, true)
	f.Add(uint8(2), uint8(8), uint32(123), true, false)
	f.Add(uint8(3), uint8(3), uint32(9999), false, false)

	f.Fuzz(func(t *testing.T, nQubits uint8, nInstrs uint8, seed uint32, useCoupling, useCR bool) {
		nq := int(nQubits)
		if nq < 2 || nq > 3 {
			return
		}
		ni := int(nInstrs)
		if ni == 0 || ni > 20 {
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))

		// Create drive frames.
		ports := make([]pulse.Port, nq)
		frames := make([]pulse.Frame, nq)
		fm := make(FrameMap, nq)
		for i := range nq {
			ports[i] = pulse.MustPort(fmt.Sprintf("d%d", i), 1e-9)
			frames[i] = pulse.MustFrame(
				fmt.Sprintf("q%d_drive", i), ports[i], 0, 0,
			)
			fm[fmt.Sprintf("q%d_drive", i)] = i
		}

		var opts []Option

		// Optionally add coupling.
		if useCoupling {
			cm := CouplingMap{orderedPair(0, 1): {ZZ: rng.Float64() * 1e6}}
			opts = append(opts, WithCoupling(cm))
		}

		// Optionally add CR frame.
		var crFrame pulse.Frame
		if useCR {
			crPort := pulse.MustPort("dcr", 1e-9)
			crFrame = pulse.MustFrame("cr01", crPort, 0, 0)
			ports = append(ports, crPort)
			frames = append(frames, crFrame)
			fm["cr01"] = 0 // mapped to qubit 0 in FrameMap
			crFrames := CRFrameMap{"cr01": {0, 1}}
			opts = append(opts, WithCRFrames(crFrames))
		}

		b := pulse.NewBuilder("fuzz2q")
		for _, p := range ports {
			b.AddPort(p)
		}
		for _, fr := range frames {
			b.AddFrame(fr)
		}

		wf := waveform.MustConstant(complex(0.05, 0), 5e-9)

		for range ni {
			if useCR && rng.Intn(4) == 0 {
				b.Play(crFrame, wf)
				continue
			}
			fr := frames[rng.Intn(nq)]
			switch rng.Intn(6) {
			case 0:
				b.Play(fr, wf)
			case 1:
				b.Delay(fr, 1e-8+rng.Float64()*1e-7)
			case 2:
				b.SetPhase(fr, rng.Float64()*2*math.Pi)
			case 3:
				b.ShiftPhase(fr, rng.Float64()*math.Pi)
			case 4:
				b.Barrier(fr)
			case 5:
				b.Capture(fr, 1e-7)
			}
		}

		prog, err := b.Build()
		if err != nil {
			return
		}

		sim := New(nq, fm, opts...)
		if err := sim.Evolve(prog); err != nil {
			return
		}

		sv := sim.StateVector()
		var norm float64
		for _, a := range sv {
			norm += real(a)*real(a) + imag(a)*imag(a)
		}
		if math.Abs(norm-1) > 1e-8 {
			t.Errorf("statevector norm = %g, want 1 (nq=%d, coupling=%v, cr=%v)",
				norm, nq, useCoupling, useCR)
		}
	})
}
