OPENQASM 3.0;
include "stdgates.inc";

qubit[2] q;
bit[2] c;

rx(pi/4) q[0];
ry(pi/3) q[0];
rz(pi/6) q[1];
cp(pi/2) q[0], q[1];
c = measure q;
