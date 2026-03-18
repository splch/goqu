//go:build js && wasm

package main

import "syscall/js"

func main() {
	js.Global().Set("runQASM", js.FuncOf(runQASMJS))
	js.Global().Set("renderBloch", js.FuncOf(renderBlochJS))
	js.Global().Set("getStateVector", js.FuncOf(getStateVectorJS))
	js.Global().Set("getProbabilities", js.FuncOf(getProbabilitiesJS))
	js.Global().Set("runNoisyQASM", js.FuncOf(runNoisyQASMJS))
	js.Global().Set("compareIdealNoisy", js.FuncOf(compareIdealNoisyJS))
	js.Global().Set("runClifford", js.FuncOf(runCliffordJS))
	js.Global().Set("cliffordStepThrough", js.FuncOf(cliffordStepThroughJS))
	js.Global().Set("transpileQASM", js.FuncOf(transpileQASMJS))
	js.Global().Set("getTargetInfo", js.FuncOf(getTargetInfoJS))
	js.Global().Set("computeExpectation", js.FuncOf(computeExpectationJS))
	js.Global().Set("sweepExpectation", js.FuncOf(sweepExpectationJS))
	js.Global().Set("sweep2D", js.FuncOf(sweep2DJS))
	js.Global().Set("computeGradient", js.FuncOf(computeGradientJS))
	js.Global().Set("runOracleAlgorithm", js.FuncOf(runOracleAlgorithmJS))
	js.Global().Set("runSearchAlgorithm", js.FuncOf(runSearchAlgorithmJS))
	js.Global().Set("groverStepThrough", js.FuncOf(groverStepThroughJS))
	js.Global().Set("runQFT", js.FuncOf(runQFTJS))
	js.Global().Set("buildAnsatz", js.FuncOf(buildAnsatzJS))
	js.Global().Set("runQAOA", js.FuncOf(runQAOAJS))
	js.Global().Set("runVQE", js.FuncOf(runVQEJS))
	js.Global().Set("runZNE", js.FuncOf(runZNEJS))
	js.Global().Set("insertDD", js.FuncOf(insertDDJS))
	js.Global().Set("twirlCircuit", js.FuncOf(twirlCircuitJS))
	js.Global().Set("runQPE", js.FuncOf(runQPEJS))
	js.Global().Set("runShor", js.FuncOf(runShorJS))
	js.Global().Set("runTrotter", js.FuncOf(runTrotterJS))
	js.Global().Set("channelInfo", js.FuncOf(channelInfoJS))
	select {}
}
