// QASM code sandbox - editable code + circuit diagram + histogram + Bloch sphere.

import { ensureWasm } from "../core/wasm.js";
import { isDark } from "../core/theme.js";

function esc(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function initSandbox(el) {
  const code = el.querySelector(".code");
  const output = el.querySelector(".output");
  if (!code || !output) return;

  const runBtn = el.querySelector('[data-action="run"]');
  const resetBtn = el.querySelector('[data-action="reset"]');
  const shotsSelect = el.querySelector("[data-shots]");
  const originalCode = code.value;

  function getShots() {
    if (shotsSelect) return parseInt(shotsSelect.value, 10);
    return parseInt(el.dataset.shots || "1024", 10);
  }

  async function run() {
    await ensureWasm();
    output.innerHTML = '<p style="color:var(--color-text-muted)">Running...</p>';
    // Yield to browser for repaint.
    await new Promise((r) => setTimeout(r, 0));
    try {
      const r = JSON.parse(window.runQASM(code.value, getShots(), isDark()));
      if (r.error) {
        output.innerHTML = '<pre class="error">' + esc(r.error) + "</pre>";
        return;
      }
      output.innerHTML = (r.circuit || "") + (r.histogram || "") + (r.bloch || "");
    } catch (e) {
      output.innerHTML = '<pre class="error">' + esc(e.message) + "</pre>";
    }
  }

  if (runBtn) runBtn.addEventListener("click", run);
  if (resetBtn) resetBtn.addEventListener("click", () => { code.value = originalCode; });

  // Auto-run if flagged.
  if (el.hasAttribute("data-autorun")) {
    run();
  }
}
