package pulse_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

func buildRandomProgram(nPorts, nFrames, nInstrs int, seed uint32) (*pulse.Program, error) {
	rng := rand.New(rand.NewSource(int64(seed)))

	if nPorts == 0 {
		nPorts = 1
	}
	if nFrames == 0 {
		nFrames = 1
	}

	ports := make([]pulse.Port, nPorts)
	for i := range ports {
		ports[i] = pulse.MustPort(fmt.Sprintf("d%d", i), 1e-9)
	}

	frames := make([]pulse.Frame, nFrames)
	for i := range frames {
		portIdx := rng.Intn(nPorts)
		frames[i] = pulse.MustFrame(
			fmt.Sprintf("f%d", i),
			ports[portIdx],
			float64(rng.Intn(10))*1e9,
			rng.Float64()*2*math.Pi,
		)
	}

	b := pulse.NewBuilder("fuzz")
	for _, p := range ports {
		b.AddPort(p)
	}
	for _, f := range frames {
		b.AddFrame(f)
	}

	wf := waveform.MustConstant(complex(0.5, 0), 1e-7)

	for range nInstrs {
		f := frames[rng.Intn(nFrames)]
		switch rng.Intn(8) {
		case 0:
			b.Play(f, wf)
		case 1:
			b.Delay(f, 1e-8+rng.Float64()*1e-7)
		case 2:
			b.SetPhase(f, rng.Float64()*2*math.Pi)
		case 3:
			b.ShiftPhase(f, rng.Float64()*2*math.Pi-math.Pi)
		case 4:
			b.SetFrequency(f, rng.Float64()*10e9)
		case 5:
			b.ShiftFrequency(f, rng.Float64()*1e9-0.5e9)
		case 6:
			b.Barrier(f)
		case 7:
			b.Capture(f, 1e-7+rng.Float64()*1e-6)
		}
	}

	return b.Build()
}

// FuzzBuildProgram builds random pulse programs and verifies they don't panic.
func FuzzBuildProgram(f *testing.F) {
	f.Add(uint8(1), uint8(1), uint8(5), uint32(42))
	f.Add(uint8(2), uint8(3), uint8(10), uint32(0))
	f.Add(uint8(4), uint8(4), uint8(20), uint32(123))
	f.Add(uint8(1), uint8(2), uint8(1), uint32(99))

	f.Fuzz(func(t *testing.T, nPorts uint8, nFrames uint8, nInstrs uint8, seed uint32) {
		if nPorts == 0 || nPorts > 8 {
			return
		}
		if nFrames == 0 || nFrames > 8 {
			return
		}
		if nInstrs == 0 || nInstrs > 30 {
			return
		}

		prog, err := buildRandomProgram(int(nPorts), int(nFrames), int(nInstrs), seed)
		if err != nil {
			return
		}

		stats := prog.Stats()
		if stats.NumInstructions == 0 {
			t.Error("built program has 0 instructions")
		}
	})
}

// FuzzProgramStats verifies that TotalDuration equals the manual sum of
// Play/Delay/Capture durations.
func FuzzProgramStats(f *testing.F) {
	f.Add(uint8(1), uint8(2), uint8(8), uint32(42))
	f.Add(uint8(2), uint8(3), uint8(15), uint32(0))
	f.Add(uint8(1), uint8(1), uint8(3), uint32(1))

	f.Fuzz(func(t *testing.T, nPorts uint8, nFrames uint8, nInstrs uint8, seed uint32) {
		if nPorts == 0 || nPorts > 8 {
			return
		}
		if nFrames == 0 || nFrames > 8 {
			return
		}
		if nInstrs == 0 || nInstrs > 30 {
			return
		}

		prog, err := buildRandomProgram(int(nPorts), int(nFrames), int(nInstrs), seed)
		if err != nil {
			return
		}

		stats := prog.Stats()
		if stats.TotalDuration < 0 {
			t.Errorf("TotalDuration = %g, want >= 0", stats.TotalDuration)
		}

		// Manual sum.
		var manual float64
		for _, inst := range prog.Instructions() {
			switch v := inst.(type) {
			case pulse.Play:
				manual += v.Waveform.Duration()
			case pulse.Delay:
				manual += v.Duration
			case pulse.Capture:
				manual += v.Duration
			}
		}

		if math.Abs(stats.TotalDuration-manual) > 1e-15 {
			t.Errorf("TotalDuration = %g, manual sum = %g", stats.TotalDuration, manual)
		}
	})
}
