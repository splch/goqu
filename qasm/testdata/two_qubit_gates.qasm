OPENQASM 3.0;
include "stdgates.inc";

qubit[2] q;
bit[2] c;

cx q[0], q[1];
cz q[0], q[1];
cy q[0], q[1];
swap q[0], q[1];
cp(pi/4) q[0], q[1];
crz(pi/3) q[0], q[1];
crx(pi/4) q[0], q[1];
cry(pi/5) q[0], q[1];
c = measure q;
