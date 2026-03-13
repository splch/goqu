// gate2q.metal — Two-qubit gate application compute shader.
//
// Applies a 4x4 unitary matrix to the state vector. For two target qubits
// q0 (lower) and q1 (higher), each thread processes one group of 4 amplitudes
// determined by the two qubit positions.
//
// Uses double2 as a stand-in for complex<double>.

#include <metal_stdlib>
using namespace metal;

struct Gate2QParams {
    uint qubit0;      // lower qubit index
    uint qubit1;      // higher qubit index (must be > qubit0)
    uint nAmps;       // 2^numQubits
    // 4x4 matrix in row-major order, 16 elements.
    double2 m[16];
};

inline double2 cmul(double2 a, double2 b) {
    return double2(a.x*b.x - a.y*b.y, a.x*b.y + a.y*b.x);
}

inline double2 cadd(double2 a, double2 b) {
    return double2(a.x + b.x, a.y + b.y);
}

kernel void gate2q(
    device double2 *sv [[buffer(0)]],
    constant Gate2QParams &params [[buffer(1)]],
    uint gid [[thread_position_in_grid]]
) {
    uint q0 = params.qubit0;
    uint q1 = params.qubit1;

    // Compute the base index by inserting 0-bits at positions q0 and q1.
    // Each thread maps to a unique combination of all other bits.
    uint idx = gid;
    // Insert zero at q0 position.
    uint lo0 = idx & ((1u << q0) - 1u);
    uint hi0 = idx >> q0;
    idx = (hi0 << (q0 + 1u)) | lo0;
    // Insert zero at q1 position.
    uint lo1 = idx & ((1u << q1) - 1u);
    uint hi1 = idx >> q1;
    idx = (hi1 << (q1 + 1u)) | lo1;

    uint i00 = idx;
    uint i01 = idx | (1u << q0);
    uint i10 = idx | (1u << q1);
    uint i11 = idx | (1u << q0) | (1u << q1);

    if (i11 >= params.nAmps) return;

    double2 a[4] = { sv[i00], sv[i01], sv[i10], sv[i11] };
    double2 r[4];

    for (int row = 0; row < 4; row++) {
        r[row] = double2(0.0, 0.0);
        for (int col = 0; col < 4; col++) {
            r[row] = cadd(r[row], cmul(params.m[row * 4 + col], a[col]));
        }
    }

    sv[i00] = r[0];
    sv[i01] = r[1];
    sv[i10] = r[2];
    sv[i11] = r[3];
}
