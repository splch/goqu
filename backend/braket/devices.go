package braket

import "github.com/splch/qgo/transpile/target"

// deviceARNs maps short device names to full AWS Braket device ARNs.
var deviceARNs = map[string]string{
	"ionq.forte":    "arn:aws:braket:us-east-1::device/qpu/ionq/Forte-Enterprise-1",
	"iqm.garnet":    "arn:aws:braket:eu-north-1::device/qpu/iqm/Garnet",
	"rigetti.ankaa": "arn:aws:braket:us-west-1::device/qpu/rigetti/Ankaa-3",
	"sv1":           "arn:aws:braket:::device/quantum-simulator/amazon/sv1",
}

// deviceTargets maps short device names to transpilation targets.
var deviceTargets = map[string]target.Target{
	"ionq.forte":    target.IonQForte,
	"iqm.garnet":    iqmGarnet,
	"rigetti.ankaa": rigettiAnkaa,
	"sv1":           target.Simulator,
}

var iqmGarnet = target.Target{
	Name:       "iqm.garnet",
	NumQubits:  20,
	BasisGates: []string{"CZ", "RX", "RY", "RZ"},
}

var rigettiAnkaa = target.Target{
	Name:       "rigetti.ankaa",
	NumQubits:  84,
	BasisGates: []string{"CZ", "RX", "RZ"},
}

// DeviceARN returns the full ARN for a short device name.
func DeviceARN(name string) (string, bool) {
	arn, ok := deviceARNs[name]
	return arn, ok
}

// DeviceTarget returns the transpilation target for a device.
// Returns target.Simulator if the device is unknown.
func DeviceTarget(name string) target.Target {
	if t, ok := deviceTargets[name]; ok {
		return t
	}
	return target.Simulator
}
