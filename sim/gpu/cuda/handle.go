//go:build cuda

package cuda

/*
#cgo LDFLAGS: -lcustatevec -lcudart
#include <custatevec.h>
#include <cuda_runtime.h>

// createHandle wraps custatevecCreate.
static custatevecStatus_t goCreateHandle(custatevecHandle_t *handle) {
    return custatevecCreate(handle);
}

// destroyHandle wraps custatevecDestroy.
static custatevecStatus_t goDestroyHandle(custatevecHandle_t handle) {
    return custatevecDestroy(handle);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type cusvHandle struct {
	h      C.custatevecHandle_t
	stream C.cudaStream_t
}

type deviceAlloc struct {
	ptr  unsafe.Pointer
	size int // number of complex128 elements
}

func createHandle() (cusvHandle, error) {
	var h cusvHandle
	if st := C.goCreateHandle(&h.h); st != C.CUSTATEVEC_STATUS_SUCCESS {
		return h, fmt.Errorf("custatevecCreate failed: status %d", int(st))
	}
	if st := C.cudaStreamCreate(&h.stream); st != C.cudaSuccess {
		C.goDestroyHandle(h.h)
		return h, fmt.Errorf("cudaStreamCreate failed: status %d", int(st))
	}
	return h, nil
}

func destroyHandle(h cusvHandle) {
	C.cudaStreamDestroy(h.stream)
	C.goDestroyHandle(h.h)
}
