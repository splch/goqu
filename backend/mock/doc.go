// Package mock provides a configurable [backend.Backend] for testing job
// managers and pipelines without network calls.
//
// [New] accepts options to control behavior: [WithLatency] adds artificial
// delay, [WithFixedResult] sets a canned result, [WithStatusSequence]
// scripts successive status responses, and [WithSubmitError] or
// [WithResultError] inject errors.
package mock
