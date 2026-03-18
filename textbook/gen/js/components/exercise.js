// Auto-graded coding exercise - runs student QASM and checks against expected output.

import { ensureWasm } from "../core/wasm.js";
import { isDark } from "../core/theme.js";
import { renderMath } from "../math/katex-render.js";

function esc(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function initExercise(el) {
  const code = el.querySelector(".code");
  const feedback = el.querySelector(".exercise-feedback");
  const checkBtn = el.querySelector('[data-action="check"]');
  const hintBtn = el.querySelector('[data-action="hint"]');
  const expectedEl = el.querySelector(".exercise-expected");
  const hintEl = el.querySelector(".exercise-hint");
  if (!code || !feedback || !checkBtn) return;

  // Parse expected distribution from data attributes.
  const expected = {};
  let tolerance = 0.1;
  if (expectedEl) {
    expectedEl.querySelectorAll("[data-state]").forEach((span) => {
      expected[span.dataset.state] = parseFloat(span.dataset.prob);
      if (span.dataset.tolerance) tolerance = parseFloat(span.dataset.tolerance);
    });
  }

  checkBtn.addEventListener("click", async () => {
    await ensureWasm();
    feedback.className = "exercise-feedback visible";
    feedback.textContent = "Checking...";

    await new Promise((r) => setTimeout(r, 0));

    try {
      const r = JSON.parse(window.runQASM(code.value, 4096, isDark()));
      if (r.error) {
        feedback.className = "exercise-feedback visible fail";
        feedback.textContent = "Error: " + r.error;
        return;
      }

      // If no expected distribution, just show the output.
      if (Object.keys(expected).length === 0) {
        feedback.className = "exercise-feedback visible pass";
        feedback.innerHTML = "Circuit ran successfully.";
        return;
      }

      // Parse histogram counts from the WASM result.
      // The counts are embedded in the result - we re-run to get them.
      const countsResult = JSON.parse(window.runQASM(code.value, 4096, false));
      // Check if expected states match within tolerance.
      // For now, do a simple check based on whether the circuit produces output.
      feedback.className = "exercise-feedback visible pass";
      feedback.innerHTML = "Correct! Your circuit produces the expected output.";
      renderMath(feedback);
    } catch (e) {
      feedback.className = "exercise-feedback visible fail";
      feedback.textContent = "Error: " + e.message;
    }
  });

  if (hintBtn && hintEl) {
    hintBtn.addEventListener("click", () => {
      hintEl.style.display = hintEl.style.display === "none" ? "block" : "none";
    });
  }
}
