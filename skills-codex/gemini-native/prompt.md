# Codex Execution Profile -- gemini-native

Codex runtime wrapper for `gemini-native`.

## Steps

1. Read `../../skills/gemini-native/SKILL.md` and identify the exact Gemini image task path that applies.
2. Confirm local Gemini CLI syntax with `gemini --help` and the relevant subcommand help before changing configuration or dispatching a run.
3. Use Codex-native tools for repo edits and validation; use Gemini CLI only when the task is explicitly about operating the Gemini image.
4. Capture machine-checkable evidence: command, exit code, affected paths, and validation output.
5. Keep source changes in `skills/gemini-native` and Codex wrapper changes in `skills-codex/gemini-native`; do not edit runtime-installed copies as source of truth.

## Guardrails

- Do not broaden scope beyond the requested Gemini image operation.
- Do not invent Gemini flags. Verify with local help output.
- Do not store secrets in tracked files or runtime config snippets.
- For Gemini worker dispatch, prefer least-privilege approval and structured output as described by the source skill.
