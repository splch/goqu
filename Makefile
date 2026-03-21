# Separate Go modules that live outside the root go.mod.
EXTRA_MODULES := backend/braket backend/google backend/rigetti \
                 observe/otelbridge observe/prombridge \
                 sim/gpu/cuda sim/gpu/metal

.PHONY: test test-race test-all lint vet fuzz bench \
        test-gpu-cuda bench-gpu-cuda \
        coverage clean hooks \
        textbook textbook-pdf textbook-serve textbook-clean

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------

test:
	go test -count=1 -timeout=5m ./...

test-race:
	go test -race -count=1 -timeout=5m ./...

test-all: test-race
	@for mod in $(EXTRA_MODULES); do \
		echo "--- $$mod ---"; \
		if ls $$mod/*_test.go >/dev/null 2>&1; then \
			(cd $$mod && go test -race -count=1 -timeout=5m ./...); \
		else \
			(cd $$mod && go build ./...); \
		fi; \
	done

# ---------------------------------------------------------------------------
# Static analysis
# ---------------------------------------------------------------------------

lint:
	golangci-lint run ./...
	@for mod in $(EXTRA_MODULES); do \
		echo "--- lint $$mod ---"; \
		(cd $$mod && golangci-lint run ./...); \
	done

vet:
	go vet ./...
	@for mod in $(EXTRA_MODULES); do \
		(cd $$mod && go vet ./...); \
	done

# ---------------------------------------------------------------------------
# Fuzzing (30s per target)
# ---------------------------------------------------------------------------

FUZZ_TIME ?= 30s

fuzz:
	go test ./qasm/parser -run=^$$ -fuzz=FuzzParse -fuzztime=$(FUZZ_TIME)
	go test ./qasm/parser -run=^$$ -fuzz=FuzzRoundTrip -fuzztime=$(FUZZ_TIME)
	go test ./qasm/emitter -run=^$$ -fuzz=FuzzEmit -fuzztime=$(FUZZ_TIME)
	go test ./qasm/emitter -run=^$$ -fuzz=FuzzEmitAllGateTypes -fuzztime=$(FUZZ_TIME)
	go test ./transpile/pass -run=^$$ -fuzz=FuzzDecomposeToTarget -fuzztime=$(FUZZ_TIME)
	go test ./transpile/pass -run=^$$ -fuzz=FuzzDecomposeToSimulator -fuzztime=$(FUZZ_TIME)
	go test ./transpile/pass -run=^$$ -fuzz=FuzzCancelAdjacent -fuzztime=$(FUZZ_TIME)
	go test ./transpile/pass -run=^$$ -fuzz=FuzzMergeRotations -fuzztime=$(FUZZ_TIME)
	go test ./transpile/pass -run=^$$ -fuzz=FuzzCancelAdjacentInversePairs -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzMarshalCircuit -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzMarshalNativeCircuit -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzDetectGateset -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzBitstring -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzRadiansToTurns -fuzztime=$(FUZZ_TIME)
	go test ./backend/ionq -run=^$$ -fuzz=FuzzMarshalPulseShapes -fuzztime=$(FUZZ_TIME)
	go test ./pulse -run=^$$ -fuzz=FuzzBuildProgram -fuzztime=$(FUZZ_TIME)
	go test ./pulse -run=^$$ -fuzz=FuzzProgramStats -fuzztime=$(FUZZ_TIME)
	go test ./pulse/qasmparse -run=^$$ -fuzz=FuzzParsePulse -fuzztime=$(FUZZ_TIME)
	go test ./pulse/qasmparse -run=^$$ -fuzz=FuzzRoundTripPulse -fuzztime=$(FUZZ_TIME)
	go test ./pulse/waveform -run=^$$ -fuzz=FuzzWaveformSample -fuzztime=$(FUZZ_TIME)
	go test ./sim/pulsesim -run=^$$ -fuzz=FuzzEvolve -fuzztime=$(FUZZ_TIME)
	go test ./sim/pulsesim -run=^$$ -fuzz=FuzzEvolve2Q -fuzztime=$(FUZZ_TIME)
	cd backend/braket && go test -run=^$$ -fuzz=FuzzSerializePulseProgram -fuzztime=$(FUZZ_TIME) ./...

# ---------------------------------------------------------------------------
# Benchmarks
# ---------------------------------------------------------------------------

bench:
	go test ./sim/statevector/ -bench=. -count=5 -benchmem -run=^$$ -timeout=10m
	go test ./sim/densitymatrix/ -bench=. -count=5 -benchmem -run=^$$ -timeout=10m

test-gpu-cuda:
	cd sim/gpu/cuda && go test -tags cuda -count=1 -timeout=5m ./...

bench-gpu-cuda:
	cd sim/gpu/cuda && go test -tags cuda -bench=. -count=5 -benchmem -run=^$$ -timeout=10m

# ---------------------------------------------------------------------------
# Coverage
# ---------------------------------------------------------------------------

coverage:
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ---------------------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------------------

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks/"

clean:
	rm -f coverage.out coverage.html
	go clean -testcache

# ---------------------------------------------------------------------------
# Textbook
# ---------------------------------------------------------------------------

textbook:
	go run textbook/gen/main.go
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" textbook/wasm_exec.js
	cd textbook/wasm && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-w -s" -o ../main.wasm .
	@command -v wasm-opt >/dev/null 2>&1 && wasm-opt -Oz --enable-bulk-memory -o textbook/main.wasm textbook/main.wasm || true

textbook-pdf: textbook
	@test -d textbook/node_modules || (cd textbook && npm install --silent)
	cd textbook && node gen-pdf.mjs

textbook-serve: textbook
	@echo "Serving textbook at http://localhost:8080"
	python3 -m http.server 8080 -d textbook

textbook-clean:
	rm -f textbook/index.html textbook/style.css textbook/main.wasm textbook/wasm_exec.js textbook/_print.html textbook/goqu-textbook.pdf
	rm -rf textbook/chapters/
