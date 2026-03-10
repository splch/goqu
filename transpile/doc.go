// Package transpile provides the quantum circuit transpilation framework.
//
// A [Pass] is a function that transforms a circuit for a given hardware
// target. Use [Pipeline] to compose multiple passes into a single
// sequential pass. Individual pass implementations live in the pass
// sub-package; pre-built optimization pipelines are in pipeline.
package transpile
