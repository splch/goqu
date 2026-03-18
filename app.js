const go = new Go();
WebAssembly.instantiateStreaming(fetch(document.currentScript?.dataset.wasm || "../main.wasm"), go.importObject)
  .then(r => { go.run(r.instance); document.querySelectorAll(".sandbox[data-autorun]").forEach(s => run(s.querySelector("button"))); });

function run(btn) {
  const box = btn.closest(".sandbox");
  const code = box.querySelector(".code").value;
  const shots = parseInt(box.dataset.shots || "1024", 10);
  const out = box.querySelector(".output");
  out.innerHTML = "<p>Running...</p>";
  setTimeout(() => {
    try {
      const r = JSON.parse(window.runQASM(code, shots));
      if (r.error) { out.innerHTML = '<pre class="error">' + esc(r.error) + "</pre>"; return; }
      out.innerHTML = (r.circuit || "") + (r.histogram || "") + (r.bloch || "");
    } catch (e) { out.innerHTML = '<pre class="error">' + esc(e.message) + "</pre>"; }
  }, 0);
}

function bloch(btn) {
  const box = btn.closest(".sandbox");
  const v = (id) => parseFloat(box.querySelector("#" + id)?.value || "0");
  const out = box.querySelector(".output");
  try {
    out.innerHTML = window.renderBloch(v("aRe"), v("aIm"), v("bRe"), v("bIm"));
  } catch (e) { out.innerHTML = '<pre class="error">' + esc(e.message) + "</pre>"; }
}

function esc(s) { return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
