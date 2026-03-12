package operator

import (
	"math"
	"testing"

	"github.com/splch/qgo/sim/noise"
)

func TestAverageGateFidelity_Identity(t *testing.T) {
	k := identity1Q()
	f := AverageGateFidelity(k)
	if math.Abs(f-1.0) > testTol {
		t.Errorf("AverageGateFidelity(identity) = %f, want 1.0", f)
	}
}

func TestProcessFidelity_Identity(t *testing.T) {
	k := identity1Q()
	f := ProcessFidelity(k)
	if math.Abs(f-1.0) > testTol {
		t.Errorf("ProcessFidelity(identity) = %f, want 1.0", f)
	}
}

func TestAverageGateFidelity_Depolarizing(t *testing.T) {
	// For single-qubit depolarizing channel with parameter p:
	// F_pro = (1-p) + p/3 * (0+0+0) = 1 - p + p * (Tr(I)^2 + Tr(X)^2 + Tr(Y)^2 + Tr(Z)^2 - Tr(I)^2) / (3*4)
	// Actually, let's compute directly:
	// Kraus operators: sqrt(1-p)*I, sqrt(p/3)*X, sqrt(p/3)*Y, sqrt(p/3)*Z
	// F_pro = (1/d^2) * sum_k |Tr(E_k)|^2
	// = (1/4) * ((1-p)*|Tr(I)|^2 + (p/3)*|Tr(X)|^2 + (p/3)*|Tr(Y)|^2 + (p/3)*|Tr(Z)|^2)
	// = (1/4) * ((1-p)*4 + (p/3)*0 + (p/3)*0 + (p/3)*0)
	// = 1 - p
	// F_avg = (d * F_pro + 1) / (d + 1) = (2*(1-p) + 1) / 3 = (3 - 2p) / 3 = 1 - 2p/3

	tests := []struct {
		p    float64
		fAvg float64
	}{
		{0.0, 1.0},
		{0.1, 1 - 2*0.1/3},
		{0.3, 1 - 2*0.3/3},
		{0.75, 1 - 2*0.75/3}, // maximally mixed
	}
	for _, tt := range tests {
		ch := noise.Depolarizing1Q(tt.p)
		k := FromChannel(ch)
		f := AverageGateFidelity(k)
		if math.Abs(f-tt.fAvg) > testTol {
			t.Errorf("AverageGateFidelity(depol(%.2f)) = %f, want %f", tt.p, f, tt.fAvg)
		}
	}
}

func TestProcessFidelity_Depolarizing(t *testing.T) {
	p := 0.3
	ch := noise.Depolarizing1Q(p)
	k := FromChannel(ch)
	f := ProcessFidelity(k)
	expected := 1 - p
	if math.Abs(f-expected) > testTol {
		t.Errorf("ProcessFidelity(depol(%.2f)) = %f, want %f", p, f, expected)
	}
}

func TestAverageGateFidelity_AmplitudeDamping(t *testing.T) {
	// For amplitude damping with gamma:
	// E0 = [[1,0],[0,sqrt(1-gamma)]], E1 = [[0,sqrt(gamma)],[0,0]]
	// Tr(E0) = 1 + sqrt(1-gamma), Tr(E1) = 0
	// F_pro = |1 + sqrt(1-gamma)|^2 / 4
	// F_avg = (2*F_pro + 1) / 3
	gamma := 0.3
	ch := noise.AmplitudeDamping(gamma)
	k := FromChannel(ch)
	f := AverageGateFidelity(k)

	trE0 := 1 + math.Sqrt(1-gamma)
	fPro := trE0 * trE0 / 4
	expected := (2*fPro + 1) / 3
	if math.Abs(f-expected) > testTol {
		t.Errorf("AverageGateFidelity(amp_damp(%.2f)) = %f, want %f", gamma, f, expected)
	}
}

func TestFidelity_BoundsCheck(t *testing.T) {
	// Fidelity should be in [0, 1] for all CPTP channels.
	channels := []noise.Channel{
		noise.Depolarizing1Q(0.5),
		noise.AmplitudeDamping(0.8),
		noise.PhaseDamping(0.6),
		noise.BitFlip(0.4),
		noise.PhaseFlip(0.3),
	}
	for _, ch := range channels {
		k := FromChannel(ch)
		fAvg := AverageGateFidelity(k)
		fPro := ProcessFidelity(k)
		if fAvg < -testTol || fAvg > 1+testTol {
			t.Errorf("%s: AverageGateFidelity = %f, out of [0,1]", ch.Name(), fAvg)
		}
		if fPro < -testTol || fPro > 1+testTol {
			t.Errorf("%s: ProcessFidelity = %f, out of [0,1]", ch.Name(), fPro)
		}
	}
}

func TestAverageGateFidelity_2Q_Identity(t *testing.T) {
	id4 := identityMatrix(4)
	k := NewKraus(2, [][]complex128{id4})
	f := AverageGateFidelity(k)
	if math.Abs(f-1.0) > testTol {
		t.Errorf("AverageGateFidelity(2Q identity) = %f, want 1.0", f)
	}
}

func TestProcessFidelity_Relationship(t *testing.T) {
	// Verify: F_avg = (d * F_pro + 1) / (d + 1)
	ch := noise.Depolarizing1Q(0.4)
	k := FromChannel(ch)
	fAvg := AverageGateFidelity(k)
	fPro := ProcessFidelity(k)
	expected := (2*fPro + 1) / 3 // d=2 for 1 qubit
	if math.Abs(fAvg-expected) > testTol {
		t.Errorf("F_avg = %f, but (d*F_pro+1)/(d+1) = %f", fAvg, expected)
	}
}
