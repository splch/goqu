// Package otelbridge provides an [observe.Hooks] implementation that creates
// OpenTelemetry spans for all goqu operations.
//
// This is a separate Go module with external dependencies. Core goqu packages
// never import it - applications opt in by attaching the hooks to context:
//
//	hooks := otelbridge.NewHooks()
//	ctx = observe.WithHooks(ctx, hooks)
//
// [NewHooks] creates the hooks; [WithTracer] overrides the default tracer.
// Spans form a nested hierarchy: goqu.transpile > goqu.transpile.{pass} >
// goqu.http.
package otelbridge
