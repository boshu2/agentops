---
name: codex-exec
description: 'Run one caller-supplied Codex command non-interactively and capture evidence. Triggers: "run Codex headless", "capture Codex evidence".'
skill_api_version: 1
user-invocable: false
hexagonal_role: driving-adapter
practices:
- pragmatic-programmer
consumes:
- codex-command-packet
produces:
- codex-run-output
context_rel:
- kind: supplier-to
  with: validate
context:
  window: inherit
  intent:
    mode: none
  sections:
    exclude:
    - HISTORY
  intel_scope: none
metadata:
  capabilities: [codex_exec]
  effects: [run_codex_process, sandbox_tiered_workspace_and_network_effects]
  canonical_status: canonical
  disposition: keep_optional_adapter
  tier: orchestration
  dependencies: []
  stability: stable
output_contract: process exit status and captured Codex output artifact
---
# Codex Exec — one-shot runtime adapter

Run exactly one caller-supplied Codex prompt and capture its result. This skill
does not choose work, retry failures, validate by itself, or control continuation.

## Constraints

- Route every invocation through `scripts/run.sh`; a direct `codex exec` call is
  outside this package's cleanup guarantee because only the wrapper owns a
  private process group and verifies its removal.
- Supply an existing absolute workspace, a regular prompt file, an absolute
  artifact path, and a deadline from 1 through 3600 seconds because these bind
  the effect surface and its terminal observation.
- Read-only is the default. Workspace-write requires the exact approval token
  printed by the wrapper, bound to workspace, model, and frozen prompt digest;
  danger-full-access is not exposed.
- The wrapper must attest the installed command surface, login, private process
  group, and output capability before the prompt is submitted.

One prompt, one process, one captured artifact is what makes the run auditable:
when nothing loops, every byte of output traces to exactly one invocation, and
a disagreement about what happened is settled by the artifact.
Stop after 1 process and its one terminal receipt; this is the checkable stop
condition for every invocation.

Named failure mode — **stdin hang**: a non-TTY run left waiting forever on an
open stdin nobody will write to; always pipe the prompt or close the stream.

Anti-pattern: granting workspace-write or network access "in case the prompt
needs it". Corrective: match the sandbox to the declared effects; a review
prompt runs read-only, full stop.

## Procedure

1. Confirm `codex login status` for the intended profile.
2. Set the working root explicitly with `-C`.
3. Match the sandbox to the requested effects: read-only for offline review,
   workspace-write for authorized edits, and broader access only when the caller
   explicitly requires network or external effects.
4. Pipe the prompt to stdin (or close stdin) in non-TTY execution so the process
   cannot wait indefinitely for input.
5. A wall-clock **deadline is mandatory** — never run codex unbounded. Use the
   caller's deadline, or the declared default of **600s (10 min)** when the
   caller supplies none, and record which one applied. Enforce it so the whole
   process **tree** is reaped, not just the direct child: run codex in its own
   process group and kill the group on expiry: `setsid` (own process group) +
   `kill -KILL -<pgid>` on the group; `--kill-after` only escalates TERM→KILL
   and plain `timeout <secs> codex …` signals only the direct child. If the
   package launcher cannot guarantee process-group reaping, **do not execute** — that
   host lacks the cleanup capability this skill requires (capability
   unavailable, fail closed). Deadline expiry is
   **fail-closed**: the run is killed and reported as timed-out / not proven,
   partial output preserved — never a completed review.
6. Capture the final response with `-o`, JSONL, or an output schema.
7. Report the typed run result, then stop: the process exit status, the captured
   artifact path, which deadline applied and whether it fired, that the
   process tree was reaped (a run without guaranteed reaping never starts), and whether
   the `codex` binary was present at all. Cancellation is the caller's; this
   skill neither retries nor continues on its own.

Terminal outcomes are explicit: **binary absent** (no `codex` on PATH) → report
unavailable and stop; **deadline expiry** → fail-closed, report the kill and
preserved partial output, never a completed review; **nonzero exit** → runtime
evidence, not a semantic verdict. The caller decides whether to launch another
invocation.

## Bounded surface

```bash
skills/codex-exec/scripts/run.sh \
  --workspace "$WORKSPACE" \
  --prompt "$PROMPT_FILE" \
  --output "$OUTPUT" \
  --deadline 600 \
  --sandbox read-only
```

The bundled Perl launcher calls `setsid(2)` directly, escalates TERM to KILL,
waits for the group leader, and checks that the process group no longer exists.
It preserves partial output on timeout. A local wrapper test proves those
mechanics; it does not prove Codex's service or model implementation safe.

## Quality and done

Done means the wrapper returned one terminal receipt and stopped: binary and
capabilities attested; deadline recorded; output artifact present or explicitly
partial; exit status recorded; and `process_group_reaped=true`. Missing output,
failed attestation, timeout, or surviving cleanup is a nonzero result before any
semantic-completion claim.

For a validator, the prompt must name the acceptance digest, exact subject
manifest digest, author context ID, evidence, and required checked/not-checked
report. The validator context ID must be distinct from the author's before a
`PASS` verdict is possible. When the caller elects a cross-model fresh
validator, record model identities per
the `agent-native` model-dispatch recipe and match the sandbox to
declared effects.

- Normal success requires a nonempty captured artifact and a zero child exit.
- Timeout success is impossible: expiry returns 124 only after the private
  process group is absent, with partial output preserved.
- Missing capability, approval, or cleanup proof yields a nonzero result and no
  semantic completion claim.
