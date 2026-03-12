// Package ibm implements a [backend.Backend] for IBM Quantum via the
// Qiskit Runtime V2 API.
//
// [New] creates a backend from an API key and instance CRN. Use [WithDevice]
// to select a system (e.g. "ibm_brisbane"). Circuits are submitted as
// OpenQASM 3.0 in Sampler V2 PUB format. IAM token exchange is handled
// automatically with caching and refresh.
//
// Options: [WithBaseURL], [WithIAMURL], [WithHTTPClient], [WithLogger],
// [WithAPIVersion].
package ibm
