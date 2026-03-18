// Re-renders KaTeX math in a given element.
// Used after dynamic DOM insertion (e.g., flashcard flip, exercise feedback).

export function renderMath(el) {
  if (typeof renderMathInElement === "function") {
    renderMathInElement(el, {
      delimiters: [
        { left: "$$", right: "$$", display: true },
        { left: "$", right: "$", display: false },
      ],
      throwOnError: false,
    });
  }
}
