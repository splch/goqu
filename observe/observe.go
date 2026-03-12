// Package observe provides zero-dependency observability hooks for qgo.
package observe

import "context"

// Hooks contains optional callbacks for observing qgo operations.
// Any nil field is silently skipped.
type Hooks struct {
	// WrapTranspile is called around the entire transpilation pipeline.
	WrapTranspile func(ctx context.Context, level int, in CircuitInfo) (context.Context, func(out CircuitInfo, err error))

	// WrapPass is called around each individual transpilation pass.
	WrapPass func(ctx context.Context, pass string, in CircuitInfo) (context.Context, func(out CircuitInfo, err error))

	// WrapJob is called around job submission through result retrieval.
	// The done function receives the job ID (empty if submission failed).
	WrapJob func(ctx context.Context, info JobInfo) (context.Context, func(jobID string, err error))

	// WrapSim is called around simulation execution.
	WrapSim func(ctx context.Context, info SimInfo) (context.Context, func(err error))

	// WrapHTTP is called around backend HTTP requests.
	WrapHTTP func(ctx context.Context, info HTTPInfo) (context.Context, func(statusCode int, err error))

	// WrapSweep is called around a parameter sweep execution.
	WrapSweep func(ctx context.Context, info SweepInfo) (context.Context, func(err error))

	// OnJobPoll is called each time a job is polled for status.
	OnJobPoll func(ctx context.Context, info JobPollInfo)
}

// CircuitInfo holds circuit statistics for observability.
type CircuitInfo struct {
	Name          string
	NumQubits     int
	GateCount     int
	TwoQubitGates int
	Depth         int
	Params        int
}

// JobInfo describes a job being submitted.
type JobInfo struct {
	Backend string
	Shots   int
	Qubits  int
}

// JobPollInfo describes a job status poll event.
type JobPollInfo struct {
	Backend  string
	JobID    string
	State    string
	Attempt  int
	QueuePos int
}

// SimInfo describes a simulation execution.
type SimInfo struct {
	NumQubits int
	GateCount int
	Shots     int
}

// HTTPInfo describes a backend HTTP request.
type HTTPInfo struct {
	Method  string
	Path    string
	Backend string
}

// SweepInfo describes a parameter sweep execution.
type SweepInfo struct {
	NumPoints int
	NumParams int
	Shots     int
	NumQubits int
}

type hooksKey struct{}

// WithHooks returns a context carrying the given Hooks.
func WithHooks(ctx context.Context, h *Hooks) context.Context {
	return context.WithValue(ctx, hooksKey{}, h)
}

// FromContext returns the Hooks from the context, or nil if none are set.
func FromContext(ctx context.Context) *Hooks {
	h, _ := ctx.Value(hooksKey{}).(*Hooks)
	return h
}
