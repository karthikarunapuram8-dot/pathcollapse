GOFLAGS ?=

.PHONY: bench benchmark-report cpu-profile mem-profile race test build clean

## Run all benchmarks with allocation reporting.
bench:
	go test -bench=. -benchmem -count=3 ./...

## Run all benchmarks (count=1) and write results to BENCHMARKS.md.
benchmark-report:
	bash scripts/benchmark-report.sh

## Capture a CPU profile for FindPaths on the chain benchmarks.
## Output: cpu.prof  (view with: go tool pprof cpu.prof)
cpu-profile:
	go test -bench=BenchmarkFindPaths -benchmem -cpuprofile=cpu.prof ./pkg/graph/
	@echo "Profile written to cpu.prof — run: go tool pprof cpu.prof"

## Capture a heap profile for FindPaths on the chain benchmarks.
## Output: mem.prof  (view with: go tool pprof -alloc_objects mem.prof)
mem-profile:
	go test -bench=BenchmarkFindPaths -benchmem -memprofile=mem.prof ./pkg/graph/
	@echo "Profile written to mem.prof — run: go tool pprof -alloc_objects mem.prof"

## Run the full test suite with the race detector (required CI gate).
race:
	go test -race ./...

## Run all tests.
test:
	go test ./...

## Build all binaries.
build:
	go build ./...

## Remove generated profile files.
clean:
	rm -f cpu.prof mem.prof
