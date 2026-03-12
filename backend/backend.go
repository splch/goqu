// Package backend defines the interface for quantum execution backends.
package backend

import (
	"context"
	"sort"
	"time"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/transpile/target"
)

// Backend represents a quantum execution target.
type Backend interface {
	// Name returns the backend identifier (e.g., "ionq.aria-1").
	Name() string

	// Target returns hardware constraints for transpilation.
	Target() target.Target

	// Submit sends a circuit for execution.
	Submit(ctx context.Context, req *SubmitRequest) (*Job, error)

	// Status checks the current state of a job.
	Status(ctx context.Context, jobID string) (*JobStatus, error)

	// Result retrieves completed job results.
	Result(ctx context.Context, jobID string) (*Result, error)

	// Cancel attempts to cancel a pending/running job.
	Cancel(ctx context.Context, jobID string) error
}

// SubmitRequest contains the parameters for submitting a quantum job.
// Exactly one of Circuit or PulseProgram must be non-nil.
type SubmitRequest struct {
	Circuit      *ir.Circuit    // gate-level circuit (nil if PulseProgram set)
	PulseProgram *pulse.Program // OpenPulse program (nil if Circuit set)
	Shots        int
	Name         string
	Metadata     map[string]string
	Options      map[string]any // backend-specific options (e.g., ionq.PulseShapes)
}

// Job represents a submitted quantum job.
type Job struct {
	ID      string
	Backend string
	State   JobState
}

// JobState represents the lifecycle state of a quantum job.
type JobState int

const (
	StateSubmitted JobState = iota
	StateReady
	StateRunning
	StateCompleted
	StateFailed
	StateCancelled
)

// String returns the human-readable state name.
func (s JobState) String() string {
	switch s {
	case StateSubmitted:
		return "submitted"
	case StateReady:
		return "ready"
	case StateRunning:
		return "running"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Terminal reports whether this state is final (no further transitions).
func (s JobState) Terminal() bool {
	return s == StateCompleted || s == StateFailed || s == StateCancelled
}

// JobStatus contains detailed information about a job's current state.
type JobStatus struct {
	ID        string
	State     JobState
	Progress  float64 // 0.0–1.0, or -1 if unknown
	QueuePos  int     // position in queue, or -1 if unknown
	CreatedAt time.Time
	UpdatedAt time.Time
	Error     string // populated when State == StateFailed
}

// Result contains the output of a completed quantum job.
type Result struct {
	Counts        map[string]int     // bitstring → shot count
	Probabilities map[string]float64 // bitstring → probability
	Shots         int
	Metadata      map[string]string
}

// ToCounts returns measurement counts. If only probabilities are available,
// they are converted to counts using the largest-remainder method.
func (r *Result) ToCounts() map[string]int {
	if len(r.Counts) > 0 {
		return r.Counts
	}
	if len(r.Probabilities) == 0 || r.Shots <= 0 {
		return nil
	}

	type entry struct {
		key  string
		base int
		frac float64
	}

	entries := make([]entry, 0, len(r.Probabilities))
	total := 0
	for k, p := range r.Probabilities {
		exact := p * float64(r.Shots)
		base := int(exact)
		entries = append(entries, entry{k, base, exact - float64(base)})
		total += base
	}

	// Sort by fractional part descending to distribute remainders fairly.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].frac != entries[j].frac {
			return entries[i].frac > entries[j].frac
		}
		return entries[i].key < entries[j].key // stable tiebreak
	})

	remaining := r.Shots - total
	for i := 0; i < remaining && i < len(entries); i++ {
		entries[i].base++
	}

	counts := make(map[string]int, len(entries))
	for _, e := range entries {
		if e.base > 0 {
			counts[e.key] = e.base
		}
	}
	return counts
}

// ToProbabilities returns measurement probabilities. If only counts are
// available, they are normalized by the total number of shots.
func (r *Result) ToProbabilities() map[string]float64 {
	if len(r.Probabilities) > 0 {
		return r.Probabilities
	}
	if len(r.Counts) == 0 {
		return nil
	}

	total := 0
	for _, c := range r.Counts {
		total += c
	}
	if total == 0 {
		return nil
	}

	probs := make(map[string]float64, len(r.Counts))
	for k, c := range r.Counts {
		probs[k] = float64(c) / float64(total)
	}
	return probs
}
