// Package qir provides QIR (Quantum Intermediate Representation) support for goqu.
//
// QIR is an industry-standard quantum IR built on LLVM IR, created by the
// QIR Alliance. It enables interoperability between quantum programming
// frameworks and hardware platforms such as Azure Quantum, Quantinuum, and
// NVIDIA cuQuantum.
//
// This package defines the QIR profile types used by the emitter and parser
// sub-packages. The emitter converts goqu circuits to QIR-compliant LLVM IR
// text (.ll files), while the parser reads QIR LLVM IR back into goqu circuits.
//
// The implementation targets QIR specification v2.0 with opaque pointers
// (LLVM 16+) and has zero external dependencies.
package qir
