// Package viz renders quantum simulation results as SVG visualizations.
//
// It provides three visualization types:
//
//   - [Histogram] / [HistogramProb]: bar charts of measurement counts or probabilities
//   - [Bloch]: single-qubit state rendered on the Bloch sphere
//   - [StateCity]: isometric 3D bar chart of density matrix elements
//
// All outputs are self-contained SVG strings with no external dependencies.
// Each function comes in two forms: Foo returns a string, FprintFoo writes to an [io.Writer].
//
// Styling is controlled via [WithStyle] using [DefaultStyle] (light) or [DarkStyle] (dark).
package viz
