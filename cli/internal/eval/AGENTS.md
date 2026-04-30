---
package: cli/internal/eval
status: active
owner: agentopsd
---

# cli/internal/eval

Deterministic evaluation engine for AgentOps suites. Loads YAML/JSON suite
definitions, runs cases against a chosen runtime, scores per-dimension, and
emits a `RunRecord` JSON artifact. Optional baseline compare/promote layered
on top.

## Ownership

- Owned by the agentopsd extraction track (epic `agentops-tqc`). Pattern
  ported from olympus's per-folder ownership convention (six service-level
  AGENTS.md files in `~/dev/personal/olympus`).
- This package is the **engine**; CLI command wiring lives in
  `cli/cmd/ao/eval_*.go`. Suite YAML examples live under repo `tests/` and
  the user-facing manifests in `skills/scenario/`.

## Public interfaces

| Symbol | Purpose |
|---|---|
| `RunSuite(opts RunOptions) (*RunRecord, error)` | Top-level entry: load suite, run all cases, score, write run record |
| `LoadSuite(path string) (*Suite, []byte, error)` | Read + parse a suite manifest; returns raw bytes for SHA recording |
| `Compare(candidate, baseline *RunRecord, opts CompareOptions) (*BaselineComparison, error)` | Compare a fresh run against a stored baseline; produces verdict (improvement/regression/advisory) |
| `PromoteBaseline(record *RunRecord, opts BaselineOptions) error` | Stamp a record as the new baseline (mode promote) |
| `ComputeCoverage(...)` | Coverage rollup over multiple runs |
| `Suite`, `Case`, `Expectation`, `RunRecord`, `CaseResult`, `BaselineComparison` | Public DTOs with stable JSON tags |

## Non-obvious rules

- **Tier ↔ runtime gate.** `validDeterministicRuntime` rejects runtimes that
  aren't in deterministic scope. The engine is for `Tier=deterministic`/`headless`
  paths only; live/release tiers are out of scope here.
- **Schema versions are pinned.** `Suite.SchemaVersion` and
  `RunRecord.SchemaVersion = 1`. Bumping requires coordinated changes to
  `cli/cmd/ao/eval_*.go` and any baselines on disk.
- **Visibility split is contractual.** `VisibilityPublicCanary` vs
  `VisibilityPrivateHoldout` — holdout suites should never round-trip through
  any sharable artifact path. Honor this when adding writers.
- **Verdicts vs Statuses are separate enums.** `Status` describes a single
  case (`pass`/`fail`/`error`/`skipped`/`inconclusive`); `Verdict` describes
  run-level + comparison outcome (adds `improvement`/`regression`/`advisory`).
  Don't conflate.
- **`RunOptions.Now` is injectable** for deterministic tests; production
  callers pass `nil` to use `time.Now`.
- **No JSON schema validation here.** Manifest parsing uses Go struct tags;
  schema-level checks (if any) are the caller's problem.

## Cross-references

- `cli/internal/types/quest/` — sibling package; olympus contract types.
  Eval does not depend on it but shares "schema is the contract" discipline.
- `cli/cmd/ao/eval_*.go` — CLI commands that wrap `RunSuite`/`Compare`/`PromoteBaseline`.
- `skills/scenario/` — author-side suite manifests.
- `cli/internal/ratchet/` — independent gate mechanism; not used by eval.
