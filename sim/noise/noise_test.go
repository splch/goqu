package noise

import (
	"math"
	"math/cmplx"
	"testing"
)

// checkKrausComplete verifies sum_k E_k-dagger E_k = I for a channel.
func checkKrausComplete(t *testing.T, ch Channel) {
	t.Helper()
	dim := 1 << ch.Qubits()
	// Compute sum_k E_k-dagger E_k
	sum := make([]complex128, dim*dim)
	for _, ek := range ch.Kraus() {
		for i := range dim {
			for j := range dim {
				var v complex128
				for k := range dim {
					v += cmplx.Conj(ek[k*dim+i]) * ek[k*dim+j]
				}
				sum[i*dim+j] += v
			}
		}
	}
	// Check approximately equal to identity
	for i := range dim {
		for j := range dim {
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			got := sum[i*dim+j]
			if cmplx.Abs(got-want) > 1e-10 {
				t.Errorf("Kraus not complete for %s: sum[%d][%d]=%v, want %v",
					ch.Name(), i, j, got, want)
			}
		}
	}
}

func TestDepolarizing1Q_Completeness(t *testing.T) {
	for _, p := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := Depolarizing1Q(p)
		checkKrausComplete(t, ch)
	}
}

func TestDepolarizing1Q_ZeroP(t *testing.T) {
	ch := Depolarizing1Q(0)
	kraus := ch.Kraus()
	if len(kraus) != 4 {
		t.Fatalf("expected 4 Kraus ops, got %d", len(kraus))
	}
	// At p=0, first op should be identity, others should be zero matrices
	e0 := kraus[0]
	if cmplx.Abs(e0[0]-1) > 1e-14 || cmplx.Abs(e0[3]-1) > 1e-14 {
		t.Error("E0 should be identity at p=0")
	}
	// Other operators should have zero scale
	for k := 1; k < 4; k++ {
		for _, v := range kraus[k] {
			if cmplx.Abs(v) > 1e-14 {
				t.Errorf("Kraus[%d] should be zero at p=0, got %v", k, v)
			}
		}
	}
}

func TestDepolarizing1Q_Properties(t *testing.T) {
	ch := Depolarizing1Q(0.3)
	if ch.Name() != "depolarizing1q(0.3000)" {
		t.Errorf("unexpected name: %s", ch.Name())
	}
	if ch.Qubits() != 1 {
		t.Errorf("expected 1 qubit, got %d", ch.Qubits())
	}
	if len(ch.Kraus()) != 4 {
		t.Errorf("expected 4 Kraus ops, got %d", len(ch.Kraus()))
	}
}

func TestDepolarizing1Q_Panic(t *testing.T) {
	for _, p := range []float64{-0.1, 1.1} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Depolarizing1Q(%f) should panic", p)
				}
			}()
			Depolarizing1Q(p)
		}()
	}
}

func TestDepolarizing2Q_Completeness(t *testing.T) {
	for _, p := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := Depolarizing2Q(p)
		checkKrausComplete(t, ch)
	}
}

func TestDepolarizing2Q_Properties(t *testing.T) {
	ch := Depolarizing2Q(0.05)
	if ch.Qubits() != 2 {
		t.Errorf("expected 2 qubits, got %d", ch.Qubits())
	}
	if len(ch.Kraus()) != 16 {
		t.Errorf("expected 16 Kraus ops, got %d", len(ch.Kraus()))
	}
}

func TestAmplitudeDamping_Completeness(t *testing.T) {
	for _, g := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := AmplitudeDamping(g)
		checkKrausComplete(t, ch)
	}
}

func TestAmplitudeDamping_ZeroGamma(t *testing.T) {
	ch := AmplitudeDamping(0)
	kraus := ch.Kraus()
	if len(kraus) != 2 {
		t.Fatalf("expected 2 Kraus ops, got %d", len(kraus))
	}
	// E0 should be identity
	e0 := kraus[0]
	if cmplx.Abs(e0[0]-1) > 1e-14 || cmplx.Abs(e0[3]-1) > 1e-14 {
		t.Error("E0 should be identity at gamma=0")
	}
	// E1 should be zero
	for _, v := range kraus[1] {
		if cmplx.Abs(v) > 1e-14 {
			t.Error("E1 should be zero at gamma=0")
		}
	}
}

func TestAmplitudeDamping_FullDecay(t *testing.T) {
	ch := AmplitudeDamping(1.0)
	kraus := ch.Kraus()
	// E0 = [[1,0],[0,0]], E1 = [[0,1],[0,0]]
	e0 := kraus[0]
	e1 := kraus[1]
	if cmplx.Abs(e0[0]-1) > 1e-14 || cmplx.Abs(e0[3]) > 1e-14 {
		t.Error("E0 incorrect at gamma=1")
	}
	if cmplx.Abs(e1[1]-1) > 1e-14 {
		t.Error("E1 incorrect at gamma=1")
	}
}

func TestAmplitudeDamping_Panic(t *testing.T) {
	for _, g := range []float64{-0.1, 1.1} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("AmplitudeDamping(%f) should panic", g)
				}
			}()
			AmplitudeDamping(g)
		}()
	}
}

func TestPhaseDamping_Completeness(t *testing.T) {
	for _, l := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := PhaseDamping(l)
		checkKrausComplete(t, ch)
	}
}

func TestPhaseDamping_ZeroLambda(t *testing.T) {
	ch := PhaseDamping(0)
	kraus := ch.Kraus()
	// E0 should be identity, E1 should be zero
	e0 := kraus[0]
	if cmplx.Abs(e0[0]-1) > 1e-14 || cmplx.Abs(e0[3]-1) > 1e-14 {
		t.Error("E0 should be identity at lambda=0")
	}
	for _, v := range kraus[1] {
		if cmplx.Abs(v) > 1e-14 {
			t.Error("E1 should be zero at lambda=0")
		}
	}
}

func TestPhaseDamping_Panic(t *testing.T) {
	for _, l := range []float64{-0.1, 1.1} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("PhaseDamping(%f) should panic", l)
				}
			}()
			PhaseDamping(l)
		}()
	}
}

func TestBitFlip_Completeness(t *testing.T) {
	for _, p := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := BitFlip(p)
		checkKrausComplete(t, ch)
	}
}

func TestBitFlip_Properties(t *testing.T) {
	ch := BitFlip(0.1)
	if ch.Qubits() != 1 {
		t.Errorf("expected 1 qubit, got %d", ch.Qubits())
	}
	if len(ch.Kraus()) != 2 {
		t.Errorf("expected 2 Kraus ops, got %d", len(ch.Kraus()))
	}
}

func TestBitFlip_Panic(t *testing.T) {
	for _, p := range []float64{-0.1, 1.1} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("BitFlip(%f) should panic", p)
				}
			}()
			BitFlip(p)
		}()
	}
}

func TestPhaseFlip_Completeness(t *testing.T) {
	for _, p := range []float64{0, 0.01, 0.1, 0.5, 1.0} {
		ch := PhaseFlip(p)
		checkKrausComplete(t, ch)
	}
}

func TestPhaseFlip_Properties(t *testing.T) {
	ch := PhaseFlip(0.2)
	if ch.Qubits() != 1 {
		t.Errorf("expected 1 qubit, got %d", ch.Qubits())
	}
	if len(ch.Kraus()) != 2 {
		t.Errorf("expected 2 Kraus ops, got %d", len(ch.Kraus()))
	}
}

func TestPhaseFlip_Panic(t *testing.T) {
	for _, p := range []float64{-0.1, 1.1} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("PhaseFlip(%f) should panic", p)
				}
			}()
			PhaseFlip(p)
		}()
	}
}

func TestThermalRelaxation_Completeness(t *testing.T) {
	cases := []struct {
		t1, t2, time float64
	}{
		{100, 50, 10},
		{100, 100, 10},
		{100, 200, 10},
		{50, 30, 5},
		{100, 50, 0},
	}
	for _, tc := range cases {
		ch := ThermalRelaxation(tc.t1, tc.t2, tc.time)
		checkKrausComplete(t, ch)
	}
}

func TestThermalRelaxation_ZeroTime(t *testing.T) {
	ch := ThermalRelaxation(100, 50, 0)
	// With zero gate time, should have minimal noise
	kraus := ch.Kraus()
	// First operator should be close to identity
	e0 := kraus[0]
	if cmplx.Abs(e0[0]-1) > 1e-10 || cmplx.Abs(e0[3]-1) > 1e-10 {
		t.Errorf("E0 should be near identity at time=0, got [%v, %v; %v, %v]",
			e0[0], e0[1], e0[2], e0[3])
	}
}

func TestThermalRelaxation_Panics(t *testing.T) {
	// t2 > 2*t1
	expectPanic(t, "t2>2*t1", func() { ThermalRelaxation(50, 110, 10) })
	// negative t1
	expectPanic(t, "negative t1", func() { ThermalRelaxation(-1, 1, 10) })
	// negative t2
	expectPanic(t, "negative t2", func() { ThermalRelaxation(100, -1, 10) })
	// negative time
	expectPanic(t, "negative time", func() { ThermalRelaxation(100, 50, -1) })
}

func expectPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic", name)
		}
	}()
	f()
}

func TestReadoutError_Apply(t *testing.T) {
	re := NewReadoutError(0.1, 0.05)
	// Pure |0> state: p0=1, p1=0
	np0, np1 := re.Apply(1, 0)
	// np0 = (1-0.1)*1 + 0.05*0 = 0.9
	// np1 = 0.1*1 + (1-0.05)*0 = 0.1
	if math.Abs(np0-0.9) > 1e-14 {
		t.Errorf("expected np0=0.9, got %f", np0)
	}
	if math.Abs(np1-0.1) > 1e-14 {
		t.Errorf("expected np1=0.1, got %f", np1)
	}

	// Pure |1> state: p0=0, p1=1
	np0, np1 = re.Apply(0, 1)
	// np0 = (1-0.1)*0 + 0.05*1 = 0.05
	// np1 = 0.1*0 + (1-0.05)*1 = 0.95
	if math.Abs(np0-0.05) > 1e-14 {
		t.Errorf("expected np0=0.05, got %f", np0)
	}
	if math.Abs(np1-0.95) > 1e-14 {
		t.Errorf("expected np1=0.95, got %f", np1)
	}

	// Equal superposition: p0=0.5, p1=0.5
	np0, np1 = re.Apply(0.5, 0.5)
	// np0 = 0.9*0.5 + 0.05*0.5 = 0.475
	// np1 = 0.1*0.5 + 0.95*0.5 = 0.525
	if math.Abs(np0-0.475) > 1e-14 {
		t.Errorf("expected np0=0.475, got %f", np0)
	}
	if math.Abs(np1-0.525) > 1e-14 {
		t.Errorf("expected np1=0.525, got %f", np1)
	}
}

func TestReadoutError_ProbConservation(t *testing.T) {
	re := NewReadoutError(0.1, 0.05)
	for _, p0 := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
		p1 := 1 - p0
		np0, np1 := re.Apply(p0, p1)
		sum := np0 + np1
		if math.Abs(sum-1) > 1e-14 {
			t.Errorf("probabilities don't sum to 1: %f + %f = %f", np0, np1, sum)
		}
	}
}

func TestReadoutError_Panic(t *testing.T) {
	expectPanic(t, "p01<0", func() { NewReadoutError(-0.1, 0.5) })
	expectPanic(t, "p01>1", func() { NewReadoutError(1.1, 0.5) })
	expectPanic(t, "p10<0", func() { NewReadoutError(0.5, -0.1) })
	expectPanic(t, "p10>1", func() { NewReadoutError(0.5, 1.1) })
}

func TestNoiseModel_Lookup_ResolutionOrder(t *testing.T) {
	m := New()

	defaultCh := Depolarizing1Q(0.01)
	gateCh := Depolarizing1Q(0.05)
	qubitCh := Depolarizing1Q(0.10)

	m.AddDefaultError(1, defaultCh)
	m.AddGateError("H", gateCh)
	m.AddGateQubitError("H", []int{0}, qubitCh)

	// Most specific: gate+qubit
	ch := m.Lookup("H", []int{0})
	if ch != qubitCh {
		t.Error("expected qubit-specific channel")
	}

	// Gate-level: H on qubit 1 (no qubit-specific entry)
	ch = m.Lookup("H", []int{1})
	if ch != gateCh {
		t.Error("expected gate-level channel")
	}

	// Default: X on qubit 0 (no gate-specific entry)
	ch = m.Lookup("X", []int{0})
	if ch != defaultCh {
		t.Error("expected default channel")
	}

	// No match
	ch = m.Lookup("CNOT", []int{0, 1})
	if ch != nil {
		t.Error("expected nil for unmatched 2-qubit gate")
	}
}

func TestNoiseModel_ReadoutFor(t *testing.T) {
	m := New()

	re := NewReadoutError(0.1, 0.05)
	m.AddReadoutError(0, re)

	got := m.ReadoutFor(0)
	if got != re {
		t.Error("expected readout error for qubit 0")
	}

	got = m.ReadoutFor(1)
	if got != nil {
		t.Error("expected nil for qubit 1")
	}
}

func TestFormatKey(t *testing.T) {
	key := formatKey("CNOT", []int{0, 1})
	if key != "CNOT:0,1" {
		t.Errorf("expected CNOT:0,1, got %s", key)
	}
	key = formatKey("H", []int{3})
	if key != "H:3" {
		t.Errorf("expected H:3, got %s", key)
	}
}

func TestDepolarizing1Q_MaxMixed_KrausValues(t *testing.T) {
	ch := Depolarizing1Q(0.75)
	kraus := ch.Kraus()
	// At p=0.75: sqrt(1-p)=0.5, sqrt(p/3)=0.5
	// E0 = 0.5*I, E1 = 0.5*X, E2 = 0.5*Y, E3 = 0.5*Z
	// E0[0] and E0[3] should be 0.5
	if math.Abs(real(kraus[0][0])-0.5) > 1e-10 {
		t.Errorf("E0[0,0] = %v, want 0.5", kraus[0][0])
	}
	if math.Abs(real(kraus[0][3])-0.5) > 1e-10 {
		t.Errorf("E0[1,1] = %v, want 0.5", kraus[0][3])
	}
}

func TestThermalRelaxation_T2EqualsT1(t *testing.T) {
	ch := ThermalRelaxation(100, 100, 10)
	checkKrausComplete(t, ch)
}

func TestThermalRelaxation_T2Equals2T1(t *testing.T) {
	ch := ThermalRelaxation(50, 100, 10)
	checkKrausComplete(t, ch)
}

func TestGeneralizedAmplitudeDamping_Completeness(t *testing.T) {
	cases := []struct {
		p, gamma float64
	}{
		{0.5, 0.1},
		{0.8, 0.5},
		{1.0, 0.3},
		{0.0, 0.3},
	}
	for _, tc := range cases {
		ch := GeneralizedAmplitudeDamping(tc.p, tc.gamma)
		checkKrausComplete(t, ch)
	}
}

func TestGeneralizedAmplitudeDamping_ReducesToAD(t *testing.T) {
	// At p=1, GAD should match standard AmplitudeDamping
	gamma := 0.4
	gad := GeneralizedAmplitudeDamping(1.0, gamma)
	ad := AmplitudeDamping(gamma)
	gadK := gad.Kraus()
	adK := ad.Kraus()
	// E0 and E1 of GAD at p=1 should match AD's E0 and E1
	for i := range 2 {
		for j := range 4 {
			if cmplx.Abs(gadK[i][j]-adK[i][j]) > 1e-14 {
				t.Errorf("GAD(1,%.1f) Kraus[%d][%d]=%v, AD(%.1f) Kraus[%d][%d]=%v",
					gamma, i, j, gadK[i][j], gamma, i, j, adK[i][j])
			}
		}
	}
	// E2 and E3 of GAD at p=1 should be zero (sqrt(1-p)=0)
	for i := 2; i < 4; i++ {
		for _, v := range gadK[i] {
			if cmplx.Abs(v) > 1e-14 {
				t.Errorf("GAD(1,%.1f) Kraus[%d] should be zero, got %v", gamma, i, v)
			}
		}
	}
}

func TestGeneralizedAmplitudeDamping_ZeroGamma(t *testing.T) {
	// gamma=0 should give identity: E0=sqrt(p)*I, E2=sqrt(1-p)*I, E1=E3=0
	// Sum of E0†E0 + E2†E2 = p*I + (1-p)*I = I
	// Use p=0.3 (not 0.5) so sqrt(p) != sqrt(1-p).
	ch := GeneralizedAmplitudeDamping(0.3, 0)
	kraus := ch.Kraus()
	if len(kraus) != 4 {
		t.Fatalf("expected 4 Kraus ops, got %d", len(kraus))
	}
	// E0 = sqrt(p)*I
	sp := math.Sqrt(0.3)
	e0 := kraus[0]
	if cmplx.Abs(e0[0]-complex(sp, 0)) > 1e-14 || cmplx.Abs(e0[3]-complex(sp, 0)) > 1e-14 {
		t.Errorf("E0 should be sqrt(p)*I, got [%v,%v;%v,%v]", e0[0], e0[1], e0[2], e0[3])
	}
	// E1 should be zero
	for _, v := range kraus[1] {
		if cmplx.Abs(v) > 1e-14 {
			t.Error("E1 should be zero at gamma=0")
		}
	}
	// E2 = sqrt(1-p)*I
	s1p := math.Sqrt(0.7)
	e2 := kraus[2]
	if cmplx.Abs(e2[0]-complex(s1p, 0)) > 1e-14 || cmplx.Abs(e2[3]-complex(s1p, 0)) > 1e-14 {
		t.Errorf("E2 should be sqrt(1-p)*I, got [%v,%v;%v,%v]", e2[0], e2[1], e2[2], e2[3])
	}
	// E3 should be zero
	for _, v := range kraus[3] {
		if cmplx.Abs(v) > 1e-14 {
			t.Error("E3 should be zero at gamma=0")
		}
	}
	checkKrausComplete(t, ch)
}

func TestGeneralizedAmplitudeDamping_Panic(t *testing.T) {
	expectPanic(t, "p<0", func() { GeneralizedAmplitudeDamping(-0.1, 0.5) })
	expectPanic(t, "p>1", func() { GeneralizedAmplitudeDamping(1.1, 0.5) })
	expectPanic(t, "gamma<0", func() { GeneralizedAmplitudeDamping(0.5, -0.1) })
	expectPanic(t, "gamma>1", func() { GeneralizedAmplitudeDamping(0.5, 1.1) })
}

func TestAmplitudeDamping_HalfGamma(t *testing.T) {
	ch := AmplitudeDamping(0.5)
	kraus := ch.Kraus()
	// E0 = diag(1, sqrt(1-0.5)) = diag(1, sqrt(0.5))
	s := math.Sqrt(0.5)
	if math.Abs(real(kraus[0][0])-1.0) > 1e-10 {
		t.Errorf("E0[0,0] = %v, want 1.0", kraus[0][0])
	}
	if math.Abs(real(kraus[0][3])-s) > 1e-10 {
		t.Errorf("E0[1,1] = %v, want %v", kraus[0][3], s)
	}
	// E1[0,1] = sqrt(gamma) = sqrt(0.5)
	if math.Abs(real(kraus[1][1])-s) > 1e-10 {
		t.Errorf("E1[0,1] = %v, want %v", kraus[1][1], s)
	}
}
