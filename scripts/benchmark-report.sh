#!/usr/bin/env bash
# Generate BENCHMARKS.md from go test -bench output.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Running benchmarks (this may take several minutes)..."
BENCH_LINES=$(go test -bench=. -benchmem -count=1 ./... 2>&1 | grep "^Benchmark" || true)

if [[ -z "$BENCH_LINES" ]]; then
    echo "No benchmark output found." >&2
    exit 1
fi

{
cat <<'HEADER'
# Benchmark Report

HEADER

echo "Generated: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo ""
echo "| Benchmark | ns/op | B/op | allocs/op |"
echo "|-----------|------:|-----:|----------:|"

echo "$BENCH_LINES" | awk '{
    name = $1
    sub(/-[0-9]+$/, "", name)   # strip -8 / -10 GOMAXPROCS suffix
    ns = ""; bop = ""; aop = ""
    for (i = 3; i < NF; i++) {
        u = $(i+1)
        if      (u == "ns/op")      { ns  = $i; i++ }
        else if (u == "B/op")       { bop = $i; i++ }
        else if (u == "allocs/op")  { aop = $i; i++ }
    }
    printf "| %s | %s | %s | %s |\n", name, ns, bop, aop
}'

} > "$ROOT/BENCHMARKS.md"

echo "Written to BENCHMARKS.md"
