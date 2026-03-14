#import <Metal/Metal.h>
#import <Foundation/Foundation.h>
#include "metal_bridge.h"
#include <string.h>
#include <stdlib.h>

#if __has_feature(objc_arc)
#error "This file must be compiled without ARC (-fno-objc-arc)"
#endif

// ---------------------------------------------------------------------------
// Shader source (embedded) — 1Q and 2Q gate kernels using float2 (complex64).
// ---------------------------------------------------------------------------

static const char *kShaderSource =
"#include <metal_stdlib>\n"
"using namespace metal;\n"
"\n"
"// ---- 1-qubit gate ----\n"
"struct Gate1QParams {\n"
"    uint qubit;\n"
"    uint nAmps;\n"
"    float m[8];\n"
"};\n"
"\n"
"inline float2 cmul(float2 a, float2 b) {\n"
"    return float2(a.x*b.x - a.y*b.y, a.x*b.y + a.y*b.x);\n"
"}\n"
"inline float2 cadd(float2 a, float2 b) {\n"
"    return float2(a.x + b.x, a.y + b.y);\n"
"}\n"
"\n"
"kernel void gate1q(\n"
"    device float2 *sv [[buffer(0)]],\n"
"    constant Gate1QParams &params [[buffer(1)]],\n"
"    uint gid [[thread_position_in_grid]]\n"
") {\n"
"    uint halfBlock = 1u << params.qubit;\n"
"    uint block = halfBlock << 1u;\n"
"    uint blockIdx = gid / halfBlock;\n"
"    uint offset   = gid % halfBlock;\n"
"    uint b0 = blockIdx * block;\n"
"    uint i0 = b0 + offset;\n"
"    uint i1 = i0 + halfBlock;\n"
"    if (i1 >= params.nAmps) return;\n"
"    float2 a0 = sv[i0];\n"
"    float2 a1 = sv[i1];\n"
"    float2 m00 = float2(params.m[0], params.m[1]);\n"
"    float2 m01 = float2(params.m[2], params.m[3]);\n"
"    float2 m10 = float2(params.m[4], params.m[5]);\n"
"    float2 m11 = float2(params.m[6], params.m[7]);\n"
"    sv[i0] = cadd(cmul(m00, a0), cmul(m01, a1));\n"
"    sv[i1] = cadd(cmul(m10, a0), cmul(m11, a1));\n"
"}\n"
"\n"
"// ---- 2-qubit gate ----\n"
"struct Gate2QParams {\n"
"    uint qubit0;\n"
"    uint qubit1;\n"
"    uint nAmps;\n"
"    float m[32];\n"
"};\n"
"\n"
"kernel void gate2q(\n"
"    device float2 *sv [[buffer(0)]],\n"
"    constant Gate2QParams &params [[buffer(1)]],\n"
"    uint gid [[thread_position_in_grid]]\n"
") {\n"
"    uint q0 = params.qubit0;\n"
"    uint q1 = params.qubit1;\n"
"    uint idx = gid;\n"
"    uint lo0 = idx & ((1u << q0) - 1u);\n"
"    uint hi0 = idx >> q0;\n"
"    idx = (hi0 << (q0 + 1u)) | lo0;\n"
"    uint lo1 = idx & ((1u << q1) - 1u);\n"
"    uint hi1 = idx >> q1;\n"
"    idx = (hi1 << (q1 + 1u)) | lo1;\n"
"    uint i00 = idx;\n"
"    uint i01 = idx | (1u << q0);\n"
"    uint i10 = idx | (1u << q1);\n"
"    uint i11 = idx | (1u << q0) | (1u << q1);\n"
"    if (i11 >= params.nAmps) return;\n"
"    float2 a[4] = { sv[i00], sv[i01], sv[i10], sv[i11] };\n"
"    float2 r[4];\n"
"    for (int row = 0; row < 4; row++) {\n"
"        r[row] = float2(0.0, 0.0);\n"
"        for (int col = 0; col < 4; col++) {\n"
"            float2 mv = float2(params.m[(row*4+col)*2], params.m[(row*4+col)*2+1]);\n"
"            r[row] = cadd(r[row], cmul(mv, a[col]));\n"
"        }\n"
"    }\n"
"    sv[i00] = r[0];\n"
"    sv[i01] = r[1];\n"
"    sv[i10] = r[2];\n"
"    sv[i11] = r[3];\n"
"}\n";

// ---------------------------------------------------------------------------
// C-side parameter structs (must match shader layout exactly).
// ---------------------------------------------------------------------------

typedef struct {
    uint32_t qubit;
    uint32_t nAmps;
    float m[8];
} Gate1QParams;

typedef struct {
    uint32_t qubit0;
    uint32_t qubit1;
    uint32_t nAmps;
    float m[32];
} Gate2QParams;

// ---------------------------------------------------------------------------
// MetalSim — opaque handle holding all Metal state.
// ---------------------------------------------------------------------------

struct MetalSim {
    int numQubits;
    int numAmps;
    id<MTLDevice>                device;
    id<MTLCommandQueue>          queue;
    id<MTLComputePipelineState>  pipe1q;
    id<MTLComputePipelineState>  pipe2q;
    id<MTLBuffer>                svBuffer;
    // Transient per-pass state (non-nil between BeginPass/EndPass).
    id<MTLCommandBuffer>         cmdBuf;
    id<MTLComputeCommandEncoder> encoder;
    NSUInteger tgSize1q;
    NSUInteger tgSize2q;
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

static char* errdup(NSError *err) {
    if (err) {
        return strdup([[err localizedDescription] UTF8String]);
    }
    return strdup("unknown Metal error");
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

MetalSim* MetalCreate(int numQubits, char** errOut) {
    @autoreleasepool {
        if (numQubits < 1) {
            *errOut = strdup("numQubits must be >= 1");
            return NULL;
        }

        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (!device) {
            *errOut = strdup("no Metal device available");
            return NULL;
        }

        // Compile shaders.
        NSError *error = nil;
        NSString *src = [NSString stringWithUTF8String:kShaderSource];
        id<MTLLibrary> library = [device newLibraryWithSource:src options:nil error:&error];
        if (!library) {
            *errOut = errdup(error);
            [device release];
            return NULL;
        }

        id<MTLFunction> func1q = [library newFunctionWithName:@"gate1q"];
        id<MTLFunction> func2q = [library newFunctionWithName:@"gate2q"];
        if (!func1q || !func2q) {
            *errOut = strdup("shader function not found");
            if (func1q) [func1q release];
            if (func2q) [func2q release];
            [library release];
            [device release];
            return NULL;
        }

        id<MTLComputePipelineState> pipe1q =
            [device newComputePipelineStateWithFunction:func1q error:&error];
        if (!pipe1q) {
            *errOut = errdup(error);
            [func1q release]; [func2q release];
            [library release]; [device release];
            return NULL;
        }
        id<MTLComputePipelineState> pipe2q =
            [device newComputePipelineStateWithFunction:func2q error:&error];
        if (!pipe2q) {
            *errOut = errdup(error);
            [pipe1q release]; [func1q release]; [func2q release];
            [library release]; [device release];
            return NULL;
        }

        [func1q release];
        [func2q release];
        [library release];

        // Allocate shared-memory state-vector buffer (float2 = 8 bytes per amplitude).
        int numAmps = 1 << numQubits;
        NSUInteger bufSize = (NSUInteger)numAmps * 2 * sizeof(float);
        id<MTLBuffer> svBuffer = [device newBufferWithLength:bufSize
                                                     options:MTLResourceStorageModeShared];
        if (!svBuffer) {
            *errOut = strdup("failed to allocate state vector buffer");
            [pipe2q release]; [pipe1q release]; [device release];
            return NULL;
        }

        // Initialize to |0...0>: first amplitude = (1, 0).
        float *ptr = (float*)[svBuffer contents];
        memset(ptr, 0, bufSize);
        ptr[0] = 1.0f;

        id<MTLCommandQueue> queue = [device newCommandQueue];

        MetalSim *sim = (MetalSim*)calloc(1, sizeof(MetalSim));
        sim->numQubits = numQubits;
        sim->numAmps   = numAmps;
        sim->device    = device;
        sim->queue     = queue;
        sim->pipe1q    = pipe1q;
        sim->pipe2q    = pipe2q;
        sim->svBuffer  = svBuffer;
        sim->cmdBuf    = nil;
        sim->encoder   = nil;
        sim->tgSize1q  = [pipe1q maxTotalThreadsPerThreadgroup];
        sim->tgSize2q  = [pipe2q maxTotalThreadsPerThreadgroup];
        if (sim->tgSize1q > 256) sim->tgSize1q = 256;
        if (sim->tgSize2q > 256) sim->tgSize2q = 256;
        return sim;
    }
}

void MetalDestroy(MetalSim* sim) {
    if (!sim) return;
    @autoreleasepool {
        if (sim->encoder) { [sim->encoder endEncoding]; [sim->encoder release]; }
        if (sim->cmdBuf)  { [sim->cmdBuf release]; }
        [sim->svBuffer release];
        [sim->pipe2q release];
        [sim->pipe1q release];
        [sim->queue release];
        [sim->device release];
    }
    free(sim);
}

float* MetalStatePtr(MetalSim* sim) {
    return (float*)[sim->svBuffer contents];
}

int MetalNumAmps(MetalSim* sim) {
    return sim->numAmps;
}

void MetalResetState(MetalSim* sim) {
    float *ptr = (float*)[sim->svBuffer contents];
    memset(ptr, 0, (size_t)sim->numAmps * 2 * sizeof(float));
    ptr[0] = 1.0f;
}

int MetalBeginPass(MetalSim* sim, char** errOut) {
    @autoreleasepool {
        id<MTLCommandBuffer> buf = [sim->queue commandBuffer];
        if (!buf) {
            *errOut = strdup("failed to create command buffer");
            return -1;
        }
        sim->cmdBuf = [buf retain];

        id<MTLComputeCommandEncoder> enc = [sim->cmdBuf computeCommandEncoder];
        if (!enc) {
            *errOut = strdup("failed to create compute encoder");
            [sim->cmdBuf release];
            sim->cmdBuf = nil;
            return -1;
        }
        sim->encoder = [enc retain];
    }
    return 0;
}

int MetalGate1Q(MetalSim* sim, uint32_t qubit, const float* matrix,
                char** errOut) {
    (void)errOut;
    Gate1QParams params;
    params.qubit = qubit;
    params.nAmps = (uint32_t)sim->numAmps;
    memcpy(params.m, matrix, 8 * sizeof(float));

    [sim->encoder setComputePipelineState:sim->pipe1q];
    [sim->encoder setBuffer:sim->svBuffer offset:0 atIndex:0];
    [sim->encoder setBytes:&params length:sizeof(params) atIndex:1];

    NSUInteger numThreads = (NSUInteger)sim->numAmps / 2;
    NSUInteger tgSize = sim->tgSize1q;
    if (tgSize > numThreads) tgSize = numThreads;
    if (tgSize < 1) tgSize = 1;
    NSUInteger numGroups = (numThreads + tgSize - 1) / tgSize;

    [sim->encoder dispatchThreadgroups:MTLSizeMake(numGroups, 1, 1)
                 threadsPerThreadgroup:MTLSizeMake(tgSize, 1, 1)];
    [sim->encoder memoryBarrierWithScope:MTLBarrierScopeBuffers];
    return 0;
}

int MetalGate2Q(MetalSim* sim, uint32_t qubit0, uint32_t qubit1,
                const float* matrix, char** errOut) {
    (void)errOut;
    Gate2QParams params;
    params.qubit0 = qubit0;
    params.qubit1 = qubit1;
    params.nAmps  = (uint32_t)sim->numAmps;
    memcpy(params.m, matrix, 32 * sizeof(float));

    [sim->encoder setComputePipelineState:sim->pipe2q];
    [sim->encoder setBuffer:sim->svBuffer offset:0 atIndex:0];
    [sim->encoder setBytes:&params length:sizeof(params) atIndex:1];

    NSUInteger numThreads = (NSUInteger)sim->numAmps / 4;
    NSUInteger tgSize = sim->tgSize2q;
    if (tgSize > numThreads) tgSize = numThreads;
    if (tgSize < 1) tgSize = 1;
    NSUInteger numGroups = (numThreads + tgSize - 1) / tgSize;

    [sim->encoder dispatchThreadgroups:MTLSizeMake(numGroups, 1, 1)
                 threadsPerThreadgroup:MTLSizeMake(tgSize, 1, 1)];
    [sim->encoder memoryBarrierWithScope:MTLBarrierScopeBuffers];
    return 0;
}

int MetalEndPass(MetalSim* sim, char** errOut) {
    @autoreleasepool {
        [sim->encoder endEncoding];
        [sim->encoder release];
        sim->encoder = nil;

        [sim->cmdBuf commit];
        [sim->cmdBuf waitUntilCompleted];

        if ([sim->cmdBuf status] == MTLCommandBufferStatusError) {
            NSError *error = [sim->cmdBuf error];
            *errOut = errdup(error);
            [sim->cmdBuf release];
            sim->cmdBuf = nil;
            return -1;
        }

        [sim->cmdBuf release];
        sim->cmdBuf = nil;
    }
    return 0;
}
