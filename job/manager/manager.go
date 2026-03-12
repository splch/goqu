// Package manager provides concurrent job submission and polling.
package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/sweep"
)

// Manager handles concurrent job submission, polling, and result retrieval.
type Manager struct {
	mu       sync.RWMutex
	backends map[string]backend.Backend
	pollFreq time.Duration
	maxConc  int
	sem      chan struct{} // concurrency limiter
	logger   *slog.Logger
}

// Option configures a Manager.
type Option func(*Manager)

// WithPollFrequency sets how often the manager polls for job status.
func WithPollFrequency(d time.Duration) Option {
	return func(m *Manager) { m.pollFreq = d }
}

// WithMaxConcurrent sets the maximum number of concurrent job submissions.
func WithMaxConcurrent(n int) Option {
	return func(m *Manager) { m.maxConc = n }
}

// WithLogger sets the structured logger for the manager.
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) { m.logger = l }
}

// New creates a job manager.
func New(opts ...Option) *Manager {
	m := &Manager{
		backends: make(map[string]backend.Backend),
		pollFreq: 2 * time.Second,
		maxConc:  10,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	m.sem = make(chan struct{}, m.maxConc)
	return m
}

// Register adds a backend to the manager.
func (m *Manager) Register(name string, b backend.Backend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backends[name] = b
}

func (m *Manager) backend(name string) (backend.Backend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	if !ok {
		return nil, fmt.Errorf("manager: unknown backend %q", name)
	}
	return b, nil
}

// ResultOrError wraps a result or an error from an async submission.
type ResultOrError struct {
	Result  *backend.Result
	Backend string
	JobID   string
	Err     error
}

// Submit sends a job to a backend, polls until completion, and returns the result.
func (m *Manager) Submit(ctx context.Context, name string, req *backend.SubmitRequest) (*backend.Result, error) {
	b, err := m.backend(name)
	if err != nil {
		return nil, err
	}

	hooks := observe.FromContext(ctx)
	var jobDone func(string, error)
	if hooks != nil && hooks.WrapJob != nil {
		info := observe.JobInfo{Backend: name, Shots: req.Shots}
		if req.Circuit != nil {
			info.Qubits = req.Circuit.NumQubits()
		}
		ctx, jobDone = hooks.WrapJob(ctx, info)
	}

	m.logger.InfoContext(ctx, "submitting job",
		slog.String("backend", name),
		slog.Int("shots", req.Shots),
	)

	job, err := b.Submit(ctx, req)
	if err != nil {
		if jobDone != nil {
			jobDone("", err)
		}
		return nil, fmt.Errorf("manager: submit to %s: %w", name, err)
	}

	m.logger.InfoContext(ctx, "job submitted",
		slog.String("backend", name),
		slog.String("job_id", job.ID),
	)

	if err := m.pollUntilDone(ctx, b, job.ID, name); err != nil {
		if jobDone != nil {
			jobDone(job.ID, err)
		}
		return nil, err
	}

	result, err := b.Result(ctx, job.ID)
	if jobDone != nil {
		jobDone(job.ID, err)
	}
	if err == nil {
		m.logger.InfoContext(ctx, "job completed",
			slog.String("backend", name),
			slog.String("job_id", job.ID),
		)
	}
	return result, err
}

// SubmitAsync sends a job and returns a channel that delivers the result.
func (m *Manager) SubmitAsync(ctx context.Context, name string, req *backend.SubmitRequest) <-chan ResultOrError {
	ch := make(chan ResultOrError, 1)
	go func() {
		defer close(ch)

		// Acquire semaphore slot.
		select {
		case m.sem <- struct{}{}:
			defer func() { <-m.sem }()
		case <-ctx.Done():
			ch <- ResultOrError{Err: ctx.Err()}
			return
		}

		result, err := m.Submit(ctx, name, req)
		ch <- ResultOrError{
			Result:  result,
			Backend: name,
			Err:     err,
		}
	}()
	return ch
}

// SubmitBatch sends the same request to multiple backends concurrently.
// Results are delivered on the returned channel as they complete.
func (m *Manager) SubmitBatch(ctx context.Context, backends []string, req *backend.SubmitRequest) <-chan ResultOrError {
	ch := make(chan ResultOrError, len(backends))
	var wg sync.WaitGroup
	for _, name := range backends {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			// Acquire semaphore slot.
			select {
			case m.sem <- struct{}{}:
				defer func() { <-m.sem }()
			case <-ctx.Done():
				ch <- ResultOrError{Backend: n, Err: ctx.Err()}
				return
			}
			result, err := m.Submit(ctx, n, req)
			ch <- ResultOrError{
				Result:  result,
				Backend: n,
				Err:     err,
			}
		}(name)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}

// Watch returns a channel that delivers status updates for a job until
// it reaches a terminal state.
func (m *Manager) Watch(ctx context.Context, name string, jobID string) <-chan *backend.JobStatus {
	ch := make(chan *backend.JobStatus, 8)
	go func() {
		defer close(ch)

		b, err := m.backend(name)
		if err != nil {
			return
		}

		hooks := observe.FromContext(ctx)
		ticker := time.NewTicker(m.pollFreq)
		defer ticker.Stop()

		attempt := 0
		for {
			attempt++
			status, err := b.Status(ctx, jobID)
			if err != nil {
				return
			}

			if hooks != nil && hooks.OnJobPoll != nil {
				hooks.OnJobPoll(ctx, observe.JobPollInfo{
					Backend:  name,
					JobID:    jobID,
					State:    status.State.String(),
					Attempt:  attempt,
					QueuePos: status.QueuePos,
				})
			}

			select {
			case ch <- status:
			case <-ctx.Done():
				return
			}

			if status.State.Terminal() {
				return
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// SweepResultOrError wraps a result from a sweep point submission.
type SweepResultOrError struct {
	Index    int
	Bindings map[string]float64
	Result   *backend.Result
	Backend  string
	JobID    string
	Err      error
}

// SubmitSweep resolves the sweep, binds each parameter point to the circuit,
// and submits each bound circuit to the named backend. Results are delivered
// on the returned channel as they complete.
func (m *Manager) SubmitSweep(ctx context.Context, name string, c *ir.Circuit, shots int, sw sweep.Sweep) <-chan SweepResultOrError {
	bindings := sw.Resolve()
	ch := make(chan SweepResultOrError, len(bindings))

	var wg sync.WaitGroup
	for i, bind := range bindings {
		wg.Add(1)
		go func(idx int, b map[string]float64) {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case m.sem <- struct{}{}:
				defer func() { <-m.sem }()
			case <-ctx.Done():
				ch <- SweepResultOrError{Index: idx, Bindings: b, Backend: name, Err: ctx.Err()}
				return
			}

			bound, err := ir.Bind(c, b)
			if err != nil {
				ch <- SweepResultOrError{Index: idx, Bindings: b, Backend: name, Err: err}
				return
			}

			result, err := m.Submit(ctx, name, &backend.SubmitRequest{
				Circuit: bound,
				Shots:   shots,
			})
			ch <- SweepResultOrError{
				Index:    idx,
				Bindings: b,
				Result:   result,
				Backend:  name,
				Err:      err,
			}
		}(i, bind)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}

func (m *Manager) pollUntilDone(ctx context.Context, b backend.Backend, jobID, backendName string) error {
	ticker := time.NewTicker(m.pollFreq)
	defer ticker.Stop()

	hooks := observe.FromContext(ctx)
	attempt := 0

	for {
		attempt++
		status, err := b.Status(ctx, jobID)
		if err != nil {
			return fmt.Errorf("manager: poll %s on %s: %w", jobID, backendName, err)
		}

		if hooks != nil && hooks.OnJobPoll != nil {
			hooks.OnJobPoll(ctx, observe.JobPollInfo{
				Backend:  backendName,
				JobID:    jobID,
				State:    status.State.String(),
				Attempt:  attempt,
				QueuePos: status.QueuePos,
			})
		}

		m.logger.DebugContext(ctx, "polling job",
			slog.String("backend", backendName),
			slog.String("job_id", jobID),
			slog.String("state", status.State.String()),
			slog.Int("attempt", attempt),
		)

		switch status.State {
		case backend.StateCompleted:
			return nil
		case backend.StateFailed:
			return fmt.Errorf("manager: job %s on %s failed: %s", jobID, backendName, status.Error)
		case backend.StateCancelled:
			return fmt.Errorf("manager: job %s on %s was cancelled", jobID, backendName)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
