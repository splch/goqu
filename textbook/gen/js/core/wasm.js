// Lazy WASM loader - only loads the Go WASM binary on first use.

let wasmReady = null;

export function ensureWasm() {
  if (!wasmReady) {
    wasmReady = (async () => {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(
        fetch("../main.wasm"),
        go.importObject
      );
      go.run(result.instance);
    })();
  }
  return wasmReady;
}
