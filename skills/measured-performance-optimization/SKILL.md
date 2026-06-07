---
name: measured-performance-optimization
description: |-
  Use when optimizing a hot path from saved profiles, measurements, and verified improvements.
  Triggers:
practices:
- profile-before-optimize
- correctness-preserving-refactor
- evidence-driven-iteration
hexagonal_role: domain
consumes:
- code
- benchmark
produces:
- profile
- benchmark
- stdout
context_rel:
- kind: shared-kernel
  with: complexity
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  external_dependencies:
  - code
  - benchmark
output_contract: 'artifacts: optimization-report.md + profile/baseline + negative-evidence ledger'
user-invocable: false
---
# measured-performance-optimization — measure, change one thing, prove it, or roll back

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.** This skill is a
disciplined optimization loop, not advice. Every iteration produces evidence.

## ⚠️ Critical Constraints

- **No measurement, no change.** You may NOT edit code for performance before a
  baseline profile + benchmark artifact exists on disk. **Why:** un-profiled
  optimization is guessing; you cannot prove improvement against nothing, and you
  will "optimize" code that is not the bottleneck.
  - WRONG: `// looks slow, let me rewrite this loop` then commit.
  - CORRECT: `Run scripts/bench.sh baseline` → read profile → target the top frame.

- **One change per iteration.** Apply exactly one optimization, then re-measure.
  **Why:** batched changes make a regression impossible to attribute, so you
  cannot roll back the one that hurt while keeping the ones that helped.

- **Correctness is non-negotiable — roll back on ANY output drift.** Before/after
  outputs must be byte-identical (or within a documented float tolerance).
  **Why:** a faster wrong answer is a defect, not an optimization.
  - WRONG: `git commit -m "2x faster"` when one golden output now differs.
  - CORRECT: outputs differ → `git checkout -- <file>` → log to the ledger → next idea.

- **Roll back on no measurable win.** If the change does not beat the baseline by
  more than the measured noise band, revert it. **Why:** complexity that buys
  nothing is pure cost (readability, future bugs) — keep only changes that pay.

- **Append every dead end to the negative-evidence ledger.** **Why:** without it
  the loop re-tries the same failed approach across sessions and burns budget on
  known-dead paths.

## Why This Exists

Agents reach for "clever" rewrites that feel fast, ship them unmeasured, and
either optimize the wrong thing or silently change behavior. This skill forces
the engineering discipline: a saved baseline, a single-variable change, a proof
surface, automatic rollback, and a memory of what already failed. It turns
"make it faster" from a vibe into a ledgered, reversible experiment loop.

## Quick Start

```bash
# from the target repo, with a runnable benchmark/repro for the hot path
bash {baseDir}/scripts/bench.sh baseline      # capture profile + timing artifact
bash {baseDir}/scripts/optguard.sh --baseline # snapshot outputs + timing to compare against
# ... apply ONE optimization ...
bash {baseDir}/scripts/optguard.sh --verify   # outputs identical? faster than noise? else FAIL → roll back
```

`optguard.sh --verify` exit code IS the gate: 0 = keep the change, non-0 = roll it back.

## The Loop (Methodology)

### Phase 0 — Define the target and the proof
1. Identify the **hot path** and a **deterministic, repeatable benchmark** for it
   (a script, `go test -bench`, `pytest-benchmark`, a `hyperfine` command).
   If none exists, write the minimal one first — without it the loop cannot run.
2. Pin the environment: same machine, same input, quiesce background load, pin the
   CPU governor if available. Record N iterations and report median + noise band.

**Checkpoint:** a one-command benchmark exists and is reproducible (run it twice,
medians within the noise band). Do not proceed otherwise.

### Phase 1 — Measure (baseline artifact)
3. `bash scripts/bench.sh baseline` → writes `profile/baseline.json` (timing,
   peak memory, allocation count when available) and a profiler output
   (pprof / perf / py-spy, etc.) into `profile/`.
4. Read the profile. Rank frames by cost. The top 1–3 frames are your only
   legitimate targets. See [profiling tools by language](references/profiling-and-tooling.md).

**Checkpoint:** you can name the single most expensive frame and its share of
total cost. If cost is flat (no hotspot), say so and stop — nothing to extremely-optimize.

### Phase 2 — Hypothesize (consult the ledger FIRST)
5. Read `optimization-report.md`'s negative-evidence ledger. **Skip any approach
   already logged as failed for this frame.**
6. Pick the highest-leverage untried lever for the top frame, in this order:
   algorithmic complexity → data structure / cache locality → allocation
   reduction → parallelism / core saturation → micro-optimization. See the
   [optimization lever catalog](references/optimization-levers.md).

### Phase 3 — Apply ONE change + guard
7. `bash scripts/optguard.sh --baseline` to snapshot current outputs + timing.
8. Apply exactly one optimization.
9. `bash scripts/optguard.sh --verify`:
   - **outputs drift** (exit 1) → revert (`git checkout`/`git stash`), log
     `ROLLED BACK (correctness)` to the ledger, return to Phase 2.
   - **no measurable win** (exit 2) → revert, log `ROLLED BACK (no win)`, return to Phase 2.
   - **faster + identical outputs** (exit 0) → keep it, record the win in the
     report, re-profile (Phase 1) — the bottleneck has moved.

**Checkpoint between every iteration:** the working tree is either a proven-faster
state or reverted to the last proven-faster state. Never a half-applied middle.

### Phase 4 — Stop and report
10. Stop when: the target metric is met, OR the remaining hotspot is flat, OR the
    next idea's risk/readability cost outweighs its win. Finalize
    `optimization-report.md` with the cumulative speedup and the full ledger.

## Output Specification

Write to the **target repo's working dir** (not this skill dir):
- `profile/baseline.json` and `profile/<profiler>.{out,txt}` — raw measurement.
- `optimization-report.md` — required sections, in order:
  - `## Target` — hot path, benchmark command, target metric.
  - `## Baseline` — median, noise band, top frames, environment.
  - `## Changes Kept` — table: `change | frame | before | after | speedup | outputs verified`.
  - `## Negative Evidence (ledger)` — table: `approach | frame | result | why it failed | date`.
  - `## Result` — cumulative speedup, final profile, what remains flat.

## Exit Codes (scripts/optguard.sh --verify)

| Code | Meaning | Action |
|------|---------|--------|
| 0 | outputs identical AND faster than noise band | keep the change |
| 1 | outputs drifted (correctness regression) | ROLL BACK, log ledger |
| 2 | no measurable win (≤ noise band) or perf regression | ROLL BACK, log ledger |
| 3 | usage / missing baseline snapshot | fix invocation, re-run |

## Quality Rubric

- A `profile/baseline.json` exists and predates the first code change (check `git log` / mtimes).
- Every entry in `## Changes Kept` has `outputs verified = yes` and a speedup above the noise band.
- The negative-evidence ledger has ≥1 entry OR an explicit note that no approach was rejected — never empty-and-absent.
- Each kept change is a single, attributable commit/diff (no batched optimizations).

## Examples

- **CPU-bound inner loop:** profile shows 70% in a nested `O(n²)` scan → replace
  with a hash index (`O(n)`) → outputs identical, 6× faster → keep, re-profile.
- **Memory-bound traversal:** profile shows cache misses dominate → switch
  array-of-structs to struct-of-arrays for the hot field → 2.1× → keep.
- **Allocation churn:** alloc profile shows per-call slice growth → preallocate
  to known capacity / reuse a buffer pool → fewer GC pauses → keep.
- **Dead end logged:** tried goroutine fan-out on a memory-bound stage → no win
  (bandwidth-bound, not CPU-bound) → revert, ledger: `parallelism | stage X | no
  win | memory-bandwidth bound`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Benchmark medians vary wildly | noisy environment | pin CPU, quiesce load, raise N, widen noise band |
| Change "feels" faster but verify says no win | within noise / wrong frame | trust the artifact; re-profile to find the real hotspot |
| Outputs differ only in float low bits | non-associative FP reordering | set + document a tolerance in optguard, or revert |
| Same idea keeps getting re-tried | ledger not consulted | read the ledger at Phase 2 before hypothesizing |
| Big-O already optimal but still slow | constant factors | target cache locality / allocations / SIMD, not complexity |

## See Also

| I need to… | Reference |
|------------|-----------|
| Pick a profiler for my language | [references/profiling-and-tooling.md](references/profiling-and-tooling.md) |
| Choose which optimization lever to pull | [references/optimization-levers.md](references/optimization-levers.md) |
| Run the loop's measurement scripts | `scripts/bench.sh`, `scripts/optguard.sh` |
| Validate this skill's own structure | `scripts/validate.sh` |
