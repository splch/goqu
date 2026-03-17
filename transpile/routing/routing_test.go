package routing

import (
	"math/rand/v2"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestRouteLinearChainInsertsSWAPs(t *testing.T) {
	// CNOT(0,3) on a 4-qubit linear chain (0-1-2-3): should route the circuit.
	// The result should only have 2-qubit gates on connected pairs.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 3}},
	}
	c := ir.New("route_linear", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-4",
		NumQubits:  4,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
			{Q0: 2, Q1: 3},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// The result should have at least one operation.
	if len(result.Ops()) == 0 {
		t.Fatal("expected at least one op in routed circuit")
	}

	// All 2-qubit gates must be on connected pairs.
	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			q0, q1 := op.Qubits[0], op.Qubits[1]
			if !tgt.IsConnected(q0, q1) {
				t.Errorf("op %d: %s on (%d,%d) is not connected in target",
					i, op.Gate.Name(), q0, q1)
			}
		}
	}
}

func TestRouteAllToAllUnchanged(t *testing.T) {
	// All-to-all target (nil connectivity) should return the circuit unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 3}},
	}
	c := ir.New("all_to_all", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "all-to-all",
		NumQubits:  4,
		BasisGates: []string{"*"},
		// Connectivity: nil means all-to-all
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Circuit should be unchanged.
	if result != c {
		t.Error("expected Route to return the same circuit for all-to-all target")
	}
}

func TestRouteAdjacentQubitsNoSWAP(t *testing.T) {
	// CNOT(0,1) on a linear chain: qubits are already adjacent, no SWAP needed.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("adjacent", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-4",
		NumQubits:  4,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
			{Q0: 2, Q1: 3},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Should contain no SWAP gates.
	for _, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			t.Error("expected no SWAP gates for adjacent CNOT")
		}
	}
}

func TestRouteSingleQubitGates(t *testing.T) {
	// Single-qubit gates should be remapped but not require SWAPs.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	c := ir.New("single_q", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-2",
		NumQubits:  2,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops, got %d", len(result.Ops()))
	}
	for _, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			t.Error("expected no SWAP gates for single-qubit-only circuit")
		}
	}
}

func TestRouteMultipleCNOTs(t *testing.T) {
	// Multiple CNOTs on a linear chain.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("multi_cnot", 3, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-3",
		NumQubits:  3,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Should have more ops than original (SWAPs inserted for 0->2 CNOT).
	if len(result.Ops()) < 2 {
		t.Errorf("expected at least 2 ops in routed circuit, got %d", len(result.Ops()))
	}
}

func TestTrivialLayout(t *testing.T) {
	layout := TrivialLayout(4)
	for i, v := range layout {
		if v != i {
			t.Errorf("TrivialLayout[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestInverseLayout(t *testing.T) {
	layout := []int{2, 0, 1, 3}
	inv := InverseLayout(layout, len(layout))

	// layout[0]=2 means logical 0 -> physical 2, so inv[2]=0
	expected := []int{1, 2, 0, 3}
	for i, v := range inv {
		if v != expected[i] {
			t.Errorf("InverseLayout[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestRouteIonQAllToAll(t *testing.T) {
	// IonQ targets have all-to-all connectivity, circuit should be unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 5}},
		{Gate: gate.CNOT, Qubits: []int{3, 7}},
	}
	c := ir.New("ionq", 10, 0, ops, nil)

	result, err := Route(c, target.IonQForte)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if result != c {
		t.Error("expected Route to return same circuit for IonQ (all-to-all)")
	}
}

func TestRouteWithOptionsDeterministic(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 3}},
		{Gate: gate.CNOT, Qubits: []int{1, 2}},
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("det", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-4",
		NumQubits:  4,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1}, {Q0: 1, Q1: 2}, {Q0: 2, Q1: 3},
		},
	}

	seed := uint64(42)
	opts := Options{Trials: 5, Seed: &seed, Parallelism: 1}

	r1, err := RouteWithOptions(c, tgt, opts)
	if err != nil {
		t.Fatalf("RouteWithOptions: %v", err)
	}

	r2, err := RouteWithOptions(c, tgt, opts)
	if err != nil {
		t.Fatalf("RouteWithOptions: %v", err)
	}

	ops1 := r1.Ops()
	ops2 := r2.Ops()
	if len(ops1) != len(ops2) {
		t.Fatalf("determinism: got %d vs %d ops", len(ops1), len(ops2))
	}
	for i := range ops1 {
		if ops1[i].Gate.Name() != ops2[i].Gate.Name() {
			t.Errorf("op %d: gate %s vs %s", i, ops1[i].Gate.Name(), ops2[i].Gate.Name())
		}
		for j := range ops1[i].Qubits {
			if ops1[i].Qubits[j] != ops2[i].Qubits[j] {
				t.Errorf("op %d qubit %d: %d vs %d", i, j, ops1[i].Qubits[j], ops2[i].Qubits[j])
			}
		}
	}
}

func TestRouteWithOptionsTrials(t *testing.T) {
	// More trials should produce equal or fewer SWAPs.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 4}},
		{Gate: gate.CNOT, Qubits: []int{1, 3}},
		{Gate: gate.CNOT, Qubits: []int{2, 5}},
		{Gate: gate.CNOT, Qubits: []int{0, 5}},
	}
	c := ir.New("trials", 6, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-6",
		NumQubits:  6,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1}, {Q0: 1, Q1: 2}, {Q0: 2, Q1: 3},
			{Q0: 3, Q1: 4}, {Q0: 4, Q1: 5},
		},
	}

	seed := uint64(123)
	opts1 := Options{Trials: 1, Seed: &seed, Parallelism: 1}
	r1, _ := RouteWithOptions(c, tgt, opts1)
	swaps1 := countSwaps(r1.Ops())

	opts20 := Options{Trials: 20, Seed: &seed, Parallelism: 1}
	r20, _ := RouteWithOptions(c, tgt, opts20)
	swaps20 := countSwaps(r20.Ops())

	if swaps20 > swaps1 {
		t.Errorf("20 trials (%d SWAPs) worse than 1 trial (%d SWAPs)", swaps20, swaps1)
	}
}

func TestRouteReleaseValve(t *testing.T) {
	// Build a circuit that could cause the router to get stuck without a release valve.
	// Many non-adjacent CNOTs on a linear chain.
	var ops []ir.Operation
	for i := 0; i < 10; i++ {
		ops = append(ops, ir.Operation{
			Gate:   gate.CNOT,
			Qubits: []int{0, 7},
		})
	}
	c := ir.New("release", 8, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-8",
		NumQubits:  8,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1}, {Q0: 1, Q1: 2}, {Q0: 2, Q1: 3},
			{Q0: 3, Q1: 4}, {Q0: 4, Q1: 5}, {Q0: 5, Q1: 6},
			{Q0: 6, Q1: 7},
		},
	}

	seed := uint64(99)
	opts := Options{Trials: 5, Seed: &seed, ReleaseValveThreshold: 20}
	result, err := RouteWithOptions(c, tgt, opts)
	if err != nil {
		t.Fatalf("RouteWithOptions: %v", err)
	}

	// Verify all 2-qubit gates are on connected pairs.
	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			if !tgt.IsConnected(op.Qubits[0], op.Qubits[1]) {
				t.Errorf("op %d: %s on (%d,%d) not connected",
					i, op.Gate.Name(), op.Qubits[0], op.Qubits[1])
			}
		}
	}

	// Should have routed all original CNOTs (check at least 10 non-SWAP 2Q gates).
	nonSwap2Q := 0
	for _, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 && op.Gate.Name() != "SWAP" {
			nonSwap2Q++
		}
	}
	if nonSwap2Q != 10 {
		t.Errorf("expected 10 routed CNOTs, got %d", nonSwap2Q)
	}
}

func TestRouteLargeGridScalability(t *testing.T) {
	// 10 gates on a 5x5 grid (25 qubits) to test scalability beyond small chains.
	var pairs []target.QubitPair
	rows, cols := 5, 5
	for r := range rows {
		for c := range cols {
			q := r*cols + c
			if c+1 < cols {
				pairs = append(pairs, target.QubitPair{Q0: q, Q1: q + 1})
			}
			if r+1 < rows {
				pairs = append(pairs, target.QubitPair{Q0: q, Q1: q + cols})
			}
		}
	}
	tgt := target.Target{
		Name:         "grid-5x5",
		NumQubits:    25,
		BasisGates:   []string{"*"},
		Connectivity: pairs,
	}

	rng := rand.New(rand.NewPCG(42, 0))
	var ops []ir.Operation
	for i := 0; i < 10; i++ {
		q0 := rng.IntN(25)
		q1 := rng.IntN(25)
		for q1 == q0 {
			q1 = rng.IntN(25)
		}
		ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	}
	c := ir.New("grid", 25, 0, ops, nil)

	seed := uint64(7)
	opts := Options{Trials: 5, Seed: &seed}
	result, err := RouteWithOptions(c, tgt, opts)
	if err != nil {
		t.Fatalf("RouteWithOptions: %v", err)
	}

	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			if !tgt.IsConnected(op.Qubits[0], op.Qubits[1]) {
				t.Errorf("op %d: %s on (%d,%d) not connected",
					i, op.Gate.Name(), op.Qubits[0], op.Qubits[1])
			}
		}
	}
}

func TestRandomLayout(t *testing.T) {
	rng := rand.New(rand.NewPCG(0, 0))
	n := 10
	layout := RandomLayout(n, n, rng)

	if len(layout) != n {
		t.Fatalf("RandomLayout length = %d, want %d", len(layout), n)
	}

	// Must be a valid permutation of [0, n).
	seen := make(map[int]bool)
	for _, v := range layout {
		if v < 0 || v >= n {
			t.Errorf("RandomLayout value %d out of range [0, %d)", v, n)
		}
		if seen[v] {
			t.Errorf("RandomLayout duplicate value %d", v)
		}
		seen[v] = true
	}
}

func TestRouteConnectivityValid(t *testing.T) {
	// Larger circuit: all routed 2Q gates must satisfy connectivity.
	var ops []ir.Operation
	rng := rand.New(rand.NewPCG(55, 0))
	for i := 0; i < 15; i++ {
		q0 := rng.IntN(6)
		q1 := rng.IntN(6)
		for q1 == q0 {
			q1 = rng.IntN(6)
		}
		ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q0, q1}})
	}
	c := ir.New("conn_valid", 6, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-6",
		NumQubits:  6,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1}, {Q0: 1, Q1: 2}, {Q0: 2, Q1: 3},
			{Q0: 3, Q1: 4}, {Q0: 4, Q1: 5},
		},
	}

	seed := uint64(0)
	result, err := RouteWithOptions(c, tgt, Options{Trials: 10, Seed: &seed})
	if err != nil {
		t.Fatalf("RouteWithOptions: %v", err)
	}

	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			if !tgt.IsConnected(op.Qubits[0], op.Qubits[1]) {
				t.Errorf("op %d: %s on (%d,%d) not connected",
					i, op.Gate.Name(), op.Qubits[0], op.Qubits[1])
			}
		}
	}
}
