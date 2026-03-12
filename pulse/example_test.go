package pulse_test

import (
	"fmt"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

func ExampleBuilder() {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 5e9, 0)
	gauss := waveform.MustGaussian(1.0, 160e-9, 40e-9)

	prog, err := pulse.NewBuilder("x90").
		AddPort(port).
		AddFrame(frame).
		ShiftPhase(frame, 0).
		Play(frame, gauss).
		Capture(frame, 1e-6).
		Build()
	if err != nil {
		panic(err)
	}

	stats := prog.Stats()
	fmt.Printf("name=%s ports=%d frames=%d instructions=%d\n",
		prog.Name(), stats.NumPorts, stats.NumFrames, stats.NumInstructions)
	// Output: name=x90 ports=1 frames=1 instructions=3
}
