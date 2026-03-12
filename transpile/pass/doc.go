// Package pass provides individual transpilation passes for circuit
// optimization and target compliance.
//
// Each pass has the [transpile.Pass] signature. Available passes:
//
//   - [DecomposeToTarget]: replace non-basis gates with target-native sequences
//   - [FixDirection]: correct 2-qubit gate direction for asymmetric connectivity
//   - [CancelAdjacent]: remove consecutive inverse gate pairs
//   - [MergeRotations]: combine same-axis rotations on the same qubit
//   - [CommuteThroughCNOT]: move single-qubit gates past CNOTs
//   - [ParallelizeOps]: reorder independent gates for minimum depth
//   - [RemoveBarriers]: strip barrier pseudo-gates
//   - [ValidateTarget]: verify basis, connectivity, and depth constraints
//
// For composed passes, see the [pipeline] package.
package pass
