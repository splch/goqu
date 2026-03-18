# Goqu Textbook Chapter Generation - Master Prompt Template

This document defines the HTML format, component templates, QASM reference, and quality
guidelines for generating chapters of the Goqu Interactive Quantum Computing Textbook.

## HTML Format Reference

Each chapter is a raw HTML fragment (no `<html>`, `<head>`, `<body>`, or `<h1>` tags).
The layout template adds the full document shell, `<h1>Chapter N: Title</h1>`, sidebar
navigation, KaTeX, and WASM loader automatically.

### Structure

1. Opening `<p>` paragraphs before the first `<section>` - introduce the chapter theme
   and connect to prior material
2. Multiple `<section id="slug-id">` blocks, each with:
   - `<h2>N.M Section Title</h2>` (numbered as ChapterNum.SectionNum)
   - Content paragraphs, subsections with `<h3>`, tables, lists
   - Interactive components embedded inline
3. A final review section with flashcards
4. Comment separators between sections: `<!-- ================================================================== -->`

### Callout Boxes

Three types:
```html
<div class="callout callout-key">
  <strong>Key Concept.</strong>
  <p>Important takeaway that readers should remember.</p>
</div>

<div class="callout callout-info">
  <strong>Note.</strong>
  <p>Additional context, historical notes, or clarifications.</p>
</div>

<div class="callout callout-warning">
  <strong>Common Misconception.</strong>
  <p>Corrects a frequent misunderstanding.</p>
</div>
```

### Math

Use KaTeX-compatible LaTeX:
- Inline: `$...$`
- Display: `$$...$$`
- Kets: `$|0\rangle$`, `$|1\rangle$`, `$|\psi\rangle$` (NOT `\ket{}`)
- Bras: `$\langle 0|$` (NOT `\bra{}`)
- Inner products: `$\langle \psi | \phi \rangle$`
- Matrices: `$\begin{pmatrix} a & b \\ c & d \end{pmatrix}$`
- Tensor product: `$\otimes$`

### Tables

```html
<table>
  <thead><tr><th>Header 1</th><th>Header 2</th></tr></thead>
  <tbody>
    <tr><td>Data</td><td>Data</td></tr>
  </tbody>
</table>
```

For quantum state tables, add class: `<table class="state-table">`

---

## Interactive Component Templates

### 1. Sandbox (Editable QASM with Run/Reset)

```html
<div class="sandbox" data-component="sandbox" data-shots="1024">
  <textarea class="code" rows="6">OPENQASM 3.0;
qubit[N] q;
bit[N] c;
h q[0];
c = measure q;</textarea>
  <div class="sandbox-controls">
    <button data-action="run">Run</button>
    <button data-action="reset">Reset</button>
  </div>
  <div class="output"></div>
</div>
```

- `data-shots`: number of measurement shots (default 1024)
- `data-autorun`: if present, runs automatically on page load
- Output shows: circuit SVG + histogram SVG + Bloch sphere SVG (1-qubit only)

### 2. Simulation (Slider-Driven Parametric Circuit)

```html
<div class="simulation" data-component="simulation">
  <div class="sim-controls">
    <label>$\theta$: <input type="range" data-param="theta" min="0" max="6.283" step="0.01" value="0"><output>0.00</output></label>
  </div>
  <pre class="sim-qasm">OPENQASM 3.0;
qubit[1] q;
bit[1] c;
ry({theta}) q[0];
c = measure q;</pre>
  <div class="sim-output"></div>
</div>
```

- Slider `data-param` names MUST match `{param}` placeholders in the QASM template
- Multiple sliders supported
- Auto-runs on load and on every slider change

### 3. Exercise (Graded Coding Challenge)

```html
<div class="exercise" data-component="exercise">
  <div class="exercise-header">Exercise N.M: Title</div>
  <div class="exercise-prompt"><p>Instructions telling the student what to build.</p></div>
  <textarea class="code" rows="6">OPENQASM 3.0;
qubit[N] q;
bit[N] c;
// Your code here
c = measure q;</textarea>
  <div class="exercise-expected">
    <span data-state="0" data-prob="0.5" data-tolerance="0.1"></span>
    <span data-state="1" data-prob="0.5" data-tolerance="0.1"></span>
  </div>
  <div class="exercise-hint" style="display:none"><p>Hint text here.</p></div>
  <div class="exercise-controls">
    <button data-action="check">Check</button>
    <button data-action="hint">Hint</button>
  </div>
  <div class="exercise-feedback"></div>
</div>
```

- `data-state`: basis state label (e.g., "0", "1", "00", "01", "10", "11")
- `data-prob`: expected probability (0.0 to 1.0)
- `data-tolerance`: acceptable deviation (default 0.1)
- Grading uses exact statevector probabilities, not sampled counts
- The starter code should be incomplete (student fills in the gaps)

### 4. Flashcard Deck (SM-2 Spaced Repetition)

```html
<div class="flashcard-deck" data-component="flashcard-deck">
  <div class="flashcard-deck-header">Chapter N Review</div>
  <div class="flashcard" data-card-id="chNN-topic-slug">
    <div class="card-front">Question text with $math$?</div>
    <div class="card-back">
      Answer text with $math$.
      <div class="flashcard-controls">
        <button class="easy">Easy</button>
        <button class="hard">Hard</button>
      </div>
    </div>
  </div>
  <!-- 6-10 cards per chapter -->
</div>
```

- `data-card-id` must be globally unique across all chapters (format: `chNN-descriptive-slug`)
- Cards cover key definitions, notation, theorems, and conceptual distinctions
- Always placed at the end of the chapter

### 5. Predict-Observe-Explain (POE)

```html
<div class="poe" data-component="poe">
  <div class="poe-progress">
    <div class="poe-progress-step active">Predict</div>
    <div class="poe-progress-step">Observe</div>
    <div class="poe-progress-step">Explain</div>
  </div>
  <div class="poe-step poe-predict active">
    <h3>Predict</h3>
    <p>Pose a question about what will happen when the circuit runs.</p>
    <textarea rows="3" placeholder="Write your prediction here..."></textarea>
    <button data-action="next">Submit Prediction</button>
  </div>
  <div class="poe-step poe-observe">
    <h3>Observe</h3>
    <p>Brief context for what the student should look at.</p>
    <div class="sandbox" data-component="sandbox" data-shots="1024" data-autorun>
      <textarea class="code" rows="6">OPENQASM 3.0;
qubit[N] q;
bit[N] c;
// circuit here
c = measure q;</textarea>
      <div class="sandbox-controls">
        <button data-action="run">Run Again</button>
      </div>
      <div class="output"></div>
    </div>
    <button data-action="next">Continue to Explanation</button>
  </div>
  <div class="poe-step poe-explain">
    <h3>Explain</h3>
    <p>Detailed explanation of why the result occurred.</p>
    <button data-action="complete">Complete</button>
  </div>
</div>
```

- Use at conceptual turning points where quantum behavior defies classical intuition
- The sandbox inside the Observe step uses `data-autorun`

### 6. Custom JS Widgets (for non-QASM interactivity)

For classical-only chapters (1, 2, 16) that need interactive elements without QASM:

```html
<div class="widget" id="widget-name">
  <!-- Widget HTML structure -->
</div>

<script>
(function() {
  "use strict";
  // Self-contained widget code
  var container = document.getElementById("widget-name");
  // DOM manipulation here
})();
</script>
```

- Use an IIFE to avoid polluting the global scope
- Place the `<script>` at the end of the chapter fragment
- Keep widget code self-contained (no external dependencies)

---

## QASM Gate Reference

### Syntax

```
OPENQASM 3.0;
qubit[N] q;
bit[N] c;
// gates go here
c = measure q;
```

Individual qubit measurement: `c[0] = measure q[0];`

### Fixed Gates (no parameters)

| Gate | QASM | Qubits | Description |
|------|------|--------|-------------|
| Identity | `id q[0];` | 1 | No operation |
| Hadamard | `h q[0];` | 1 | Creates equal superposition |
| Pauli-X | `x q[0];` | 1 | Bit flip (NOT) |
| Pauli-Y | `y q[0];` | 1 | Combined bit+phase flip |
| Pauli-Z | `z q[0];` | 1 | Phase flip |
| S | `s q[0];` | 1 | sqrt(Z), pi/2 phase |
| S-dagger | `sdg q[0];` | 1 | Inverse of S |
| T | `t q[0];` | 1 | sqrt(S), pi/4 phase |
| T-dagger | `tdg q[0];` | 1 | Inverse of T |
| sqrt(X) | `sx q[0];` | 1 | Square root of X |
| CNOT | `cx q[0], q[1];` | 2 | Controlled-X (control, target) |
| CZ | `cz q[0], q[1];` | 2 | Controlled-Z |
| CY | `cy q[0], q[1];` | 2 | Controlled-Y |
| SWAP | `swap q[0], q[1];` | 2 | Swap two qubits |
| Toffoli | `ccx q[0], q[1], q[2];` | 3 | Double-controlled X |
| Fredkin | `cswap q[0], q[1], q[2];` | 3 | Controlled-SWAP |
| CCZ | `ccz q[0], q[1], q[2];` | 3 | Double-controlled Z |
| iSWAP | `iswap q[0], q[1];` | 2 | SWAP with i phase |
| CH | `ch q[0], q[1];` | 2 | Controlled-Hadamard |

### Parameterized Gates

| Gate | QASM | Qubits | Description |
|------|------|--------|-------------|
| RX | `rx(theta) q[0];` | 1 | X-axis rotation |
| RY | `ry(theta) q[0];` | 1 | Y-axis rotation |
| RZ | `rz(theta) q[0];` | 1 | Z-axis rotation |
| Phase | `p(phi) q[0];` | 1 | Phase gate (= u1) |
| U3 | `U(theta,phi,lambda) q[0];` | 1 | General single-qubit |
| CP | `cp(phi) q[0], q[1];` | 2 | Controlled-Phase |
| CRX | `crx(theta) q[0], q[1];` | 2 | Controlled-RX |
| CRY | `cry(theta) q[0], q[1];` | 2 | Controlled-RY |
| CRZ | `crz(theta) q[0], q[1];` | 2 | Controlled-RZ |
| RXX | `rxx(theta) q[0], q[1];` | 2 | XX Ising coupling |
| RYY | `ryy(theta) q[0], q[1];` | 2 | YY Ising coupling |
| RZZ | `rzz(theta) q[0], q[1];` | 2 | ZZ Ising coupling |

### Angle Constants

- `pi` is available as a built-in constant
- Examples: `rx(pi) q[0];`, `rz(pi/4) q[0];`, `cp(pi/2) q[0], q[1];`

### Custom Gate Definitions

```
gate oracle q0, q1 {
  x q0;
  cx q0, q1;
  x q0;
}
oracle q[0], q[1];
```

### Modifiers

- `ctrl @ h q[0], q[1];` - Controlled-H
- `negctrl @ x q[0], q[1];` - Negative-controlled X
- `inv @ s q[0];` - Inverse of S (= sdg)
- `pow(2) @ s q[0];` - S squared (= Z)

---

## Quality Guidelines

1. **Register**: University textbook - authoritative but accessible. Write as a knowledgeable professor explaining to a motivated student.
2. **Progressive disclosure**: Introduce one concept at a time. Motivate each concept before defining it.
3. **Accuracy**: All mathematical statements must be correct. All QASM code must parse and produce the stated results.
4. **Length**: 600-1200 lines per chapter. Longer for core chapters (4-12), shorter for expository chapters (37-42).
5. **Sections**: 4-6 sections per chapter, plus a review/flashcard section.
6. **Interactive elements**: At least 2 per quantum chapter (sandbox, exercise, simulation, or POE). Classical/expository chapters need at least flashcards.
7. **Exercises**: Expected probabilities must be physically correct. Include hints.
8. **Flashcards**: 6-10 per chapter. Cover key definitions, notation, theorems, conceptual distinctions.
9. **Continuity**: Reference prior chapters by number when building on earlier concepts. Set up forward references to future chapters when appropriate.
10. **No emojis**: Do not include emoji characters anywhere in the HTML.
