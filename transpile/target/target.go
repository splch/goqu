// Package target defines hardware target descriptions for transpilation.
package target

import "fmt"

// Target describes a quantum hardware target.
type Target struct {
	Name            string
	NumQubits       int
	BasisGates      []string    // e.g., ["CX","RZ","SX","X"] or ["GPI","GPI2","MS"]
	Connectivity    []QubitPair // nil = all-to-all
	GateFidelities  map[string]float64
	MaxCircuitDepth int // 0 = unlimited
}

// QubitPair represents a connected pair of physical qubits.
type QubitPair struct{ Q0, Q1 int }

// HasBasisGate reports whether name is in the target's basis set.
// A basis set containing "*" matches all gates.
func (t Target) HasBasisGate(name string) bool {
	for _, b := range t.BasisGates {
		if b == "*" || b == name {
			return true
		}
	}
	return false
}

// HasDirection reports whether the target supports a 2Q gate from q0 to q1
// in that specific direction. Returns true for all-to-all targets (nil Connectivity).
// For directed targets, checks exact (Q0==q0, Q1==q1) match.
func (t Target) HasDirection(q0, q1 int) bool {
	if t.Connectivity == nil {
		return true
	}
	for _, p := range t.Connectivity {
		if p.Q0 == q0 && p.Q1 == q1 {
			return true
		}
	}
	return false
}

// IsConnected reports whether q0 and q1 are directly connected.
// Returns true for all-to-all targets (nil Connectivity).
func (t Target) IsConnected(q0, q1 int) bool {
	if t.Connectivity == nil {
		return true
	}
	for _, p := range t.Connectivity {
		if (p.Q0 == q0 && p.Q1 == q1) || (p.Q0 == q1 && p.Q1 == q0) {
			return true
		}
	}
	return false
}

// AdjacencyMap returns a map from qubit to its connected neighbors.
// Returns nil for all-to-all targets.
func (t Target) AdjacencyMap() map[int][]int {
	if t.Connectivity == nil {
		return nil
	}
	adj := make(map[int][]int)
	for _, p := range t.Connectivity {
		adj[p.Q0] = append(adj[p.Q0], p.Q1)
		adj[p.Q1] = append(adj[p.Q1], p.Q0)
	}
	return adj
}

// DistanceMatrix returns shortest-path distances between all qubit pairs
// using BFS. Returns nil for all-to-all targets.
func (t Target) DistanceMatrix() [][]int {
	if t.Connectivity == nil {
		return nil
	}
	adj := t.AdjacencyMap()
	n := t.NumQubits
	dist := make([][]int, n)
	for i := range n {
		dist[i] = make([]int, n)
		for j := range n {
			dist[i][j] = -1
		}
		// BFS from qubit i.
		dist[i][i] = 0
		queue := []int{i}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range adj[cur] {
				if dist[i][nb] == -1 {
					dist[i][nb] = dist[i][cur] + 1
					queue = append(queue, nb)
				}
			}
		}
	}
	return dist
}

// Predefined targets.
var (
	IonQForte = Target{
		Name:       "IonQ Forte",
		NumQubits:  36,
		BasisGates: []string{"GPI", "GPI2", "MS"},
		// all-to-all connectivity (nil)
	}

	IonQAria = Target{
		Name:       "IonQ Aria",
		NumQubits:  25,
		BasisGates: []string{"GPI", "GPI2", "MS"},
		// all-to-all connectivity (nil)
	}

	// ibmEagle127Connectivity is the 127-qubit heavy-hex coupling map shared by
	// all IBM Eagle-class processors (Eagle, Brisbane, Sherbrooke).
	// Source: qiskit-ibm-runtime conf_sherbrooke.json
	ibmEagle127Connectivity = []QubitPair{
		{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 5}, {5, 6}, {6, 7}, {7, 8},
		{8, 9}, {9, 10}, {10, 11}, {11, 12}, {12, 13}, {0, 14}, {4, 15},
		{8, 16}, {12, 17}, {14, 18}, {18, 19}, {19, 20}, {20, 21}, {21, 22},
		{15, 22}, {22, 23}, {23, 24}, {24, 25}, {25, 26}, {16, 26}, {26, 27},
		{27, 28}, {28, 29}, {29, 30}, {17, 30}, {30, 31}, {31, 32}, {20, 33},
		{24, 34}, {28, 35}, {32, 36}, {33, 39}, {37, 38}, {38, 39}, {39, 40},
		{40, 41}, {41, 42}, {42, 43}, {34, 43}, {43, 44}, {44, 45}, {45, 46},
		{46, 47}, {35, 47}, {47, 48}, {48, 49}, {49, 50}, {50, 51}, {36, 51},
		{37, 52}, {41, 53}, {45, 54}, {49, 55}, {52, 56}, {56, 57}, {57, 58},
		{58, 59}, {59, 60}, {53, 60}, {60, 61}, {61, 62}, {62, 63}, {63, 64},
		{54, 64}, {64, 65}, {65, 66}, {66, 67}, {67, 68}, {55, 68}, {68, 69},
		{69, 70}, {58, 71}, {62, 72}, {66, 73}, {70, 74}, {71, 77}, {75, 76},
		{76, 77}, {77, 78}, {78, 79}, {79, 80}, {80, 81}, {72, 81}, {81, 82},
		{82, 83}, {83, 84}, {84, 85}, {73, 85}, {85, 86}, {86, 87}, {87, 88},
		{88, 89}, {74, 89}, {75, 90}, {79, 91}, {83, 92}, {87, 93}, {90, 94},
		{94, 95}, {95, 96}, {96, 97}, {97, 98}, {91, 98}, {98, 99}, {99, 100},
		{100, 101}, {101, 102}, {92, 102}, {102, 103}, {103, 104}, {104, 105},
		{105, 106}, {93, 106}, {106, 107}, {107, 108}, {96, 109}, {100, 110},
		{104, 111}, {108, 112}, {109, 114}, {113, 114}, {114, 115}, {115, 116},
		{116, 117}, {117, 118}, {110, 118}, {118, 119}, {119, 120}, {120, 121},
		{121, 122}, {111, 122}, {122, 123}, {123, 124}, {124, 125}, {125, 126},
		{112, 126},
	}

	IBMEagle = Target{
		Name:         "IBM Eagle",
		NumQubits:    127,
		BasisGates:   []string{"CX", "ID", "RZ", "SX", "X"},
		Connectivity: ibmEagle127Connectivity,
	}

	IBMBrisbane = Target{
		Name:         "ibm.brisbane",
		NumQubits:    127,
		BasisGates:   []string{"CX", "RZ", "SX", "X", "I"},
		Connectivity: ibmEagle127Connectivity,
	}

	IBMSherbrooke = Target{
		Name:         "ibm.sherbrooke",
		NumQubits:    127,
		BasisGates:   []string{"CX", "RZ", "SX", "X", "I"},
		Connectivity: ibmEagle127Connectivity,
	}

	QuantinuumH1 = Target{
		Name:       "Quantinuum H1",
		NumQubits:  20,
		BasisGates: []string{"RZZ", "RZ", "RY"},
		// all-to-all connectivity (nil)
	}

	QuantinuumH2 = Target{
		Name:       "Quantinuum H2",
		NumQubits:  56,
		BasisGates: []string{"RZZ", "RZ", "RY"},
		// all-to-all connectivity (nil)
	}

	GoogleWillow = Target{
		Name:       "Google Willow",
		NumQubits:  105,
		BasisGates: []string{"CZ", "RZ", "RX"},
		// 2D grid connectivity; nil = all-to-all approximation.
		// Exact connectivity can be fetched from the Quantum Engine API at runtime.
	}

	GoogleSycamore = Target{
		Name:       "Google Sycamore",
		NumQubits:  53,
		BasisGates: []string{"CZ", "RZ", "RX"},
		// 2D grid connectivity; nil = all-to-all approximation.
		// Exact connectivity can be fetched from the Quantum Engine API at runtime.
	}

	RigettiAnkaa = Target{
		Name:       "Rigetti Ankaa-3",
		NumQubits:  84,
		BasisGates: []string{"CZ", "RX", "RZ"},
		// Native hardware gates are RX, RZ, and iSWAP, but the QCS translation
		// service accepts CZ and decomposes it to iSWAP internally. We use CZ
		// here because goqu lacks a native iSWAP gate and CZ is the standard
		// 2Q entangling gate in Quil programs submitted to QCS.
		// Ankaa uses a square-octagon lattice; nil = all-to-all approximation.
		// Exact connectivity can be fetched from QCS ISA API at runtime.
	}

	Simulator = Target{
		Name:       "Simulator",
		NumQubits:  28,
		BasisGates: []string{"*"},
	}
)

// ValidateConnectivity checks that the target's connectivity graph is connected
// (all qubits reachable from qubit 0) and that all qubit indices are in range.
// Returns nil for all-to-all targets (nil Connectivity).
func (t Target) ValidateConnectivity() error {
	if t.Connectivity == nil {
		return nil
	}
	n := t.NumQubits
	for _, p := range t.Connectivity {
		if p.Q0 < 0 || p.Q0 >= n || p.Q1 < 0 || p.Q1 >= n {
			return fmt.Errorf("target %q: edge (%d,%d) out of range [0,%d)", t.Name, p.Q0, p.Q1, n)
		}
	}
	adj := t.AdjacencyMap()
	visited := make([]bool, n)
	queue := []int{0}
	visited[0] = true
	count := 1
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range adj[cur] {
			if !visited[nb] {
				visited[nb] = true
				count++
				queue = append(queue, nb)
			}
		}
	}
	if count != n {
		return fmt.Errorf("target %q: connectivity graph has %d reachable qubits out of %d (disconnected)", t.Name, count, n)
	}
	return nil
}

func init() {
	// Validate predefined targets with explicit connectivity at startup.
	// IBMBrisbane and IBMSherbrooke share IBMEagle's connectivity, so
	// validating IBMEagle covers all three.
	if err := IBMEagle.ValidateConnectivity(); err != nil {
		panic(err)
	}
}
