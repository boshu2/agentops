---
package: cli/internal/quality
status: active
owner: agentopsd
---

# cli/internal/quality

Quality / health surfaces for an AgentOps repo: doctor checks, knowledge
metrics, golden snapshots, stale-reference scanning, and Codex plugin
parity. Powers `ao doctor`, `ao metrics`, and several CI gates.

## Ownership

- Owned by the agentopsd extraction track. Per-folder ownership pattern
  ported from olympus's per-service AGENTS.md files.
- Read-only against the repo and `~/.codex/`. Writes only to the dirs
  callers pass in (golden snapshots, metric outputs).

## Public interfaces

| Symbol | Purpose |
|---|---|
| `RunDoctor(opts DoctorOptions) error` | Run all checks, render table or JSON, fail if any required check fails |
| `ComputeResult([]Check) DoctorOutput` | Aggregate checks into HEALTHY/DEGRADED/UNHEALTHY |
| `RunMetrics(...)` (`metrics_run.go`) | Compute knowledge-flywheel metrics from `.agents/` corpus |
| `WriteGolden / CompareGolden / DiffGolden` (`metrics_golden.go`) | Golden-snapshot test harness for metrics regressions |
| `ComputeHealthDelta(baseDir) float64` | Average age (days) of active learnings — flywheel staleness signal |
| `CountFilesInDir(dir) int` | Count `.md`, `.jsonl`, `.json` files (non-recursive) — used in metrics |
| `ScanStaleRefs(...)`, `DeprecatedCommands` map | Find docs referencing renamed/retired commands |
| `CodexInstallMeta`, `CodexNativePluginSkillsPath`, parity helpers | Inspect installed Codex plugin state |
| `ParseUtilityFromMarkdown / ParseUtilityFromJSONL` | Extract `utility:` front-matter for ranking |

## Non-obvious rules

- **`DeprecatedCommands` is the canonical rename map.** When a command is
  renamed (e.g. the `ao know <verb>` → `ao <verb>` flatten), add the old
  form here. Stale-ref scanners and CI gates read this map; updating only
  one of (`Cmd*.go`, this map, docs) leaves drift CI will catch.
- **Three doctor statuses, three result levels.** Check status is
  `pass` / `warn` / `fail`; aggregate result is `HEALTHY` / `DEGRADED` /
  `UNHEALTHY`. A required `fail` forces `UNHEALTHY` and `RunDoctor` returns
  an error (non-zero exit). A non-required `fail` only degrades.
- **Golden tests live with metrics, not with `_test.go`.** Snapshots are
  authored artifacts; `metrics_golden.go` is production code that reads
  them. Don't move into `testdata/` or rename to `*_test.go`.
- **Codex plugin parity is split-brain.** The repo carries `skills/`
  (canonical) and `skills-codex/` (manually maintained), and this package
  inspects what was actually **installed** under `~/.codex/plugins/`.
  Three-way drift is real; surface it, don't auto-fix.
- **Front-matter parsers are deliberately tolerant.** `ParseUtilityFrom*`
  return 0 on any failure (missing file, no front matter, malformed). Treat
  0 as "unknown", not "low utility".
- **Imports `cli/internal/types`** for shared learning/memory DTOs — keep
  that direction; do not import `quality` from `types`.

## Cross-references

- `cli/cmd/ao/doctor.go`, `metrics_*.go` — CLI wiring.
- `cli/internal/types/` — shared learning/memory types this package reads.
- `scripts/sync-skill-counts.sh`, `scripts/audit-codex-parity.sh` — sibling
  shell tools that overlap with this package's responsibilities.
- `skills/flywheel/SKILL.md` — surfaces health metrics computed here.
