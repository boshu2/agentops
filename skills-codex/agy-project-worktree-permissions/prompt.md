# Codex Execution Profile — agy-project-worktree-permissions

Read the base skill `../../skills/agy-project-worktree-permissions/SKILL.md`
first (the seven Rules, the five-phase workflow, output spec, rubric). The base
is the source of truth; this is the step-ordered Codex path. One inviolable rule:
**non-overlapping `--add-dir` scopes are the proof, not a nicety — overlapping
write scope is a FAIL even if the bead closed.** The runtime driven here is
`agy --print` (the AGY/Antigravity executor); never `claude -p` (LAW 0).

## Codex tool mapping

- **Read settings / git state** → `shell_command` (`grep`, `cat`, `git -C … worktree list`).
- **Write the isolation artifact** → `apply_patch` or shell redirection to
  `<repo>/evidence/agy-isolation-<bead>.md`.
- **Run author / judge** → `shell_command` invoking `agy --print …` (AGY is the
  worker runtime; Codex here is the orchestrator that lays out scopes, asserts the
  invariants, and persists the proof).
- **author != judge** → two SEPARATE `agy --print` invocations (distinct
  conversations = distinct contexts); never `-c`/`--continue` from author into judge.

## Steps

1. **Verify the image + guard (Phase 1).** `which agy && agy --version`; confirm
   `~/.gemini/settings.json` has the `dcg` BeforeTool hook on `run_shell_command`
   BEFORE any auto-approve run (Rule 3). Stop if the guard is missing.
2. **Lay out disjoint scopes (Phase 2).** Pin `AUTHOR_DIR` and a read-mostly
   `JUDGE_DIR` that is neither a parent nor child of it. Concurrent authors each
   get a `git worktree` + its own `--add-dir` (Rule 4). Re-slice if scopes overlap
   (Rule 1) before running anything.
3. **Author run — tight scope, auto-approve (Phase 3).**
   `agy --print --add-dir "$AUTHOR_DIR" --dangerously-skip-permissions "…implement
   one bead in scope, commit scoped, write evidence, do NOT close it…"`.
4. **Judge run — separate context, read-mostly, no auto-approve (Phase 4).**
   `agy --print --add-dir "$JUDGE_DIR" "…validate bead against evidence only, emit
   PASS/WARN/FAIL, do not edit code…"`. Default permissions (Rule 2); a fresh
   conversation (Rule 5).
5. **Assert + persist (Phase 5).** Write the isolation artifact naming both
   scopes, both permission tiers, `scopes_disjoint: true`, `dcg_guard: present`,
   the two distinct `conversation_id`s, and the verdict. *That artifact* is the
   proof — not the fact that a bead closed.

## Guardrails

- **Disjoint scopes + role-matched permissions are the membrane.** Author =
  auto-approve + tight scope; judge = default permissions + read-mostly disjoint
  scope. A judge that can auto-edit is a false-close path (Rule 2).
- **`dcg` guard stays on under auto-approve** (Rule 3) — never disable it to make
  a proof "pass." That is the exact break-glass the image forbids (Rule 6).
- **author != judge across contexts** — two `agy --print` conversations, two ids
  recorded (Rule 5).
- **Invoke-never-rebuild (Rule 7).** Do not write under `~/dev/agentops`, do not
  push agentops, do not re-author AGY — own a thin adapter only.
- **Never `claude -p` (LAW 0).** The AGY worker runtime is `agy --print`; other-vendor
  worker dispatch goes through its native executor, never `claude -p`.
- **Backstage only.** Never surface this content in client-facing material.
