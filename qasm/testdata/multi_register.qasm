OPENQASM 3.0;
include "stdgates.inc";

qubit[2] data;
qubit[1] ancilla;
bit[3] c;

h data[0];
cx data[0], data[1];
cx data[0], ancilla[0];
c[0] = measure data[0];
c[1] = measure data[1];
c[2] = measure ancilla[0];
