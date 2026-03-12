// Package ionq implements a [backend.Backend] for the IonQ quantum cloud
// via its REST API (v0.4).
//
// [New] creates a backend from an API key. Use [WithDevice] to select a
// target (e.g. "simulator", "qpu.aria-1", "qpu.forte-1"). The client
// auto-detects QIS vs native gateset from circuit gates and converts
// rotation angles from radians to turns at the serialization boundary.
//
// HTTP requests use exponential backoff for 429 and 5xx responses.
// Options: [WithBaseURL], [WithHTTPClient], [WithLogger].
package ionq
