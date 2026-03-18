// Dark mode detection for passing to WASM SVG rendering.

export function isDark() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}
