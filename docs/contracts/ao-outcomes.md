# Spec: `ao outcomes run`

> **Status:** Draft, ready for implementation
> **Issue:** soc-yjzp.6
> **Schema:** `schemas/rubric.v1.schema.json`
> **Anthropic analog:** Managed Agents Outcomes (May 2026)

## Goal

A unified verb that closes the rubric → run → grade → retry loop locally and against any model. AgentOps already has the parts (council judges = grader, `/rpi` retry shape = retry loop, `ao goals measure` = rubric concept) — this verb wires them together.

## CLI surface

```
ao outcomes run --rubric=<path> --target=<command> [--grader=<command>] [--max-attempts=<n>] [--json] [--output=<path>]
```

Required:
- `--rubric=<path>` — path to a rubric file (JSON, validated against `schemas/rubric.v1.schema.json`)
- `--target=<command>` — shell command to run on each attempt; its stdout/stderr is what the grader sees

Optional:
- `--grader=<command>` — shell command that scores target output. Defaults to `ao outcomes grade --rubric=<path>` (built-in grader; see "Built-in grader" below). Pluggable so users can wire any LLM call.
- `--max-attempts=<n>` — overrides rubric's `retry_budget`
- `--json` — emit structured result on stdout, suppress human-readable progress
- `--output=<path>` — write structured result to file as well as stdout

## Behavior — happy path

1. Load rubric file, validate against schema. If invalid, exit 2 with first validation error.
2. Verify `criteria[].weight` sums to 1.0 ± 0.001. If not, exit 2.
3. Set `attempts_remaining := rubric.retry_budget` (or `--max-attempts`).
4. Loop:
   a. Run `--target` as a subprocess. Capture stdout, stderr, exit code.
   b. If target exits non-zero, that's the target's failure (not the grader's). Pass through the exit code immediately (do NOT retry on target failure — the grader decides what's a quality failure).
   c. Pipe target output + rubric to `--grader`. Grader emits a JSON verdict to stdout (see "Grader contract").
   d. If grader verdict `passed: true`, emit success result and exit 0.
   e. If grader verdict `passed: false`, decrement attempts_remaining. If > 0, log retry reason and loop back to (a).
   f. If `attempts_remaining == 0`, emit failure result and exit 1.

## Error & rescue map

| Failure | Behavior | Exit code |
|---|---|---|
| Rubric file not found | Print path, exit | 2 |
| Rubric malformed (schema fails) | Print first validation error, exit | 2 |
| Rubric weights don't sum to 1 ± 0.001 | Print sum + criteria list, exit | 2 |
| Target subprocess fails to spawn | Print error, exit | 2 |
| Target exits non-zero | Pass through target's exit code (no retry) | passthrough |
| Grader subprocess fails to spawn | Print error, exit | 2 |
| Grader exceeds `grader_timeout_seconds` | Mark attempt as fail, retry if budget > 0 | — |
| Grader emits malformed verdict JSON | Treat as fail with reason="grader malformed", retry if budget > 0 | — |
| Retry budget exhausted | Emit `{passed: false, ...}`, exit | 1 |
| Pass on attempt N ≤ budget | Emit `{passed: true, attempts: N, ...}`, exit | 0 |
| User SIGINT / SIGTERM | Cancel target/grader, emit partial result, exit | 130 |

## Grader contract

A grader is any shell command that:
- Reads target output from stdin (combined stdout+stderr)
- Receives `--rubric=<path>` as an arg
- Emits JSON to stdout matching:

```json
{
  "passed": true,
  "score": 0.85,
  "criteria_scores": [
    {"id": "tests-pass", "score": 1.0, "reason": "all 12 tests passed"},
    {"id": "no-regressions", "score": 1.0, "reason": "delta vs baseline is +3 pass / 0 fail"},
    {"id": "code-quality", "score": 0.6, "reason": "two new functions exceed CC 15 (warn-level)"}
  ]
}
```

`passed` is the verdict the loop reads. `score` is for logging. `criteria_scores` are surfaced to help with retry prompts.

## Built-in grader (`ao outcomes grade`)

Scope-deferred for v1. Initial release ships with `--grader` required. The default `ao outcomes grade` is filed as `soc-yjzp.6.1` (not yet created — see "Follow-ups" below) and would integrate with the existing `cli/internal/llm/` package the same way `ao forge review --reviewer-model` does.

## Result JSON

```json
{
  "rubric": "rubrics/release-readiness.json",
  "passed": true,
  "score": 0.85,
  "attempts": 2,
  "max_attempts": 3,
  "duration_seconds": 47.2,
  "history": [
    {"attempt": 1, "passed": false, "score": 0.6, "duration_seconds": 22.1, "reason": "code-quality below threshold"},
    {"attempt": 2, "passed": true, "score": 0.85, "duration_seconds": 25.1}
  ],
  "final_target_output": "...",
  "final_grader_verdict": { /* full grader JSON */ }
}
```

## Files

| File | Action | Purpose |
|---|---|---|
| `cli/cmd/ao/outcomes.go` | NEW | Cobra command tree (`outcomes run`, `outcomes grade` stub) |
| `cli/cmd/ao/outcomes_run.go` | NEW | `run` subcommand implementation |
| `cli/cmd/ao/outcomes_run_test.go` | NEW | L1 unit tests |
| `cli/internal/outcomes/rubric.go` | NEW | Rubric loader + schema validation + weight-sum check |
| `cli/internal/outcomes/runner.go` | NEW | Retry loop |
| `cli/internal/outcomes/grader.go` | NEW | Subprocess grader invocation + verdict parsing |
| `tests/integration/outcomes-run.bats` | NEW | L2 golden-rubric round-trip |
| `schemas/rubric.v1.schema.json` | EXISTS | Validated against |

**Do NOT touch:** `cli/cmd/ao/session_outcome.go` (different concept — transcript signal detection).

## Tests

### L1 (Go unit)

- `TestLoadRubric_Valid` — golden rubric loads, weights sum, criteria parse
- `TestLoadRubric_InvalidWeights` — weights sum ≠ 1 → error
- `TestLoadRubric_MissingFile` — file not found → error
- `TestLoadRubric_MalformedJSON` — bad JSON → error
- `TestRunner_PassFirstAttempt` — grader returns pass on attempt 1, exit 0
- `TestRunner_PassAfterRetry` — grader fails attempt 1, passes attempt 2, exit 0, history has 2 entries
- `TestRunner_BudgetExhausted` — grader fails N times, exit 1, history full
- `TestRunner_TargetFailsPassesThrough` — target exits 5, runner exits 5, no retry
- `TestRunner_GraderTimeout` — grader exceeds timeout, treated as fail, retried
- `TestRunner_MalformedGraderVerdict` — grader outputs garbage, treated as fail, retried

### L2 (Bats integration)

- `tests/integration/outcomes-run.bats`:
  - `golden rubric, target passes, grader passes → exit 0`
  - `golden rubric, target passes, grader fails twice then passes → exit 0, attempts=3`
  - `golden rubric, retry budget exhausted → exit 1`
  - `target exits non-zero → exit code passes through`
  - `malformed rubric → exit 2`

Stub grader: `tests/fixtures/outcomes/stub-grader.sh` reads target output, looks for a magic string, emits JSON pass/fail.

## Acceptance

- `ao outcomes run --help` shows the documented flags
- `schemas/rubric.v1.schema.json` is valid JSON Schema 2020-12
- `cd cli && go test ./cmd/ao/... -run TestOutcomes && go test ./internal/outcomes/...` passes
- `bats tests/integration/outcomes-run.bats` passes
- Retry budget bound enforced (max 10 hard cap from schema)

## Follow-ups (out of scope for v1)

- `soc-yjzp.6.1` (not yet filed) — built-in `ao outcomes grade` using `cli/internal/llm/` Ollama or council-judge integration
- Counter-stat eval against the eval workbench (Anthropic's "+10pt task success" claim — links to GOALS.md Directive #10)
- `--watch` mode that re-runs on file change (deferred until `ao watch` lands per yjzp.7)

## Reference: pre-mortem fixes (from `.agents/council/2026-05-06-pre-mortem-managed-agents-parity.md`)

This spec implements Fix 2 (rubric schema fields) and Fix 3 (error & rescue map) verbatim. See pre-mortem doc for the original fix language.
