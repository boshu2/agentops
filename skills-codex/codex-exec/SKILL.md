---
name: codex-exec
description: Use when running Codex workers or validators
---
# codex-exec

Drive headless Codex worker and validator agents with `codex exec` on the ChatGPT Pro subscription (OAuth) — the Codex side of the flywheel. The one inviolable rule: **subscription billing, never per-token API billing.**

## ⚠️ Critical Constraints

- **Never API-bill a worker.** Do NOT set `OPENAI_API_KEY` in a worker's env, and do NOT use `codex login --with-api-key`. **Why:** that flips Codex from flat-rate sub billing to per-token API billing — the Codex twin of the banned `claude -p`. A factory cycle on API keys silently burns real money. (Mirror of the "never `claude -p` for workers" rule.)
  - WRONG: `OPENAI_API_KEY=sk-... codex exec -C "$REPO" "<task>"` · CORRECT: `codex login status  # Logged in using ChatGPT` then `codex exec -C "$REPO" -s workspace-write "<task>"`
- **Confirm the sub before dispatch.** Run `codex login status` and require `Logged in using ChatGPT`. **Why:** a worker that "runs fine" on a leaked API token bills per token; the check is the only thing standing between a green run and a surprise invoice.
- **Pipe the prompt (or close stdin) in any non-TTY lane — else codex HANGS.** A positional-arg `codex exec "<prompt>"` run with non-TTY stdin (background, `&`, NTM pane, cron, piped, inherited-pipe) still reads stdin and can block forever without EOF. For unattended/factory lanes, pipe the prompt to `codex exec … -` or close stdin with `</dev/null`.
- **Pick the sandbox deliberately.** `-s read-only` for offline validators, `-s workspace-write` for workers that must edit, `-s danger-full-access` only inside an already-sandboxed host. **Why:** `codex exec` runs model-generated shell commands; the sandbox is the blast radius.
- **Network-touching validators are the exception — use `-s danger-full-access`.** A validator that must `git fetch`, clone a repo, or hit any network endpoint will FALSE-FAIL under `-s read-only`, because the sandbox blocks `connect` syscalls — the failure is an infrastructure artifact, not a real verdict. On an already-sandboxed host, give network validators `-s danger-full-access`. Offline validators stay `-s read-only`.
- **`--dangerously-bypass-approvals-and-sandbox` is for externally-sandboxed hosts only.** **Why:** it removes every guardrail in one flag; use it only when the OS/container is the sandbox.
- **Don't strand work in `--ephemeral`.** It skips session persistence, so there is nothing to `resume`. **Why:** a crashed ephemeral run cannot be recovered or continued.
- **Multi-account lanes go through `caam`, not env-var juggling.** **Why:** `caam exec codex <profile> --` keeps each Pro lane isolated; hand-setting `CODEX_HOME` invites cross-account token bleed.

## Why This Exists

`codex exec` runs Codex non-interactively: it takes a prompt (argument or stdin), executes against a working directory under a sandbox policy, and prints the final agent message. It is the Codex analogue of an NTM Claude pane — the right tool for factory/loop workers and validators that need a second vendor lane.

Auth is the whole game. `codex login status` must read **`Logged in using ChatGPT`** (the Pro/Plus subscription via OAuth). That billing is flat-rate. The moment Codex is authed with an API key (`OPENAI_API_KEY`, `codex login --with-api-key`), every token is metered against the API account — the exact failure mode `claude -p` causes on the Claude side. This skill exists to keep Codex workers on the sub. Without it, a loop that dispatches Codex turns can quietly run on metered API billing and produce a surprise invoice.

Verified against `codex-cli 0.139.0` (`codex exec --help`, `codex exec resume --help`). **Re-verify the flag surface on the next codex-cli bump** — the flag tables (Phase 2 key flags, the Phase 3 resume currency note) are version-pinned; a CLI upgrade can add/rename/move flags, so re-run the two `--help` commands and reconcile before trusting this skill against a newer codex.

## Folded triggers (ag-s43tg wave 1): `codex-goals` + `codex-mcp-plugins` + `codex-sandbox-evidence` route here

- **`codex-goals` → this skill.** Use when asked to define an objective once and let Codex iterate
  until done (Codex Goals): express the objective as the `codex exec` prompt with an explicit
  done-condition ("Run tests. Report PASS/FAIL."), then drive iterations through
  `codex exec resume --last` (Phase 3) until the verdict lands — same sub-billing and sandbox
  rules as any worker lane.
- **`codex-mcp-plugins` → this skill.** Use when wiring MCP servers or plugins into Codex CLI
  or the AgentOps Codex skill bundle: MCP/plugin config lives in `$CODEX_HOME/config.toml`
  (layer per-lane variants via `-p/--profile <name>.config.toml`); verify the wired server is
  visible to a worker with a cheap `codex exec -s read-only` probe before dispatching real work.
- **`codex-sandbox-evidence` → this skill.** Use when running `codex exec` in a
  least-privilege sandbox with machine-checkable proof: pick the narrowest `-s` policy
  (Phase 2), then capture the proof surface — `--json` JSONL event stream, `-o` final-message
  file, and `--output-schema` for a schema-constrained verdict — so the sandbox posture and
  the result are both auditable artifacts.

## Quick Start

```bash
codex login status                      # MUST read: Logged in using ChatGPT
test -z "${OPENAI_API_KEY:-}" || echo "ABORT: API key set"   # MUST be empty
codex exec -C "$REPO" -s read-only "Validate the change. Output VERDICT: PASS|FAIL."
```

## Workflow / Methodology

### Phase 1: Verify the lane is on the subscription
```bash
codex login status          # MUST print: Logged in using ChatGPT
echo "${OPENAI_API_KEY:+API_KEY_SET}"   # MUST print nothing
```
**Checkpoint:** Status reads `Logged in using ChatGPT` AND no `OPENAI_API_KEY` is set. If either fails, STOP — fix auth before dispatching. If API-key auth is present, run `codex login` (browser OAuth) or `--device-auth` to re-auth on the sub.

### Phase 2: Dispatch the worker / validator
```bash
# Worker: edit in a repo
codex exec -C /path/to/repo -s workspace-write \
  "Implement bead ag-123: <task>. Run tests. Report PASS/FAIL."

# Validator (offline / no network): read-only, structured verdict
codex exec -C /path/to/repo -s read-only \
  -o /tmp/verdict.txt \
  "Independently validate the change on this branch. Output VERDICT: PASS|FAIL + reasons."

# Validator that must reach the network (git fetch, clone, API): danger-full-access
# on an already-sandboxed host — -s read-only would FALSE-FAIL on blocked connect syscalls.
codex exec -C /path/to/repo -s danger-full-access \
  -o /tmp/verdict.txt \
  "Fetch origin/main, validate the change against it. Output VERDICT: PASS|FAIL + reasons."

# Stdin prompt (orchestrator piping the task in) — the SAFE DEFAULT for any
# unattended/background/NTM-pane/cron lane. The trailing `-` reads the prompt
# from the pipe and gives codex an immediate EOF, so it can't stall on
# "Reading additional input from stdin..." (see Critical Constraints).
printf '%s' "$TASK_PROMPT" | codex exec -C "$REPO" -s workspace-write -

# If you must pass the prompt positionally in a non-TTY lane, close stdin:
codex exec -C "$REPO" -s workspace-write "<task>" </dev/null

# Named profile (a specific model/config lane)
codex exec -p worker-fast -C "$REPO" -s workspace-write "<task>"

# Machine-readable event stream for a loop to parse
codex exec --json -C "$REPO" -s read-only "<task>" > events.jsonl
```
Key flags: `-C/--cd <DIR>` working root · `-s/--sandbox <read-only|workspace-write|danger-full-access>` · `-p/--profile <name>` layers `$CODEX_HOME/<name>.config.toml` · `-m/--model` · `-o/--output-last-message FILE` · `--json` JSONL events · `--output-schema FILE` constrains final-response shape · `--add-dir` extra writable dir · `--skip-git-repo-check` for non-repo dirs · `--ephemeral` no persistence.
**Checkpoint:** Worker exited 0; the final message (stdout / `-o` file / last JSONL `item`) carries the expected verdict or artifact.

### Phase 3: Resume to continue a session
```bash
cd "$REPO" && codex exec resume --last "Address the validator's findings, then re-run tests."
codex exec resume <SESSION_ID> "<follow-up>"
codex exec resume --last --json "<follow-up>" > events2.jsonl
```
`resume` takes a UUID session id or thread name; `--last` picks the newest in the cwd; `--all` disables cwd filtering. **Currency note (codex-cli 0.139.0):** `resume` accepts `-m/--model`, `-o/--output-last-message`, `--json`, `--output-schema`, `--ephemeral`, `-i/--image`, `--skip-git-repo-check` — but **NOT** `-C/--cd` and **NOT** `-s/--sandbox`. The resumed session inherits its original working root and sandbox policy, so `cd` into the repo first (the resume picks the newest session in the cwd) rather than passing `-C`/`-s`. `resume` cannot change the sandbox via `-s`, but `--dangerously-bypass-approvals-and-sandbox` (and `--dangerously-bypass-hook-trust`) IS available on resume — the resume-side lever for an externally-sandboxed host that needs a network-touching follow-up.
**Checkpoint:** The resumed session id matches the intended thread and the follow-up landed in the same working tree.

## Output Specification

- **Artifact directory:** for an automated handoff, capture output under
  `$REPO/.agents/evidence/codex-exec/<run-id>/`; an interactive caller may keep
  the default final message on stdout.
- **Filename convention:** use `final-message.txt` with
  `-o/--output-last-message`, `events.jsonl` with `--json`, or
  `final-message.json` when `--output-schema` constrains the last message.
- **Serialization/schema format:** `final-message.txt` is a nonempty regular file;
  `events.jsonl` is one JSON event object per line and must include a nonempty
  completed `agent_message`; `final-message.json` must satisfy the exact JSON
  Schema supplied to `--output-schema`.
- **Validator command:** after capturing the process status as `$codex_rc` and
  the selected artifact as `$output_path`, validate the transport before
  interpreting the agent's result:

  ```bash
  set -euo pipefail
  test "$codex_rc" -eq 0
  test -n "$REPO"
  test -f "$output_path"
  test ! -L "$output_path"
  test -s "$output_path"
  artifact_root="$REPO/.agents/evidence/codex-exec/"
  relative_path="${output_path#"$artifact_root"}"
  test "$relative_path" != "$output_path"
  run_id="${relative_path%%/*}"
  artifact_name="${relative_path#*/}"
  test -n "$run_id"
  test "$artifact_name" != "$relative_path"
  case "$run_id" in .|..) exit 1 ;; esac
  case "$artifact_name" in */*) exit 1 ;; esac
  physical_repo="$(cd "$REPO" && pwd -P)"
  physical_root="$(cd "$artifact_root" && pwd -P)"
  test "$physical_root" = "$physical_repo/.agents/evidence/codex-exec"
  physical_dir="$(cd "$(dirname "$output_path")" && pwd -P)"
  test "${physical_dir#"$physical_root"/}" != "$physical_dir"
  case "$artifact_name" in
    final-message.txt) test -s "$output_path" ;;
    events.jsonl)
      jq -se 'length > 0 and all(.[]; type == "object") and
              any(.[]; .type == "item.completed" and
                       .item.type == "agent_message" and
                       ((.item.text | type) == "string") and
                       (.item.text | length) > 0)' "$output_path" ;;
    final-message.json)
      test -n "${output_schema:-}"
      command -v check-jsonschema >/dev/null
      check-jsonschema --schemafile "$output_schema" "$output_path" ;;
    *) exit 1 ;;
  esac
  ```
- **Downstream handoff:** only after this transport validator succeeds may the
  owning worker loop or independent judge parse the result-specific contract;
  a nonzero Codex exit or invalid artifact is infrastructure failure, not a
  PASS/FAIL verdict.

## Exit Codes

A loop should branch on the process exit code, not on scraped text.

| Code | Meaning | Loop action |
|------|---------|-------------|
| `0` | Run completed; final message emitted | Parse the result (`-o FILE` / last `--json` item); proceed |
| non-zero | Codex error — auth failure, sandbox denial, bad flag, or model/tool error | Do NOT treat as a verdict; re-check `codex login status`, sandbox mode, and flags, then retry or escalate |

## Quality Rubric

- [ ] `codex login status` showed `Logged in using ChatGPT` BEFORE dispatch
- [ ] No `OPENAI_API_KEY` in the worker env and no `--with-api-key` anywhere
- [ ] Sandbox mode matches role: offline validator → `-s read-only`; worker that edits → `-s workspace-write`; **network-touching validator** (`git fetch`/clone/API) → `-s danger-full-access` (read-only false-FAILs on blocked `connect`)
- [ ] Working root set explicitly with `-C`, not assumed from cwd
- [ ] Result captured deterministically (`-o FILE`, `--json`, or `--output-schema`) — not scraped from terminal noise
- [ ] Long/multi-turn work uses `resume` (not `--ephemeral`) so it can be recovered
- [ ] Multi-account lanes dispatched via `caam exec codex <profile> --`

## Validator and runtime rules

Before dispatching a judge or changing the Codex receipt path, load
[validator and runtime rules](references/validator-and-runtime-rules.md).

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Tokens billing to API account | API-key auth or `OPENAI_API_KEY` set | `unset OPENAI_API_KEY`; `codex login` (OAuth) or `--device-auth`; re-check `codex login status` |
| `Logged in with API key` in status | Wrong auth path | Re-auth on the sub via browser OAuth / `--device-auth`; never `--with-api-key` for workers |
| `not a git repository` error | `-C` points outside a repo | Add `--skip-git-repo-check` (or point `-C` at a repo) |
| Worker can't write files | `-s read-only` | Use `-s workspace-write`; add `--add-dir` for paths outside the root |
| `resume` finds nothing | Prior run used `--ephemeral`, or wrong cwd | Don't use `--ephemeral` for resumable work; try `resume --all` to drop cwd filtering |
| Hangs on `Reading additional input from stdin...` | Non-TTY stdin (background/NTM pane/cron) with no EOF | Pipe the prompt (`printf '%s' "$P" \| codex exec … -`) or add `</dev/null` |
| Empty / truncated output in a loop | Parsing terminal text | Use `-o FILE` or `--json` and read the structured result |
| Rate-limited on the Pro lane | One account saturated | Switch lanes with `caam exec codex <other-profile> --` |

## See Also / References

- `ntm` — Claude worker panes (the Claude-side lane; never `claude -p`)
- `agent-native` + `ntm` — driving Codex as an interactive NTM pane vs this skill's headless `codex exec`. Their dispatch mechanics differ even though auth rules overlap.
- `account-rotation` — host-routed account switching; on Codex/Gemini and Linux/WSL lanes the swap tool is `caam` (isolated multi-account profiles for the 4-lane flywheel: Claude Max ×2 + Codex Pro + Gemini)
- `dcg` — destructive-command guard that can enforce the never-API-bill rule · Memory "Never claude -p for workers 2026-06-06" — the Claude-side twin of this skill's core rule
