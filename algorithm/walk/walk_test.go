package walk_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/walk"
)

func TestRun_ZeroSteps(t *testing.T) {
	res, err := walk.Run(context.Background(), walk.Config{Steps: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cl, qu := res.Classical, res.Quantum
	if len(cl) != 1 || len(qu) != 1 {
		t.Fatalf("expected length 1, got classical=%d quantum=%d", len(cl), len(qu))
	}
	if cl[0] != 1.0 {
		t.Errorf("classical[0] = %f, want 1.0", cl[0])
	}
	if qu[0] != 1.0 {
		t.Errorf("quantum[0] = %f, want 1.0", qu[0])
	}
}

func TestRun_Negative(t *testing.T) {
	_, err := walk.Run(context.Background(), walk.Config{Steps: -1})
	if err == nil {
		t.Error("expected error for negative steps")
	}
}

func TestRun_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := walk.Run(ctx, walk.Config{Steps: 10})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRun_OneStep(t *testing.T) {
	res, err := walk.Run(context.Background(), walk.Config{Steps: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cl, qu := res.Classical, res.Quantum
	// Size should be 3: positions -1, 0, 1.
	if len(cl) != 3 || len(qu) != 3 {
		t.Fatalf("expected length 3, got classical=%d quantum=%d", len(cl), len(qu))
	}
	// Classical: after 1 step, P(-1) = 0.5, P(0) = 0, P(1) = 0.5.
	assertClose(t, "classical[-1]", cl[0], 0.5)
	assertClose(t, "classical[0]", cl[1], 0.0)
	assertClose(t, "classical[1]", cl[2], 0.5)
}

func TestRun_Distribution_Sums_To_One(t *testing.T) {
	for _, steps := range []int{1, 5, 10, 20, 50} {
		res, err := walk.Run(context.Background(), walk.Config{Steps: steps})
		if err != nil {
			t.Errorf("steps=%d: unexpected error: %v", steps, err)
			continue
		}
		cl, qu := res.Classical, res.Quantum
		size := 2*steps + 1
		if len(cl) != size || len(qu) != size {
			t.Errorf("steps=%d: expected length %d, got classical=%d quantum=%d",
				steps, size, len(cl), len(qu))
			continue
		}
		clSum := sum(cl)
		quSum := sum(qu)
		assertClose(t, "classical sum", clSum, 1.0)
		assertClose(t, "quantum sum", quSum, 1.0)
	}
}

func TestRun_Classical_Symmetry(t *testing.T) {
	steps := 20
	res, err := walk.Run(context.Background(), walk.Config{Steps: steps})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cl := res.Classical
	// Classical random walk is symmetric around the origin.
	for i := range steps {
		left := cl[steps-1-i]
		right := cl[steps+1+i]
		if math.Abs(left-right) > 1e-12 {
			t.Errorf("classical asymmetry at offset %d: left=%f right=%f", i+1, left, right)
		}
	}
}

func TestRun_Quantum_Asymmetry(t *testing.T) {
	// The Hadamard walk starting from |R> is known to be asymmetric.
	// The direction of the bias depends on the coin convention; the key
	// property is that the mean is non-zero (unlike the symmetric classical walk).
	steps := 50
	res, err := walk.Run(context.Background(), walk.Config{Steps: steps})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	qu := res.Quantum

	// Compute the mean position.
	mean := 0.0
	for i, p := range qu {
		pos := float64(i - steps)
		mean += pos * p
	}
	// The mean should be significantly different from zero.
	if math.Abs(mean) < 1.0 {
		t.Errorf("expected asymmetric quantum walk (|mean| > 1), got mean=%f", mean)
	}
}

func TestRun_Quantum_Ballistic_Spread(t *testing.T) {
	// The quantum walk spreads ballistically: standard deviation grows
	// linearly with steps, unlike classical which grows as sqrt(steps).
	stepsA := 50
	stepsB := 100

	resA, err := walk.Run(context.Background(), walk.Config{Steps: stepsA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resB, err := walk.Run(context.Background(), walk.Config{Steps: stepsB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdA := stddev(resA.Quantum, stepsA)
	stdB := stddev(resB.Quantum, stepsB)

	// Ratio of std devs should be close to ratio of steps (linear scaling).
	ratio := stdB / stdA
	expected := float64(stepsB) / float64(stepsA)
	// Allow generous tolerance since we're just checking ballistic > diffusive.
	if ratio < expected*0.7 {
		t.Errorf("quantum walk not ballistic: stddev ratio=%f, expected near %f", ratio, expected)
	}
}

func TestRun_Classical_Diffusive_Spread(t *testing.T) {
	// Classical walk spreads diffusively: std dev grows as sqrt(steps).
	stepsA := 50
	stepsB := 200

	resA, err := walk.Run(context.Background(), walk.Config{Steps: stepsA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resB, err := walk.Run(context.Background(), walk.Config{Steps: stepsB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdA := stddev(resA.Classical, stepsA)
	stdB := stddev(resB.Classical, stepsB)

	ratio := stdB / stdA
	expected := math.Sqrt(float64(stepsB) / float64(stepsA))
	if math.Abs(ratio-expected) > expected*0.1 {
		t.Errorf("classical walk not diffusive: stddev ratio=%f, expected %f", ratio, expected)
	}
}

func sum(s []float64) float64 {
	total := 0.0
	for _, v := range s {
		total += v
	}
	return total
}

func stddev(dist []float64, steps int) float64 {
	mean := 0.0
	for i, p := range dist {
		pos := float64(i - steps)
		mean += pos * p
	}
	variance := 0.0
	for i, p := range dist {
		pos := float64(i - steps)
		d := pos - mean
		variance += d * d * p
	}
	return math.Sqrt(variance)
}

func assertClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("%s = %f, want %f", label, got, want)
	}
}
