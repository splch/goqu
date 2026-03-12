// Package routing inserts SWAP gates to satisfy hardware connectivity
// constraints using the SABRE algorithm.
//
// [Route] applies SABRE with production defaults. [RouteWithOptions]
// accepts an [Options] struct for tuning trials, bidirectional iterations,
// decay, and parallelism.
//
// Layout helpers [TrivialLayout], [RandomLayout], and [InverseLayout]
// provide initial qubit mappings. Circuits targeting all-to-all
// connectivity are returned unchanged.
package routing
