//go:build js && wasm

package main

import (
	"fmt"
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
