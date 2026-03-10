// Package retry provides retry policies with exponential backoff and a
// circuit breaker for protecting against repeated backend failures.
//
// [Do] executes a function with retries according to a [Policy].
// [DefaultPolicy] returns sensible defaults (3 attempts, 1 s initial
// delay, 2x backoff, 30 s cap).
//
// [CircuitBreaker] tracks consecutive failures and opens after a
// configurable threshold, rejecting further calls until a reset period
// elapses.
package retry
