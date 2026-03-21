.PHONY: test test-race test-all lint vet fuzz bench bench-gpu-cuda test-gpu-cuda coverage clean hooks textbook textbook-pdf textbook-serve textbook-clean

test:
	go test -count=1 -timeout=5m ./...

test-race:
	go test -race -count=1 -timeout=5m ./...

test-all: test-race
	cd backend/braket && go test -race -count=1 -timeout=5m ./...
	cd observe/otelbridge && go build ./...
	cd observe/prombridge && go build ./...
	cd sim/gpu/cuda && go build ./...
	cd sim/gpu/metal && go build ./...

lint:
	golangci-lint run ./...
	cd backend/braket && golangci-lint run ./...

vet:
	go vet ./...
	cd backend/braket && go vet ./...
	cd observe/otelbridge && go vet ./...
	cd observe/prombridge && go vet ./...
	cd sim/gpu/cuda && go vet ./...
	cd sim/gpu/metal && go vet ./...

fuzz:
	go test ./qasm/parser -run=^$$ -fuzz=FuzzParse -fuzztime=30s
	go test ./qasm/parser -run=^$$ -fuzz=FuzzRoundTrip -fuzztime=30s
	go test ./qasm/emitter -run=^$$ -fuzz=FuzzEmit -fuzztime=30s
	go test ./qasm/emitter -run=^$$ -fuzz=FuzzEmitAllGateTypes -fuzztime=30s
	go test ./transpile/pass -run=^$$ -fuzz=FuzzDecomposeToTarget -fuzztime=30s
	go test ./transpile/pass -run=^$$ -fuzz=FuzzDecomposeToSimulator -fuzztime=30s
	go test ./transpile/pass -run=^$$ -fuzz=FuzzCancelAdjacent -fuzztime=30s
	go test ./transpile/pass -run=^$$ -fuzz=FuzzMergeRotations -fuzztime=30s
	go test ./transpile/pass -run=^$$ -fuzz=FuzzCancelAdjacentInversePairs -fuzztime=30s
	go test ./backend/ionq -run=^$$ -fuzz=FuzzMarshalCircuit -fuzztime=30s
	go test ./backend/ionq -run=^$$ -fuzz=FuzzMarshalNativeCircuit -fuzztime=30s
	go test ./backend/ionq -run=^$$ -fuzz=FuzzDetectGateset -fuzztime=30s
	go test ./backend/ionq -run=^$$ -fuzz=FuzzBitstring -fuzztime=30s
	go test ./backend/ionq -run=^$$ -fuzz=FuzzRadiansToTurns -fuzztime=30s

bench:
	go test ./sim/statevector/ -bench=. -count=5 -benchmem -run=^$$ -timeout=10m
	go test ./sim/densitymatrix/ -bench=. -count=5 -benchmem -run=^$$ -timeout=10m

test-gpu-cuda:
	cd sim/gpu/cuda && go test -tags cuda -count=1 -timeout=5m ./...

bench-gpu-cuda:
	cd sim/gpu/cuda && go test -tags cuda -bench=. -count=5 -benchmem -run=^$$ -timeout=10m

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks/"

clean:
	rm -f coverage.out coverage.html
	go clean -testcache

textbook:
	go run textbook/gen/main.go
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" textbook/wasm_exec.js
	cd textbook/wasm && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-w -s" -o ../main.wasm .
	@command -v wasm-opt >/dev/null 2>&1 && wasm-opt -Oz --enable-bulk-memory -o textbook/main.wasm textbook/main.wasm || true

textbook-pdf: textbook
	cd textbook && npm install --silent && node gen-pdf.mjs

textbook-serve: textbook
	@echo "Serving textbook at http://localhost:8080"
	python3 -m http.server 8080 -d textbook

textbook-clean:
	rm -f index.html style.css textbook/main.wasm textbook/wasm_exec.js textbook/chapters/*
