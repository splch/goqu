// Package mock provides a configurable Backend for testing.
package mock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend is a configurable mock backend for testing job managers and pipelines.
type Backend struct {
	name    string
	tgt     target.Target
	latency time.Duration

	result    *backend.Result
	submitErr error
	resultErr error

	mu     sync.Mutex
	calls  map[string]int // jobID → status poll count
	states []backend.JobState
}

// Option configures a mock Backend.
type Option func(*Backend)

// WithLatency adds an artificial delay to Submit and Status calls.
func WithLatency(d time.Duration) Option {
	return func(b *Backend) { b.latency = d }
}

// WithFixedResult makes Result always return the given result.
func WithFixedResult(r *backend.Result) Option {
	return func(b *Backend) { b.result = r }
}

// WithStatusSequence configures the status states returned by successive
// Status calls for a given job. The last state is repeated indefinitely.
func WithStatusSequence(states ...backend.JobState) Option {
	return func(b *Backend) { b.states = states }
}

// WithSubmitError makes Submit always return the given error.
func WithSubmitError(err error) Option {
	return func(b *Backend) { b.submitErr = err }
}

// WithResultError makes Result always return the given error.
func WithResultError(err error) Option {
	return func(b *Backend) { b.resultErr = err }
}

// WithTarget sets the target description returned by Target().
func WithTarget(t target.Target) Option {
	return func(b *Backend) { b.tgt = t }
}

// New creates a mock backend with the given name and options.
func New(name string, opts ...Option) *Backend {
	b := &Backend{
		name:  name,
		tgt:   target.Simulator,
		calls: make(map[string]int),
		states: []backend.JobState{
			backend.StateSubmitted,
			backend.StateRunning,
			backend.StateCompleted,
		},
		result: &backend.Result{
			Probabilities: map[string]float64{"00": 0.5, "11": 0.5},
			Shots:         1000,
		},
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Backend) Name() string         { return b.name }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit returns a new job or the configured error.
func (b *Backend) Submit(ctx context.Context, _ *backend.SubmitRequest) (*backend.Job, error) {
	if err := b.sleep(ctx); err != nil {
		return nil, err
	}
	if b.submitErr != nil {
		return nil, b.submitErr
	}
	id := generateID()
	b.mu.Lock()
	b.calls[id] = 0
	b.mu.Unlock()
	return &backend.Job{
		ID:      id,
		Backend: b.name,
		State:   b.states[0],
	}, nil
}

// Status returns the next state in the configured sequence.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	if err := b.sleep(ctx); err != nil {
		return nil, err
	}
	b.mu.Lock()
	idx, ok := b.calls[jobID]
	if !ok {
		b.mu.Unlock()
		return nil, fmt.Errorf("mock: unknown job %s", jobID)
	}
	b.calls[jobID]++
	b.mu.Unlock()

	if idx >= len(b.states) {
		idx = len(b.states) - 1
	}
	return &backend.JobStatus{
		ID:    jobID,
		State: b.states[idx],
	}, nil
}

// Result returns the configured result or error.
func (b *Backend) Result(ctx context.Context, _ string) (*backend.Result, error) {
	if err := b.sleep(ctx); err != nil {
		return nil, err
	}
	if b.resultErr != nil {
		return nil, b.resultErr
	}
	return b.result, nil
}

// Cancel is a no-op.
func (b *Backend) Cancel(_ context.Context, _ string) error {
	return nil
}

// StatusCallCount returns how many times Status was called for a given job.
func (b *Backend) StatusCallCount(jobID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[jobID]
}

func (b *Backend) sleep(ctx context.Context) error {
	if b.latency <= 0 {
		return nil
	}
	select {
	case <-time.After(b.latency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func generateID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
