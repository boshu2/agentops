# cp-m8md — Goals lag reality: wire tonight's enforcement surfaces + light up the dead scenario layer

**Lane:** SilverLynx · **Branch:** `fix/cp-m8md-goals-realignment` · **Date:** 2026-06-10
**Worktree:** `/tmp/cp-m8md-wt` (off detached HEAD `e8c27a7fe`)

GOALS.md predated the entire control-plane run (Jun 7+). None of the night's enforcement surfaces were fitness gates, and the executable-spec scenario layer reads `unknown / 0%` for every directive. Two structural gaps, plus a feeder-context link, addressed below. Honesty rule held throughout: **a gate is wired only if it runs RELIABLY and goes green first.**

---

## Part A — Four enforcement surfaces wired as Gates

All four were run ONCE to prove green before wiring. Three live in control-plane (cross-repo); they resolve `$CONTROL_PLANE_ROOT` (default `/Users/bo/dev/control-plane`) and SKIP cleanly with a notice when absent. The fourth is a new agentops script reading the CP ledger read-only.

| Gate ID | Command (abridged) | Weight | First-run result |
|---|---|---|---|
| `door9-no-api-print` | `( cd $CP && bash bin/no-api-print-scan.sh )` | 6 | **PASS** — CLEAN, no forbidden API-print invocations across 224 tracked files (LAW 0 as fitness) |
| `image-conformance` | `( cd $CP && bash bin/image-conformance.sh --built-only )` | 5 | **PASS** — 28/28 (4 images × 7 checks: bundle, SSOT, model-pin, duties-role, dcg-pack-role, verify.sh) |
| `retirement-ceiling` | `( cd $CP && bash bin/ctl-retirement-count.sh --gate 37 )` | 4 | **PASS** — current 37 <= ceiling 37 |
| `gated-close-rate` | `bash scripts/check-gated-close-rate.sh --threshold 70 --window 20` | 4 | **PASS** — 16/20 = 80% (re-measured later: 17/20 = 85%) |

### Reliability fix (the honesty rule in action)

First measure run, `door9-no-api-print` SKIPped (null output) and `image-conformance` **FAILed** under `ao goals measure` even though both passed when run from the control-plane root. Root cause: both CP scripts are **cwd-sensitive** —
- `no-api-print-scan.sh` scans `git ls-files`, which needs the CP repo cwd.
- the worker images' `verify.sh` has a Rust build-farm-redirect check that only resolves from the repo root (`agy-worker verify.sh rc!=0` from any other cwd).

A gate that flips PASS/FAIL by cwd is not reliable, so per the three-gap honesty rule the gate commands were pinned with `( cd "$CP" && ... )`. After the fix all four go green deterministically through `ao goals measure`.

### Retirement ceiling — set at the REAL count, not the anticipated one

Bead prose anticipated `39` (37 + "2 new ctl scripts tonight"). Measured reality: `RETIREMENT_COUNT=37 BASELINE=37 DELTA=0` — the +2 did not materialize. The honest ceiling is the real count, **37**, which holds NOW and blocks regrowth. A 39 ceiling would leave two slots of phantom slack. Gate set at 37.

### gated-close-rate — threshold measured, set just below

New script: `scripts/check-gated-close-rate.sh`. Of the last N closes in the CP `br` ledger (read-only `br list --status closed --json`), the fraction whose `close_reason` contains `close-admission gate PASS`. Measured 2026-06-10 (durable across windows):

| Window | gated/total | rate |
|---|---|---|
| N=10 | 8/10 | 80% |
| N=15 | 12/15 | 80% |
| N=20 | 17/20 | 85% |

Threshold set at **70%** — just below the floor of the measured band. The 4 non-stamped closes (cp-06xi, cp-27gq, cp-aa5t, cp-9jqb) are a recent cluster that closed before the close-admission gate stamp was routine. The script SKIPs cleanly when `br`/`jq`/the CP root are absent (mirrors `check-corpus-freshness.sh` house pattern), so it is CI-safe on greenfield boxes.

---

## Part B — The dead scenario layer: PRE-PRODUCTION verdict (NOT faked)

**Verdict: genuinely pre-production. The producer was never wired.** Documented as a GOALS.md note so directives stop reading universally-zero; no fixture written.

Investigation:
- `ao scenario --help` exposes only `add | init | list | validate`. **No `evaluate`/`run` subcommand exists.**
- `ao goals scenarios --lint` → "No executable-spec link defects found." The directive↔scenario link graph is clean; specs under `spec/scenarios/` are authored and valid.
- The consumer side is fully built: `scenario-results.v1` schema + loader + writer + `EvaluateSatisfaction` aggregator (`cli/internal/goalsfitness/`, `cli/internal/scenarioresults/`) + the `ao goals measure` scenario table.
- **The producer is absent:** `.agents/rpi/scenario-results.json` does not exist in agentops OR control-plane. The only callers of `scenarioresults.Writer.Append` are unit tests (`scenarioresults_test.go`). Code comments + ADR-0003 describe an "RPI evaluator" / council STEP-1.8 that should produce the artifact — never shipped.
- Consequence: with no artifact, `EvaluateSatisfaction` correctly returns `VerdictUnknown` (zero evidence is never pass, never fail). Every directive renders `unknown / 0% satisfied / 0 evaluated / 80% threshold`.

**Why no fixture:** writing a `scenario-results.json` to turn the panel green would manufacture satisfaction with no evaluation behind it — a self-declared verdict, exactly what the verification membrane forbids. The honest state ("instrument not yet wired") is recorded in a new GOALS.md subsection: **"Scenario satisfaction layer — PRE-PRODUCTION (do not read as RED)"**, which states the layer is observational, not a gate, and excluded from the weighted score.

### Light-it-up plan (proposal)

Ship a **producer**, then wire it into a cadence:
1. **Producer (pick one):**
   - (a) An `ao scenario evaluate` subcommand that runs each linked scenario's Given/When/Then against a council judge and appends results via the existing `scenarioresults.Writer.Append` (the writer, schema, and merge/keep-latest logic already exist — only the judge-invocation + append wiring is owed).
   - (b) A council STEP-1.8 hook in the validation path that writes one result per merged bead.
2. **Cadence:** invoke the producer from the nightly dream-cycle job (`.github/workflows/nightly.yml`) or the control-plane controller tick, so satisfaction accumulates over sessions like the flywheel does.
3. **Then** the scenario table flips from `unknown` to real pass/fail, and the directive-level `scenario_satisfaction` thresholds (default 0.8) become meaningful gates.

Until (1) ships, the directives' real fitness signal is the **Gates** table.

---

## Part C — flywheel-compounding feeder link

`flywheel-compounding` FAILs (σρ ≈ 0.002–0.006 vs δ ≈ 17) — this is the cp-aa5t "0%-routed corpus influence" finding expressed as math: the corpus has content but little cross-session citation. Added a feeder-context note to the gate's description (context, not a threshold change): **cp-s82z** (corpus backfill) raises σ (citation volume); **cp-0gyc** (operator library) raises ρ (citation concentration / influence). The gate stays RED by design until those feeders land — long-cycle/corpus-state, not single-push-fixable. Do not threshold-game; close the feeders.

---

## VERIFY — before / after

- `ao goals validate --json` → `{ "valid": true, "errors": 0 }` (structure OK; pre-existing WARNs for unwired `check-*` scripts are warnings, not errors).
- `ao goals scenarios --lint` → clean (no link defects).

| Measure | BEFORE | AFTER |
|---|---|---|
| gates passing / total | 24 / 28 | **28 / 32** |
| gates failing | 4 | 4 (unchanged) |
| weighted pass / total | 125 / 142 | **144 / 161** |

**Four new gates added, all PASS. Zero regressions** — the 4 failures are identical before and after: `flywheel-compounding` (long-cycle, now annotated), `go-complexity-ceiling`, `compile-freshness` + `compile-no-oscillation` (both `runtime-artifact` gates needing a local `ao defrag` run). None are touched by this work.

## Artifacts
- New: `scripts/check-gated-close-rate.sh`
- Edited: `GOALS.md` (4 gate rows; flywheel-compounding feeder note; scenario-layer pre-production subsection)
