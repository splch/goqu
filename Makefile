.PHONY: test test-race test-all lint vet fuzz bench coverage clean hooks

test:
	go test -count=1 -timeout=5m ./...

test-race:
	go test -race -count=1 -timeout=5m ./...

test-all: test-race
	cd backend/braket && go test -race -count=1 -timeout=5m ./...
	cd observe/otelbridge && go build ./...
	cd observe/prombridge && go build ./...

lint:
	golangci-lint run ./...
	cd backend/braket && golangci-lint run ./...

vet:
	go vet ./...
	cd backend/braket && go vet ./...
	cd observe/otelbridge && go vet ./...
	cd observe/prombridge && go vet ./...

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
