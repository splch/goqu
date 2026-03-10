package backend

import "testing"

func TestJobStateString(t *testing.T) {
	tests := []struct {
		state JobState
		want  string
	}{
		{StateSubmitted, "submitted"},
		{StateReady, "ready"},
		{StateRunning, "running"},
		{StateCompleted, "completed"},
		{StateFailed, "failed"},
		{StateCancelled, "cancelled"},
		{JobState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("JobState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestJobStateTerminal(t *testing.T) {
	terminal := []JobState{StateCompleted, StateFailed, StateCancelled}
	nonTerminal := []JobState{StateSubmitted, StateReady, StateRunning}

	for _, s := range terminal {
		if !s.Terminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.Terminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestResultToCounts(t *testing.T) {
	r := &Result{
		Probabilities: map[string]float64{"00": 0.5, "11": 0.5},
		Shots:         1000,
	}
	counts := r.ToCounts()
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 1000 {
		t.Errorf("ToCounts total = %d, want 1000", total)
	}
	if counts["00"] != 500 || counts["11"] != 500 {
		t.Errorf("ToCounts = %v, want 00:500 11:500", counts)
	}
}

func TestResultToCountsRounding(t *testing.T) {
	// 1/3 probability with 10 shots: can't divide evenly.
	r := &Result{
		Probabilities: map[string]float64{"00": 1.0 / 3, "01": 1.0 / 3, "10": 1.0 / 3},
		Shots:         10,
	}
	counts := r.ToCounts()
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 10 {
		t.Errorf("ToCounts total = %d, want 10", total)
	}
}

func TestResultToCountsFromCounts(t *testing.T) {
	r := &Result{
		Counts: map[string]int{"00": 250, "11": 750},
		Shots:  1000,
	}
	counts := r.ToCounts()
	if counts["00"] != 250 || counts["11"] != 750 {
		t.Errorf("ToCounts = %v, want 00:250 11:750", counts)
	}
}

func TestResultToCountsEmpty(t *testing.T) {
	r := &Result{}
	if counts := r.ToCounts(); counts != nil {
		t.Errorf("ToCounts on empty result = %v, want nil", counts)
	}
}

func TestResultToProbabilities(t *testing.T) {
	r := &Result{
		Counts: map[string]int{"00": 250, "11": 750},
		Shots:  1000,
	}
	probs := r.ToProbabilities()
	if probs["00"] != 0.25 || probs["11"] != 0.75 {
		t.Errorf("ToProbabilities = %v, want 00:0.25 11:0.75", probs)
	}
}

func TestResultToProbabilitiesFromProbs(t *testing.T) {
	r := &Result{
		Probabilities: map[string]float64{"00": 0.5, "11": 0.5},
	}
	probs := r.ToProbabilities()
	if probs["00"] != 0.5 || probs["11"] != 0.5 {
		t.Errorf("ToProbabilities = %v, want 00:0.5 11:0.5", probs)
	}
}

func TestResultToProbabilitiesEmpty(t *testing.T) {
	r := &Result{}
	if probs := r.ToProbabilities(); probs != nil {
		t.Errorf("ToProbabilities on empty result = %v, want nil", probs)
	}
}
