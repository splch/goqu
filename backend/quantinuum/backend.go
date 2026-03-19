package quantinuum

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend submits and retrieves quantum jobs via the Quantinuum Machine API.
type Backend struct {
	client *httpClient
	device string // "H1-1", "H1-2", "H2-1", "H1-1E", "H2-1E", "H1-1SC", etc.
	tgt    target.Target
	jobs   sync.Map // jobID → jobMeta; entries are never evicted - recreate Backend for long-lived use
	logger *slog.Logger
}

type jobMeta struct {
	qubits int
	shots  int
}

// Option configures a Quantinuum Backend.
type Option func(*Backend)

// WithDevice sets the Quantinuum device target (default: "H1-1").
func WithDevice(device string) Option {
	return func(b *Backend) { b.device = device }
}

// WithBaseURL overrides the Quantinuum API base URL.
func WithBaseURL(url string) Option {
	return func(b *Backend) { b.client.baseURL = url }
}

// WithLoginURL overrides the Quantinuum login endpoint URL.
func WithLoginURL(url string) Option {
	return func(b *Backend) { b.client.auth.loginURL = url }
}

// WithHTTPClient provides a custom HTTP client for API requests.
func WithHTTPClient(c *http.Client) Option {
	return func(b *Backend) { b.client.base = c }
}

// WithLogger sets the structured logger for the Quantinuum backend.
func WithLogger(l *slog.Logger) Option {
	return func(b *Backend) { b.logger = l }
}

// New creates a Quantinuum backend with the given email and password credentials.
func New(email, password string, opts ...Option) *Backend {
	auth := newTokenProvider(email, password)
	b := &Backend{
		client: newHTTPClient(auth, "", nil),
		device: "H1-1",
		tgt:    target.QuantinuumH1,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	b.tgt = deviceTarget(b.device)
	b.client.backend = b.Name()
	return b
}

func (b *Backend) Name() string          { return "quantinuum." + b.device }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit to Quantinuum for execution.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.PulseProgram != nil {
		return nil, fmt.Errorf("quantinuum: pulse programs are not supported")
	}
	if req.Circuit == nil {
		return nil, fmt.Errorf("quantinuum: nil circuit")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("quantinuum: shots must be positive")
	}

	qasm, err := serializeCircuit(req.Circuit)
	if err != nil {
		return nil, err
	}

	body := &jobRequest{
		Machine:  b.device,
		Language: "OPENQASM 2.0",
		Program:  qasm,
		Count:    req.Shots,
		Name:     req.Name,
	}

	b.logger.InfoContext(ctx, "submitting to Quantinuum",
		slog.String("device", b.device),
		slog.Int("shots", req.Shots),
		slog.Int("qubits", req.Circuit.NumQubits()),
	)

	var resp jobResponse
	if err := b.client.do(ctx, http.MethodPost, "/job", body, &resp); err != nil {
		return nil, err
	}

	b.jobs.Store(resp.Job, jobMeta{qubits: req.Circuit.NumQubits(), shots: req.Shots})

	b.logger.InfoContext(ctx, "job submitted to Quantinuum",
		slog.String("job_id", resp.Job),
	)

	return &backend.Job{
		ID:      resp.Job,
		Backend: b.Name(),
		State:   backend.StateSubmitted,
	}, nil
}

// Status returns the current state of a job.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	var resp jobStatusResponse
	if err := b.client.do(ctx, http.MethodGet, "/job/"+jobID, nil, &resp); err != nil {
		return nil, err
	}

	status := &backend.JobStatus{
		ID:       resp.Job,
		State:    parseState(resp.Status),
		Progress: -1,
		QueuePos: -1,
	}
	if resp.Error != "" {
		status.Error = resp.Error
	}
	if status.State == backend.StateCompleted {
		status.Progress = 1.0
	}
	return status, nil
}

// Result retrieves the measurement results from a completed job.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	var resp jobStatusResponse
	if err := b.client.do(ctx, http.MethodGet, "/job/"+jobID, nil, &resp); err != nil {
		return nil, err
	}
	if parseState(resp.Status) != backend.StateCompleted {
		return nil, fmt.Errorf("quantinuum: job %s is %s, not completed", jobID, resp.Status)
	}

	shots := 0
	if v, ok := b.jobs.Load(jobID); ok {
		meta := v.(jobMeta)
		shots = meta.shots
	}

	return parseResults(resp, shots)
}

// Cancel requests cancellation of a job.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	return b.client.do(ctx, http.MethodDelete, "/job/"+jobID, nil, nil)
}

func parseState(s string) backend.JobState {
	switch s {
	case "queued", "submitted":
		return backend.StateSubmitted
	case "running":
		return backend.StateRunning
	case "completed":
		return backend.StateCompleted
	case "failed":
		return backend.StateFailed
	case "cancelling", "cancelled", "canceled":
		return backend.StateCancelled
	default:
		return backend.StateSubmitted
	}
}

func deviceTarget(device string) target.Target {
	switch {
	case strings.HasPrefix(device, "H2"):
		return target.QuantinuumH2
	case strings.HasPrefix(device, "H1"):
		return target.QuantinuumH1
	default:
		return target.QuantinuumH1
	}
}
