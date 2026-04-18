# Benchmark Report

Generated: 2026-04-17 (Apple M1 Pro, darwin/arm64, Go 1.22)

## Macrobenchmarks — enterprise-scale graphs

| Benchmark | Nodes | Edges | Time/op | B/op | allocs/op | Custom |
|-----------|------:|------:|--------:|-----:|----------:|--------|
| FindPaths_10k | 10,030 | 60,811 | 14.2 µs | 936 | 14 | 2 paths/op |
| Optimizer_10k | 10,030 | 60,811 | 440 µs | 596 KB | 6,316 | 10 breakpoints/op |
| Diff_10k | 10,030 | 60,811 | 47.6 ms | 8.7 MB | 611 | 9 drift-items/op |
| Ingest_100k_edges | 5,000 nodes | 100,000 edges | 192 ms | 144 MB | 525,118 | 100,000 edges |

## Microbenchmarks — regression gates

| Benchmark | Time/op | B/op | allocs/op |
|-----------|--------:|-----:|----------:|
| FindPaths_Chain10 | 1,209 ns | 904 | 9 |
| FindPaths_Chain50 | 5,910 ns | 3,595 | 10 |
| FindPaths_Chain100 | 11,757 ns | 7,180 | 10 |
| FindPaths_FanOut | 9,509 ns | 740 | 10 |

## Optimizer before/after (Commit 8)

Greedy set-cover with 500 scored paths, 10 breakpoints requested:

| | ns/op | B/op | allocs/op |
|-|------:|-----:|----------:|
| Before (map[int]bool pathIdxs, re-scan each iter) | 1,170,000 | 601,525 | 1,780 |
| After ([]int pathIdxs + incremental count updates) | 435,000 | 595,997 | 6,316 |
| **Delta** | **−2.7×** | −1% | +3.5× allocs (smaller slices, same byte footprint) |

The alloc count increase comes from slice-growth amortisation replacing map-bucket allocs;
total bytes are equivalent. Wall-clock time improved **2.7×**.
