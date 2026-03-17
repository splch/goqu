package target

import (
	"testing"
)

func TestHasBasisGate_IBM(t *testing.T) {
	ibm := IBMEagle
	yes := []string{"CX", "ID", "RZ", "SX", "X"}
	for _, g := range yes {
		if !ibm.HasBasisGate(g) {
			t.Errorf("IBMEagle.HasBasisGate(%q) = false, want true", g)
		}
	}
	no := []string{"H", "CNOT", "S", "T", "GPI", "SWAP"}
	for _, g := range no {
		if ibm.HasBasisGate(g) {
			t.Errorf("IBMEagle.HasBasisGate(%q) = true, want false", g)
		}
	}
}

func TestHasBasisGate_IonQ(t *testing.T) {
	ionq := IonQForte
	yes := []string{"GPI", "GPI2", "MS"}
	for _, g := range yes {
		if !ionq.HasBasisGate(g) {
			t.Errorf("IonQForte.HasBasisGate(%q) = false, want true", g)
		}
	}
	no := []string{"CX", "H", "RZ", "X"}
	for _, g := range no {
		if ionq.HasBasisGate(g) {
			t.Errorf("IonQForte.HasBasisGate(%q) = true, want false", g)
		}
	}
}

func TestHasBasisGate_Simulator(t *testing.T) {
	sim := Simulator
	// Wildcard should match everything.
	gates := []string{"H", "CX", "RZ", "GPI", "ANYTHING", "MS", "U3", "SWAP"}
	for _, g := range gates {
		if !sim.HasBasisGate(g) {
			t.Errorf("Simulator.HasBasisGate(%q) = false, want true (wildcard)", g)
		}
	}
}

func TestHasBasisGate_QuantinuumH1(t *testing.T) {
	q := QuantinuumH1
	yes := []string{"RZZ", "RZ", "RY"}
	for _, g := range yes {
		if !q.HasBasisGate(g) {
			t.Errorf("QuantinuumH1.HasBasisGate(%q) = false, want true", g)
		}
	}
	if q.HasBasisGate("CX") {
		t.Error("QuantinuumH1.HasBasisGate(\"CX\") = true, want false")
	}
}

func TestHasBasisGate_EmptyBasis(t *testing.T) {
	tgt := Target{BasisGates: []string{}}
	if tgt.HasBasisGate("H") {
		t.Error("empty basis set should match nothing")
	}
}

func TestIsConnected_AllToAll(t *testing.T) {
	// IonQ targets have nil connectivity (all-to-all).
	ionq := IonQForte
	pairs := [][2]int{{0, 1}, {0, 35}, {15, 20}, {34, 35}}
	for _, p := range pairs {
		if !ionq.IsConnected(p[0], p[1]) {
			t.Errorf("IonQForte.IsConnected(%d, %d) = false, want true (all-to-all)", p[0], p[1])
		}
	}
}

func TestIsConnected_IBMConstrained(t *testing.T) {
	ibm := IBMEagle
	// Qubit 0 connects to qubit 1 (backbone even index).
	if !ibm.IsConnected(0, 1) {
		t.Error("IBMEagle.IsConnected(0, 1) = false, want true")
	}
	// Symmetry: IsConnected should work both directions.
	if !ibm.IsConnected(1, 0) {
		t.Error("IBMEagle.IsConnected(1, 0) = false, want true (symmetric)")
	}
}

func TestIsConnected_CustomLinearChain(t *testing.T) {
	// 4-qubit linear chain: 0-1-2-3
	chain := Target{
		NumQubits: 4,
		Connectivity: []QubitPair{
			{0, 1}, {1, 2}, {2, 3},
		},
	}
	// Direct neighbors.
	if !chain.IsConnected(0, 1) {
		t.Error("chain.IsConnected(0, 1) = false, want true")
	}
	if !chain.IsConnected(2, 3) {
		t.Error("chain.IsConnected(2, 3) = false, want true")
	}
	// Symmetry.
	if !chain.IsConnected(3, 2) {
		t.Error("chain.IsConnected(3, 2) = false, want true")
	}
	// Non-neighbors.
	if chain.IsConnected(0, 2) {
		t.Error("chain.IsConnected(0, 2) = true, want false")
	}
	if chain.IsConnected(0, 3) {
		t.Error("chain.IsConnected(0, 3) = true, want false")
	}
	if chain.IsConnected(1, 3) {
		t.Error("chain.IsConnected(1, 3) = true, want false")
	}
}

func TestAdjacencyMap_AllToAll(t *testing.T) {
	ionq := IonQAria
	adj := ionq.AdjacencyMap()
	if adj != nil {
		t.Errorf("IonQAria.AdjacencyMap() = %v, want nil (all-to-all)", adj)
	}
}

func TestAdjacencyMap_LinearChain(t *testing.T) {
	chain := Target{
		NumQubits: 4,
		Connectivity: []QubitPair{
			{0, 1}, {1, 2}, {2, 3},
		},
	}
	adj := chain.AdjacencyMap()
	if adj == nil {
		t.Fatal("AdjacencyMap() returned nil for constrained target")
	}

	// Check neighbor counts.
	tests := map[int]int{
		0: 1, // only connected to 1
		1: 2, // connected to 0 and 2
		2: 2, // connected to 1 and 3
		3: 1, // only connected to 2
	}
	for q, wantCount := range tests {
		if got := len(adj[q]); got != wantCount {
			t.Errorf("len(adj[%d]) = %d, want %d", q, got, wantCount)
		}
	}

	// Check specific neighbors.
	if !contains(adj[0], 1) {
		t.Error("adj[0] should contain 1")
	}
	if !contains(adj[1], 0) || !contains(adj[1], 2) {
		t.Error("adj[1] should contain 0 and 2")
	}
	if !contains(adj[3], 2) {
		t.Error("adj[3] should contain 2")
	}
}

func TestDistanceMatrix_AllToAll(t *testing.T) {
	ionq := IonQForte
	dist := ionq.DistanceMatrix()
	if dist != nil {
		t.Error("IonQForte.DistanceMatrix() should be nil (all-to-all)")
	}
}

func TestDistanceMatrix_LinearChain(t *testing.T) {
	// 4-qubit linear chain: 0-1-2-3
	chain := Target{
		NumQubits: 4,
		Connectivity: []QubitPair{
			{0, 1}, {1, 2}, {2, 3},
		},
	}
	dist := chain.DistanceMatrix()
	if dist == nil {
		t.Fatal("DistanceMatrix() returned nil for constrained target")
	}
	if len(dist) != 4 {
		t.Fatalf("DistanceMatrix() has %d rows, want 4", len(dist))
	}

	// Expected distance matrix for a linear chain.
	want := [][]int{
		{0, 1, 2, 3},
		{1, 0, 1, 2},
		{2, 1, 0, 1},
		{3, 2, 1, 0},
	}
	for i := range 4 {
		for j := range 4 {
			if dist[i][j] != want[i][j] {
				t.Errorf("dist[%d][%d] = %d, want %d", i, j, dist[i][j], want[i][j])
			}
		}
	}
}

func TestDistanceMatrix_Star(t *testing.T) {
	// Star topology: center qubit 0 connected to 1, 2, 3.
	star := Target{
		NumQubits: 4,
		Connectivity: []QubitPair{
			{0, 1}, {0, 2}, {0, 3},
		},
	}
	dist := star.DistanceMatrix()
	if dist == nil {
		t.Fatal("DistanceMatrix() returned nil")
	}

	// Self-distances.
	for i := range 4 {
		if dist[i][i] != 0 {
			t.Errorf("dist[%d][%d] = %d, want 0", i, i, dist[i][i])
		}
	}
	// Center to leaves: distance 1.
	for j := 1; j <= 3; j++ {
		if dist[0][j] != 1 {
			t.Errorf("dist[0][%d] = %d, want 1", j, dist[0][j])
		}
		if dist[j][0] != 1 {
			t.Errorf("dist[%d][0] = %d, want 1", j, dist[j][0])
		}
	}
	// Leaf to leaf: distance 2 (through center).
	if dist[1][2] != 2 {
		t.Errorf("dist[1][2] = %d, want 2", dist[1][2])
	}
	if dist[2][3] != 2 {
		t.Errorf("dist[2][3] = %d, want 2", dist[2][3])
	}
}

func TestPredefinedTargets(t *testing.T) {
	targets := []struct {
		tgt  Target
		name string
		nq   int
	}{
		{IonQForte, "IonQ Forte", 36},
		{IonQAria, "IonQ Aria", 25},
		{IBMEagle, "IBM Eagle", 127},
		{QuantinuumH1, "Quantinuum H1", 20},
		{QuantinuumH2, "Quantinuum H2", 56},
		{Simulator, "Simulator", 28},
	}
	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tgt.Name != tt.name {
				t.Errorf("Name = %q, want %q", tt.tgt.Name, tt.name)
			}
			if tt.tgt.NumQubits != tt.nq {
				t.Errorf("NumQubits = %d, want %d", tt.tgt.NumQubits, tt.nq)
			}
			if len(tt.tgt.BasisGates) == 0 {
				t.Error("BasisGates is empty")
			}
		})
	}
}

func TestIBMEagleHasConnectivity(t *testing.T) {
	if IBMEagle.Connectivity == nil {
		t.Error("IBMEagle should have constrained connectivity")
	}
	if len(IBMEagle.Connectivity) != 144 {
		t.Errorf("IBMEagle.Connectivity has %d edges, want 144", len(IBMEagle.Connectivity))
	}
}

func TestConstrainedTargetsConnected(t *testing.T) {
	targets := []Target{IBMEagle, IBMBrisbane, IBMSherbrooke}
	for _, tgt := range targets {
		t.Run(tgt.Name, func(t *testing.T) {
			if err := tgt.ValidateConnectivity(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestIonQTargetsAllToAll(t *testing.T) {
	if IonQForte.Connectivity != nil {
		t.Error("IonQForte should have nil connectivity (all-to-all)")
	}
	if IonQAria.Connectivity != nil {
		t.Error("IonQAria should have nil connectivity (all-to-all)")
	}
	if QuantinuumH1.Connectivity != nil {
		t.Error("QuantinuumH1 should have nil connectivity (all-to-all)")
	}
}

// contains checks if a slice contains a value.
func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
