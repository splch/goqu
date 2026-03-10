package ionq

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend submits and retrieves quantum jobs via the IonQ REST API.
type Backend struct {
	client *httpClient
	device string       // "simulator", "qpu.aria-1", "qpu.forte-1", etc.
	tgt    target.Target
	jobs sync.Map // jobID → jobMeta
}

type jobMeta struct {
	qubits int
	shots  int
}

// Option configures an IonQ Backend.
type Option func(*Backend)

// WithDevice sets the IonQ device target (default: "simulator").
func WithDevice(device string) Option {
	return func(b *Backend) { b.device = device }
}

// WithBaseURL overrides the IonQ API base URL.
func WithBaseURL(url string) Option {
	return func(b *Backend) { b.client.baseURL = url }
}

// WithHTTPClient provides a custom HTTP client for requests.
func WithHTTPClient(c *http.Client) Option {
	return func(b *Backend) { b.client.base = c }
}

// New creates an IonQ backend with the given API key.
func New(apiKey string, opts ...Option) *Backend {
	b := &Backend{
		client: newHTTPClient(apiKey, "", nil),
		device: "simulator",
		tgt:    target.Simulator,
	}
	for _, opt := range opts {
		opt(b)
	}
	b.tgt = deviceTarget(b.device)
	return b
}

func (b *Backend) Name() string         { return "ionq." + b.device }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit to IonQ for execution.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.Circuit == nil {
		return nil, fmt.Errorf("ionq: nil circuit")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("ionq: shots must be positive")
	}

	input, err := marshalCircuit(req.Circuit)
	if err != nil {
		return nil, err
	}

	body := &ionqJobRequest{
		Type:     "ionq.circuit.v1",
		Name:     req.Name,
		Shots:    req.Shots,
		Backend:  b.device,
		Metadata: req.Metadata,
		Input:    *input,
	}

	var resp ionqJobResponse
	if err := b.client.do(ctx, http.MethodPost, "/jobs", body, &resp); err != nil {
		return nil, err
	}

	b.jobs.Store(resp.ID, jobMeta{qubits: req.Circuit.NumQubits(), shots: req.Shots})

	return &backend.Job{
		ID:      resp.ID,
		Backend: b.Name(),
		State:   parseState(resp.Status),
	}, nil
}

// Status returns the current state of a job.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	var resp ionqStatusResponse
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

// Result retrieves the probability distribution from a completed job.
// Uses the v0.4 /jobs/{id}/results/probabilities endpoint.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	// First check job status.
	var statusResp ionqStatusResponse
	if err := b.client.do(ctx, http.MethodGet, "/jobs/"+jobID, nil, &statusResp); err != nil {
		return nil, err
	}
	if parseState(statusResp.Status) != backend.StateCompleted {
		return nil, fmt.Errorf("ionq: job %s is %s, not completed", jobID, statusResp.Status)
	}

	// Fetch results from the dedicated v0.4 endpoint.
	var rawProbs map[string]float64
	if err := b.client.do(ctx, http.MethodGet, "/jobs/"+jobID+"/results/probabilities", nil, &rawProbs); err != nil {
		return nil, fmt.Errorf("ionq: fetch results: %w", err)
	}

	// Determine qubit count and shot count from cached submission or status response.
	numQubits := statusResp.Qubits
	var shots int
	if v, ok := b.jobs.Load(jobID); ok {
		meta := v.(jobMeta)
		numQubits = meta.qubits
		shots = meta.shots
	}
	if numQubits == 0 {
		return nil, fmt.Errorf("ionq: cannot determine qubit count for job %s", jobID)
	}

	// Convert IonQ integer keys to bitstring keys.
	probs := make(map[string]float64, len(rawProbs))
	for key, prob := range rawProbs {
		n, err := strconv.Atoi(key)
		if err != nil {
			return nil, fmt.Errorf("ionq: invalid result key %q: %w", key, err)
		}
		probs[bitstring(n, numQubits)] = prob
	}

	return &backend.Result{
		Probabilities: probs,
		Shots:         shots,
		Metadata:      statusResp.Metadata,
	}, nil
}

// Cancel requests cancellation of a job.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	return b.client.do(ctx, http.MethodPut, "/jobs/"+jobID+"/status/cancel", nil, nil)
}

func parseState(s string) backend.JobState {
	switch s {
	case "submitted":
		return backend.StateSubmitted
	case "ready":
		return backend.StateReady
	case "running":
		return backend.StateRunning
	case "completed":
		return backend.StateCompleted
	case "failed":
		return backend.StateFailed
	case "canceled", "cancelled":
		return backend.StateCancelled
	default:
		return backend.StateSubmitted
	}
}

func deviceTarget(device string) target.Target {
	switch {
	case device == "simulator":
		return target.Simulator
	case len(device) >= 8 && device[:8] == "qpu.aria":
		return target.IonQAria
	case len(device) >= 9 && device[:9] == "qpu.forte":
		return target.IonQForte
	default:
		return target.Simulator
	}
}
