package braket

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	braketservice "github.com/aws/aws-sdk-go-v2/service/braket"
	brakettypes "github.com/aws/aws-sdk-go-v2/service/braket/types"
	s3service "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/builder"
)

// --- Mock implementations ---

type mockBraket struct {
	createFunc func(ctx context.Context, input *braketservice.CreateQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CreateQuantumTaskOutput, error)
	getFunc    func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error)
	cancelFunc func(ctx context.Context, input *braketservice.CancelQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CancelQuantumTaskOutput, error)
}

func (m *mockBraket) CreateQuantumTask(ctx context.Context, input *braketservice.CreateQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CreateQuantumTaskOutput, error) {
	return m.createFunc(ctx, input, optFns...)
}

func (m *mockBraket) GetQuantumTask(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
	return m.getFunc(ctx, input, optFns...)
}

func (m *mockBraket) CancelQuantumTask(ctx context.Context, input *braketservice.CancelQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CancelQuantumTaskOutput, error) {
	return m.cancelFunc(ctx, input, optFns...)
}

type mockS3 struct {
	getFunc func(ctx context.Context, input *s3service.GetObjectInput, optFns ...func(*s3service.Options)) (*s3service.GetObjectOutput, error)
}

func (m *mockS3) GetObject(ctx context.Context, input *s3service.GetObjectInput, optFns ...func(*s3service.Options)) (*s3service.GetObjectOutput, error) {
	return m.getFunc(ctx, input, optFns...)
}

// helper to create a backend with mock clients.
func newTestBackend(mb *mockBraket, ms *mockS3) *Backend {
	return &Backend{
		braket:    mb,
		s3:        ms,
		device:    "sv1",
		deviceArn: "arn:aws:braket:::device/quantum-simulator/amazon/sv1",
		s3Bucket:  "test-bucket",
		s3Prefix:  "qgo/",
		tgt:       DeviceTarget("sv1"),
		logger:    noopLogger(),
	}
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Tests ---

func TestSerializeCircuit(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}

	action, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	var prog braketProgram
	if err := json.Unmarshal([]byte(action), &prog); err != nil {
		t.Fatalf("action is not valid JSON: %v", err)
	}
	if prog.Header.Name != "braket.ir.openqasm.program" {
		t.Errorf("header name = %q, want %q", prog.Header.Name, "braket.ir.openqasm.program")
	}
	if prog.Header.Version != "1" {
		t.Errorf("header version = %q, want %q", prog.Header.Version, "1")
	}
	if !strings.Contains(prog.Source, "OPENQASM 3.0") {
		t.Errorf("source missing OPENQASM header: %s", prog.Source)
	}
	if !strings.Contains(prog.Source, "qubit[2]") {
		t.Errorf("source missing qubit declaration: %s", prog.Source)
	}
}

func TestParseResults(t *testing.T) {
	data := []byte(`{
		"measurementCounts": {"00": 500, "11": 500},
		"measurementProbabilities": {"00": 0.5, "11": 0.5},
		"measuredQubits": [0, 1]
	}`)

	result, err := parseResults(data, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shots != 1000 {
		t.Errorf("shots = %d, want 1000", result.Shots)
	}
	if result.Counts["00"] != 500 {
		t.Errorf("counts[00] = %d, want 500", result.Counts["00"])
	}
	if result.Counts["11"] != 500 {
		t.Errorf("counts[11] = %d, want 500", result.Counts["11"])
	}
	if result.Probabilities["00"] != 0.5 {
		t.Errorf("probabilities[00] = %v, want 0.5", result.Probabilities["00"])
	}
}

func TestParseResultsInvalidJSON(t *testing.T) {
	_, err := parseResults([]byte("not json"), 100)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDeviceARNMapping(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"ionq.forte", "arn:aws:braket:us-east-1::device/qpu/ionq/Forte-Enterprise-1", true},
		{"iqm.garnet", "arn:aws:braket:eu-north-1::device/qpu/iqm/Garnet", true},
		{"rigetti.ankaa", "arn:aws:braket:us-west-1::device/qpu/rigetti/Ankaa-3", true},
		{"sv1", "arn:aws:braket:::device/quantum-simulator/amazon/sv1", true},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		arn, ok := DeviceARN(tt.name)
		if ok != tt.ok {
			t.Errorf("DeviceARN(%q) ok = %v, want %v", tt.name, ok, tt.ok)
		}
		if arn != tt.want {
			t.Errorf("DeviceARN(%q) = %q, want %q", tt.name, arn, tt.want)
		}
	}
}

func TestDeviceTargetMapping(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{"ionq.forte", "IonQ Forte"},
		{"iqm.garnet", "iqm.garnet"},
		{"rigetti.ankaa", "rigetti.ankaa"},
		{"sv1", "Simulator"},
		{"unknown", "Simulator"},
	}
	for _, tt := range tests {
		got := DeviceTarget(tt.device)
		if got.Name != tt.want {
			t.Errorf("DeviceTarget(%q).Name = %q, want %q", tt.device, got.Name, tt.want)
		}
	}
}

func TestMapTaskStatus(t *testing.T) {
	tests := []struct {
		s    brakettypes.QuantumTaskStatus
		want backend.JobState
	}{
		{brakettypes.QuantumTaskStatusCreated, backend.StateSubmitted},
		{brakettypes.QuantumTaskStatusQueued, backend.StateSubmitted},
		{brakettypes.QuantumTaskStatusRunning, backend.StateRunning},
		{brakettypes.QuantumTaskStatusCompleted, backend.StateCompleted},
		{brakettypes.QuantumTaskStatusFailed, backend.StateFailed},
		{brakettypes.QuantumTaskStatusCancelling, backend.StateCancelled},
		{brakettypes.QuantumTaskStatusCancelled, backend.StateCancelled},
		{brakettypes.QuantumTaskStatus("UNKNOWN"), backend.StateSubmitted},
	}
	for _, tt := range tests {
		if got := mapTaskStatus(tt.s); got != tt.want {
			t.Errorf("mapTaskStatus(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestSubmitFlow(t *testing.T) {
	var capturedInput *braketservice.CreateQuantumTaskInput
	mb := &mockBraket{
		createFunc: func(ctx context.Context, input *braketservice.CreateQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CreateQuantumTaskOutput, error) {
			capturedInput = input
			return &braketservice.CreateQuantumTaskOutput{
				QuantumTaskArn: aws.String("arn:aws:braket:us-east-1:123:quantum-task/abc-123"),
			}, nil
		},
	}
	b := newTestBackend(mb, nil)

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   1000,
		Name:    "bell-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	if job.ID != "arn:aws:braket:us-east-1:123:quantum-task/abc-123" {
		t.Errorf("job ID = %q, want task ARN", job.ID)
	}
	if job.Backend != "braket.sv1" {
		t.Errorf("backend = %q, want braket.sv1", job.Backend)
	}
	if job.State != backend.StateSubmitted {
		t.Errorf("state = %v, want submitted", job.State)
	}

	// Verify the CreateQuantumTask input.
	if capturedInput == nil {
		t.Fatal("CreateQuantumTask was not called")
	}
	if aws.ToString(capturedInput.DeviceArn) != "arn:aws:braket:::device/quantum-simulator/amazon/sv1" {
		t.Errorf("device ARN = %q", aws.ToString(capturedInput.DeviceArn))
	}
	if aws.ToString(capturedInput.OutputS3Bucket) != "test-bucket" {
		t.Errorf("S3 bucket = %q, want test-bucket", aws.ToString(capturedInput.OutputS3Bucket))
	}
	if capturedInput.Shots == nil || *capturedInput.Shots != 1000 {
		t.Errorf("shots unexpected, want 1000")
	}

	// Verify the action contains valid Braket schema.
	var prog braketProgram
	if err := json.Unmarshal([]byte(aws.ToString(capturedInput.Action)), &prog); err != nil {
		t.Fatalf("action is not valid JSON: %v", err)
	}
	if prog.Header.Name != "braket.ir.openqasm.program" {
		t.Errorf("action header = %q", prog.Header.Name)
	}
}

func TestStatusFlow(t *testing.T) {
	now := time.Now()
	mb := &mockBraket{
		getFunc: func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
			return &braketservice.GetQuantumTaskOutput{
				QuantumTaskArn: input.QuantumTaskArn,
				Status:         brakettypes.QuantumTaskStatusRunning,
				CreatedAt:      &now,
			}, nil
		},
	}
	b := newTestBackend(mb, nil)

	status, err := b.Status(context.Background(), "arn:task/123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateRunning {
		t.Errorf("state = %v, want running", status.State)
	}
	if status.ID != "arn:task/123" {
		t.Errorf("ID = %q, want arn:task/123", status.ID)
	}
	if status.Progress != -1 {
		t.Errorf("progress = %v, want -1 (running)", status.Progress)
	}
}

func TestStatusCompleted(t *testing.T) {
	mb := &mockBraket{
		getFunc: func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
			return &braketservice.GetQuantumTaskOutput{
				QuantumTaskArn: input.QuantumTaskArn,
				Status:         brakettypes.QuantumTaskStatusCompleted,
			}, nil
		},
	}
	b := newTestBackend(mb, nil)

	status, err := b.Status(context.Background(), "arn:task/123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("state = %v, want completed", status.State)
	}
	if status.Progress != 1.0 {
		t.Errorf("progress = %v, want 1.0", status.Progress)
	}
}

func TestStatusFailed(t *testing.T) {
	mb := &mockBraket{
		getFunc: func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
			return &braketservice.GetQuantumTaskOutput{
				QuantumTaskArn: input.QuantumTaskArn,
				Status:         brakettypes.QuantumTaskStatusFailed,
				FailureReason:  aws.String("device offline"),
			}, nil
		},
	}
	b := newTestBackend(mb, nil)

	status, err := b.Status(context.Background(), "arn:task/123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateFailed {
		t.Errorf("state = %v, want failed", status.State)
	}
	if status.Error != "device offline" {
		t.Errorf("error = %q, want %q", status.Error, "device offline")
	}
}

func TestResultFromS3(t *testing.T) {
	resultsJSON := `{
		"measurementCounts": {"00": 500, "11": 500},
		"measurementProbabilities": {"00": 0.5, "11": 0.5},
		"measuredQubits": [0, 1]
	}`

	mb := &mockBraket{
		getFunc: func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
			return &braketservice.GetQuantumTaskOutput{
				QuantumTaskArn:  input.QuantumTaskArn,
				Status:          brakettypes.QuantumTaskStatusCompleted,
				OutputS3Bucket:  aws.String("results-bucket"),
				OutputS3Directory: aws.String("qgo/task-123"),
			}, nil
		},
	}
	ms := &mockS3{
		getFunc: func(ctx context.Context, input *s3service.GetObjectInput, optFns ...func(*s3service.Options)) (*s3service.GetObjectOutput, error) {
			if aws.ToString(input.Bucket) != "results-bucket" {
				t.Errorf("S3 bucket = %q, want results-bucket", aws.ToString(input.Bucket))
			}
			if aws.ToString(input.Key) != "qgo/task-123/results.json" {
				t.Errorf("S3 key = %q, want qgo/task-123/results.json", aws.ToString(input.Key))
			}
			return &s3service.GetObjectOutput{
				Body: io.NopCloser(strings.NewReader(resultsJSON)),
			}, nil
		},
	}
	b := newTestBackend(mb, ms)

	// Pre-store the job meta so shots are known.
	b.jobs.Store("arn:task/123", jobMeta{shots: 1000})

	result, err := b.Result(context.Background(), "arn:task/123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Shots != 1000 {
		t.Errorf("shots = %d, want 1000", result.Shots)
	}
	if result.Counts["00"] != 500 {
		t.Errorf("counts[00] = %d, want 500", result.Counts["00"])
	}
	if result.Probabilities["11"] != 0.5 {
		t.Errorf("probabilities[11] = %v, want 0.5", result.Probabilities["11"])
	}
}

func TestResultNotCompleted(t *testing.T) {
	mb := &mockBraket{
		getFunc: func(ctx context.Context, input *braketservice.GetQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.GetQuantumTaskOutput, error) {
			return &braketservice.GetQuantumTaskOutput{
				QuantumTaskArn: input.QuantumTaskArn,
				Status:         brakettypes.QuantumTaskStatusRunning,
			}, nil
		},
	}
	b := newTestBackend(mb, nil)

	_, err := b.Result(context.Background(), "arn:task/123")
	if err == nil {
		t.Fatal("expected error for non-completed task")
	}
	if !strings.Contains(err.Error(), "not completed") {
		t.Errorf("error = %q, expected to contain 'not completed'", err.Error())
	}
}

func TestCancelFlow(t *testing.T) {
	var capturedArn string
	mb := &mockBraket{
		cancelFunc: func(ctx context.Context, input *braketservice.CancelQuantumTaskInput, optFns ...func(*braketservice.Options)) (*braketservice.CancelQuantumTaskOutput, error) {
			capturedArn = aws.ToString(input.QuantumTaskArn)
			return &braketservice.CancelQuantumTaskOutput{}, nil
		},
	}
	b := newTestBackend(mb, nil)

	if err := b.Cancel(context.Background(), "arn:task/456"); err != nil {
		t.Fatal(err)
	}
	if capturedArn != "arn:task/456" {
		t.Errorf("cancelled ARN = %q, want arn:task/456", capturedArn)
	}
}

func TestSubmitNilCircuit(t *testing.T) {
	b := newTestBackend(nil, nil)
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
	if !strings.Contains(err.Error(), "nil circuit") {
		t.Errorf("error = %q, expected to contain 'nil circuit'", err.Error())
	}
}

func TestSubmitZeroShots(t *testing.T) {
	b := newTestBackend(nil, nil)
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
	if !strings.Contains(err.Error(), "shots must be positive") {
		t.Errorf("error = %q, expected to contain 'shots must be positive'", err.Error())
	}
}

func TestSubmitMissingS3Bucket(t *testing.T) {
	b := &Backend{
		device:    "sv1",
		deviceArn: "arn:test",
		s3Bucket:  "", // missing
		logger:    noopLogger(),
	}
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c, Shots: 100})
	if err == nil {
		t.Fatal("expected error for missing S3 bucket")
	}
	if !strings.Contains(err.Error(), "S3 bucket is required") {
		t.Errorf("error = %q, expected to contain 'S3 bucket is required'", err.Error())
	}
}

func TestSubmitMissingDeviceARN(t *testing.T) {
	b := &Backend{
		device:    "unknown-device",
		deviceArn: "", // missing
		s3Bucket:  "bucket",
		logger:    noopLogger(),
	}
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c, Shots: 100})
	if err == nil {
		t.Fatal("expected error for missing device ARN")
	}
	if !strings.Contains(err.Error(), "device ARN is required") {
		t.Errorf("error = %q, expected to contain 'device ARN is required'", err.Error())
	}
}

func TestBackendName(t *testing.T) {
	b := &Backend{device: "sv1"}
	if b.Name() != "braket.sv1" {
		t.Errorf("Name() = %q, want braket.sv1", b.Name())
	}
}

func TestBackendInterface(t *testing.T) {
	// Compile-time check that Backend implements backend.Backend.
	var _ backend.Backend = (*Backend)(nil)
}
