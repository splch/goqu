// Package routing inserts SWAP gates to satisfy hardware connectivity
// constraints using the SABRE algorithm.
//
// Hardware qubits have limited connectivity -- a two-qubit gate can only
// execute between physically adjacent qubits. When a circuit requires a
// 2-qubit gate between non-adjacent qubits, SWAP gates must be inserted
// to move the logical qubits into adjacent positions. The SABRE algorithm
// (Li et al., arXiv:1809.02573) heuristically minimizes the number of
// inserted SWAPs by evaluating candidate swaps against a cost function
// that considers both the current front layer and a lookahead window of
// future gates.
//
// [Route] applies SABRE with production defaults. [RouteWithOptions]
// accepts an [Options] struct for tuning trials, bidirectional iterations,
// decay, and parallelism.
//
// Layout helpers [TrivialLayout], [RandomLayout], and [InverseLayout]
// provide initial qubit mappings. Circuits targeting all-to-all
// connectivity are returned unchanged.
package routing
