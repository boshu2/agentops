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

One prompt, one process, one captured artifact is what makes the run auditable:
when nothing loops, every byte of output traces to exactly one invocation, and
a disagreement about what happened is settled by the artifact.

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
5. Bound the run with an explicit wall-clock timeout (wrap with `timeout`
   /`gtimeout`, or use the profile's own deadline). If no timeout mechanism is
   available, disclose that the run is unbounded rather than let it hang
   silently, and treat a killed run as a distinct terminal outcome — not a
   review result.
6. Capture the final response with `-o`, JSONL, or an output schema.
7. Report the typed run result, then stop: the process exit status, the captured
   artifact path, whether a timeout fired (and the process tree was reaped), and
   whether the `codex` binary was even present. Cancellation is the caller's;
   this skill neither retries nor continues on its own.

Terminal outcomes are explicit: **binary absent** (no `codex` on PATH) →
report unavailable and stop; **timed out** → report the kill and preserved
partial output, never as a completed review; **nonzero exit** → runtime
evidence, not a semantic verdict. The caller decides whether to launch another
invocation.

## Example

```bash
printf '%s\n' "$PROMPT" | timeout "${CODEX_TIMEOUT:-600}" \
  codex exec -C "$WORKSPACE" -s read-only -o "$OUTPUT" -
```

For a validator, the prompt must name the acceptance digest, exact subject
manifest digest, author context ID, evidence, and required checked/not-checked
report. The validator context ID must be distinct from the author's before a
`PASS` verdict is possible. When the caller elects a cross-model fresh
validator, record model identities per
the `agent-native` model-dispatch recipe and match the sandbox to
declared effects.
