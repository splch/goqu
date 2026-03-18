//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/splch/goqu/viz"
)

func renderBlochJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return ""
	}
	state := []complex128{
		complex(args[0].Float(), args[1].Float()),
		complex(args[2].Float(), args[3].Float()),
	}
	dark := len(args) >= 5 && args[4].Truthy()
	var opts []viz.Option
	if dark {
		opts = append(opts, viz.WithStyle(viz.DarkStyle()))
	}
	return viz.Bloch(state, opts...)
}
