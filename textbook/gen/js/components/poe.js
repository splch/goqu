// Predict-Observe-Explain interactive module.

import { initSandbox } from "./sandbox.js";

export function initPOE(el) {
  const steps = el.querySelectorAll(".poe-step");
  const progressSteps = el.querySelectorAll(".poe-progress-step");
  if (steps.length === 0) return;

  let currentStep = 0;

  function showStep(idx) {
    steps.forEach((s, i) => {
      s.classList.toggle("active", i === idx);
    });
    progressSteps.forEach((s, i) => {
      s.classList.remove("active", "done");
      if (i < idx) s.classList.add("done");
      if (i === idx) s.classList.add("active");
    });
    currentStep = idx;

    // Initialize sandboxes in the newly visible step.
    const activeStep = steps[idx];
    if (activeStep) {
      activeStep.querySelectorAll('.sandbox:not([data-initialized])').forEach((sb) => {
        sb.setAttribute("data-initialized", "true");
        initSandbox(sb);
      });
    }
  }

  // Wire up next/complete buttons.
  el.querySelectorAll('[data-action="next"]').forEach((btn) => {
    btn.addEventListener("click", () => {
      if (currentStep < steps.length - 1) {
        showStep(currentStep + 1);
      }
    });
  });

  el.querySelectorAll('[data-action="complete"]').forEach((btn) => {
    btn.addEventListener("click", () => {
      progressSteps.forEach((s) => {
        s.classList.remove("active");
        s.classList.add("done");
      });
      // Optional: mark as completed in progress tracking.
    });
  });

  // Show first step.
  showStep(0);
}
