// Chapter completion and progress tracking via localStorage.

const PROGRESS_KEY = "goqu-progress";

function loadProgress() {
  try {
    return JSON.parse(localStorage.getItem(PROGRESS_KEY) || "{}");
  } catch {
    return {};
  }
}

function saveProgress(data) {
  localStorage.setItem(PROGRESS_KEY, JSON.stringify(data));
}

export function markVisited(chapterSlug) {
  const data = loadProgress();
  if (!data[chapterSlug]) data[chapterSlug] = {};
  data[chapterSlug].visited = true;
  data[chapterSlug].lastVisit = Date.now();
  saveProgress(data);
}

export function markExerciseComplete(chapterSlug, exerciseId) {
  const data = loadProgress();
  if (!data[chapterSlug]) data[chapterSlug] = {};
  if (!data[chapterSlug].exercises) data[chapterSlug].exercises = {};
  data[chapterSlug].exercises[exerciseId] = Date.now();
  saveProgress(data);
}

export function getProgress() {
  return loadProgress();
}
