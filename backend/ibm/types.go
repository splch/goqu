// Package ibm implements a Backend for IBM Quantum (Qiskit Runtime V2).
package ibm

import "encoding/json"

// ibmJobRequest is the JSON body for POST /jobs.
type ibmJobRequest struct {
	ProgramID string       `json:"program_id"`
	Backend   string       `json:"backend"`
	Params    ibmJobParams `json:"params"`
}

type ibmJobParams struct {
	Pubs    [][]string `json:"pubs"`
	Version int        `json:"version"`
}

// ibmJobResponse is returned by POST /jobs.
type ibmJobResponse struct {
	ID string `json:"id"`
}

// ibmStatusResponse is returned by GET /jobs/{id}.
type ibmStatusResponse struct {
	ID      string          `json:"id"`
	Status  string          `json:"status"`
	Backend string          `json:"backend,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *ibmError       `json:"error,omitempty"`
}

type ibmError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ibmResultResponse is returned by GET /jobs/{id}/results.
type ibmResultResponse struct {
	Results []ibmPubResult `json:"results"`
}

type ibmPubResult struct {
	Data ibmResultData `json:"data"`
}

type ibmResultData struct {
	// Sampler V2 returns serialized bit arrays per classical register.
	// For simplified parsing, we use a map of register name -> samples.
	CRegSamples map[string][][]int `json:"c"`
}

// ibmAPIError is the standard error response format.
type ibmAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
