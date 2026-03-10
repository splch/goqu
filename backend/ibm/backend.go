package ibm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend submits and retrieves quantum jobs via the IBM Quantum Runtime V2 API.
type Backend struct {
	client *httpClient
	device string       // "ibm_brisbane", "ibm_sherbrooke", etc.
	tgt    target.Target
	jobs   sync.Map // jobID -> jobMeta
	logger *slog.Logger
}

type jobMeta struct {
	qubits int
	shots  int
}

// Option configures an IBM Quantum Backend.
type Option func(*Backend)

// WithDevice sets the IBM Quantum device target (default: "ibm_brisbane").
func WithDevice(device string) Option {
	return func(b *Backend) { b.device = device }
}

// WithBaseURL overrides the IBM Quantum API base URL.
func WithBaseURL(url string) Option {
	return func(b *Backend) { b.client.baseURL = url }
}

// WithIAMURL overrides the IAM token endpoint URL.
func WithIAMURL(url string) Option {
	return func(b *Backend) { b.client.auth.iamURL = url }
}

// WithHTTPClient provides a custom HTTP client for API requests.
func WithHTTPClient(c *http.Client) Option {
	return func(b *Backend) { b.client.base = c }
}

// WithLogger sets the structured logger for the IBM backend.
func WithLogger(l *slog.Logger) Option {
	return func(b *Backend) { b.logger = l }
}

// WithAPIVersion overrides the IBM-API-Version header value.
func WithAPIVersion(v string) Option {
	return func(b *Backend) { b.client.apiVersion = v }
}

// New creates an IBM Quantum backend with the given API key and instance CRN.
func New(apiKey, instanceCRN string, opts ...Option) *Backend {
	auth := newTokenProvider(apiKey)
	b := &Backend{
		client: newHTTPClient(auth, instanceCRN, "", "", nil),
		device: "ibm_brisbane",
		tgt:    target.IBMBrisbane,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	b.tgt = deviceTarget(b.device)
	b.client.backend = b.Name()
	return b
}

func (b *Backend) Name() string         { return "ibm." + b.device }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit to IBM Quantum for execution.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.Circuit == nil {
		return nil, fmt.Errorf("ibm: nil circuit")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("ibm: shots must be positive")
	}

	qasm, err := serializeCircuit(req.Circuit)
	if err != nil {
		return nil, err
	}

	body := &ibmJobRequest{
		ProgramID: "sampler",
		Backend:   b.device,
		Params: ibmJobParams{
			Pubs:    [][]string{{qasm}},
			Version: 2,
		},
	}

	b.logger.InfoContext(ctx, "submitting to IBM Quantum",
		slog.String("device", b.device),
		slog.Int("shots", req.Shots),
		slog.Int("qubits", req.Circuit.NumQubits()),
	)

	var resp ibmJobResponse
	if err := b.client.do(ctx, http.MethodPost, "/jobs", body, &resp); err != nil {
		return nil, err
	}

	b.jobs.Store(resp.ID, jobMeta{qubits: req.Circuit.NumQubits(), shots: req.Shots})

	b.logger.InfoContext(ctx, "job submitted to IBM Quantum",
		slog.String("job_id", resp.ID),
	)

	return &backend.Job{
		ID:      resp.ID,
		Backend: b.Name(),
		State:   backend.StateSubmitted,
	}, nil
}

// Status returns the current state of a job.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	var resp ibmStatusResponse
	if err := b.client.do(ctx, http.MethodGet, "/jobs/"+jobID, nil, &resp); err != nil {
		return nil, err
	}

	status := &backend.JobStatus{
		ID:       resp.ID,
		State:    parseState(resp.Status),
		Progress: -1,
		QueuePos: -1,
	}
	if resp.Error != nil {
		status.Error = resp.Error.Message
	}
	if status.State == backend.StateCompleted {
		status.Progress = 1.0
	}
	return status, nil
}

// Result retrieves the measurement results from a completed job.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	// First check job status.
	var statusResp ibmStatusResponse
	if err := b.client.do(ctx, http.MethodGet, "/jobs/"+jobID, nil, &statusResp); err != nil {
		return nil, err
	}
	if parseState(statusResp.Status) != backend.StateCompleted {
		return nil, fmt.Errorf("ibm: job %s is %s, not completed", jobID, statusResp.Status)
	}

	// Fetch results.
	var resultResp ibmResultResponse
	if err := b.client.do(ctx, http.MethodGet, "/jobs/"+jobID+"/results", nil, &resultResp); err != nil {
		return nil, fmt.Errorf("ibm: fetch results: %w", err)
	}

	// Determine qubit count and shot count from cached submission.
	numQubits := 0
	shots := 0
	if v, ok := b.jobs.Load(jobID); ok {
		meta := v.(jobMeta)
		numQubits = meta.qubits
		shots = meta.shots
	}
	if numQubits == 0 {
		return nil, fmt.Errorf("ibm: cannot determine qubit count for job %s", jobID)
	}

	result, err := parseResults(resultResp, numQubits, shots)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Cancel requests cancellation of a job.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	return b.client.do(ctx, http.MethodPost, "/jobs/"+jobID+"/cancel", nil, nil)
}

func parseState(s string) backend.JobState {
	switch s {
	case "Queued":
		return backend.StateSubmitted
	case "Running":
		return backend.StateRunning
	case "Completed":
		return backend.StateCompleted
	case "Failed":
		return backend.StateFailed
	case "Cancelled":
		return backend.StateCancelled
	default:
		return backend.StateSubmitted
	}
}

func deviceTarget(device string) target.Target {
	switch device {
	case "ibm_brisbane":
		return target.IBMBrisbane
	case "ibm_sherbrooke":
		return target.IBMSherbrooke
	default:
		return target.IBMBrisbane
	}
}
