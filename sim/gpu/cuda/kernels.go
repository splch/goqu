//go:build cuda

package cuda

/*
#include <custatevec.h>

// applyMatrix wraps custatevecApplyMatrix for gate application.
static custatevecStatus_t goApplyMatrix(
    custatevecHandle_t handle,
    void *sv, cudaDataType_t svDataType, int nQubits,
    const void *matrix, cudaDataType_t matrixDataType,
    custatevecMatrixLayout_t layout,
    int adjoint,
    const int32_t *targets, int nTargets,
    const int32_t *controls, const int32_t *controlBitValues, int nControls,
    custatevecComputeType_t computeType,
    void *workspace, size_t workspaceSize
) {
    return custatevecApplyMatrix(
        handle, sv, svDataType, (uint32_t)nQubits,
        matrix, matrixDataType, layout, (int32_t)adjoint,
        targets, (uint32_t)nTargets,
        controls, controlBitValues, (uint32_t)nControls,
        computeType, workspace, workspaceSize
    );
}

// getApplyMatrixWorkspaceSize wraps custatevecApplyMatrixGetWorkspaceSize.
static custatevecStatus_t goGetWorkspaceSize(
    custatevecHandle_t handle,
    cudaDataType_t svDataType, int nQubits,
    const void *matrix, cudaDataType_t matrixDataType,
    custatevecMatrixLayout_t layout,
    int adjoint,
    int nTargets, int nControls,
    custatevecComputeType_t computeType,
    size_t *workspaceSize
) {
    return custatevecApplyMatrixGetWorkspaceSize(
        handle, svDataType, (uint32_t)nQubits,
        matrix, matrixDataType, layout, (int32_t)adjoint,
        (uint32_t)nTargets, (uint32_t)nControls,
        computeType, workspaceSize
    );
}
*/
import "C"
import (
	"fmt"
	"math"
	"unsafe"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// applyGate sends a gate matrix to the GPU and applies it via custatevecApplyMatrix.
func applyGate(s *Sim, targets []int, controls []int, m []complex128) error {
	nTargets := len(targets)
	nControls := len(controls)

	// Convert to int32 slices for C.
	tgts := make([]C.int32_t, nTargets)
	for i, t := range targets {
		tgts[i] = C.int32_t(t)
	}

	var ctrlPtr *C.int32_t
	var ctrlBitsPtr *C.int32_t
	var ctrlBits []C.int32_t
	if nControls > 0 {
		ctrls := make([]C.int32_t, nControls)
		ctrlBits = make([]C.int32_t, nControls)
		for i, c := range controls {
			ctrls[i] = C.int32_t(c)
			ctrlBits[i] = 1 // control on |1>
		}
		ctrlPtr = &ctrls[0]
		ctrlBitsPtr = &ctrlBits[0]
	}

	// Query workspace size.
	var wsSize C.size_t
	st := C.goGetWorkspaceSize(
		s.handle.h,
		C.CUDA_C_64F, C.int(s.numQubits),
		unsafe.Pointer(&m[0]), C.CUDA_C_64F,
		C.CUSTATEVEC_MATRIX_LAYOUT_ROW,
		0,
		C.int(nTargets), C.int(nControls),
		C.CUSTATEVEC_COMPUTE_64F,
		&wsSize,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return fmt.Errorf("custatevecApplyMatrixGetWorkspaceSize failed: status %d", int(st))
	}

	// Allocate workspace if needed.
	var wsPtr unsafe.Pointer
	if wsSize > 0 {
		if cst := C.cudaMalloc(&wsPtr, wsSize); cst != C.cudaSuccess {
			return fmt.Errorf("cudaMalloc workspace failed: status %d", int(cst))
		}
		defer C.cudaFree(wsPtr)
	}

	// Apply the gate.
	st = C.goApplyMatrix(
		s.handle.h,
		s.devicePtr.ptr, C.CUDA_C_64F, C.int(s.numQubits),
		unsafe.Pointer(&m[0]), C.CUDA_C_64F,
		C.CUSTATEVEC_MATRIX_LAYOUT_ROW,
		0,
		&tgts[0], C.int(nTargets),
		ctrlPtr, ctrlBitsPtr, C.int(nControls),
		C.CUSTATEVEC_COMPUTE_64F,
		wsPtr, wsSize,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return fmt.Errorf("custatevecApplyMatrix failed: status %d", int(st))
	}
	return nil
}

// evolve applies all gate operations in the circuit on the GPU.
func evolve(s *Sim, c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	if err := resetDevice(s.devicePtr); err != nil {
		return err
	}

	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		if op.Gate.Name() == "reset" {
			// Transfer to host, reset qubit, transfer back.
			host := copyToHost(s.devicePtr)
			resetQubitCPU(host, s.numQubits, op.Qubits[0])
			C.goMemcpyH2D(s.devicePtr.ptr, unsafe.Pointer(&host[0]), C.size_t(len(host)*16))
			continue
		}
		if _, ok := op.Gate.(gate.StatePrepable); ok {
			// State prep: copy amplitudes directly to GPU.
			sp := op.Gate.(gate.StatePrepable)
			amps := sp.Amplitudes()
			if len(op.Qubits) == s.numQubits {
				allInOrder := true
				for i, q := range op.Qubits {
					if q != i {
						allInOrder = false
						break
					}
				}
				if allInOrder {
					// Fast path: copy amplitudes directly.
					C.goMemcpyH2D(s.devicePtr.ptr, unsafe.Pointer(&amps[0]), C.size_t(len(amps)*16))
					continue
				}
			}
			// Slow path: decompose into 1Q/2Q gates.
			applied := op.Gate.Decompose(op.Qubits)
			for _, a := range applied {
				m := a.Gate.Matrix()
				if m == nil {
					continue
				}
				if err := applyGate(s, a.Qubits, nil, m); err != nil {
					return err
				}
			}
			continue
		}

		m := op.Gate.Matrix()
		if m == nil {
			continue
		}

		if cg, ok := op.Gate.(gate.ControlledGate); ok {
			nControls := cg.NumControls()
			controls := op.Qubits[:nControls]
			targets := op.Qubits[nControls:]
			innerM := cg.Inner().Matrix()
			if innerM != nil {
				if err := applyGate(s, targets, controls, innerM); err != nil {
					return err
				}
				continue
			}
		}

		// General gate: all qubits are targets.
		if err := applyGate(s, op.Qubits, nil, m); err != nil {
			return err
		}
	}
	return nil
}

// resetQubitCPU deterministically resets a qubit to |0⟩ on the host side.
func resetQubitCPU(state []complex128, numQubits, qubit int) {
	halfBlock := 1 << qubit
	block := halfBlock << 1
	nAmps := 1 << numQubits
	for b0 := 0; b0 < nAmps; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset
			i1 := i0 + halfBlock
			a0, a1 := state[i0], state[i1]
			norm := math.Sqrt(real(a0)*real(a0) + imag(a0)*imag(a0) +
				real(a1)*real(a1) + imag(a1)*imag(a1))
			if norm > 1e-15 {
				state[i0] = complex(norm, 0)
			} else {
				state[i0] = 0
			}
			state[i1] = 0
		}
	}
}

// newSim creates a GPU simulator with cuStateVec.
func newSim(numQubits int) (*Sim, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("cuda: numQubits must be >= 1, got %d", numQubits)
	}
	h, err := createHandle()
	if err != nil {
		return nil, err
	}
	d, err := allocDevice(numQubits)
	if err != nil {
		destroyHandle(h)
		return nil, err
	}
	return &Sim{numQubits: numQubits, handle: h, devicePtr: d}, nil
}

// stateVector copies the full state from GPU to host.
func stateVector(s *Sim) []complex128 {
	return copyToHost(s.devicePtr)
}

// closeSim frees GPU resources.
func closeSim(s *Sim) error {
	freeDevice(&s.devicePtr)
	destroyHandle(s.handle)
	return nil
}
