OPENQASM 3.0;
include "stdgates.inc";

qubit[1] q;
bit[1] c;

id q[0];
h q[0];
x q[0];
y q[0];
z q[0];
s q[0];
sdg q[0];
t q[0];
tdg q[0];
sx q[0];
rx(pi/4) q[0];
ry(pi/4) q[0];
rz(pi/4) q[0];
p(pi/4) q[0];
U(pi/4, pi/3, pi/6) q[0];
c[0] = measure q[0];
