package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

type lessonMeta struct {
	ID    string
	Title string
	Desc  string
}

var lessons = []lessonMeta{
	{"01-classical", "Classical Computing Review", "Bits, logic gates, and why we need quantum"},
	{"02-qubits", "Qubits & Quantum States", "Superposition, probability amplitudes, and Dirac notation"},
	{"03-gates", "Quantum Gates", "Pauli gates, Hadamard, rotations, and the Bloch sphere"},
	{"04-circuits", "Quantum Circuits & Measurement", "Building circuits, wire diagrams, and measurement"},
	{"05-entanglement", "Entanglement & Bell States", "CNOT, Bell states, and quantum correlations"},
	{"06-deutsch-jozsa", "Deutsch-Jozsa Algorithm", "Your first quantum speedup"},
	{"07-grover", "Grover's Search", "Amplitude amplification and quadratic speedup"},
	{"08-qft-qpe", "QFT & Phase Estimation", "Quantum Fourier Transform and eigenvalue estimation"},
	{"09-shor", "Shor's Algorithm", "Quantum factoring and its implications"},
	{"10-variational", "Variational Algorithms", "VQE, QAOA, and hybrid quantum-classical computing"},
	{"11-noise", "Noise & Error Mitigation", "Decoherence, noise models, and mitigation strategies"},
	{"12-hardware", "Real Quantum Hardware", "Hardware targets, transpilation, and running on QPUs"},
}

type pageData struct {
	Title      string
	Lessons    []lessonMeta
	Lesson     *lessonMeta
	LessonIdx  int
	PrevLesson *lessonMeta
	NextLesson *lessonMeta
	DemoSVG    string
	DemoHist   string
	DemoInfo   string
	ExtraSVG   string
	ExtraHist  string
	ExtraInfo  string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:   "Learn Quantum Computing with Goqu",
		Lessons: lessons,
	}
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleLesson(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var lesson *lessonMeta
	var idx int
	for i, l := range lessons {
		if l.ID == id {
			lesson = &lessons[i]
			idx = i
			break
		}
	}
	if lesson == nil {
		http.NotFound(w, r)
		return
	}

	data := pageData{
		Title:     lesson.Title + " — Goqu",
		Lessons:   lessons,
		Lesson:    lesson,
		LessonIdx: idx,
	}
	if idx > 0 {
		data.PrevLesson = &lessons[idx-1]
	}
	if idx < len(lessons)-1 {
		data.NextLesson = &lessons[idx+1]
	}

	// Generate default demo for the lesson
	switch id {
	case "02-qubits":
		if res, err := demoSuperposition("H", 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
		}
	case "03-gates":
		if res, err := demoGate("H", 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
		}
	case "04-circuits":
		if res, err := demoCircuit("H0,CNOT", 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
		}
	case "05-entanglement":
		if res, err := demoBellState("phi+", 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
		}
	case "06-deutsch-jozsa":
		if res, err := demoDeutschJozsa("balanced", 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
	case "07-grover":
		if res, err := demoGrover(5, 3, 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
	case "08-qft-qpe":
		if res, err := demoQPE(0.25, 3, 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
	case "09-shor":
		if res, err := demoShorCircuit(1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
	case "10-variational":
		if res, err := demoVariational(math.Pi/4, 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
	case "11-noise":
		if res, err := demoNoise(0.05, 1024); err == nil {
			data.DemoSVG = res.SVG
			data.DemoHist = histogramHTML(res.Counts)
			data.DemoInfo = res.StateInfo
		}
		// Also show ideal for comparison
		if res, err := demoBellState("phi+", 1024); err == nil {
			data.ExtraSVG = res.SVG
			data.ExtraHist = histogramHTML(res.Counts)
			data.ExtraInfo = "Ideal (no noise)"
		}
	case "12-hardware":
		if res, err := demoTranspile("ionq"); err == nil {
			data.DemoSVG = res.SVG
			data.DemoInfo = res.StateInfo
		}
	}

	templateName := id + ".html"
	if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, fmt.Sprintf("template %s: %v", templateName, err), 500)
	}
}

func handleSandbox(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:   "Circuit Sandbox — Goqu",
		Lessons: lessons,
	}
	// Default: 2-qubit Bell state
	if res, err := demoSandbox(2, []string{"H:0", "CNOT:0:1"}, 1024); err == nil {
		data.DemoSVG = res.SVG
		data.DemoHist = histogramHTML(res.Counts)
	}
	if err := templates.ExecuteTemplate(w, "sandbox.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleSimulate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	demo := r.FormValue("demo")
	shots, _ := strconv.Atoi(r.FormValue("shots"))
	if shots < 1 || shots > 8192 {
		shots = 1024
	}

	var res *DemoResult
	var err error

	switch demo {
	case "superposition":
		gateName := r.FormValue("gate")
		res, err = demoSuperposition(gateName, shots)
	case "gate":
		gateName := r.FormValue("gate")
		res, err = demoGate(gateName, shots)
	case "circuit":
		gates := r.FormValue("gates")
		res, err = demoCircuit(gates, shots)
	case "bell":
		variant := r.FormValue("variant")
		res, err = demoBellState(variant, shots)
	case "deutsch-jozsa":
		oracleType := r.FormValue("oracle")
		res, err = demoDeutschJozsa(oracleType, shots)
	case "grover":
		marked, _ := strconv.Atoi(r.FormValue("marked"))
		nq, _ := strconv.Atoi(r.FormValue("qubits"))
		if nq < 2 {
			nq = 3
		}
		res, err = demoGrover(marked, nq, shots)
	case "qpe":
		phase, _ := strconv.ParseFloat(r.FormValue("phase"), 64)
		bits, _ := strconv.Atoi(r.FormValue("bits"))
		res, err = demoQPE(phase, bits, shots)
	case "shor":
		res, err = demoShorCircuit(shots)
	case "variational":
		theta, _ := strconv.ParseFloat(r.FormValue("theta"), 64)
		res, err = demoVariational(theta, shots)
	case "noise":
		level, _ := strconv.ParseFloat(r.FormValue("level"), 64)
		res, err = demoNoise(level, shots)
	case "transpile":
		targetName := r.FormValue("target")
		res, err = demoTranspile(targetName)
	default:
		http.Error(w, "unknown demo", 400)
		return
	}

	if err != nil {
		fmt.Fprintf(w, `<div class="error">Error: %s</div>`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="demo-result">`)
	fmt.Fprintf(w, `<div class="circuit-svg">%s</div>`, res.SVG)
	if res.StateInfo != "" {
		fmt.Fprintf(w, `<div class="state-info">%s</div>`, res.StateInfo)
	}
	if demo != "transpile" {
		fmt.Fprintf(w, `%s`, histogramHTML(res.Counts))
	}
	fmt.Fprintf(w, `</div>`)
}

func handleSandboxSimulate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	nq, _ := strconv.Atoi(r.FormValue("qubits"))
	if nq < 1 || nq > 6 {
		nq = 2
	}
	shots, _ := strconv.Atoi(r.FormValue("shots"))
	if shots < 1 || shots > 8192 {
		shots = 1024
	}

	gatesStr := r.FormValue("gates")
	var gateList []string
	if gatesStr != "" {
		gateList = strings.Split(gatesStr, ",")
	}

	res, err := demoSandbox(nq, gateList, shots)
	if err != nil {
		fmt.Fprintf(w, `<div class="error">Error: %s</div>`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="demo-result">`)
	fmt.Fprintf(w, `<div class="circuit-svg">%s</div>`, res.SVG)
	fmt.Fprintf(w, `%s`, histogramHTML(res.Counts))
	fmt.Fprintf(w, `</div>`)
}
