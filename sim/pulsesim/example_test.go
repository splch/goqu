package pulsesim_test

import (
	"fmt"
	"math"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
	"github.com/splch/qgo/sim/pulsesim"
)

func ExampleSim_Evolve() {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 0, 0)

	// Constant waveform calibrated for a pi rotation.
	T := 100e-9
	amp := math.Pi / T
	wf := waveform.MustConstant(complex(amp, 0), T)

	prog := pulse.NewProgram("pi-pulse",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{pulse.Play{Frame: frame, Waveform: wf}},
		nil,
	)

	sim := pulsesim.New(1, pulsesim.FrameMap{"q0_drive": 0})
	if err := sim.Evolve(prog); err != nil {
		panic(err)
	}

	sv := sim.StateVector()
	for i, a := range sv {
		p := real(a)*real(a) + imag(a)*imag(a)
		fmt.Printf("|%d> probability: %.1f\n", i, p)
	}
	// Output:
	// |0> probability: 0.0
	// |1> probability: 1.0
}
