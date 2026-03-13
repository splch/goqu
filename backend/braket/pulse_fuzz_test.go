package braket

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

func buildRandomPulseProgram(nPorts, nFrames, nInstrs int, seed uint32) (*pulse.Program, error) {
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

// FuzzSerializePulseProgram builds random pulse programs and serializes them.
// Verifies no panics and valid JSON output with the Braket schema header.
func FuzzSerializePulseProgram(f *testing.F) {
	f.Add(uint8(1), uint8(1), uint8(5), uint32(42))
	f.Add(uint8(2), uint8(3), uint8(10), uint32(0))
	f.Add(uint8(1), uint8(2), uint8(8), uint32(123))

	f.Fuzz(func(t *testing.T, nPorts uint8, nFrames uint8, nInstrs uint8, seed uint32) {
		if nPorts == 0 || nPorts > 4 {
			return
		}
		if nFrames == 0 || nFrames > 4 {
			return
		}
		if nInstrs == 0 || nInstrs > 20 {
			return
		}

		prog, err := buildRandomPulseProgram(int(nPorts), int(nFrames), int(nInstrs), seed)
		if err != nil {
			return
		}

		action, err := serializePulseProgram(prog)
		if err != nil {
			return
		}

		// Must be valid JSON.
		var parsed braketProgram
		if err := json.Unmarshal([]byte(action), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if parsed.Header.Name != "braket.ir.openqasm.program" {
			t.Errorf("schema = %q, want braket.ir.openqasm.program", parsed.Header.Name)
		}
		if !strings.HasPrefix(parsed.Source, "OPENQASM 3.0;") {
			t.Error("source should start with OPENQASM 3.0;")
		}
	})
}
