OPENQASM 3.0;
include "stdgates.inc";

qubit[2] q;
bit[2] c;

h q[0];
c[0] = measure q[0];
if (c == 1) {
    x q[1];
}
c[1] = measure q[1];
