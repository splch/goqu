// Package manager provides concurrent job submission, polling, and result
// retrieval across multiple quantum backends.
//
// A [Manager] holds registered backends and offers [Manager.Submit] for
// synchronous execution, [Manager.SubmitAsync] for single-backend async,
// [Manager.SubmitBatch] for multi-backend fan-out, and [Manager.Watch]
// for streaming status updates.
//
// Concurrency is bounded by [WithMaxConcurrent]. Observability hooks from
// the context are invoked automatically around job lifecycle events.
package manager
