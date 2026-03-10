OPENQASM 3.0;
include "stdgates.inc";

qubit[3] q;
bit[3] c;

reset q;
U(0.3, 0.2, 0.1) q[0];
h q[1];
cx q[1], q[2];
barrier q;
cx q[0], q[1];
h q[0];
c[0] = measure q[0];
c[1] = measure q[1];
if (c == 1) z q[2];
c[2] = measure q[2];
