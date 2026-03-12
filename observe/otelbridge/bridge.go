// Package otelbridge provides OpenTelemetry span hooks for qgo operations.
package otelbridge

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/splch/qgo/observe"
)

const tracerName = "github.com/splch/qgo"

// Option configures the OTel bridge.
type Option func(*config)

type config struct {
	tracer trace.Tracer
}

// WithTracer overrides the default OTel tracer.
func WithTracer(t trace.Tracer) Option {
	return func(c *config) { c.tracer = t }
}

// NewHooks returns observe.Hooks that create OTel spans for all qgo operations.
// Child spans are automatically nested via context propagation.
func NewHooks(opts ...Option) *observe.Hooks {
	cfg := config{tracer: otel.Tracer(tracerName)}
	for _, o := range opts {
		o(&cfg)
	}
	t := cfg.tracer

	return &observe.Hooks{
		WrapTranspile: func(ctx context.Context, level int, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			ctx, span := t.Start(ctx, "qgo.transpile",
				trace.WithAttributes(
					attribute.Int("qgo.transpile.level", level),
					attribute.String("qgo.circuit.name", in.Name),
					attribute.Int("qgo.circuit.qubits", in.NumQubits),
					attribute.Int("qgo.circuit.gates.in", in.GateCount),
					attribute.Int("qgo.circuit.depth.in", in.Depth),
				))
			return ctx, func(out observe.CircuitInfo, err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				} else {
					span.SetAttributes(
						attribute.Int("qgo.circuit.gates.out", out.GateCount),
						attribute.Int("qgo.circuit.depth.out", out.Depth),
						attribute.Int("qgo.circuit.two_qubit.out", out.TwoQubitGates),
					)
				}
				span.End()
			}
		},

		WrapPass: func(ctx context.Context, pass string, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			ctx, span := t.Start(ctx, "qgo.transpile."+pass,
				trace.WithAttributes(
					attribute.Int("qgo.circuit.gates.in", in.GateCount),
					attribute.Int("qgo.circuit.depth.in", in.Depth),
				))
			return ctx, func(out observe.CircuitInfo, err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				} else {
					span.SetAttributes(
						attribute.Int("qgo.circuit.gates.out", out.GateCount),
						attribute.Int("qgo.circuit.depth.out", out.Depth),
					)
				}
				span.End()
			}
		},

		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			ctx, span := t.Start(ctx, "qgo.job",
				trace.WithAttributes(
					attribute.String("qgo.backend", info.Backend),
					attribute.Int("qgo.shots", info.Shots),
					attribute.Int("qgo.qubits", info.Qubits),
				))
			return ctx, func(jobID string, err error) {
				if jobID != "" {
					span.SetAttributes(attribute.String("qgo.job_id", jobID))
				}
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		WrapSim: func(ctx context.Context, info observe.SimInfo) (context.Context, func(error)) {
			ctx, span := t.Start(ctx, "qgo.simulate",
				trace.WithAttributes(
					attribute.Int("qgo.qubits", info.NumQubits),
					attribute.Int("qgo.gates", info.GateCount),
					attribute.Int("qgo.shots", info.Shots),
				))
			return ctx, func(err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		WrapHTTP: func(ctx context.Context, info observe.HTTPInfo) (context.Context, func(int, error)) {
			ctx, span := t.Start(ctx, "qgo.http",
				trace.WithAttributes(
					attribute.String("http.method", info.Method),
					attribute.String("http.target", info.Path),
					attribute.String("qgo.backend", info.Backend),
				))
			return ctx, func(statusCode int, err error) {
				if statusCode > 0 {
					span.SetAttributes(attribute.Int("http.status_code", statusCode))
				}
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		OnJobPoll: func(ctx context.Context, info observe.JobPollInfo) {
			span := trace.SpanFromContext(ctx)
			span.AddEvent("qgo.job.poll", trace.WithAttributes(
				attribute.String("qgo.job_id", info.JobID),
				attribute.String("qgo.state", info.State),
				attribute.Int("qgo.attempt", info.Attempt),
			))
		},
	}
}
