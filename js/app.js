// Goqu Textbook - Application entry point.
// Scans for [data-component] elements and initializes them.

import { initSandbox } from "./components/sandbox.js";
import { initSimulation } from "./components/simulation.js";
import { initExercise } from "./components/exercise.js";
import { initFlashcardDeck } from "./components/flashcard.js";
import { initPOE } from "./components/poe.js";

const registry = {
  sandbox: initSandbox,
  simulation: initSimulation,
  exercise: initExercise,
  "flashcard-deck": initFlashcardDeck,
  poe: initPOE,
};

document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("[data-component]").forEach((el) => {
    const name = el.dataset.component;
    const init = registry[name];
    if (init) init(el);
  });
});
