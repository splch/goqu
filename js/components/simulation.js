// Parameter slider simulation - adjusts QASM template parameters in real time.

import { ensureWasm } from "../core/wasm.js";
import { isDark } from "../core/theme.js";

function esc(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function initSimulation(el) {
  const qasmTemplate = el.querySelector(".sim-qasm");
  const output = el.querySelector(".sim-output");
  const sliders = el.querySelectorAll('input[type="range"]');
  if (!qasmTemplate || !output || sliders.length === 0) return;

  const template = qasmTemplate.textContent;

  function getParams() {
    const params = {};
    sliders.forEach((s) => {
      params[s.dataset.param] = parseFloat(s.value);
      const out = s.parentElement.querySelector("output");
      if (out) out.textContent = parseFloat(s.value).toFixed(2);
    });
    return params;
  }

  function buildQasm(params) {
    let qasm = template;
    for (const [k, v] of Object.entries(params)) {
      qasm = qasm.replaceAll("{" + k + "}", v.toString());
    }
    return qasm;
  }

  let pending = false;

  async function update() {
    if (pending) return;
    pending = true;
    await ensureWasm();
    await new Promise((r) => setTimeout(r, 0));
    try {
      const params = getParams();
      const qasm = buildQasm(params);
      const r = JSON.parse(window.runQASM(qasm, 1024, isDark()));
      if (r.error) {
        output.innerHTML = '<pre class="error">' + esc(r.error) + "</pre>";
      } else {
        output.innerHTML = (r.circuit || "") + (r.histogram || "") + (r.bloch || "");
      }
    } catch (e) {
      output.innerHTML = '<pre class="error">' + esc(e.message) + "</pre>";
    }
    pending = false;
  }

  sliders.forEach((s) => s.addEventListener("input", update));

  // Initial render.
  update();
}
