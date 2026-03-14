//go:build cuda

package cuda

import (
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/statevector"
)

// BenchmarkGPU_GHZ benchmarks GPU GHZ circuit creation across qubit counts.
func BenchmarkGPU_GHZ(b *testing.B) {
	for _, nq := range []int{12, 16, 20, 24, 28} {
		bld := builder.New("ghz", nq)
		bld.H(0)
		for i := range nq - 1 {
			bld.CNOT(i, i+1)
		}
		c, err := bld.Build()
		if err != nil {
			b.Fatal(err)
		}

		b.Run(qName("GPU", nq), func(b *testing.B) {
			sim, err := New(nq)
			if err != nil {
				b.Skip("CUDA not available:", err)
			}
			defer sim.Close()
			b.ResetTimer()
			for range b.N {
				sim.Evolve(c)
			}
		})
	}
}

// BenchmarkCPU_GHZ benchmarks CPU GHZ for comparison.
func BenchmarkCPU_GHZ(b *testing.B) {
	for _, nq := range []int{12, 16, 20, 24, 28} {
		bld := builder.New("ghz", nq)
		bld.H(0)
		for i := range nq - 1 {
			bld.CNOT(i, i+1)
		}
		c, err := bld.Build()
		if err != nil {
			b.Fatal(err)
		}

		b.Run(qName("CPU", nq), func(b *testing.B) {
			sim := statevector.New(nq)
			b.ResetTimer()
			for range b.N {
				sim.Evolve(c)
			}
		})
	}
}

func qName(prefix string, nq int) string {
	return prefix + "_" + string(rune('0'+nq/10)) + string(rune('0'+nq%10)) + "Q"
}
