OPENQASM 3.0;
include "stdgates.inc";

gate mygate a, b {
    h a;
    cx a, b;
}

qubit[2] q;
bit[2] c;

mygate q[0], q[1];
c = measure q;
