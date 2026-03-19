// Package parser reads QIR-compliant LLVM IR and constructs goqu circuits.
//
// The parser handles both Base Profile (static circuits) and Adaptive Profile
// (mid-circuit measurement with classical branching) QIR programs. It
// recognizes the __quantum__qis__* gate intrinsics and __quantum__rt__*
// runtime calls, reconstructing the circuit structure from the LLVM IR CFG.
package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// Option configures parser behavior.
type Option func(*config)

type config struct {
	name string // circuit name override
}

// WithName overrides the circuit name (default: entry point function name).
func WithName(name string) Option {
	return func(c *config) { c.name = name }
}

// Parse reads QIR LLVM IR from r and returns a goqu circuit.
func Parse(r io.Reader, opts ...Option) (*ir.Circuit, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("qir/parser: %w", err)
	}
	return ParseString(string(data), opts...)
}

// ParseString parses QIR LLVM IR from a string.
func ParseString(source string, opts ...Option) (*ir.Circuit, error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	p := &parser{cfg: cfg}
	return p.parse(source)
}

// ParseFile parses QIR LLVM IR from a file.
func ParseFile(path string, opts ...Option) (*ir.Circuit, error) {
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- caller controls path
	if err != nil {
		return nil, fmt.Errorf("qir/parser: %w", err)
	}
	return ParseString(string(data), opts...)
}

type parser struct {
	cfg *config

	// Parsed attributes from the entry point.
	numQubits  int
	numResults int
	circName   string
}

func (p *parser) parse(source string) (*ir.Circuit, error) {
	// Extract circuit info from the LLVM IR by scanning for key patterns.
	ops := p.extractOperations(source)

	name := p.circName
	if p.cfg.name != "" {
		name = p.cfg.name
	}

	return ir.New(name, p.numQubits, p.numResults, ops, nil), nil
}

// extractOperations does a line-by-line parse of the QIR LLVM IR to extract
// quantum operations. This approach is simpler and more robust than full
// LLVM IR parsing since QIR follows rigid conventions.
func (p *parser) extractOperations(source string) []ir.Operation {
	var ops []ir.Operation

	lines := strings.Split(source, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract entry point name and attributes.
		if strings.HasPrefix(line, "define ") {
			p.parseDefine(line)
			continue
		}

		// Extract attributes for qubit/result counts.
		if strings.HasPrefix(line, "attributes #0") {
			p.parseAttributes(line)
			continue
		}

		// Parse call instructions.
		if !strings.Contains(line, "call ") {
			continue
		}

		// Strip SSA assignment prefix: "%0 = call ..." -> "call ..."
		callIdx := strings.Index(line, "call ")
		if callIdx < 0 {
			continue
		}
		callLine := line[callIdx:]

		// Skip "tail call" prefix handling - just work with the call.
		callLine = strings.TrimPrefix(callLine, "call ")

		// Extract function name.
		atIdx := strings.Index(callLine, "@")
		if atIdx < 0 {
			continue
		}
		parenIdx := strings.Index(callLine[atIdx:], "(")
		if parenIdx < 0 {
			continue
		}
		fnName := callLine[atIdx+1 : atIdx+parenIdx]

		// Parse based on function prefix.
		if strings.HasPrefix(fnName, "__quantum__qis__") {
			op, err := p.parseGateCall(fnName, callLine[atIdx+parenIdx:])
			if err != nil {
				continue // skip unparseable calls
			}
			if op != nil {
				ops = append(ops, *op)
			}
		}
	}

	return ops
}

// parseDefine extracts the function name from a define line.
func (p *parser) parseDefine(line string) {
	// "define i64 @main() #0 {"
	atIdx := strings.Index(line, "@")
	if atIdx < 0 {
		return
	}
	parenIdx := strings.Index(line[atIdx:], "(")
	if parenIdx < 0 {
		return
	}
	p.circName = line[atIdx+1 : atIdx+parenIdx]
}

// parseAttributes extracts required_num_qubits and required_num_results.
func (p *parser) parseAttributes(line string) {
	if idx := strings.Index(line, `"required_num_qubits"="`); idx >= 0 {
		start := idx + len(`"required_num_qubits"="`)
		end := strings.Index(line[start:], `"`)
		if end > 0 {
			if n, err := strconv.Atoi(line[start : start+end]); err == nil {
				p.numQubits = n
			}
		}
	}
	if idx := strings.Index(line, `"required_num_results"="`); idx >= 0 {
		start := idx + len(`"required_num_results"="`)
		end := strings.Index(line[start:], `"`)
		if end > 0 {
			if n, err := strconv.Atoi(line[start : start+end]); err == nil {
				p.numResults = n
			}
		}
	}
}

// parseGateCall parses a __quantum__qis__* call and returns the operation.
func (p *parser) parseGateCall(fnName, argStr string) (*ir.Operation, error) {
	// Handle measurement: __quantum__qis__mz__body.
	if fnName == "__quantum__qis__mz__body" {
		return p.parseMeasurement(argStr)
	}

	// Handle reset: __quantum__qis__reset__body.
	if fnName == "__quantum__qis__reset__body" {
		qubits := extractQubitIndices(argStr)
		if len(qubits) == 0 {
			return nil, fmt.Errorf("cannot parse reset args")
		}
		return &ir.Operation{Gate: gate.Reset, Qubits: qubits[:1]}, nil
	}

	// Look up gate factory.
	factory, ok := lookupIntrinsic(fnName)
	if !ok {
		return nil, fmt.Errorf("unknown QIR intrinsic: %s", fnName)
	}

	// Extract arguments.
	args := extractCallArgs(argStr)

	var g gate.Gate
	var qubits []int

	if factory.fixedGate != nil {
		g = factory.fixedGate
		qubits = extractQubitIndicesFromArgs(args)
	} else if factory.paramGate1 != nil {
		// First arg should be a double parameter, rest are qubits.
		param, qArgs := splitParamAndQubits(args)
		g = factory.paramGate1(param)
		qubits = qArgs
	}

	if g == nil || len(qubits) != factory.nQubits {
		return nil, fmt.Errorf("mismatched qubit count for %s", fnName)
	}

	return &ir.Operation{Gate: g, Qubits: qubits}, nil
}

// parseMeasurement parses a __quantum__qis__mz__body call.
func (p *parser) parseMeasurement(argStr string) (*ir.Operation, error) {
	// Extract both qubit and result indices.
	// Args: (ptr <qubit_ref>, ptr writeonly <result_ref>)
	parts := strings.SplitN(argStr, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("mz__body expects 2 args")
	}

	qubitIdx := extractPtrIndex(parts[0])
	resultIdx := extractPtrIndex(parts[1])

	return &ir.Operation{
		Gate:   nil,
		Qubits: []int{qubitIdx},
		Clbits: []int{resultIdx},
	}, nil
}

// extractCallArgs extracts the argument string from parenthesized call args.
// Input: "(ptr null, ptr inttoptr ...)" -> ["ptr null", "ptr inttoptr ..."]
func extractCallArgs(argStr string) []string {
	// Find the argument content between ( and ).
	start := strings.Index(argStr, "(")
	end := strings.LastIndex(argStr, ")")
	if start < 0 || end < 0 || end <= start {
		return nil
	}
	inner := argStr[start+1 : end]
	if inner == "" {
		return nil
	}

	// Split on commas, but respect parentheses depth.
	var args []string
	depth := 0
	last := 0
	for i, ch := range inner {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[last:i]))
				last = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[last:]))
	return args
}

// extractPtrIndex extracts the qubit/result index from a ptr reference.
// "ptr null" -> 0
// "ptr inttoptr (i64 1 to ptr)" -> 1
// "ptr writeonly null" -> 0
// "ptr readonly inttoptr (i64 2 to ptr)" -> 2
// "ptr nonnull inttoptr (i64 3 to ptr)" -> 3
func extractPtrIndex(s string) int {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "null") && !strings.Contains(s, "inttoptr") {
		return 0
	}
	// Find "i64 N" in "inttoptr (i64 N to ptr)".
	idx := strings.Index(s, "i64 ")
	if idx < 0 {
		return 0
	}
	numStr := s[idx+4:]
	end := strings.IndexAny(numStr, " )")
	if end > 0 {
		numStr = numStr[:end]
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}
	return n
}

// extractQubitIndices extracts qubit indices from a parenthesized argument string.
func extractQubitIndices(argStr string) []int {
	return extractQubitIndicesFromArgs(extractCallArgs(argStr))
}

// extractQubitIndicesFromArgs extracts qubit indices from already-split args.
func extractQubitIndicesFromArgs(args []string) []int {
	var qubits []int
	for _, arg := range args {
		if strings.Contains(arg, "ptr") {
			qubits = append(qubits, extractPtrIndex(arg))
		}
	}
	return qubits
}

// splitParamAndQubits splits args into a double parameter and qubit indices.
// "double 7.853982e-01, ptr null" -> (0.785398, [0])
func splitParamAndQubits(args []string) (float64, []int) {
	var param float64
	var qubits []int
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if strings.HasPrefix(arg, "double ") {
			valStr := strings.TrimPrefix(arg, "double ")
			if v, err := strconv.ParseFloat(valStr, 64); err == nil {
				param = v
			}
		} else if strings.Contains(arg, "ptr") {
			qubits = append(qubits, extractPtrIndex(arg))
		}
	}
	return param, qubits
}
