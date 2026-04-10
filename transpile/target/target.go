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
		BasisGates: []string{"GPI", "GPI2", "ZZ"},
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

	// sycamore53Connectivity is the 53-qubit 2D grid coupling map for
	// Google Sycamore as used in the quantum supremacy experiment (Nature 2019).
	// GridQubit(2,3) is excluded (non-functional on the physical chip), leaving
	// 53 qubits numbered row-major 0-52 with 86 edges.
	// Source: cirq-google/cirq_google/devices/known_devices.py
	sycamore53Connectivity = []QubitPair{
		// Row 0: (0,5)=0, (0,6)=1
		{0, 1}, {0, 3}, {1, 4},
		// Row 1: (1,4)=2, (1,5)=3, (1,6)=4, (1,7)=5
		{2, 3}, {2, 6}, {3, 4}, {3, 7}, {4, 5}, {4, 8}, {5, 9},
		// Row 2: (2,4)=6, (2,5)=7, (2,6)=8, (2,7)=9, (2,8)=10  [no (2,3)]
		{6, 7}, {6, 13}, {7, 8}, {7, 14}, {8, 9}, {8, 15}, {9, 10}, {9, 16},
		{10, 17},
		// Row 3: (3,2)=11 .. (3,9)=18
		{11, 12}, {11, 20}, {12, 13}, {12, 21}, {13, 14}, {13, 22}, {14, 15},
		{14, 23}, {15, 16}, {15, 24}, {16, 17}, {16, 25}, {17, 18}, {17, 26},
		{18, 27},
		// Row 4: (4,1)=19 .. (4,9)=27
		{19, 20}, {19, 29}, {20, 21}, {20, 30}, {21, 22}, {21, 31}, {22, 23},
		{22, 32}, {23, 24}, {23, 33}, {24, 25}, {24, 34}, {25, 26}, {25, 35},
		{26, 27}, {26, 36},
		// Row 5: (5,0)=28 .. (5,8)=36
		{28, 29}, {29, 30}, {29, 37}, {30, 31}, {30, 38}, {31, 32}, {31, 39},
		{32, 33}, {32, 40}, {33, 34}, {33, 41}, {34, 35}, {34, 42}, {35, 36},
		{35, 43},
		// Row 6: (6,1)=37 .. (6,7)=43
		{37, 38}, {38, 39}, {38, 44}, {39, 40}, {39, 45}, {40, 41}, {40, 46},
		{41, 42}, {41, 47}, {42, 43}, {42, 48},
		// Row 7: (7,2)=44 .. (7,6)=48
		{44, 45}, {45, 46}, {45, 49}, {46, 47}, {46, 50}, {47, 48}, {47, 51},
		// Row 8: (8,3)=49 .. (8,5)=51
		{49, 50}, {50, 51}, {50, 52},
	}

	GoogleSycamore = Target{
		Name:         "Google Sycamore",
		NumQubits:    53,
		BasisGates:   []string{"CZ", "RZ", "RX"},
		Connectivity: sycamore53Connectivity,
	}

	// willowConnectivity is the 105-qubit diamond-shaped 2D grid coupling map
	// for Google Willow. Qubits are numbered row-major 0-104 across a
	// diamond spanning rows 0-12 (widest at row 6 with 15 qubits), 182 edges.
	// Source: cirq-google Willow device spec proto
	willowConnectivity = []QubitPair{
		// Row 0: (0,6)=0, (0,7)=1, (0,8)=2
		{0, 1}, {0, 4}, {1, 2}, {1, 5}, {2, 6},
		// Row 1: (1,5)=3 .. (1,8)=6
		{3, 4}, {3, 8}, {4, 5}, {4, 9}, {5, 6}, {5, 10}, {6, 11},
		// Row 2: (2,4)=7 .. (2,10)=13
		{7, 8}, {7, 15}, {8, 9}, {8, 16}, {9, 10}, {9, 17}, {10, 11},
		{10, 18}, {11, 12}, {11, 19}, {12, 13}, {12, 20}, {13, 21},
		// Row 3: (3,3)=14 .. (3,10)=21
		{14, 15}, {14, 23}, {15, 16}, {15, 24}, {16, 17}, {16, 25},
		{17, 18}, {17, 26}, {18, 19}, {18, 27}, {19, 20}, {19, 28},
		{20, 21}, {20, 29}, {21, 30},
		// Row 4: (4,2)=22 .. (4,12)=32
		{22, 23}, {22, 34}, {23, 24}, {23, 35}, {24, 25}, {24, 36},
		{25, 26}, {25, 37}, {26, 27}, {26, 38}, {27, 28}, {27, 39},
		{28, 29}, {28, 40}, {29, 30}, {29, 41}, {30, 31}, {30, 42},
		{31, 32}, {31, 43}, {32, 44},
		// Row 5: (5,1)=33 .. (5,12)=44
		{33, 34}, {33, 46}, {34, 35}, {34, 47}, {35, 36}, {35, 48},
		{36, 37}, {36, 49}, {37, 38}, {37, 50}, {38, 39}, {38, 51},
		{39, 40}, {39, 52}, {40, 41}, {40, 53}, {41, 42}, {41, 54},
		{42, 43}, {42, 55}, {43, 44}, {43, 56}, {44, 57},
		// Row 6: (6,0)=45 .. (6,14)=59
		{45, 46}, {46, 47}, {47, 48}, {47, 60}, {48, 49}, {48, 61},
		{49, 50}, {49, 62}, {50, 51}, {50, 63}, {51, 52}, {51, 64},
		{52, 53}, {52, 65}, {53, 54}, {53, 66}, {54, 55}, {54, 67},
		{55, 56}, {55, 68}, {56, 57}, {56, 69}, {57, 58}, {57, 70},
		{58, 59}, {58, 71},
		// Row 7: (7,2)=60 .. (7,13)=71
		{60, 61}, {60, 72}, {61, 62}, {61, 73}, {62, 63}, {62, 74},
		{63, 64}, {63, 75}, {64, 65}, {64, 76}, {65, 66}, {65, 77},
		{66, 67}, {66, 78}, {67, 68}, {67, 79}, {68, 69}, {68, 80},
		{69, 70}, {69, 81}, {70, 71}, {70, 82},
		// Row 8: (8,2)=72 .. (8,12)=82
		{72, 73}, {73, 74}, {74, 75}, {74, 83}, {75, 76}, {75, 84},
		{76, 77}, {76, 85}, {77, 78}, {77, 86}, {78, 79}, {78, 87},
		{79, 80}, {79, 88}, {80, 81}, {80, 89}, {81, 82}, {81, 90},
		// Row 9: (9,4)=83 .. (9,11)=90
		{83, 84}, {83, 91}, {84, 85}, {84, 92}, {85, 86}, {85, 93},
		{86, 87}, {86, 94}, {87, 88}, {87, 95}, {88, 89}, {88, 96},
		{89, 90}, {89, 97},
		// Row 10: (10,4)=91 .. (10,10)=97
		{91, 92}, {92, 93}, {93, 94}, {93, 98}, {94, 95}, {94, 99},
		{95, 96}, {95, 100}, {96, 97}, {96, 101},
		// Row 11: (11,6)=98 .. (11,9)=101
		{98, 99}, {98, 102}, {99, 100}, {99, 103}, {100, 101}, {100, 104},
		// Row 12: (12,6)=102 .. (12,8)=104
		{102, 103}, {103, 104},
	}

	GoogleWillow = Target{
		Name:         "Google Willow",
		NumQubits:    105,
		BasisGates:   []string{"CZ", "RZ", "RX"},
		Connectivity: willowConnectivity,
	}

	// ankaa3Connectivity is the 84-qubit square lattice coupling map for
	// Rigetti Ankaa-3 (12 rows x 7 columns, qubits numbered row-major 0-83).
	// Individual qubits or couplers may be offline depending on calibration.
	// Source: Amazon Braket device properties / Rigetti QCS ISA
	ankaa3Connectivity = []QubitPair{
		// Row 0
		{0, 1}, {0, 7}, {1, 2}, {1, 8}, {2, 3}, {2, 9}, {3, 4}, {3, 10},
		{4, 5}, {4, 11}, {5, 6}, {5, 12}, {6, 13},
		// Row 1
		{7, 8}, {7, 14}, {8, 9}, {8, 15}, {9, 10}, {9, 16}, {10, 11}, {10, 17},
		{11, 12}, {11, 18}, {12, 13}, {12, 19}, {13, 20},
		// Row 2
		{14, 15}, {14, 21}, {15, 16}, {15, 22}, {16, 17}, {16, 23},
		{17, 18}, {17, 24}, {18, 19}, {18, 25}, {19, 20}, {19, 26}, {20, 27},
		// Row 3
		{21, 22}, {21, 28}, {22, 23}, {22, 29}, {23, 24}, {23, 30},
		{24, 25}, {24, 31}, {25, 26}, {25, 32}, {26, 27}, {26, 33}, {27, 34},
		// Row 4
		{28, 29}, {28, 35}, {29, 30}, {29, 36}, {30, 31}, {30, 37},
		{31, 32}, {31, 38}, {32, 33}, {32, 39}, {33, 34}, {33, 40}, {34, 41},
		// Row 5
		{35, 36}, {35, 42}, {36, 37}, {36, 43}, {37, 38}, {37, 44},
		{38, 39}, {38, 45}, {39, 40}, {39, 46}, {40, 41}, {40, 47}, {41, 48},
		// Row 6
		{42, 43}, {42, 49}, {43, 44}, {43, 50}, {44, 45}, {44, 51},
		{45, 46}, {45, 52}, {46, 47}, {46, 53}, {47, 48}, {47, 54}, {48, 55},
		// Row 7
		{49, 50}, {49, 56}, {50, 51}, {50, 57}, {51, 52}, {51, 58},
		{52, 53}, {52, 59}, {53, 54}, {53, 60}, {54, 55}, {54, 61}, {55, 62},
		// Row 8
		{56, 57}, {56, 63}, {57, 58}, {57, 64}, {58, 59}, {58, 65},
		{59, 60}, {59, 66}, {60, 61}, {60, 67}, {61, 62}, {61, 68}, {62, 69},
		// Row 9
		{63, 64}, {63, 70}, {64, 65}, {64, 71}, {65, 66}, {65, 72},
		{66, 67}, {66, 73}, {67, 68}, {67, 74}, {68, 69}, {68, 75}, {69, 76},
		// Row 10
		{70, 71}, {70, 77}, {71, 72}, {71, 78}, {72, 73}, {72, 79},
		{73, 74}, {73, 80}, {74, 75}, {74, 81}, {75, 76}, {75, 82}, {76, 83},
		// Row 11
		{77, 78}, {78, 79}, {79, 80}, {80, 81}, {81, 82}, {82, 83},
	}

	RigettiAnkaa = Target{
		Name:         "Rigetti Ankaa-3",
		NumQubits:    84,
		BasisGates:   []string{"CZ", "RX", "RZ"},
		Connectivity: ankaa3Connectivity,
		// Native hardware gates are RX, RZ, and iSWAP, but the QCS translation
		// service accepts CZ and decomposes it to iSWAP internally. We use CZ
		// here because goqu lacks a native iSWAP gate and CZ is the standard
		// 2Q entangling gate in Quil programs submitted to QCS.
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
	for _, tgt := range []Target{IBMEagle, GoogleSycamore, GoogleWillow, RigettiAnkaa} {
		if err := tgt.ValidateConnectivity(); err != nil {
			panic(err)
		}
	}
}
