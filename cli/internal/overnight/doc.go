// Package overnight implements the Dream nightly compounding loop.
//
// Dream runs a bounded outer loop against the local .agents/ corpus. Each
// iteration is a three-stage wave:
//
//   - INGEST (parallel-safe, read-only): harvest, forge-mine, and bounded
//     read-side finding generators. Generators may fan out in process and write
//     sidecars under the run output directory, but never mutate .agents/.
//
//   - REDUCE (serial, mutative, checkpointed): dedup, maturity temper,
//     defrag --apply, close-loop promote, findings→next-work router,
//     generator-sidecar aggregation, inject-cache refresh. Every mutation
//     writes through a checkpoint overlay with 2-phase commit semantics, so any
//     failure rolls the iteration back to the last known-good state.
//
//   - MEASURE (parallel-safe, read-only): retrieval-bench, metrics health,
//     corpus-quality fitness vector, inject-visibility probe, findings
//     resolution delta.
//
// After MEASURE, the loop computes a delta against the previous iteration's
// fitness vector. The loop halts on the first of: wall-clock budget
// exhaustion, plateau (K consecutive sub-epsilon deltas), or fitness
// regression beyond per-metric floors.
//
// # Anti-goals
//
// This package explicitly does NOT:
//
//   - mutate source code of any kind,
//   - invoke /rpi or any code-mutating flow,
//   - touch git (no commits, branches, pushes, or remote calls),
//   - create symlinks anywhere,
//   - fan work out to swarm / gc agents inside iterations. Only bounded
//     in-process read-side generator goroutines are allowed, and REDUCE remains
//     the serialized queue writer.
//
// Writes are confined to .agents/ and, via harvest, to ~/.agents/learnings/
// (the global hub). A concurrency guard in cli/cmd/ao/harvest.go prevents
// manual harvest runs from racing a live Dream run.
//
// # Boundaries
//
// The only subpaths Dream mutates (and therefore checkpoints) are:
//
//   - .agents/learnings/
//   - .agents/findings/
//   - .agents/patterns/
//   - .agents/knowledge/
//   - .agents/rpi/next-work.jsonl
//
// Everything else under .agents/ is untouched. See
// docs/contracts/dream-run-contract.md for the v2 contract that pins this
// list.
//
// # Delineation vs /evolve
//
// /evolve is the day-time loop: code + knowledge, operator-driven, full /rpi
// per cycle. /dream is the nightly loop: knowledge-only, bounded, never /rpi.
// Both share ao goals measure (and the new ao corpus fitness) as fitness
// sources of truth, so morning /evolve starts against a freshly-compounded
// corpus with a clean baseline.
package overnight
