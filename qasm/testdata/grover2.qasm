OPENQASM 3.0;
include "stdgates.inc";

qubit[2] q;
bit[2] c;

// Initialize superposition
h q[0];
h q[1];

// Oracle for |11>
cz q[0], q[1];

// Diffusion operator
h q[0];
h q[1];
x q[0];
x q[1];
cz q[0], q[1];
x q[0];
x q[1];
h q[0];
h q[1];

c = measure q;
