#ifndef METAL_BRIDGE_H
#define METAL_BRIDGE_H

#include <stdint.h>

typedef struct MetalSim MetalSim;

// MetalCreate creates a Metal simulator for numQubits qubits.
// Returns NULL on error; *errOut is set to a strdup'd message (caller frees).
MetalSim* MetalCreate(int numQubits, char** errOut);

// MetalDestroy releases all Metal resources.
void MetalDestroy(MetalSim* sim);

// MetalStatePtr returns a pointer to the float32 state vector in shared memory.
// Layout: 2*numAmps floats (pairs of real, imag) - i.e., complex64.
float* MetalStatePtr(MetalSim* sim);

// MetalNumAmps returns 2^numQubits.
int MetalNumAmps(MetalSim* sim);

// MetalResetState sets the state to |0...0>.
void MetalResetState(MetalSim* sim);

// MetalBeginPass creates a new command buffer and compute encoder.
int MetalBeginPass(MetalSim* sim, char** errOut);

// MetalGate1Q encodes a 1-qubit gate. matrix: 8 floats (4 complex, row-major).
int MetalGate1Q(MetalSim* sim, uint32_t qubit, const float* matrix, char** errOut);

// MetalGate2Q encodes a 2-qubit gate. qubit0 < qubit1 required.
// matrix: 32 floats (16 complex, row-major).
int MetalGate2Q(MetalSim* sim, uint32_t qubit0, uint32_t qubit1,
                const float* matrix, char** errOut);

// MetalEndPass commits the command buffer and waits for completion.
int MetalEndPass(MetalSim* sim, char** errOut);

#endif
