package qasmparse

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

// FuzzParsePulse feeds arbitrary bytes to the parser and verifies it never panics.
func FuzzParsePulse(f *testing.F) {
	f.Add([]byte(`OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, constant(0.5+0i, 1e-07));
}
`))
	f.Add([]byte(`cal { frame f = newframe(p, 0, 0); delay[1e-8s] f; }`))
	f.Add([]byte(`garbage input that should not panic`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic.
		_, _ = ParseString(string(data))
	})
}

// FuzzRoundTripPulse builds random programs, serializes to QASM, parses back,
// and verifies structural equivalence.
func FuzzRoundTripPulse(f *testing.F) {
	f.Add(uint8(1), uint8(3), uint32(42))
	f.Add(uint8(2), uint8(8), uint32(0))
	f.Add(uint8(1), uint8(1), uint32(99))

	f.Fuzz(func(t *testing.T, nFrames uint8, nInstrs uint8, seed uint32) {
		nf := int(nFrames)
		if nf == 0 || nf > 4 {
			return
		}
		ni := int(nInstrs)
		if ni == 0 || ni > 15 {
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))

		// Create ports and frames.
		ports := make([]pulse.Port, nf)
		frames := make([]pulse.Frame, nf)
		for i := range nf {
			ports[i] = pulse.MustPort(fmt.Sprintf("d%d", i), 1e-9)
			frames[i] = pulse.MustFrame(
				fmt.Sprintf("f%d", i),
				ports[i],
				float64(rng.Intn(10))*1e9,
				0,
			)
		}

		b := pulse.NewBuilder("fuzz_rt")
		for _, p := range ports {
			b.AddPort(p)
		}
		for _, fr := range frames {
			b.AddFrame(fr)
		}

		wf := waveform.MustConstant(complex(0.1, 0), 10e-9)

		for range ni {
			fr := frames[rng.Intn(nf)]
			switch rng.Intn(7) {
			case 0:
				b.Play(fr, wf)
			case 1:
				b.Delay(fr, 1e-8+rng.Float64()*1e-7)
			case 2:
				b.SetPhase(fr, rng.Float64()*6.28)
			case 3:
				b.ShiftPhase(fr, rng.Float64()*3.14)
			case 4:
				b.SetFrequency(fr, float64(rng.Intn(10))*1e9)
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

		qasmSource := buildRTSource(prog)
		parsed, err := ParseString(qasmSource)
		if err != nil {
			t.Fatalf("round-trip parse failed: %v\nSource:\n%s", err, qasmSource)
		}

		if len(parsed.Ports()) != len(prog.Ports()) {
			t.Errorf("ports: %d != %d", len(parsed.Ports()), len(prog.Ports()))
		}
		if len(parsed.Frames()) != len(prog.Frames()) {
			t.Errorf("frames: %d != %d", len(parsed.Frames()), len(prog.Frames()))
		}
		if len(parsed.Instructions()) != len(prog.Instructions()) {
			t.Errorf("instrs: %d != %d", len(parsed.Instructions()), len(prog.Instructions()))
		}

		// Verify instruction-level values.
		origInstrs := prog.Instructions()
		parsedInstrs := parsed.Instructions()
		for i := range origInstrs {
			if i >= len(parsedInstrs) {
				break
			}
			switch ov := origInstrs[i].(type) {
			case pulse.Delay:
				pv, ok := parsedInstrs[i].(pulse.Delay)
				if !ok {
					t.Errorf("[%d] type mismatch", i)
					continue
				}
				if rd(ov.Duration, pv.Duration) > 1e-6 {
					t.Errorf("delay[%d]: %g != %g", i, pv.Duration, ov.Duration)
				}
			case pulse.SetPhase:
				pv, ok := parsedInstrs[i].(pulse.SetPhase)
				if !ok {
					t.Errorf("[%d] type mismatch", i)
					continue
				}
				if math.Abs(ov.Phase-pv.Phase) > 1e-3 {
					t.Errorf("set_phase[%d]: %g != %g", i, pv.Phase, ov.Phase)
				}
			}
		}
	})
}

func rd(a, b float64) float64 {
	d := math.Abs(a - b)
	m := math.Max(math.Abs(a), math.Abs(b))
	if m == 0 {
		return d
	}
	return d / m
}

func buildRTSource(p *pulse.Program) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	for _, port := range p.Ports() {
		fmt.Fprintf(&sb, "extern port %s;\n", port.Name())
	}
	sb.WriteString("cal {\n")
	for _, f := range p.Frames() {
		fmt.Fprintf(&sb, "  frame %s = newframe(%s, %g, %g);\n",
			f.Name(), f.Port().Name(), f.Frequency(), f.Phase())
	}
	for _, inst := range p.Instructions() {
		switch v := inst.(type) {
		case pulse.Play:
			fmt.Fprintf(&sb, "  play(%s, %s);\n", v.Frame.Name(), v.Waveform.Name())
		case pulse.Delay:
			fmt.Fprintf(&sb, "  delay[%gs] %s;\n", v.Duration, v.Frame.Name())
		case pulse.SetPhase:
			fmt.Fprintf(&sb, "  set_phase(%s, %g);\n", v.Frame.Name(), v.Phase)
		case pulse.ShiftPhase:
			fmt.Fprintf(&sb, "  shift_phase(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.SetFrequency:
			fmt.Fprintf(&sb, "  set_frequency(%s, %g);\n", v.Frame.Name(), v.Frequency)
		case pulse.ShiftFrequency:
			fmt.Fprintf(&sb, "  shift_frequency(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.Barrier:
			names := make([]string, len(v.Frames))
			for i, f := range v.Frames {
				names[i] = f.Name()
			}
			fmt.Fprintf(&sb, "  barrier %s;\n", strings.Join(names, ", "))
		case pulse.Capture:
			fmt.Fprintf(&sb, "  capture_v0(%s, %gs);\n", v.Frame.Name(), v.Duration)
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}
