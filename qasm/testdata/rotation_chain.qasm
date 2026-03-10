OPENQASM 3.0;
include "stdgates.inc";

qubit[1] q;
bit[1] c;

rx(pi/8) q[0];
ry(pi/4) q[0];
rz(pi/2) q[0];
rx(pi) q[0];
ry(3 * pi / 4) q[0];
rz(-pi / 3) q[0];
c[0] = measure q[0];
