# Codex Hookless Lifecycle

> **Note:** This document describes the legacy hookless fallback for Codex versions prior to v0.115.0. Starting with Codex v0.115.0 (March 2026), native hooks are supported and installed by `scripts/install-codex-plugin.sh`. Current Codex native hooks cover `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, and `PermissionRequest`, but not `SessionEnd`, so the hookless fallback and explicit closeout path still matter when transcript-driven maintenance is required.

AgentOps originally assumed a hook-capable runtime lifecycle such as Claude/OpenCode `session-start`, `session-end`, and `stop`. Codex Desktop does not expose that lifecycle surface under `~/.codex`, so the Codex runtime needs an explicit fallback that keeps the flywheel working without pretending hooks exist.

## Why This Exists

- Hook-capable runtimes can wire startup recall, transcript forging, and close-loop maintenance into runtime events.
- Codex can capture and retrieve knowledge, but those lifecycle steps do not run automatically unless AgentOps exposes an explicit command path.
- The fallback keeps the same flywheel stages while making the lifecycle honest, testable, and stateful.

## Runtime Modes

| Mode | Detection | Start path | Closeout path | Guarantees |
|------|-----------|------------|---------------|------------|
| `hook-capable` | Claude/OpenCode runtime plus installed hook surfaces | SessionStart hook or explicit `ao inject` | SessionEnd/Stop hooks or explicit `ao forge transcript` + `ao flywheel close-loop` | Startup injection and close-loop maintenance can be automatic when hooks are installed |
| `codex-hookless-fallback` | Codex env/session metadata and no hook surface under `~/.codex` | `ao codex start` or skill-driven `ao codex ensure-start` | `ao codex stop` or skill-driven `ao codex ensure-stop` | Explicit startup context, transcript discovery fallback, citation capture, session-end-equivalent maintenance, and persisted lifecycle state |
| `manual` | No hooks and no Codex-specific runtime detection | `ao inject` / `ao lookup` | `ao forge transcript` + `ao flywheel close-loop` | Portable low-level workflow with no hidden lifecycle assumptions |

## Command Responsibilities

### `ao codex start`

- Detect Codex runtime and session identity.
- Inspect repo-local `.agents/` state.
- Run safe close-loop maintenance unless `--no-maintenance` is set.
- Surface relevant learnings, patterns, findings, recent sessions, research, and next work.
- Sync `MEMORY.md` and write `.agents/ao/codex/startup-context.md`.
- Record `retrieved` citations for surfaced artifacts.
- Persist lifecycle state to `.agents/ao/codex/state.json`.

### `ao codex ensure-start`

- Run the Codex startup path once per thread.
- Skip duplicate startup automatically when the current Codex thread already has a recorded startup context.
- Give entry skills one reusable startup primitive instead of teaching each skill to parse lifecycle state directly.

### `ao codex stop`

- Resolve the best available Codex transcript.
- Prefer an archived transcript under `~/.codex/archived_sessions/`.
- Fall back to a synthesized transcript from `~/.codex/history.jsonl` when no archive exists.
- Forge/extract from the resolved transcript and queue or persist learnings safely.
- Run close-loop maintenance unless `--no-close-loop` is set.
- Run the same post-close hygiene that SessionEnd owns in hook-capable runtimes: deduplication, contradiction scan, expiry/eviction/curation maintenance, finding-compiler refresh, and auto-prune.
- Sync `MEMORY.md` and persist stop state to `.agents/ao/codex/state.json`.

### `ao codex ensure-stop`

- Run the Codex closeout path once per thread.
- Return an explicit no-op result when the same Codex thread was already closed out.
- Give closeout-owner skills one reusable closeout primitive instead of teaching each skill to parse lifecycle state directly.

### `ao codex status`

- Report the active runtime mode.
- Show capture, retrieval, promotion, and citation health.
- Surface pending and quarantined knowledge counts.
- Reflect the last explicit start/stop lifecycle events.

## Transcript Discovery Order

`ao codex stop` uses this order:

1. Explicit `--transcript`
2. Archived Codex transcript matching the current or requested session ID
3. Synthesized transcript from `~/.codex/history.jsonl`
4. Latest archived Codex transcript

This keeps closeout reliable even when Codex has not archived the latest session yet.

## Citation and Retrieval Model

- `ao lookup` remains the primary automatic citation path for curated knowledge.
- `ao codex start` records `retrieved` citations for the startup artifacts it surfaces.
- `ao search --cite retrieved|reference|applied` provides an assisted path when a search result is actually adopted.
- `ao codex status` and the flywheel metrics read the same citation ledger, so hookless mode and hook-capable mode share the same accounting.

## Promotion Hygiene

The Codex fallback reuses the existing pool and close-loop hygiene instead of inventing a Codex-only promotion path.

- Minimum structure checks reject malformed or underspecified pending artifacts.
- Truncation and low-signal artifacts can be quarantined under `.agents/knowledge/pending/.quarantine/`.
- Repo/source grounding still matters before promotion.
- Deduplication, contradiction checks, and supersession rules still run through the pool pipeline.
- Codex lifecycle commands surface the resulting health instead of silently promoting poor fragments.

## Guarantees and Limits

### Hook-capable mode guarantees

- Startup and closeout can be automatic if hooks are installed and enabled.
- Skills may rely on the runtime lifecycle for context injection and maintenance.

### Codex hookless mode guarantees

- One obvious start command: `ao codex start`
- One obvious closeout command: `ao codex stop`
- One reusable skill-safe startup guard: `ao codex ensure-start`
- One reusable skill-safe closeout guard: `ao codex ensure-stop`
- Explicit health inspection: `ao codex status`
- No dependence on hidden hook infrastructure for recall, citation, or close-loop metrics

### Non-guarantees

- Codex does not gain runtime hooks by virtue of this fallback.
- Search citation is assisted rather than inferred from every result automatically; the user or skill must choose `--cite` when adoption is known.
- Poor extracted artifacts are not promoted automatically just because they were generated in Codex.

## Verification

Use these non-release checks to verify the Codex fallback from the current worktree:

- `bash scripts/test-codex-hookless-lifecycle.sh` builds the local `ao` binary, seeds a temp Codex home and temp repos, then verifies `ao codex ensure-start`, retrieval/citation, `ao codex ensure-stop`, `ao codex status`, and a tracker-degraded no-beads `ao rpi phased` flow.
- `bash scripts/test-codex-native-install.sh --skip-lint` verifies the checked-in Codex plugin bundle and public installer flow in a temp home without cutting a tag.

## See Also

- [Knowledge Flywheel](../knowledge-flywheel.md)
- [Session Lifecycle Workflow](../workflows/session-lifecycle.md)
- [Context Lifecycle Contract](../context-lifecycle.md)
