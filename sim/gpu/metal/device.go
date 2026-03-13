//go:build !metal

package metal

import (
	"fmt"

	"github.com/splch/goqu/circuit/ir"
)

type metalDevice struct{}

var errNoMetal = fmt.Errorf("metal: not available (build with -tags metal)")

func newSim(_ int) (*Sim, error) {
	return nil, errNoMetal
}

func run(_ *Sim, _ *ir.Circuit, _ int) (map[string]int, error) {
	return nil, errNoMetal
}

func evolve(_ *Sim, _ *ir.Circuit) error {
	return errNoMetal
}

func stateVector(_ *Sim) []complex128 {
	return nil
}

func closeSim(_ *Sim) error {
	return nil
}
