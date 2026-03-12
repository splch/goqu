// Package braket implements a Backend for Amazon Braket quantum cloud.
package braket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	braketservice "github.com/aws/aws-sdk-go-v2/service/braket"
	brakettypes "github.com/aws/aws-sdk-go-v2/service/braket/types"
	s3service "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// braketAPI is the subset of the Braket service client used by this backend.
type braketAPI interface {
	CreateQuantumTask(ctx context.Context, input *braketservice.CreateQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CreateQuantumTaskOutput, error)
	GetQuantumTask(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error)
	CancelQuantumTask(ctx context.Context, input *braketservice.CancelQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CancelQuantumTaskOutput, error)
}

// s3API is the subset of the S3 service client used by this backend.
type s3API interface {
	GetObject(ctx context.Context, input *s3service.GetObjectInput, optFns ...func(*s3service.Options)) (*s3service.GetObjectOutput, error)
}

// Backend submits and retrieves quantum jobs via Amazon Braket.
type Backend struct {
	braket    braketAPI
	s3        s3API
	device    string
	deviceArn string
	s3Bucket  string
	s3Prefix  string
	tgt       target.Target
	logger    *slog.Logger
	jobs      sync.Map // taskARN → jobMeta; entries are never evicted — recreate Backend for long-lived use
}

type jobMeta struct {
	shots int
}

// Option configures a Braket Backend.
type Option func(*Backend)

// WithDevice sets the short device name (e.g., "sv1", "ionq.forte").
func WithDevice(device string) Option {
	return func(b *Backend) { b.device = device }
}

// WithDeviceARN overrides the device ARN directly, bypassing the name lookup.
func WithDeviceARN(arn string) Option {
	return func(b *Backend) { b.deviceArn = arn }
}

// WithS3Bucket sets the S3 bucket for task output (required).
func WithS3Bucket(bucket string) Option {
	return func(b *Backend) { b.s3Bucket = bucket }
}

// WithS3Prefix sets the S3 key prefix for task output (default: "qgo/").
func WithS3Prefix(prefix string) Option {
	return func(b *Backend) { b.s3Prefix = prefix }
}

// WithLogger sets the structured logger for the Braket backend.
func WithLogger(l *slog.Logger) Option {
	return func(b *Backend) { b.logger = l }
}

// New creates a Braket backend from an AWS config.
func New(cfg aws.Config, opts ...Option) *Backend {
	b := &Backend{
		braket:   braketservice.NewFromConfig(cfg),
		s3:       s3service.NewFromConfig(cfg),
		device:   "sv1",
		s3Bucket: "",
		s3Prefix: "qgo/",
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	// Resolve device ARN from short name if not explicitly set.
	if b.deviceArn == "" {
		b.deviceArn, _ = DeviceARN(b.device)
	}
	b.tgt = DeviceTarget(b.device)
	return b
}

func (b *Backend) Name() string          { return "braket." + b.device }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit or pulse program to Amazon Braket for execution.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.Circuit == nil && req.PulseProgram == nil {
		return nil, fmt.Errorf("braket: either Circuit or PulseProgram must be set")
	}
	if req.Circuit != nil && req.PulseProgram != nil {
		return nil, fmt.Errorf("braket: cannot set both Circuit and PulseProgram")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("braket: shots must be positive")
	}
	if b.s3Bucket == "" {
		return nil, fmt.Errorf("braket: S3 bucket is required")
	}
	if b.deviceArn == "" {
		return nil, fmt.Errorf("braket: device ARN is required")
	}

	var action string
	var err error
	if req.PulseProgram != nil {
		action, err = serializePulseProgram(req.PulseProgram)
	} else {
		action, err = serializeCircuit(req.Circuit)
	}
	if err != nil {
		return nil, err
	}

	logAttrs := []any{
		slog.String("device", b.device),
		slog.String("device_arn", b.deviceArn),
		slog.Int("shots", req.Shots),
	}
	if req.Circuit != nil {
		logAttrs = append(logAttrs, slog.Int("qubits", req.Circuit.NumQubits()))
	} else {
		logAttrs = append(logAttrs, slog.String("type", "pulse_program"))
	}
	b.logger.InfoContext(ctx, "submitting to Braket", logAttrs...)

	shots64 := int64(req.Shots)
	input := &braketservice.CreateQuantumTaskInput{
		Action:            aws.String(action),
		DeviceArn:         aws.String(b.deviceArn),
		OutputS3Bucket:    aws.String(b.s3Bucket),
		OutputS3KeyPrefix: aws.String(b.s3Prefix),
		Shots:             aws.Int64(shots64),
	}

	out, err := b.braket.CreateQuantumTask(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("braket: create task: %w", err)
	}

	taskArn := aws.ToString(out.QuantumTaskArn)
	b.jobs.Store(taskArn, jobMeta{shots: req.Shots})

	b.logger.InfoContext(ctx, "task submitted to Braket",
		slog.String("task_arn", taskArn),
	)

	return &backend.Job{
		ID:      taskArn,
		Backend: b.Name(),
		State:   backend.StateSubmitted,
	}, nil
}

// Status returns the current state of a Braket quantum task.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	out, err := b.braket.GetQuantumTask(ctx, &braketservice.GetQuantumTaskInput{
		QuantumTaskArn: aws.String(jobID),
	})
	if err != nil {
		return nil, fmt.Errorf("braket: get task: %w", err)
	}

	status := &backend.JobStatus{
		ID:       jobID,
		State:    mapTaskStatus(out.Status),
		Progress: -1,
		QueuePos: -1,
	}
	if out.FailureReason != nil {
		status.Error = aws.ToString(out.FailureReason)
	}
	if status.State == backend.StateCompleted {
		status.Progress = 1.0
	}
	if out.CreatedAt != nil {
		status.CreatedAt = *out.CreatedAt
	}
	return status, nil
}

// Result retrieves the results of a completed Braket quantum task from S3.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	// Check task status and get output location.
	out, err := b.braket.GetQuantumTask(ctx, &braketservice.GetQuantumTaskInput{
		QuantumTaskArn: aws.String(jobID),
	})
	if err != nil {
		return nil, fmt.Errorf("braket: get task: %w", err)
	}

	if mapTaskStatus(out.Status) != backend.StateCompleted {
		return nil, fmt.Errorf("braket: task %s is %s, not completed", jobID, out.Status)
	}

	// Determine S3 location of results.
	bucket := aws.ToString(out.OutputS3Bucket)
	prefix := aws.ToString(out.OutputS3Directory)
	if bucket == "" || prefix == "" {
		return nil, fmt.Errorf("braket: no output location for task %s", jobID)
	}

	key := prefix + "/results.json"

	b.logger.DebugContext(ctx, "fetching results from S3",
		slog.String("bucket", bucket),
		slog.String("key", key),
	)

	s3Out, err := b.s3.GetObject(ctx, &s3service.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("braket: fetch results from s3://%s/%s: %w", bucket, key, err)
	}
	defer func() { _ = s3Out.Body.Close() }()

	data, err := io.ReadAll(s3Out.Body)
	if err != nil {
		return nil, fmt.Errorf("braket: read results: %w", err)
	}

	// Determine shots from cached submission data.
	var shots int
	if v, ok := b.jobs.Load(jobID); ok {
		shots = v.(jobMeta).shots
	} else {
		// Try to extract from results JSON.
		var raw struct {
			TaskMetadata struct {
				Shots int `json:"shots"`
			} `json:"taskMetadata"`
		}
		_ = json.Unmarshal(data, &raw) // best effort
		shots = raw.TaskMetadata.Shots
	}

	return parseResults(data, shots)
}

// Cancel requests cancellation of a Braket quantum task.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	_, err := b.braket.CancelQuantumTask(ctx, &braketservice.CancelQuantumTaskInput{
		QuantumTaskArn: aws.String(jobID),
	})
	if err != nil {
		return fmt.Errorf("braket: cancel task: %w", err)
	}
	return nil
}

// mapTaskStatus converts a Braket task status to a backend.JobState.
func mapTaskStatus(s brakettypes.QuantumTaskStatus) backend.JobState {
	switch s {
	case brakettypes.QuantumTaskStatusCreated, brakettypes.QuantumTaskStatusQueued:
		return backend.StateSubmitted
	case brakettypes.QuantumTaskStatusRunning:
		return backend.StateRunning
	case brakettypes.QuantumTaskStatusCompleted:
		return backend.StateCompleted
	case brakettypes.QuantumTaskStatusFailed:
		return backend.StateFailed
	case brakettypes.QuantumTaskStatusCancelling, brakettypes.QuantumTaskStatusCancelled:
		return backend.StateCancelled
	default:
		return backend.StateSubmitted
	}
}
