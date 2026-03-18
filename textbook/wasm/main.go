//go:build js && wasm

package main

import "syscall/js"

func main() {
	js.Global().Set("runQASM", js.FuncOf(runQASMJS))
	js.Global().Set("renderBloch", js.FuncOf(renderBlochJS))
	js.Global().Set("getStateVector", js.FuncOf(getStateVectorJS))
	js.Global().Set("getProbabilities", js.FuncOf(getProbabilitiesJS))
	select {}
}
