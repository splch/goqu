//go:build cuda

package cuda

/*
#include <cuda_runtime.h>
#include <string.h>

static cudaError_t goMalloc(void **ptr, size_t size) {
    return cudaMalloc(ptr, size);
}

static cudaError_t goFree(void *ptr) {
    return cudaFree(ptr);
}

static cudaError_t goMemcpyH2D(void *dst, const void *src, size_t size) {
    return cudaMemcpy(dst, src, size, cudaMemcpyHostToDevice);
}

static cudaError_t goMemcpyD2H(void *dst, const void *src, size_t size) {
    return cudaMemcpy(dst, src, size, cudaMemcpyDeviceToHost);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// allocDevice allocates GPU memory for 2^numQubits complex128 values
// and initializes to |0...0>.
func allocDevice(numQubits int) (deviceAlloc, error) {
	n := 1 << numQubits
	byteSize := C.size_t(n * 16) // complex128 = 16 bytes

	var d deviceAlloc
	d.size = n

	if st := C.goMalloc(&d.ptr, byteSize); st != C.cudaSuccess {
		return d, fmt.Errorf("cudaMalloc failed: status %d", int(st))
	}

	// Zero the device memory, then set |0> = 1+0i.
	if st := C.cudaMemset(d.ptr, 0, byteSize); st != C.cudaSuccess {
		C.goFree(d.ptr)
		return d, fmt.Errorf("cudaMemset failed: status %d", int(st))
	}

	// Set amplitude[0] = 1+0i on the host side, then copy.
	init := complex(1.0, 0.0)
	if st := C.goMemcpyH2D(d.ptr, unsafe.Pointer(&init), 16); st != C.cudaSuccess {
		C.goFree(d.ptr)
		return d, fmt.Errorf("cudaMemcpy H2D failed: status %d", int(st))
	}

	return d, nil
}

// resetDevice re-initializes the state vector to |0...0>.
func resetDevice(d deviceAlloc) error {
	byteSize := C.size_t(d.size * 16)
	if st := C.cudaMemset(d.ptr, 0, byteSize); st != C.cudaSuccess {
		return fmt.Errorf("cudaMemset failed: status %d", int(st))
	}
	init := complex(1.0, 0.0)
	if st := C.goMemcpyH2D(d.ptr, unsafe.Pointer(&init), 16); st != C.cudaSuccess {
		return fmt.Errorf("cudaMemcpy H2D failed: status %d", int(st))
	}
	return nil
}

// copyToHost transfers the full state vector from GPU to a host slice.
func copyToHost(d deviceAlloc) []complex128 {
	out := make([]complex128, d.size)
	C.goMemcpyD2H(unsafe.Pointer(&out[0]), d.ptr, C.size_t(d.size*16))
	return out
}

// freeDevice releases GPU memory.
func freeDevice(d *deviceAlloc) {
	if d.ptr != nil {
		C.goFree(d.ptr)
		d.ptr = nil
	}
}
