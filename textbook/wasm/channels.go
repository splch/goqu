//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/operator"
)

// channelInfoResult holds operator representation data.
type channelInfoResult struct {
	// Kraus operators as a list of flat 2D complex matrices.
	// Each operator is [{re, im}, ...] in row-major order.
	NumKraus      int
	KrausOps      [][][2]float64 // [operator][element]{re, im}
	ChoiMatrix    [][2]float64   // flat row-major {re, im}
	PTMMatrix     []float64      // flat row-major real
	ChoiDim       int
	PTMDim        int
	IsCP          bool
	IsTP          bool
	ProcessFid    float64
	AvgGateFid    float64
	Error         string
}

func buildChannel(channelType string, param float64) (noise.Channel, error) {
	switch channelType {
	case "depolarizing":
		return noise.Depolarizing1Q(param), nil
	case "amplitude_damping":
		return noise.AmplitudeDamping(param), nil
	case "phase_damping":
		return noise.PhaseDamping(param), nil
	case "bit_flip":
		return noise.BitFlip(param), nil
	case "phase_flip":
		return noise.PhaseFlip(param), nil
	default:
		return nil, fmt.Errorf("unknown channel: %s", channelType)
	}
}

// channelInfoJS returns operator representations for a noise channel.
// Args: (channelType string, param float64)
func channelInfoJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalChannelInfo(channelInfoResult{Error: "usage: channelInfo(channelType, param)"})
	}
	channelType := args[0].String()
	param := args[1].Float()

	ch, err := buildChannel(channelType, param)
	if err != nil {
		return marshalChannelInfo(channelInfoResult{Error: err.Error()})
	}

	// Convert to Kraus representation.
	k := operator.FromChannel(ch)

	// Get all representations.
	choi := operator.KrausToChoi(k)
	ptm := operator.KrausToPTM(k)

	// Get Kraus operators.
	ops := k.Operators()
	krausOps := make([][][2]float64, len(ops))
	for i, op := range ops {
		krausOps[i] = make([][2]float64, len(op))
		for j, c := range op {
			krausOps[i][j] = [2]float64{real(c), imag(c)}
		}
	}

	// Get Choi matrix.
	choiMat := choi.Matrix()
	choiFlat := make([][2]float64, len(choiMat))
	for i, c := range choiMat {
		choiFlat[i] = [2]float64{real(c), imag(c)}
	}

	// Channel properties.
	isCP := operator.IsCP(choi, 1e-10)
	isTP := operator.IsTP(k, 1e-10)
	procFid := operator.ProcessFidelity(k)
	avgFid := operator.AverageGateFidelity(k)

	dim := 1 << k.NumQubits()

	return marshalChannelInfo(channelInfoResult{
		NumKraus:   len(ops),
		KrausOps:   krausOps,
		ChoiMatrix: choiFlat,
		PTMMatrix:  ptm.Matrix(),
		ChoiDim:    dim * dim,
		PTMDim:     dim * dim,
		IsCP:       isCP,
		IsTP:       isTP,
		ProcessFid: procFid,
		AvgGateFid: avgFid,
	})
}

// channelBlochResult holds Bloch sphere mapping data for a noise channel.
type channelBlochResult struct {
	InputPoints  []blochVector
	OutputPoints []blochVector
	EllipsoidAxes blochVector
	Error         string
}

// channelBlochImageJS computes how a noise channel maps points on the Bloch sphere.
// Args: (channelType string, param float64, numPoints int)
func channelBlochImageJS(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return marshalChannelBloch(channelBlochResult{Error: "usage: channelBlochImage(channelType, param, numPoints)"})
	}
	channelType := args[0].String()
	param := args[1].Float()
	numPoints := args[2].Int()

	numPoints = max(numPoints, 4)
	numPoints = min(numPoints, 10000)

	ch, err := buildChannel(channelType, param)
	if err != nil {
		return marshalChannelBloch(channelBlochResult{Error: err.Error()})
	}

	kraus := ch.Kraus()

	// Sample points on the Bloch sphere using a Fibonacci lattice
	// for approximately uniform distribution.
	inputPoints := make([]blochVector, numPoints)
	outputPoints := make([]blochVector, numPoints)

	goldenRatio := (1 + math.Sqrt(5)) / 2

	for i := range numPoints {
		// Fibonacci sphere sampling.
		theta := math.Acos(1 - 2*float64(i+1)/float64(numPoints+1))
		phi := 2 * math.Pi * float64(i) / goldenRatio
		// No need to reduce phi; Sin/Cos handle arbitrary inputs.

		x := math.Sin(theta) * math.Cos(phi)
		y := math.Sin(theta) * math.Sin(phi)
		z := math.Cos(theta)

		inputPoints[i] = blochVector{X: x, Y: y, Z: z}

		// Convert Bloch vector to density matrix: rho = (I + x*sigmaX + y*sigmaY + z*sigmaZ) / 2
		rho := blochToDensity(x, y, z)

		// Apply channel: rho' = sum_k E_k rho E_k-dagger
		rhoOut := applyChannelToDensity(rho, kraus)

		// Extract output Bloch vector from rho'.
		ox, oy, oz := densityToBloch(rhoOut)
		outputPoints[i] = blochVector{X: ox, Y: oy, Z: oz}
	}

	// Compute ellipsoid axes by checking images of the 6 poles.
	poleInputs := [6][3]float64{
		{1, 0, 0}, {-1, 0, 0}, // +X, -X
		{0, 1, 0}, {0, -1, 0}, // +Y, -Y
		{0, 0, 1}, {0, 0, -1}, // +Z, -Z
	}
	var poleOutputs [6]blochVector
	for i, p := range poleInputs {
		rho := blochToDensity(p[0], p[1], p[2])
		rhoOut := applyChannelToDensity(rho, kraus)
		ox, oy, oz := densityToBloch(rhoOut)
		poleOutputs[i] = blochVector{X: ox, Y: oy, Z: oz}
	}

	// Ellipsoid half-axes: max extent from center to pole images along each axis.
	centerX := (poleOutputs[0].X + poleOutputs[1].X) / 2
	centerY := (poleOutputs[2].Y + poleOutputs[3].Y) / 2
	centerZ := (poleOutputs[4].Z + poleOutputs[5].Z) / 2
	rx := math.Max(math.Abs(poleOutputs[0].X-centerX), math.Abs(poleOutputs[1].X-centerX))
	ry := math.Max(math.Abs(poleOutputs[2].Y-centerY), math.Abs(poleOutputs[3].Y-centerY))
	rz := math.Max(math.Abs(poleOutputs[4].Z-centerZ), math.Abs(poleOutputs[5].Z-centerZ))

	return marshalChannelBloch(channelBlochResult{
		InputPoints:   inputPoints,
		OutputPoints:  outputPoints,
		EllipsoidAxes: blochVector{X: rx, Y: ry, Z: rz},
	})
}

// blochToDensity converts Bloch coordinates (x, y, z) to a 2x2 density matrix.
// rho = (I + x*sigmaX + y*sigmaY + z*sigmaZ) / 2
// Returns flat row-major 2x2 complex matrix.
func blochToDensity(x, y, z float64) []complex128 {
	return []complex128{
		complex((1+z)/2, 0),          // rho[0,0]
		complex(x/2, -y/2),           // rho[0,1]
		complex(x/2, y/2),            // rho[1,0]
		complex((1-z)/2, 0),          // rho[1,1]
	}
}

// densityToBloch extracts Bloch coordinates from a 2x2 density matrix.
// x = 2*Re(rho[0,1]), y = 2*Im(rho[1,0]), z = Re(rho[0,0]) - Re(rho[1,1])
func densityToBloch(rho []complex128) (x, y, z float64) {
	x = 2 * real(rho[1])  // 2*Re(rho[0,1])
	y = 2 * imag(rho[2])  // 2*Im(rho[1,0])
	z = real(rho[0]) - real(rho[3])
	return
}

// applyChannelToDensity applies a single-qubit channel given by Kraus operators
// to a 2x2 density matrix: rho' = sum_k E_k rho E_k-dagger.
func applyChannelToDensity(rho []complex128, kraus [][]complex128) []complex128 {
	result := make([]complex128, 4)
	for _, ek := range kraus {
		// temp = E_k * rho (2x2 matrix multiply)
		t00 := ek[0]*rho[0] + ek[1]*rho[2]
		t01 := ek[0]*rho[1] + ek[1]*rho[3]
		t10 := ek[2]*rho[0] + ek[3]*rho[2]
		t11 := ek[2]*rho[1] + ek[3]*rho[3]
		// result += temp * E_k-dagger
		ekd00, ekd01 := cmplx.Conj(ek[0]), cmplx.Conj(ek[2])
		ekd10, ekd11 := cmplx.Conj(ek[1]), cmplx.Conj(ek[3])
		result[0] += t00*ekd00 + t01*ekd10
		result[1] += t00*ekd01 + t01*ekd11
		result[2] += t10*ekd00 + t11*ekd10
		result[3] += t10*ekd01 + t11*ekd11
	}
	return result
}

// marshalChannelBloch marshals the channel Bloch mapping result to JSON.
func marshalChannelBloch(r channelBlochResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "inputPoints", &n)
	writeBlochPoints(&b, r.InputPoints)

	jsonKey(&b, "outputPoints", &n)
	writeBlochPoints(&b, r.OutputPoints)

	jsonKey(&b, "ellipsoidAxes", &n)
	b.WriteString(`{"rx":`)
	jsonFloat(&b, r.EllipsoidAxes.X)
	b.WriteString(`,"ry":`)
	jsonFloat(&b, r.EllipsoidAxes.Y)
	b.WriteString(`,"rz":`)
	jsonFloat(&b, r.EllipsoidAxes.Z)
	b.WriteByte('}')

	b.WriteByte('}')
	return b.String()
}

// writeBlochPoints writes a JSON array of {x, y, z} objects.
func writeBlochPoints(b *strings.Builder, pts []blochVector) {
	b.WriteByte('[')
	for i, p := range pts {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"x":`)
		jsonFloat(b, p.X)
		b.WriteString(`,"y":`)
		jsonFloat(b, p.Y)
		b.WriteString(`,"z":`)
		jsonFloat(b, p.Z)
		b.WriteByte('}')
	}
	b.WriteByte(']')
}

// marshalChannelInfo marshals the channel info result to JSON.
func marshalChannelInfo(r channelInfoResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "numKraus", &n)
	jsonInt(&b, r.NumKraus)

	// Kraus operators: array of arrays of {re, im} pairs.
	jsonKey(&b, "krausOps", &n)
	b.WriteByte('[')
	for i, op := range r.KrausOps {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('[')
		for j, c := range op {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"re":`)
			jsonFloat(&b, c[0])
			b.WriteString(`,"im":`)
			jsonFloat(&b, c[1])
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	b.WriteByte(']')

	// Choi matrix.
	jsonKey(&b, "choiMatrix", &n)
	b.WriteByte('[')
	for i, c := range r.ChoiMatrix {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"re":`)
		jsonFloat(&b, c[0])
		b.WriteString(`,"im":`)
		jsonFloat(&b, c[1])
		b.WriteByte('}')
	}
	b.WriteByte(']')

	jsonKey(&b, "choiDim", &n)
	jsonInt(&b, r.ChoiDim)

	// PTM matrix.
	jsonKey(&b, "ptmMatrix", &n)
	jsonFloats(&b, r.PTMMatrix)

	jsonKey(&b, "ptmDim", &n)
	jsonInt(&b, r.PTMDim)

	jsonKey(&b, "isCP", &n)
	jsonBool(&b, r.IsCP)

	jsonKey(&b, "isTP", &n)
	jsonBool(&b, r.IsTP)

	jsonKey(&b, "processFidelity", &n)
	jsonFloat(&b, r.ProcessFid)

	jsonKey(&b, "avgGateFidelity", &n)
	jsonFloat(&b, r.AvgGateFid)

	b.WriteByte('}')
	return b.String()
}
