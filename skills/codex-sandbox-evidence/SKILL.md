---
name: codex-sandbox-evidence
user-invocable: false
description: |
  Run codex exec under a least-privilege sandbox and emit a machine-checkable JSONL proof surface for the validator.

  Triggers: "codex exec", "codex sandbox", "least-privilege codex", "codex evidence", "codex JSONL proof", "--output-schema", "codex worker dispatch", "read-only codex", "workspace-write", "events.jsonl", "verifiable codex run", "factory/flywheel Codex worker", "harden a codex invocation".

  **Use when:**
  - Dispatching a Codex worker in the factory/flywheel loop that must produce verifiable evidence, not just a final answer
  - A validator (human or agent) needs to confirm WHAT a Codex run did — its event stream, tool calls, and structured last message
  - Hardening a Codex invocation to read-only / workspace-write so it cannot escape its lane

  **Perfect for:**
  - ACFS / Mount Olympus worker dispatch where every run must leave a proof artifact on disk
  - dcg-aligned least-privilege execution: default-deny, widen the sandbox only as the task requires

  **Not ideal for:**
  - Interactive Codex sessions (use the TUI); this is the non-interactive `codex exec` lane
  - Claude work — use NTM panes or subagents; `claude -p` is banned (bills API, not the sub)
practices:
- design-by-contract
- data-contracts
hexagonal_role: supporting
consumes:
- codex-exec
produces:
- codex-evidence-jsonl
context_rel:
- kind: customer-of
  with: codex-exec
- kind: supplier-to
  with: validate
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
  dependencies: [dcg]
  stability: stable
output_contract: A run directory containing events.jsonl (event stream), last-message.txt or last-message.json (final agent message), and the captured exit code — the validator's proof surface.
---

# codex-sandbox-evidence

Run `codex exec` with the **tightest sandbox the task allows** and capture a **JSONL proof surface** the validator can read back — the event stream, an optional schema-constrained final message, and the exit code. Least privilege in, evidence out.

## Overview / When to Use

The factory dispatches Codex workers non-interactively. A worker that only prints prose to a terminal leaves nothing a validator can verify after the session ends (per the cross-agent rule: you read a worker's *published compression*, never its live session). This skill makes every Codex run do two things at once:

1. **Run least-privilege.** Codex's `-s/--sandbox` policy defaults to and starts from `read-only`; you widen to `workspace-write` only when the task must edit files, and to `danger-full-access` essentially never. This is the dcg posture applied to the worker binary itself.
2. **Leave a proof surface.** `--json` streams structured events to stdout as JSONL; `-o/--output-last-message` writes the final message to a file; `--output-schema` forces that final message into a known shape. Together they are the artifact the validator inspects.

Use it whenever a Codex run is part of an automated loop and someone (or something) downstream has to trust the result.

## ⚠️ Critical Constraints

- **Rule 1: Start at `read-only`, escalate deliberately.** `-s read-only` is the default and the floor. Only pass `-s workspace-write` when the task edits files in the working root; reserve `-s danger-full-access` for externally-sandboxed environments. **Why:** a worker that can write or reach the network beyond its task is the exact blast-radius dcg exists to contain — least privilege is the boundary, not a suggestion.
- **Rule 2: Never `--dangerously-bypass-approvals-and-sandbox` (or `--dangerously-bypass-hook-trust`) inside the loop.** **Why:** these disable the sandbox AND the safety hooks; using them defeats the entire point of an evidence-producing, contained worker and was the kind of move that has burned this fleet before.
- **Rule 3: Capture the exit code, not just stdout.** Record `$?` immediately after `codex exec` returns. **Why:** a non-zero exit with a plausible-looking last message is the silent-failure case; the validator must key off the real exit status, because self-reported success is not evidence (evidence over comfort).
- **Rule 4: One run, one timestamped directory.** Write `events.jsonl`, the last-message file, and an `exit-code` to a fresh dir per run. **Why:** runs overwrite each other otherwise, and the validator needs to bind a verdict to exactly one event stream.
- **Rule 5: Treat the JSONL as the source of truth over the pretty stream.** **Why:** the human-readable output is for eyeballing; the validator parses `events.jsonl` — what isn't in the JSONL didn't happen as far as the proof surface is concerned.
- **Rule 6: This is the Codex lane only.** Don't reach for `claude -p` to "do the same for Claude" — it bills per-token against the API, not the Max sub, and is banned for worker dispatch. **Why:** cost discipline; Claude workers go through NTM panes or spawned subagents on OAuth.

## Workflow / Methodology

### Phase 1: Choose the sandbox policy

Pick the floor that lets the task succeed:

| Task shape | Policy | Flag |
|---|---|---|
| Read / analyze / report only | read-only (default) | `-s read-only` |
| Edit files in the working root | workspace-write | `-s workspace-write` |
| Externally sandboxed host only | full access | `-s danger-full-access` |

Add `--add-dir <DIR>` only for the specific extra writable paths a workspace-write task needs. Add `--skip-git-repo-check` if running outside a git repo. Keep `.rules` execpolicy active — do **not** pass `--ignore-rules`.

**Checkpoint:** confirm you are NOT using a `--dangerously-bypass-*` flag and the policy is the minimum the task needs before running.

### Phase 2: Set up the run directory and proof flags

```bash
RUN_DIR="$(pwd)/.codex-evidence/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RUN_DIR"
```

Wire the three evidence flags:
- `--json` → JSONL event stream on stdout (redirect to `events.jsonl`)
- `-o "$RUN_DIR/last-message.txt"` → the final agent message to a file
- `--output-schema "$RUN_DIR/schema.json"` (optional) → constrain the final message to a known JSON shape, so the validator can parse fields instead of prose

**Checkpoint:** the run dir exists and you know which of the three flags this run uses (always `--json`; add `-o`; add `--output-schema` when the validator needs structured fields).

### Phase 3: Run least-privilege, capture everything

Minimal read-only proof run:

```bash
codex exec -s read-only --json -C "$WORKDIR" \
  -o "$RUN_DIR/last-message.txt" \
  "Audit X and report findings" \
  > "$RUN_DIR/events.jsonl" 2> "$RUN_DIR/stderr.log"
echo "$?" > "$RUN_DIR/exit-code"
```

Schema-constrained, file-editing run:

```bash
codex exec -s workspace-write --json -C "$WORKDIR" \
  --output-schema "$RUN_DIR/schema.json" \
  -o "$RUN_DIR/last-message.json" \
  "Apply the fix described in BEAD-123 and summarize as {status, files_changed[]}" \
  > "$RUN_DIR/events.jsonl" 2> "$RUN_DIR/stderr.log"
echo "$?" > "$RUN_DIR/exit-code"
```

**Checkpoint:** `exit-code` is written and `events.jsonl` is non-empty before declaring the run done.

### Phase 4: Hand the proof surface to the validator

The validator reads `$RUN_DIR` — it never reads the live session. It checks the exit code, parses `events.jsonl`, and (if present) parses the schema-shaped last message. Reference the dir in the bead / Agent Mail compression so downstream agents consume the published artifact.

**Checkpoint:** the run dir path is recorded in the work artifact (bead, mail, commit note) so the evidence is discoverable.

## Output Specification

**Format:** a per-run directory of plain files (JSONL + text/JSON + exit code).
**Filename / path:** `<workdir>/.codex-evidence/<UTC-timestamp>/`
**Structure:**
- `events.jsonl` — the `--json` event stream (REQUIRED proof surface)
- `last-message.txt` or `last-message.json` — final agent message via `-o` (REQUIRED)
- `exit-code` — captured `$?` (REQUIRED)
- `schema.json` — the `--output-schema` file, when the final message is structured (OPTIONAL)
- `stderr.log` — captured stderr (recommended)

## Quality Rubric

- [ ] Sandbox policy is the minimum the task needs (`read-only` unless edits/full-access are justified)
- [ ] No `--dangerously-bypass-approvals-and-sandbox` / `--dangerously-bypass-hook-trust` / `--ignore-rules`
- [ ] `--json` stream redirected to `events.jsonl` and the file is non-empty
- [ ] Final message captured via `-o` to the run dir
- [ ] Exit code captured to `exit-code` immediately after the run
- [ ] `--output-schema` used when the validator needs structured fields (not free prose)
- [ ] Run dir is fresh + timestamped (no overwrite) and referenced in the work artifact
- [ ] Not used for Claude work (`claude -p` stays banned)

## Examples

**Read-only audit, prose result:** `codex exec -s read-only --json -o run/last.txt "Review dcg config" > run/events.jsonl; echo $? > run/exit-code`

**Resume a prior session and keep capturing evidence:** `codex exec resume --last --json -o run2/last.txt "Continue and finalize" > run2/events.jsonl; echo $? > run2/exit-code`

**Stdin prompt (piped), workspace-write, schema-shaped:** `echo "$PROMPT" | codex exec -s workspace-write --json --output-schema schema.json -o run/last.json - > run/events.jsonl`

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `events.jsonl` empty but pretty output appeared | `--json` not set, or stdout not redirected | add `--json` and redirect stdout to the file |
| Last-message file empty, content only in stream | relied on `--json` alone | also pass `-o <file>`; `--json` and `-o` are complementary |
| Run errors with "not inside a git repo" | Codex's git-repo guard | add `--skip-git-repo-check` |
| Task fails to write a needed file | sandbox too tight | escalate to `-s workspace-write` (+ `--add-dir` for paths outside the root) — minimally |
| Validator can't parse the result | final message is free prose | constrain it with `--output-schema <FILE>` |
| Run "succeeded" but downstream is wrong | exit code ignored | always `echo $? > exit-code`; key the verdict off it |

## See Also / References

- **Execute** `scripts/validate.sh` — self-check: confirms `codex` is on PATH, the frontmatter + section spine are intact, and the Form-A line budget holds. Exit 0 = skill is sound.
- `codex-exec` skill — the general non-interactive `codex exec` lane this skill hardens (this skill `consumes` it).
- `dcg` skill — destructive-command guard; the least-privilege posture this skill applies to Codex.
- `agentops:validate` — produces the PASS/WARN/FAIL verdict over this proof surface.
- `caam` — run a specific Codex account lane (`caam exec codex <profile> --`).
- Cross-agent rule: consume a worker's published compression (artifact/mail/bead), never its live session.
- `codex exec --help`, `codex sandbox --help` — authoritative flag reference (verified current 2026-06-06).
