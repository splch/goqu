// gate1q.metal — Single-qubit gate application compute shader.
//
// Applies a 2x2 unitary matrix to the state vector using the stride pattern:
//   halfBlock = 1 << qubit
//   block     = halfBlock << 1
//   For each block b0 in [0, n, block]:
//     for offset in [0, halfBlock):
//       i0 = b0 + offset       (qubit bit = 0)
//       i1 = i0 + halfBlock    (qubit bit = 1)
//       sv[i0] = m00*a0 + m01*a1
//       sv[i1] = m10*a0 + m11*a1
//
// Uses double2 as a stand-in for complex<double> since Metal has no native complex type.

#include <metal_stdlib>
using namespace metal;

struct Gate1QParams {
    uint qubit;
    uint nAmps;       // 2^numQubits
    double2 m00;      // matrix[0][0] as (real, imag)
    double2 m01;      // matrix[0][1]
    double2 m10;      // matrix[1][0]
    double2 m11;      // matrix[1][1]
};

// Complex multiplication: (a.x + a.y*i) * (b.x + b.y*i)
inline double2 cmul(double2 a, double2 b) {
    return double2(a.x*b.x - a.y*b.y, a.x*b.y + a.y*b.x);
}

// Complex addition.
inline double2 cadd(double2 a, double2 b) {
    return double2(a.x + b.x, a.y + b.y);
}

kernel void gate1q(
    device double2 *sv [[buffer(0)]],
    constant Gate1QParams &params [[buffer(1)]],
    uint gid [[thread_position_in_grid]]
) {
    uint halfBlock = 1u << params.qubit;
    uint block = halfBlock << 1u;

    // Each thread handles one (i0, i1) pair.
    uint blockIdx = gid / halfBlock;
    uint offset = gid % halfBlock;
    uint b0 = blockIdx * block;
    uint i0 = b0 + offset;
    uint i1 = i0 + halfBlock;

    if (i1 >= params.nAmps) return;

    double2 a0 = sv[i0];
    double2 a1 = sv[i1];

    sv[i0] = cadd(cmul(params.m00, a0), cmul(params.m01, a1));
    sv[i1] = cadd(cmul(params.m10, a0), cmul(params.m11, a1));
}
