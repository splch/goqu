// Package observe provides zero-dependency observability hooks for qgo.
//
// [Hooks] uses the wrap pattern: each hook receives a context and returns
// an enriched context plus a done function. This enables OpenTelemetry span
// propagation without importing OTel in core packages.
//
// All hooks are optional. Nil hooks are silently skipped with zero overhead.
// Use [WithHooks] and [FromContext] to propagate hooks via [context.Context].
//
// Bridge implementations: otelbridge (OpenTelemetry spans) and prombridge
// (Prometheus metrics).
package observe
