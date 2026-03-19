// Package qcs defines types that mirror QCS gRPC service messages.
// These will be replaced by protoc-generated types once proto files
// are obtained from github.com/rigetti/qcs-sdk-rust.
package qcs

// TranslateRequest is the input to the translation service.
// Mirrors TranslateQuilToEncryptedControllerJobRequest.
type TranslateRequest struct {
	QuilProgram        string
	NumShots           int
	QuantumProcessorID string
}

// TranslateResponse is the output of the translation service.
// Mirrors TranslateQuilToEncryptedControllerJobResponse.
type TranslateResponse struct {
	// Job contains the encrypted controller job binary.
	Job *EncryptedControllerJob
	// Metadata contains readout information from the translation.
	Metadata *QuilTranslationMetadata
}

// EncryptedControllerJob holds the opaque encrypted program binary.
// Real proto fields: job (bytes), encryption (JobEncryption with key_id + nonce).
type EncryptedControllerJob struct {
	// Job is the opaque encrypted binary for the controller.
	Job []byte
	// Encryption holds the encryption metadata.
	Encryption *JobEncryption
}

// JobEncryption holds encryption key metadata for a controller job.
type JobEncryption struct {
	KeyID string
	Nonce []byte
}

// QuilTranslationMetadata holds readout mapping from translation.
// Real proto field: readout_mappings (map<string, string>).
type QuilTranslationMetadata struct {
	ReadoutMappings map[string]string
}

// ExecuteRequest is the input to the controller execution service.
// Mirrors ExecuteControllerJobRequest.
type ExecuteRequest struct {
	EncryptedControllerJob *EncryptedControllerJob
	QuantumProcessorID     string
	// EndpointID is the execution endpoint (used with direct/engagement access).
	EndpointID  string
	PatchValues map[string][]float64 // optional runtime patch values
}

// ExecuteResponse is the output of the controller execution service.
// Mirrors ExecuteControllerJobResponse - returns a list of execution IDs.
// Real proto field: job_execution_ids (repeated string).
type ExecuteResponse struct {
	JobExecutionIDs []string
}

// StatusRequest is the input to the status query service.
// Real proto field: job_id (singular, not job_execution_id).
type StatusRequest struct {
	QuantumProcessorID string
	JobID              string
}

// JobStatus represents the state of a QCS execution.
// Values match the real proto enum (0=Unknown, 1=Queued, ...).
type JobStatus int

const (
	StatusUnknown        JobStatus = 0
	StatusQueued         JobStatus = 1
	StatusRunning        JobStatus = 2
	StatusSucceeded      JobStatus = 3
	StatusFailed         JobStatus = 4
	StatusCanceled       JobStatus = 5
	StatusPostProcessing JobStatus = 6
)

// StatusResponse is the output of the status query service.
type StatusResponse struct {
	JobID  string
	Status JobStatus
	Error  string
}

// ResultsRequest is the input to the results query service.
// Real proto field: job_execution_id.
type ResultsRequest struct {
	QuantumProcessorID string
	JobExecutionID     string
}

// ResultsResponse is the output of the results query service.
// The real proto wraps results in ControllerJobExecutionResult.
type ResultsResponse struct {
	// Result is the execution result from the controller.
	Result *ControllerJobExecutionResult
}

// ControllerJobExecutionResult holds the readout data from execution.
// The real proto has memory_values (map<string, DataValue>) and
// readout_values (map<string, ReadoutValues>).
type ControllerJobExecutionResult struct {
	// MemoryValues maps register name to readout data.
	// Keys are register names (e.g., "ro").
	MemoryValues map[string]*DataValue
	// Status is the execution status.
	Status JobStatus
	// StatusMessage is an optional human-readable status message.
	StatusMessage string
}

// DataValue represents a typed readout value from the controller.
// The real proto is a oneof of Binary, Integer, and Real variants.
// Measurement results use the Binary variant ([][]int8 / [][]int).
type DataValue struct {
	// Binary holds per-shot measurement data as [shots][qubits]int.
	// Each value is 0 or 1.
	Binary [][]int
}

// CancelRequest is the input to the cancellation service.
// Real proto field: job_ids (repeated string, not job_execution_ids).
type CancelRequest struct {
	QuantumProcessorID string
	JobIDs             []string
}

// CancelResponse is the output of the cancellation service.
type CancelResponse struct{}

// AccessorInfo holds the gateway gRPC endpoint for a quantum processor.
// Retrieved via REST: GET /v1/quantumProcessors/{id}/accessors.
type AccessorInfo struct {
	// Address is the gRPC gateway endpoint (host:port).
	Address string
}
