OPENQASM 3.0;
include "stdgates.inc";

qubit[2] q;
bit[2] c;

x q[0];
swap q[0], q[1];
c = measure q;
