// Spaced Repetition System - SM-2 algorithm with localStorage persistence.

const SRS_KEY = "goqu-srs";

function loadAll() {
  try {
    return JSON.parse(localStorage.getItem(SRS_KEY) || "{}");
  } catch {
    return {};
  }
}

function saveAll(data) {
  localStorage.setItem(SRS_KEY, JSON.stringify(data));
}

export function getCardState(cardId) {
  const data = loadAll();
  return data[cardId] || { interval: 1, ease: 2.5, due: 0, reps: 0 };
}

// SM-2 review: quality 0-5 (0=forgot, 5=perfect).
export function reviewCard(cardId, quality) {
  const data = loadAll();
  const state = data[cardId] || { interval: 1, ease: 2.5, due: 0, reps: 0 };

  if (quality >= 3) {
    state.reps++;
    if (state.reps === 1) state.interval = 1;
    else if (state.reps === 2) state.interval = 6;
    else state.interval = Math.round(state.interval * state.ease);
    state.ease = Math.max(
      1.3,
      state.ease + 0.1 - (5 - quality) * (0.08 + (5 - quality) * 0.02)
    );
  } else {
    state.reps = 0;
    state.interval = 1;
  }

  state.due = Date.now() + state.interval * 86400000;
  data[cardId] = state;
  saveAll(data);
}

export function getDueCards() {
  const data = loadAll();
  const now = Date.now();
  return Object.entries(data)
    .filter(([, state]) => state.due <= now)
    .map(([id]) => id);
}
