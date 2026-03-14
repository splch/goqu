//go:build cuda

package cuda

/*
#include <custatevec.h>

// goAbs2Sum computes probabilities on Z-basis.
static custatevecStatus_t goAbs2Sum(
    custatevecHandle_t handle,
    const void *sv, cudaDataType_t svDataType, int nQubits,
    double *abs2sum0, double *abs2sum1,
    const int32_t *bitOrdering, int bitOrderingLen
) {
    return custatevecAbs2SumOnZBasis(
        handle, sv, svDataType, (uint32_t)nQubits,
        abs2sum0, abs2sum1,
        bitOrdering, (uint32_t)bitOrderingLen
    );
}

// goSamplerCreate creates a batched sampler.
static custatevecStatus_t goSamplerCreate(
    custatevecHandle_t handle,
    const void *sv, cudaDataType_t svDataType, int nQubits,
    custatevecSamplerDescriptor_t *sampler,
    int nMaxShots,
    void *workspace, size_t workspaceSize
) {
    return custatevecSamplerCreate(
        handle, sv, svDataType, (uint32_t)nQubits,
        sampler, (uint32_t)nMaxShots,
        workspace, workspaceSize
    );
}

// goSamplerGetWorkspaceSize gets required workspace.
static custatevecStatus_t goSamplerGetWorkspaceSize(
    custatevecHandle_t handle,
    cudaDataType_t svDataType, int nQubits,
    int nMaxShots,
    size_t *workspaceSize
) {
    return custatevecSamplerPreprocess(
        handle, NULL, svDataType, (uint32_t)nQubits,
        NULL, (uint32_t)nMaxShots,
        NULL, workspaceSize
    );
}

// goSamplerSample draws samples.
static custatevecStatus_t goSamplerSample(
    custatevecSamplerDescriptor_t sampler,
    custatevecIndex_t *bitStrings,
    const int32_t *bitOrdering, int bitOrderingLen,
    const double *randnums, int nShots,
    enum custatevecSamplerOutput_t output
) {
    return custatevecSamplerSample(
        sampler, bitStrings,
        bitOrdering, (uint32_t)bitOrderingLen,
        randnums, (uint32_t)nShots, output
    );
}

// goSamplerDestroy destroys the sampler.
static custatevecStatus_t goSamplerDestroy(custatevecSamplerDescriptor_t sampler) {
    return custatevecSamplerDestroy(sampler);
}

// goSamplerPreprocess preprocesses the statevector for sampling.
static custatevecStatus_t goSamplerPreprocess(
    custatevecHandle_t handle,
    custatevecSamplerDescriptor_t sampler,
    cudaDataType_t svDataType, int nQubits,
    void *workspace, size_t workspaceSize
) {
    size_t ws = workspaceSize;
    return custatevecSamplerPreprocess(
        handle, sampler, svDataType, (uint32_t)nQubits,
        workspace, 0,
        workspace, &ws
    );
}
*/
import "C"
import (
	"fmt"
	"math/rand/v2"
	"unsafe"

	"github.com/splch/goqu/circuit/ir"
)

// run executes the circuit and samples measurement results on the GPU.
func run(s *Sim, c *ir.Circuit, shots int) (map[string]int, error) {
	if c.IsDynamic() {
		return nil, fmt.Errorf("cuda: dynamic circuits not supported")
	}
	if err := evolve(s, c); err != nil {
		return nil, err
	}
	if shots <= 0 {
		return make(map[string]int), nil
	}

	// Build bit ordering: [0, 1, ..., nQubits-1].
	bitOrdering := make([]C.int32_t, s.numQubits)
	for i := range s.numQubits {
		bitOrdering[i] = C.int32_t(i)
	}

	// Create sampler.
	var sampler C.custatevecSamplerDescriptor_t
	var wsSize C.size_t

	// Get workspace size.
	st := C.custatevecSamplerPreprocess(
		s.handle.h, nil, C.CUDA_C_64F, C.uint32_t(s.numQubits),
		nil, C.uint32_t(shots),
		nil, &wsSize,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return nil, fmt.Errorf("custatevecSamplerPreprocess (size) failed: status %d", int(st))
	}

	// Allocate workspace.
	var wsPtr unsafe.Pointer
	if wsSize > 0 {
		if cst := C.cudaMalloc(&wsPtr, wsSize); cst != C.cudaSuccess {
			return nil, fmt.Errorf("cudaMalloc sampler workspace failed: status %d", int(cst))
		}
		defer C.cudaFree(wsPtr)
	}

	// Create the sampler.
	st = C.goSamplerCreate(
		s.handle.h,
		s.devicePtr.ptr, C.CUDA_C_64F, C.int(s.numQubits),
		&sampler, C.int(shots),
		wsPtr, wsSize,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return nil, fmt.Errorf("custatevecSamplerCreate failed: status %d", int(st))
	}
	defer C.goSamplerDestroy(sampler)

	// Preprocess.
	st = C.goSamplerPreprocess(
		s.handle.h, sampler,
		C.CUDA_C_64F, C.int(s.numQubits),
		wsPtr, wsSize,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return nil, fmt.Errorf("custatevecSamplerPreprocess failed: status %d", int(st))
	}

	// Generate random numbers.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	randNums := make([]C.double, shots)
	for i := range shots {
		randNums[i] = C.double(rng.Float64())
	}

	// Sample.
	bitStrings := make([]C.custatevecIndex_t, shots)
	st = C.goSamplerSample(
		sampler,
		&bitStrings[0],
		&bitOrdering[0], C.int(s.numQubits),
		&randNums[0], C.int(shots),
		C.CUSTATEVEC_SAMPLER_OUTPUT_RANDNUM_ORDER,
	)
	if st != C.CUSTATEVEC_STATUS_SUCCESS {
		return nil, fmt.Errorf("custatevecSamplerSample failed: status %d", int(st))
	}

	// Convert to bitstring counts.
	counts := make(map[string]int)
	for _, idx := range bitStrings {
		bs := formatBitstring(int(idx), s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

func formatBitstring(idx, n int) string {
	bs := make([]byte, n)
	for i := range n {
		if idx&(1<<i) != 0 {
			bs[n-1-i] = '1'
		} else {
			bs[n-1-i] = '0'
		}
	}
	return string(bs)
}
