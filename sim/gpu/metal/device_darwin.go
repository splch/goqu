//go:build darwin

package metal

/*
#cgo CFLAGS: -fno-objc-arc
#cgo LDFLAGS: -framework Metal -framework Foundation
#include "metal_bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type metalDevice struct {
	sim *C.MetalSim
}

func newSim(numQubits int) (*Sim, error) {
	var errStr *C.char
	msim := C.MetalCreate(C.int(numQubits), &errStr)
	if msim == nil {
		msg := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return nil, fmt.Errorf("metal: %s", msg)
	}
	return &Sim{
		numQubits: numQubits,
		device:    metalDevice{sim: msim},
	}, nil
}

func closeSim(s *Sim) error {
	if s.device.sim != nil {
		C.MetalDestroy(s.device.sim)
		s.device.sim = nil
	}
	return nil
}

// stateVector returns a copy of the GPU state as []complex128 (converting float32→float64).
func stateVector(s *Sim) []complex128 {
	nAmps := 1 << s.numQubits
	out := make([]complex128, nAmps)
	ptr := C.MetalStatePtr(s.device.sim)
	src := unsafe.Slice((*float32)(unsafe.Pointer(ptr)), nAmps*2)
	for i := range nAmps {
		out[i] = complex(float64(src[2*i]), float64(src[2*i+1]))
	}
	return out
}

// stateVectorF32 returns a slice of float32 pairs backed by the shared Metal buffer.
func stateVectorF32(s *Sim) []float32 {
	nAmps := 1 << s.numQubits
	ptr := C.MetalStatePtr(s.device.sim)
	return unsafe.Slice((*float32)(unsafe.Pointer(ptr)), nAmps*2)
}

// Go wrappers for C bridge functions

func metalResetState(s *Sim) {
	C.MetalResetState(s.device.sim)
}

func metalBeginPass(s *Sim) error {
	var errStr *C.char
	if C.MetalBeginPass(s.device.sim, &errStr) != 0 {
		msg := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}

func metalEndPass(s *Sim) error {
	var errStr *C.char
	if C.MetalEndPass(s.device.sim, &errStr) != 0 {
		msg := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}

// metalGate1Q dispatches a 1Q gate. m is a 2x2 complex128 matrix (4 elements).
func metalGate1Q(s *Sim, qubit int, m []complex128) error {
	var fm [8]float32
	for i, c := range m {
		fm[2*i] = float32(real(c))
		fm[2*i+1] = float32(imag(c))
	}
	var errStr *C.char
	if C.MetalGate1Q(s.device.sim, C.uint32_t(qubit), (*C.float)(&fm[0]), &errStr) != 0 {
		msg := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}

// metalGate2Q dispatches a 2Q gate. m is a 4x4 complex128 matrix (16 elements).
func metalGate2Q(s *Sim, q0, q1 int, m []complex128) error {
	var fm [32]float32
	for i, c := range m {
		fm[2*i] = float32(real(c))
		fm[2*i+1] = float32(imag(c))
	}
	var errStr *C.char
	if C.MetalGate2Q(s.device.sim, C.uint32_t(q0), C.uint32_t(q1), (*C.float)(&fm[0]), &errStr) != 0 {
		msg := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}

// writeStateF32 writes amplitudes to the shared buffer (converting float64→float32).
func writeStateF32(s *Sim, amps []complex128) {
	buf := stateVectorF32(s)
	for i, a := range amps {
		buf[2*i] = float32(real(a))
		buf[2*i+1] = float32(imag(a))
	}
}
