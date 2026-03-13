// Package local provides a Backend backed by the statevector simulator.
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/sim/pulsesim"
	"github.com/splch/qgo/sim/statevector"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend runs circuits on the local statevector simulator.
type Backend struct {
	maxQubits int
	results   sync.Map // jobID → *backend.Result
	logger    *slog.Logger
}

// Option configures a local Backend.
type Option func(*Backend)

// WithMaxQubits sets the maximum number of qubits the simulator supports.
func WithMaxQubits(n int) Option {
	return func(b *Backend) { b.maxQubits = n }
}

// WithLogger sets the structured logger for the local backend.
func WithLogger(l *slog.Logger) Option {
	return func(b *Backend) { b.logger = l }
}

// New creates a local simulator backend.
func New(opts ...Option) *Backend {
	b := &Backend{maxQubits: 28, logger: slog.Default()}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Backend) Name() string          { return "local.simulator" }
func (b *Backend) Target() target.Target { return target.Simulator }

// Submit executes the circuit synchronously and returns a completed job.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.PulseProgram != nil {
		return b.submitPulse(ctx, req)
	}
	if req.Circuit == nil {
		return nil, fmt.Errorf("local: nil circuit")
	}
	if req.Circuit.NumQubits() > b.maxQubits {
		return nil, fmt.Errorf("local: circuit has %d qubits, max is %d", req.Circuit.NumQubits(), b.maxQubits)
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("local: shots must be positive")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	nq := req.Circuit.NumQubits()
	gc := req.Circuit.Stats().GateCount

	hooks := observe.FromContext(ctx)
	var simDone func(error)
	if hooks != nil && hooks.WrapSim != nil {
		ctx, simDone = hooks.WrapSim(ctx, observe.SimInfo{
			NumQubits: nq,
			GateCount: gc,
			Shots:     req.Shots,
		})
	}

	b.logger.InfoContext(ctx, "simulating circuit",
		slog.Int("qubits", nq),
		slog.Int("gates", gc),
		slog.Int("shots", req.Shots),
	)

	start := time.Now()
	sim := statevector.New(nq)
	counts, err := sim.Run(req.Circuit, req.Shots)
	elapsed := time.Since(start)

	if simDone != nil {
		simDone(err)
	}
	if err != nil {
		return nil, fmt.Errorf("local: %w", err)
	}

	b.logger.InfoContext(ctx, "simulation complete",
		slog.Int("qubits", nq),
		slog.Duration("elapsed", elapsed),
	)

	id := generateID()
	b.results.Store(id, &backend.Result{
		Counts: counts,
		Shots:  req.Shots,
	})

	return &backend.Job{
		ID:      id,
		Backend: b.Name(),
		State:   backend.StateCompleted,
	}, nil
}

// Status returns the status of a job. All local jobs are immediately completed.
func (b *Backend) Status(_ context.Context, jobID string) (*backend.JobStatus, error) {
	if _, ok := b.results.Load(jobID); !ok {
		return nil, fmt.Errorf("local: unknown job %s", jobID)
	}
	return &backend.JobStatus{
		ID:       jobID,
		State:    backend.StateCompleted,
		Progress: 1.0,
	}, nil
}

// Result retrieves the result of a completed job.
func (b *Backend) Result(_ context.Context, jobID string) (*backend.Result, error) {
	val, ok := b.results.Load(jobID)
	if !ok {
		return nil, fmt.Errorf("local: unknown job %s", jobID)
	}
	return val.(*backend.Result), nil
}

// Cancel is a no-op for the local backend since jobs complete synchronously.
func (b *Backend) Cancel(_ context.Context, _ string) error {
	return nil
}

// submitPulse runs a pulse program on the pulsesim simulator.
func (b *Backend) submitPulse(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.Shots <= 0 {
		return nil, fmt.Errorf("local: shots must be positive")
	}

	// Extract FrameMap from options.
	var fm pulsesim.FrameMap
	switch v := req.Options["frame_map"].(type) {
	case pulsesim.FrameMap:
		fm = v
	case map[string]int:
		fm = pulsesim.FrameMap(v)
	default:
		return nil, fmt.Errorf("local: pulse programs require Options[\"frame_map\"] of type pulsesim.FrameMap")
	}

	numQubits := 0
	for _, q := range fm {
		if q+1 > numQubits {
			numQubits = q + 1
		}
	}
	if numQubits == 0 {
		return nil, fmt.Errorf("local: empty frame map")
	}
	if numQubits > b.maxQubits {
		return nil, fmt.Errorf("local: pulse program requires %d qubits, max is %d", numQubits, b.maxQubits)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	b.logger.InfoContext(ctx, "simulating pulse program",
		slog.Int("qubits", numQubits),
		slog.Int("shots", req.Shots),
	)

	// Extract optional coupling options.
	var simOpts []pulsesim.Option
	if cm, ok := req.Options["coupling_map"].(pulsesim.CouplingMap); ok {
		simOpts = append(simOpts, pulsesim.WithCoupling(cm))
	}
	if cr, ok := req.Options["cr_frames"].(pulsesim.CRFrameMap); ok {
		simOpts = append(simOpts, pulsesim.WithCRFrames(cr))
	}

	start := time.Now()
	sim := pulsesim.New(numQubits, fm, simOpts...)
	counts, err := sim.Run(req.PulseProgram, req.Shots)
	elapsed := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("local: %w", err)
	}

	b.logger.InfoContext(ctx, "pulse simulation complete",
		slog.Int("qubits", numQubits),
		slog.Duration("elapsed", elapsed),
	)

	id := generateID()
	b.results.Store(id, &backend.Result{
		Counts: counts,
		Shots:  req.Shots,
	})

	return &backend.Job{
		ID:      id,
		Backend: b.Name(),
		State:   backend.StateCompleted,
	}, nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
