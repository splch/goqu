// Package prombridge provides an [observe.Hooks] implementation that records
// Prometheus metrics for all qgo operations.
//
// This is a separate Go module with external dependencies. Core qgo packages
// never import it — applications opt in by attaching the hooks to context:
//
//	hooks := prombridge.NewHooks(prometheus.DefaultRegisterer)
//	ctx = observe.WithHooks(ctx, hooks)
//
// [NewHooks] registers 9 metrics covering transpile, pass, job, simulation,
// and HTTP durations plus gate reduction ratios and queue position.
package prombridge
