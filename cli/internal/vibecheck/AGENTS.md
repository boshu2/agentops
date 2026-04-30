---
package: cli/internal/vibecheck
status: active
owner: agentopsd
---

# cli/internal/vibecheck

Git-timeline analyzer that produces a "vibe check" — a 0-100 score, letter
grade, five named metrics, and a list of findings. Powers `ao vibecheck`
and the `/vibe` skill's quantitative layer.

## Ownership

- Owned by the agentopsd extraction track. Per-folder ownership convention
  ported from olympus's service-level AGENTS.md files.
- This package is **read-only against git**: it shells out to `git log`, parses,
  scores. It does not mutate the repo.

## Public interfaces

| Symbol | Purpose |
|---|---|
| `Analyze(opts AnalyzeOptions) (*VibeCheckResult, error)` | Top-level orchestrator: parse timeline → metrics → detectors → grade |
| `ParseTimeline(repoPath string, since time.Time) ([]TimelineEvent, error)` | Run `git log` and parse into events |
| `ComputeMetrics([]TimelineEvent) map[string]Metric` | Run all five metrics |
| `MetricVelocity / MetricRework / MetricTrust / MetricSpirals / MetricFlow` | Individual metric calculators |
| `ComputeOverallRating(metrics) (float64, string)` | Aggregate score + letter grade |
| `RunDetectors([]TimelineEvent) []Finding` | Run all four detectors |
| `FormatMetricsSummary(metrics, score, grade) string` | Human-readable text rendering |
| DTOs: `TimelineEvent`, `VibeCheckResult`, `Metric`, `Finding` | Stable JSON output types |

## The five metrics + four detectors

- **Metrics:** `velocity` (commit cadence), `rework` (% of churn), `trust`
  (commit-message quality / signal), `spirals` (cycle detection), `flow`
  (sustained progress). Each is computed independently in its own file.
- **Detectors:** `amnesia` (forgotten context), `drift` (scope drift),
  `logging` (logging hygiene), `tests_lie` (tests passing while behavior
  broken). One file per detector; registered in `detectors.go`.

## Non-obvious rules

- **Each metric contributes equally — exactly 20 points** to the 100-point
  score. `clampScore` in `metrics.go` re-normalizes if metric count != 5
  (defensive), but the scoring assumes 5 metrics. Adding/removing a metric
  requires re-tuning the weighting.
- **Direction matters per metric.** `rework` is "lower is better"; `velocity`,
  `trust`, `flow` are "higher is better". `spirals` falls back to 0 partial
  credit when not passing — see `metricPartialCredit` in `metrics.go`.
- **Letter grade thresholds are hardcoded:** A ≥80, B ≥60, C ≥40, D ≥20,
  F otherwise. (`scoreToGrade`)
- **Severity has only two levels:** `critical` and `warning`. Health
  classification has three: `critical`, `warning`, `healthy`. Don't introduce
  new ones without updating the renderer.
- **`AnalyzeOptions.Since` is a hard window.** Events older than `Since`
  never enter `ParseTimeline`'s output, so metrics ignore them entirely.
- **Detectors return `[]Finding` (not error).** Empty slice means "ran
  cleanly, found nothing"; nil also means clean — `Analyze` normalizes nil
  to `[]Finding{}` for stable JSON output.

## Cross-references

- `cli/cmd/ao/vibecheck_*.go` — CLI command wiring.
- `skills/vibe/SKILL.md` — calls `ao vibecheck` as one input among many.
- `cli/internal/quality/` — sibling: doctor checks + skills-codex parity. Not
  used by vibecheck; shares the "scan and report" shape.
