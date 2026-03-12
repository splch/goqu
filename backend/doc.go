// Package backend defines the [Backend] interface for quantum execution
// targets and the associated request/response types.
//
// A Backend can submit circuits, poll job status, retrieve results, and
// cancel pending jobs. [Result] carries both measurement counts and
// probabilities, with conversion methods [Result.ToCounts] and
// [Result.ToProbabilities].
//
// Implementations: local simulator (backend/local), IonQ REST client
// (backend/ionq), IBM Quantum Runtime (backend/ibm), Amazon Braket
// (backend/braket), and configurable mock (backend/mock).
package backend
