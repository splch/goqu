package pauli_test

import (
	"fmt"
	"math"

	"github.com/splch/qgo/sim/pauli"
)

func ExampleExpect_bell() {
	// Bell state |Φ+> = (|00> + |11>) / √2
	s2 := 1 / math.Sqrt2
	bell := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}

	zz, _ := pauli.Parse("ZZ")
	xx, _ := pauli.Parse("XX")
	yy, _ := pauli.Parse("YY")

	fmt.Printf("<ZZ> = %.0f\n", real(pauli.Expect(bell, zz)))
	fmt.Printf("<XX> = %.0f\n", real(pauli.Expect(bell, xx)))
	fmt.Printf("<YY> = %.0f\n", real(pauli.Expect(bell, yy)))
	// Output:
	// <ZZ> = 1
	// <XX> = 1
	// <YY> = -1
}

func ExampleExpectSum_hamiltonian() {
	// Simple 2-qubit Hamiltonian: H = 0.5*ZZ + 0.3*XI
	// On |00>: <ZZ>=1, <XI>=0 → <H> = 0.5
	ket00 := []complex128{1, 0, 0, 0}

	zz := pauli.NewPauliString(0.5, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	xi := pauli.NewPauliString(0.3, map[int]pauli.Pauli{0: pauli.X}, 2)
	h, _ := pauli.NewPauliSum([]pauli.PauliString{zz, xi})

	fmt.Printf("<H> = %.1f\n", real(pauli.ExpectSum(ket00, h)))
	// Output:
	// <H> = 0.5
}

func ExampleExpectFromCounts() {
	// Z-basis measurement counts for a Bell state
	counts := map[string]int{"00": 500, "11": 500}
	ev := pauli.ExpectFromCounts(counts, []int{0, 1})
	fmt.Printf("<Z0Z1> = %.1f\n", ev)
	// Output:
	// <Z0Z1> = 1.0
}
