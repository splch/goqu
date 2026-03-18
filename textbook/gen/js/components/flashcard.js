// Spaced-repetition flashcard deck.

import { getCardState, reviewCard } from "../storage/srs.js";
import { renderMath } from "../math/katex-render.js";

export function initFlashcardDeck(el) {
  const cards = el.querySelectorAll(".flashcard");
  if (cards.length === 0) return;

  cards.forEach((card) => {
    const cardId = card.dataset.cardId;
    if (!cardId) return;

    // Load SRS state.
    const state = getCardState(cardId);
    if (state.due > 0 && state.due > Date.now()) {
      card.classList.add("reviewed");
    }

    card.addEventListener("click", (e) => {
      // Don't toggle if clicking a button.
      if (e.target.closest("button")) return;
      card.classList.toggle("flipped");
      const back = card.querySelector(".card-back");
      if (back) renderMath(back);
    });

    // SRS review buttons.
    const easyBtn = card.querySelector(".easy");
    const hardBtn = card.querySelector(".hard");

    if (easyBtn) {
      easyBtn.addEventListener("click", () => {
        reviewCard(cardId, 4); // Good recall
        card.classList.remove("flipped");
        card.classList.add("reviewed");
      });
    }

    if (hardBtn) {
      hardBtn.addEventListener("click", () => {
        reviewCard(cardId, 2); // Difficult recall
        card.classList.remove("flipped");
      });
    }
  });
}
