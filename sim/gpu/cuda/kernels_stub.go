//go:build !cuda

package cuda

import (
	"fmt"
	"unsafe"

	"github.com/splch/goqu/circuit/ir"
)

type cusvHandle struct{}

type deviceAlloc struct {
	ptr  unsafe.Pointer
	size int
}

var errNoCUDA = fmt.Errorf("cuda: not available (build with -tags cuda)")

func newSim(numQubits int) (*Sim, error) {
	return nil, errNoCUDA
}

func run(_ *Sim, _ *ir.Circuit, _ int) (map[string]int, error) {
	return nil, errNoCUDA
}

func evolve(_ *Sim, _ *ir.Circuit) error {
	return errNoCUDA
}

func stateVector(_ *Sim) []complex128 {
	return nil
}

func closeSim(_ *Sim) error {
	return nil
}
