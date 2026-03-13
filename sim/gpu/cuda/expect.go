//go:build cuda

package cuda

/*
#include <custatevec.h>

// goPauliExpect computes ⟨ψ|P|ψ⟩ for a Pauli string on the GPU.
static custatevecStatus_t goPauliExpect(
    custatevecHandle_t handle,
    const void *sv, cudaDataType_t svDataType, int nQubits,
    double *expectationValue,
    const custatevecPauli_t *pauliOps, const int32_t *basisBits, int nBasisBits
) {
    return custatevecComputeExpectation(
        handle, sv, svDataType, (uint32_t)nQubits,
        expectationValue, CUDA_R_64F,
        pauliOps, basisBits, (uint32_t)nBasisBits
    );
}
*/
import "C"
import (
	"fmt"

	"github.com/splch/goqu/sim/pauli"
)

// pauliToCUSV maps goqu Pauli values to cuStateVec Pauli enum values.
func pauliToCUSV(p pauli.Pauli) C.custatevecPauli_t {
	switch p {
	case pauli.I:
		return C.CUSTATEVEC_PAULI_I
	case pauli.X:
		return C.CUSTATEVEC_PAULI_X
	case pauli.Y:
		return C.CUSTATEVEC_PAULI_Y
	case pauli.Z:
		return C.CUSTATEVEC_PAULI_Z
	default:
		return C.CUSTATEVEC_PAULI_I
	}
}

// ExpectPauliString computes Re(⟨ψ|P|ψ⟩) for a Pauli string P on the GPU.
func (s *Sim) ExpectPauliString(ps pauli.PauliString) (float64, error) {
	if ps.NumQubits() != s.numQubits {
		return 0, fmt.Errorf("cuda: PauliString has %d qubits, simulator has %d",
			ps.NumQubits(), s.numQubits)
	}

	ops := ps.Ops()
	nBasis := len(ops)
	if nBasis == 0 {
		return real(ps.Coeff()), nil
	}

	pauliOps := make([]C.custatevecPauli_t, nBasis)
	basisBits := make([]C.int32_t, nBasis)
	i := 0
	for qubit, p := range ops {
		pauliOps[i] = pauliToCUSV(p)
		basisBits[i] = C.int32_t(qubit)
		i++
	}

	var expect C.double
	st := C.goPauliExpect(
		s.handle.h,
		s.devicePtr.ptr, C.CUDA_C_64F, C.int(s.numQubits),
		&expect,
		&pauliOps[0], &basisBits[0], C.int(nBasis),
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return 0, fmt.Errorf("custatevecComputeExpectation failed: status %d", int(st))
	}

	return real(ps.Coeff()) * float64(expect), nil
}
