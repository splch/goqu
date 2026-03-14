// Package ionq implements a Backend for the IonQ quantum cloud.
package ionq

// ionqJobRequest is the JSON body for POST /jobs.
type ionqJobRequest struct {
	Type           string              `json:"type"`
	Name           string              `json:"name,omitempty"`
	Shots          int                 `json:"shots"`
	Backend        string              `json:"backend"`
	Metadata       map[string]string   `json:"metadata,omitempty"`
	Input          ionqInput           `json:"input"`
	RuntimeOptions *ionqRuntimeOptions `json:"runtime_options,omitempty"`
}

// ionqRuntimeOptions holds optional runtime configuration for IonQ jobs.
type ionqRuntimeOptions struct {
	CustomPulseShapes map[string]any `json:"custom_pulse_shapes,omitempty"`
}

type ionqInput struct {
	Qubits  int        `json:"qubits"`
	Gateset string     `json:"gateset"`
	Circuit []ionqGate `json:"circuit"`
}

type ionqGate struct {
	Gate     string    `json:"gate"`
	Target   *int      `json:"target,omitempty"`
	Targets  []int     `json:"targets,omitempty"`
	Control  *int      `json:"control,omitempty"`
	Rotation *float64  `json:"rotation,omitempty"` // radians, for QIS gates
	Phase    *float64  `json:"phase,omitempty"`    // turns, for native gates
	Phases   []float64 `json:"phases,omitempty"`   // turns, for MS gate
	Angle    *float64  `json:"angle,omitempty"`    // turns, for MS/ZZ gate
	Time     *float64  `json:"time,omitempty"`     // microseconds, for NOP gate
}

// ionqJobResponse is returned by POST /jobs.
type ionqJobResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ionqStatusResponse is returned by GET /jobs/{id}.
// In v0.4, results are fetched separately via /jobs/{id}/results/probabilities.
type ionqStatusResponse struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Target   string            `json:"target,omitempty"`
	Qubits   int               `json:"qubits,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Warning  *ionqWarning      `json:"warning,omitempty"`
	Error    *ionqError        `json:"error,omitempty"`
}

type ionqWarning struct {
	Message string `json:"message"`
}

type ionqError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ionqAPIError is the standard error response format.
type ionqAPIError struct {
	StatusCode int    `json:"statusCode"`
	Err        string `json:"error"`
	Message    string `json:"message"`
}
