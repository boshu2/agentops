# Profiling & Tooling — pick the right measurement for the language

Read this when choosing how to capture the baseline profile in **Phase 1**. The
rule is the same across languages: capture a *saved artifact* (not a glance at a
terminal), rank frames by self-time, and target the top 1–3.

## Timing harness (always do this first)

A reproducible wall-clock harness is the floor. Prefer a tool that reports
median + spread over N runs so you get a noise band:

- `hyperfine --warmup 3 --runs 20 '<cmd>'` — language-agnostic CLI timing.
- `go test -bench=. -benchmem -count=10` — Go, with allocation counts.
- `pytest-benchmark` / `python -m timeit` — Python.
- `cargo bench` (criterion) — Rust, statistically rigorous.
- `jmh` — JVM.
- `console.time` / `tinybench` / `node --prof` — Node.

Record N, the median, and the noise band (e.g. p50 ± IQR). A "win" must exceed
the noise band — that threshold is what `scripts/optguard.sh` enforces.

## CPU profilers (find the hot frame)

| Language | Tool | Saved artifact |
|----------|------|----------------|
| Go | `go test -cpuprofile cpu.out` / `runtime/pprof` | `cpu.out` → `go tool pprof -top` |
| Python | `py-spy record -o prof.svg` / `cProfile` | flamegraph SVG / `.pstats` |
| Rust | `cargo flamegraph` / `perf record` | `flamegraph.svg` / `perf.data` |
| C/C++ | `perf record -g` / Instruments | `perf.data` |
| Node | `node --prof` / `clinic flame` | `isolate-*.log` / flamegraph |
| JVM | `async-profiler` / JFR | `.jfr` / collapsed stacks |
| Any (Linux) | `perf record -g -- <cmd>` | `perf.data` → `perf report` |

Save the artifact into `profile/`. The agent must be able to re-read it next
iteration to confirm the bottleneck *moved*.

## Memory & allocation profilers

- Go: `-memprofile mem.out`, `go tool pprof -alloc_space`.
- Python: `tracemalloc`, `memray`, `scalene` (also CPU).
- Rust: `dhat`, `heaptrack`, `valgrind --tool=massif`.
- C/C++: `valgrind --tool=massif`, `heaptrack`.
- Node: `--inspect` heap snapshots, `clinic heapprofiler`.

Allocation count is often the lever even when wall time looks CPU-bound — GC and
allocator pressure show up as scattered self-time.

## Systems-level signals

- I/O & syscalls: `strace -c`, `dtrace`, DB slow-query logs, request spans.
- Concurrency: lock-contention profiles (`go tool pprof -blockprofile`),
  thread/goroutine dumps, scheduler traces, queue depth.
- Cache locality: `perf stat -e cache-misses,LLC-load-misses` — confirms a
  memory-bound vs CPU-bound hypothesis before you try parallelism.
- Core saturation: `perf stat` IPC + `mpstat -P ALL` / `htop` — verify all cores
  are actually busy before claiming a parallelism win.

## Discipline

- Profile a **release/optimized build**, not debug — debug overhead masks the real frame.
- Warm caches and JIT first; discard warmup runs.
- One workload, one machine, one config across the whole loop — change the code, not the bench.
- A profile that shows flat cost (no dominant frame) means *stop* — there is no extreme win to extract.
