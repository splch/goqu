// Package draw renders quantum circuits as text, SVG, and LaTeX diagrams.
//
// Text output uses a Cirq-style compact format with one row per qubit
// and columns for each gate moment:
//
//	q0: ---H---@---
//	           |
//	q1: -------X---
//
// LaTeX output uses the quantikz TikZ package for publication-quality
// circuit diagrams compilable with pdflatex.
package draw
