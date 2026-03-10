package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/splch/qgo/qasm/emitter"
)

func seedFromTestdata(f *testing.F) {
	f.Helper()
	files, err := filepath.Glob(filepath.Join("..", "testdata", "*.qasm"))
	if err != nil {
		return
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		f.Add(data)
	}
}

// FuzzParse feeds arbitrary bytes to the parser. It must never panic.
func FuzzParse(f *testing.F) {
	seedFromTestdata(f)

	// Minimal valid seeds.
	f.Add([]byte(`OPENQASM 3.0; qubit[2] q; h q[0]; cx q[0], q[1];`))
	f.Add([]byte(`OPENQASM 3.0; qubit[1] q; bit[1] c; x q[0]; c[0] = measure q[0];`))
	f.Add([]byte(`OPENQASM 3.0; include "stdgates.inc"; qubit[3] q; h q[0]; cx q[0], q[1]; cx q[1], q[2];`))
	f.Add([]byte(`OPENQASM 3.0; qubit[1] q; rx(pi/4) q[0]; ry(pi/2) q[0]; rz(pi) q[0];`))
	f.Add([]byte(`OPENQASM 3.0; qubit[2] q; bit[2] c; h q[0]; c[0] = measure q[0]; if (c == 1) { x q[1]; }`))
	f.Add([]byte(`OPENQASM 3.0; qubit[1] q; barrier q; reset q;`))
	f.Add([]byte(``))
	f.Add([]byte(`not valid qasm at all!!! @@@ {{{`))
	f.Add([]byte(`OPENQASM 3.0; gate mygate a, b { h a; cx a, b; } qubit[2] q; mygate q[0], q[1];`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic regardless of input.
		_, _ = Parse(bytes.NewReader(data))
	})
}

// FuzzRoundTrip parses valid QASM, emits back, re-parses, and verifies structural equivalence.
func FuzzRoundTrip(f *testing.F) {
	seedFromTestdata(f)

	f.Add([]byte(`OPENQASM 3.0; qubit[2] q; h q[0]; cx q[0], q[1];`))
	f.Add([]byte(`OPENQASM 3.0; qubit[3] q; bit[3] c; h q[0]; cx q[0], q[1]; cx q[1], q[2]; c = measure q;`))
	f.Add([]byte(`OPENQASM 3.0; qubit[1] q; rx(pi/4) q[0]; ry(pi/2) q[0]; rz(pi) q[0];`))

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := Parse(bytes.NewReader(data))
		if err != nil {
			return // skip invalid input
		}

		qasm, err := emitter.EmitString(c1)
		if err != nil {
			t.Fatalf("emit failed on valid parsed circuit: %v", err)
		}

		c2, err := Parse(strings.NewReader(qasm))
		if err != nil {
			t.Fatalf("re-parse failed: %v\nQASM:\n%s", err, qasm)
		}

		// Structural comparison.
		if c1.NumQubits() != c2.NumQubits() {
			t.Errorf("qubit count mismatch: %d vs %d", c1.NumQubits(), c2.NumQubits())
		}
		if len(c1.Ops()) != len(c2.Ops()) {
			t.Errorf("op count mismatch: %d vs %d\nOriginal QASM:\n%s\nRe-emitted QASM:\n%s",
				len(c1.Ops()), len(c2.Ops()), string(data), qasm)
		}
	})
}
