// Package prombridge provides Prometheus metric hooks for qgo operations.
package prombridge

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/splch/qgo/observe"
)

// NewHooks returns observe.Hooks that record Prometheus metrics.
// Metrics are registered with the given prometheus.Registerer.
func NewHooks(reg prometheus.Registerer) *observe.Hooks {
	f := promauto.With(reg)

	transpileDuration := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_transpile_duration_seconds",
		Help:    "Time spent transpiling circuits.",
		Buckets: prometheus.DefBuckets,
	}, []string{"level"})

	transpilePassDuration := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_transpile_pass_duration_seconds",
		Help:    "Time spent in individual transpilation passes.",
		Buckets: prometheus.DefBuckets,
	}, []string{"pass"})

	transpileGateReduction := f.NewHistogram(prometheus.HistogramOpts{
		Name:    "qgo_transpile_gate_reduction_ratio",
		Help:    "Ratio of output to input gate count per transpilation.",
		Buckets: prometheus.LinearBuckets(0, 0.1, 20),
	})

	circuitDepth := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_circuit_depth",
		Help:    "Circuit depth before and after transpilation.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 15),
	}, []string{"stage"})

	jobsSubmitted := f.NewCounterVec(prometheus.CounterOpts{
		Name: "qgo_jobs_submitted_total",
		Help: "Total jobs submitted, labeled by backend.",
	}, []string{"backend"})

	jobsDuration := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_jobs_duration_seconds",
		Help:    "End-to-end job duration (submit to result).",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
	}, []string{"backend"})

	backendErrors := f.NewCounterVec(prometheus.CounterOpts{
		Name: "qgo_backend_errors_total",
		Help: "Backend errors, labeled by backend.",
	}, []string{"backend"})

	simDuration := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_sim_duration_seconds",
		Help:    "Simulation execution time.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 20),
	}, []string{"qubits"})

	httpDuration := f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "qgo_http_duration_seconds",
		Help:    "Backend HTTP request duration.",
		Buckets: prometheus.DefBuckets,
	}, []string{"backend", "method", "status"})

	jobQueuePosition := f.NewGaugeVec(prometheus.GaugeOpts{
		Name: "qgo_job_queue_position",
		Help: "Current queue position for active jobs.",
	}, []string{"backend", "job_id"})

	return &observe.Hooks{
		WrapTranspile: func(ctx context.Context, level int, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			circuitDepth.WithLabelValues("before").Observe(float64(in.Depth))
			start := time.Now()
			return ctx, func(out observe.CircuitInfo, err error) {
				lvl := strconv.Itoa(level)
				transpileDuration.WithLabelValues(lvl).Observe(time.Since(start).Seconds())
				if err == nil {
					circuitDepth.WithLabelValues("after").Observe(float64(out.Depth))
					if in.GateCount > 0 {
						transpileGateReduction.Observe(float64(out.GateCount) / float64(in.GateCount))
					}
				}
			}
		},

		WrapPass: func(ctx context.Context, pass string, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			start := time.Now()
			return ctx, func(out observe.CircuitInfo, err error) {
				transpilePassDuration.WithLabelValues(pass).Observe(time.Since(start).Seconds())
			}
		},

		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			jobsSubmitted.WithLabelValues(info.Backend).Inc()
			start := time.Now()
			return ctx, func(jobID string, err error) {
				jobsDuration.WithLabelValues(info.Backend).Observe(time.Since(start).Seconds())
				if err != nil {
					backendErrors.WithLabelValues(info.Backend).Inc()
				}
			}
		},

		WrapSim: func(ctx context.Context, info observe.SimInfo) (context.Context, func(error)) {
			start := time.Now()
			qubits := strconv.Itoa(info.NumQubits)
			return ctx, func(err error) {
				simDuration.WithLabelValues(qubits).Observe(time.Since(start).Seconds())
			}
		},

		WrapHTTP: func(ctx context.Context, info observe.HTTPInfo) (context.Context, func(int, error)) {
			start := time.Now()
			return ctx, func(statusCode int, err error) {
				status := strconv.Itoa(statusCode)
				if statusCode == 0 {
					status = "error"
				}
				httpDuration.WithLabelValues(info.Backend, info.Method, status).Observe(time.Since(start).Seconds())
			}
		},

		OnJobPoll: func(ctx context.Context, info observe.JobPollInfo) {
			if info.QueuePos >= 0 {
				jobQueuePosition.WithLabelValues(info.Backend, info.JobID).Set(float64(info.QueuePos))
			}
		},
	}
}
