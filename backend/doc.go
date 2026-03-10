// Package backend defines the [Backend] interface for quantum execution
// targets and the associated request/response types.
//
// A Backend can submit circuits, poll job status, retrieve results, and
// cancel pending jobs. [Result] carries both measurement counts and
// probabilities, with conversion methods [Result.ToCounts] and
// [Result.ToProbabilities].
//
// Implementations include the local simulator backend (backend/local),
// the IonQ REST client (backend/ionq), and a configurable mock
// (backend/mock).
package backend
