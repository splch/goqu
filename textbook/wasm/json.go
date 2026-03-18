//go:build js && wasm

package main

import (
	"strconv"
	"strings"
)

// Manual JSON marshaling to avoid importing encoding/json and its heavy
// reflect dependency, which adds ~1 MB to the WASM binary.

func marshalRun(r runResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.Circuit != "" {
		jsonKey(&b, "circuit", &n)
		jsonStr(&b, r.Circuit)
	}
	if r.Histogram != "" {
		jsonKey(&b, "histogram", &n)
		jsonStr(&b, r.Histogram)
	}
	if r.Bloch != "" {
		jsonKey(&b, "bloch", &n)
		jsonStr(&b, r.Bloch)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalState(r stateResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Amplitudes) > 0 {
		jsonKey(&b, "amplitudes", &n)
		b.WriteByte('[')
		for i, a := range r.Amplitudes {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"re":`)
			jsonFloat(&b, a.Re)
			b.WriteString(`,"im":`)
			jsonFloat(&b, a.Im)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	if len(r.Probabilities) > 0 {
		jsonKey(&b, "probabilities", &n)
		jsonFloats(&b, r.Probabilities)
	}
	if len(r.BlochVectors) > 0 {
		jsonKey(&b, "blochVectors", &n)
		b.WriteByte('[')
		for i, v := range r.BlochVectors {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"x":`)
			jsonFloat(&b, v.X)
			b.WriteString(`,"y":`)
			jsonFloat(&b, v.Y)
			b.WriteString(`,"z":`)
			jsonFloat(&b, v.Z)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalProb(r probResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Probabilities) > 0 {
		jsonKey(&b, "probabilities", &n)
		jsonFloats(&b, r.Probabilities)
	}
	if len(r.Labels) > 0 {
		jsonKey(&b, "labels", &n)
		b.WriteByte('[')
		for i, s := range r.Labels {
			if i > 0 {
				b.WriteByte(',')
			}
			jsonStr(&b, s)
		}
		b.WriteByte(']')
	}
	if r.Histogram != "" {
		jsonKey(&b, "histogram", &n)
		jsonStr(&b, r.Histogram)
	}
	if r.Circuit != "" {
		jsonKey(&b, "circuit", &n)
		jsonStr(&b, r.Circuit)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// jsonKey writes ,"key": (with leading comma if not first field).
func jsonKey(b *strings.Builder, key string, n *int) {
	if *n > 0 {
		b.WriteByte(',')
	}
	*n++
	b.WriteByte('"')
	b.WriteString(key)
	b.WriteString(`":`)
}

// jsonStr writes a JSON-escaped string value.
func jsonStr(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte("0123456789abcdef"[r>>4])
				b.WriteByte("0123456789abcdef"[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// jsonFloat writes a float64 as a JSON number.
func jsonFloat(b *strings.Builder, f float64) {
	b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
}

// jsonFloats writes a JSON array of float64 values.
func jsonFloats(b *strings.Builder, fs []float64) {
	b.WriteByte('[')
	for i, f := range fs {
		if i > 0 {
			b.WriteByte(',')
		}
		jsonFloat(b, f)
	}
	b.WriteByte(']')
}
