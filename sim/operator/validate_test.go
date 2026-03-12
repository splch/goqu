package operator

import (
	"testing"

	"github.com/splch/qgo/sim/noise"
)

func TestIsCP_Identity(t *testing.T) {
	k := identity1Q()
	c := KrausToChoi(k)
	if !IsCP(c, 1e-10) {
		t.Error("identity channel should be CP")
	}
}

func TestIsTP_Identity(t *testing.T) {
	k := identity1Q()
	if !IsTP(k, 1e-10) {
		t.Error("identity channel should be TP")
	}
}

func TestIsCPTP_Identity(t *testing.T) {
	k := identity1Q()
	if !IsCPTP(k, 1e-10) {
		t.Error("identity channel should be CPTP")
	}
}

func TestIsCPTP_AllNoiseChannels(t *testing.T) {
	channels := []noise.Channel{
		noise.Depolarizing1Q(0.1),
		noise.Depolarizing1Q(0.5),
		noise.Depolarizing1Q(0.75),
		noise.AmplitudeDamping(0.3),
		noise.PhaseDamping(0.4),
		noise.BitFlip(0.2),
		noise.PhaseFlip(0.15),
		noise.ThermalRelaxation(100, 80, 10),
	}
	for _, ch := range channels {
		k := FromChannel(ch)
		if !IsCPTP(k, 1e-8) {
			t.Errorf("channel %s should be CPTP", ch.Name())
		}
	}
}

func TestIsCPTP_2Q(t *testing.T) {
	ch := noise.Depolarizing2Q(0.3)
	k := FromChannel(ch)
	if !IsCPTP(k, 1e-7) {
		t.Error("2-qubit depolarizing should be CPTP")
	}
}

func TestIsTP_NonTP(t *testing.T) {
	// Construct a non-trace-preserving "channel": just a single operator
	// that is not unitary and doesn't satisfy sum E_k^dag E_k = I.
	// Use 0.5 * I, which gives 0.25 * I, not I.
	half := complex(0.5, 0)
	ops := [][]complex128{{half, 0, 0, half}}
	k := NewKraus(1, ops)
	if IsTP(k, 1e-10) {
		t.Error("0.5*I is not trace-preserving, should return false")
	}
}

func TestIsCP_NonCP(t *testing.T) {
	// A matrix that is not positive semidefinite.
	// Use a Choi matrix with a negative eigenvalue.
	m := []complex128{
		1, 0, 0, 0,
		0, -1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	c := NewChoi(1, m)
	if IsCP(c, 1e-10) {
		t.Error("matrix with negative eigenvalue should not be CP")
	}
}
