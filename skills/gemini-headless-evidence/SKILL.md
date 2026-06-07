---
name: gemini-headless-evidence
description: |-
  Use when running Gemini CLI headlessly and capturing structured, machine-checkable evidence.
  Triggers:
practices:
- design-by-contract
- evidence-over-assertion
hexagonal_role: supporting
consumes:
- gemini-native
produces:
- gemini-evidence-dir
context_rel:
- kind: supplier-to
  with: gemini-native
skill_api_version: 1
user-invocable: false
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  dependencies: [dcg]
output_contract: "A timestamped evidence directory containing Gemini output, exit code, command metadata, and a verdict or final response."
---

# gemini-headless-evidence

Use this when a Gemini CLI run is part of an automated loop and must leave proof
that another agent can inspect later. The core command is `gemini -p`, with an
explicit approval mode and `--output-format json` or `stream-json`.

## Critical Constraints

- **Capture the exit code immediately.** A plausible final answer with non-zero
  exit is still a failed run. **Why:** validators must key off process reality,
  not self-report.
- **Choose approval mode by role.** `plan` for read-only validation, `auto_edit`
  for scoped author edits, `yolo` only inside an externally sandboxed host.
  **Why:** approval mode is the runtime boundary.
- **One run, one directory.** Never append unrelated runs into the same evidence
  files. **Why:** a verdict must bind to exactly one event stream and command.
- **Prefer structured output for factory runs.** Use `--output-format
  stream-json` for event capture and `json` for final-response capture. **Why:**
  text output is for humans, not downstream validators.
- **Record the exact command.** Store argv, cwd, model, approval mode, and
  included directories. **Why:** a run that cannot be reproduced is weak
  evidence.

## Quick Start

```bash
run_dir=".agents/gemini-runs/$(date -u +%Y%m%dT%H%M%SZ)-validate"
mkdir -p "$run_dir"
{
  printf 'cwd=%s\n' "$PWD"
  printf 'cmd=%s\n' 'gemini -p <prompt> --approval-mode plan --output-format json'
} > "$run_dir/command.txt"

gemini -p "Validate this change read-only. Return VERDICT: PASS or FAIL." \
  --approval-mode plan \
  --output-format json \
  > "$run_dir/response.json" 2> "$run_dir/stderr.txt"
printf '%s\n' "$?" > "$run_dir/exit-code"
```

## Workflow

### Phase 1: Declare the role

Decide whether the run is an author, validator, researcher, or tie-breaker.
Pick the approval mode and sandbox posture from that role before launching.

| Role | Approval mode | Output |
|---|---|---|
| Author | `auto_edit` | `stream-json` |
| Validator | `plan` | `json` |
| Researcher | `plan` or default | `json` |
| Externally sandboxed batch worker | `yolo` only by explicit policy | `stream-json` |

### Phase 2: Build the evidence directory

Create a fresh directory with `command.txt`, `stdout`/`response`, `stderr.txt`,
and `exit-code`. Add `model.txt` if a non-default `--model` is used. Add
`scope.txt` for edits.

### Phase 3: Run Gemini

Use `gemini -p` with the chosen output format. Include `--worktree` for editing
workers or `--include-directories` for read-only extra context. Do not broaden
the workspace without recording why.

### Phase 4: Validate the evidence

Check:

```bash
test -s "$run_dir/exit-code"
test "$(cat "$run_dir/exit-code")" = 0
test -s "$run_dir/response.json" -o -s "$run_dir/events.jsonl"
```

If any check fails, the downstream verdict is FAIL or NEEDS-EVIDENCE.

## Output Specification

The evidence directory contains:

- `command.txt`
- `exit-code`
- `response.json` or `events.jsonl`
- `stderr.txt`
- optional `changed-files.txt`
- optional `verdict.md`

## Quality Rubric

- Exit code is captured and used in the verdict.
- Output is structured when consumed by another agent.
- Approval mode matches the role.
- Evidence dir is fresh and reproducible.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Empty response file | Gemini failed before producing output | Read `stderr.txt` and exit code |
| Validator made edits | Approval mode too broad | Rerun with `--approval-mode plan` |
| Event stream too verbose | Used `stream-json` for a final-only task | Use `--output-format json` |
| Cannot resume context | No session id captured | Use `--session-id` or record `--resume` target |

## See Also

- [gemini-native](../gemini-native/SKILL.md)
