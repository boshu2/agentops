---
name: hooks-authoring
description: Author AgentOps runtime hooks.
practices:
- design-by-contract
- code-complete
- pragmatic-programmer
hexagonal_role: domain
consumes: []
produces:
- result.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: execution
  dependencies:
  - standards
output_contract: hook script, manifest entry, tests, and validation evidence
---
# /hooks-authoring - Hook Authoring Workflow

> Purpose: help an operator who *opts in* to authoring their own runtime hooks
do so with portable behavior, clear matchers, deterministic tests, and a
kill switch.

> **AgentOps 3.0 ships zero hooks.** Workflow is guided by skills + the `ao`
CLI, and CI is the authoritative gate. This skill is for the operator who
chooses to add their own hooks to their harness — it is not part of the default
product surface. Author into your own `hooks/hooks.json` (or your harness's
equivalent); the AgentOps repo no longer carries a shipped hook manifest.

**Execute this workflow. Do not only describe it.**

## Route

Use this skill only when you are deliberately authoring or reviewing your own
runtime hooks. Validate the manifest shape against
`schemas/hooks-manifest.v1.schema.json`.

## Workflow

1. Locate (or create) your hook surface.
   - Runtime manifest: your own `hooks/hooks.json` (the AgentOps default ships none)
   - Hook scripts: your own `*.sh`
   - Schema to validate against: `schemas/hooks-manifest.v1.schema.json`
2. Select the lifecycle event and matcher.
   - For event behavior, read [references/event-taxonomy.md](references/event-taxonomy.md).
   - For matcher shape, read [references/matcher-patterns.md](references/matcher-patterns.md).
3. Define the contract before editing.
   - Inputs consumed from hook JSON.
   - Output schema and exit-code behavior.
   - Fail-open vs fail-closed decision.
   - Kill switch, timeout, and portability constraints.
4. Implement narrowly.
   - Use `set -euo pipefail` in shell hooks.
   - Resolve paths from the manifest/plugin root rather than the caller CWD.
   - Avoid `eval`, backticks, unquoted variables, and implicit globbing.
   - Keep hook output to the portable subset both Claude and Codex accept
     (avoid `hookSpecificOutput.updatedInput`, which Codex silently drops).
5. Wire the manifest.
   - Use the narrowest matcher that covers the target tool or lifecycle.
   - Add explicit timeout values.
   - Preserve existing ordering unless the behavior depends on ordering.
6. Test directly with your own fixtures.
   - For fixture patterns, read [references/test-harness.md](references/test-harness.md).
   - Run the hook with representative JSON fixtures and assert the output shape
     and exit codes yourself.
   - Run ShellCheck for touched shell files.
7. Record evidence.
   - Note touched files, fixture commands, gate output, and any intentional
     fail-open/fail-closed choices.

## Guardrails

- **Hookless-first: a hook must be a gate or a bounded adapter, never a noise-injector.** Under AgentOps 3.0 ([docs/3.0.md](../../docs/3.0.md)), the product ships **zero hooks** — workflow is guided by skills + the `ao` CLI, and CI is the authoritative gate. If you opt to author your own, a hook may *block on a real violation* (gate) or *do a bounded side-effect and stay silent unless it has a real result* (bounded adapter). It must **not** push `additionalContext`/advisory prose into the prompt window on every matching event regardless of relevance — that is the 2.x failure mode that motivated removing hooks from the product (A/B Δ=0). Context flows through explicit pulled channels (`ao lookup`, factory briefings, `/inject`), not unconditional injection.
- Do not broaden a matcher to hide a missing case; add a second hook entry.
- Do not rely on hook output fields that Codex ignores.
- Do not store session secrets, transcripts, or local runtime state in tracked
  hook fixtures.
- Keep hook authoring documentation separate from active guard behavior. For
  edit-scope enforcement, use `/scope`.

## References

- [references/event-taxonomy.md](references/event-taxonomy.md)
- [references/matcher-patterns.md](references/matcher-patterns.md)
- [references/test-harness.md](references/test-harness.md)
- [references/hooks-authoring.feature](references/hooks-authoring.feature) — Executable spec: author event-bound hook script, register in manifest, ship tests + evidence, review-mode portability check (soc-qk4b)
