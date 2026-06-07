---
name: cc-worktree-isolation
description: |-
  Use when isolating parallel Claude Code workers in separate git worktrees to prevent file collisions.
  Triggers:
practices:
- team-topologies
- continuous-delivery
hexagonal_role: supporting
consumes:
- git-worktree
produces:
- isolated-worktree-plan
context_rel:
- kind: supplier-to
  with: cc-subagents
skill_api_version: 1
user-invocable: false
context:
  window: inherit
  intent:
    mode: none
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: orchestration
  dependencies: [ntm, agent-mail, cc-hooks]
  stability: stable
output_contract: "Isolated worktrees per worker + a clean merge/cleanup plan; no overlapping edits across writers."
---

# cc-worktree-isolation

Make parallel agent writes safe. Each concurrent worker gets its own git worktree (its own working directory + branch), so two subagents editing the repo at the same time can never step on each other's uncommitted files. This is the Claude-native first move; manual `git worktree` is the fallback for non-Claude runtimes.

## Overview / When to Use

When you fan out work across parallel subagents or an NTM swarm in a single repo, the failure mode is collision: worker A writes `foo.ts`, worker B writes `foo.ts`, and one set of edits is silently lost or merges into garbage. Worktree isolation removes the shared mutable surface — every writer operates in a physically separate checkout.

Three Claude-native levers (verified against Claude Code `2.1.x`):

- **Session-level:** `claude --worktree` (`-w`) starts the whole session in a fresh isolated worktree.
- **Agent-level:** `isolation: worktree` in a `.claude/agents/*.md` frontmatter — each spawn of that teammate gets its own worktree automatically.
- **In-session:** the `EnterWorktree` / `ExitWorktree` tools create/leave a worktree mid-session (created under `.claude/worktrees/`).

Plus two tuning knobs: the `worktree.sparsePaths` setting (limit the checkout to relevant subtrees in big monorepos) and `WorktreeCreate` / `WorktreeRemove` hooks (VCS-agnostic isolation + audit/cleanup).

## ⚠️ Critical Constraints

- **One writer per file, always.** Worktrees prevent *accidental* collision; they do not let two agents legitimately co-edit one file. **Why:** isolation defers the conflict to merge time — if two branches both touch `foo.ts`, you still get a merge conflict, just later and harder. Assign file ownership upfront (the workspace rule: if two tasks touch the same file, combine into one worker).
- **EnterWorktree only when explicitly directed.** The tool fires only when the user or project instructions (CLAUDE.md / memory) say "worktree" — not on a generic "fix this bug". **Why:** silently relocating the session's CWD surprises the operator and orphans work in `.claude/worktrees/`.
- **Cannot create-nest.** You may not call `EnterWorktree` with `name` while already in a worktree session; switching into an *existing* worktree via `path` is allowed. **Why:** nested fresh worktrees produce ambiguous cleanup and base-ref confusion.
- **Know your base ref.** `worktree.baseRef` governs where new worktrees branch from: `fresh` (default) = `origin/<default-branch>`; `head` = current local HEAD. **Why:** `fresh` workers won't see your uncommitted local changes — set `head` if they must.
- **Exit-time cleanup is a decision, not a default.** `ExitWorktree action: remove` REFUSES if there are uncommitted files or unmerged commits unless `discard_changes: true`. **Why:** this is the guard against deleting real work — confirm with the operator before forcing it.
- **No `claude -p` for workers.** This is operator/flywheel infrastructure. **Why:** `claude -p` bills the API per token, not the Max subscription — banned for Claude workers. Use NTM panes or spawned subagents.

## Workflow / Methodology

### Phase 1: Decide the shape

Pick the isolation lever by how workers are spawned:

| Spawn shape | Lever |
|---|---|
| Custom teammates via `.claude/agents/*.md` | `isolation: worktree` in frontmatter (declarative — preferred) |
| A whole side session | `claude --worktree` / `-w` |
| Mid-session, one-off isolated branch | `EnterWorktree` tool |
| NTM swarm panes / non-Claude runtime | `WorktreeCreate` hook or manual `git worktree add` |

Confirm file-ownership boundaries are non-overlapping before any spawn (see Constraints).

**Checkpoint:** Every planned writer has a disjoint file set AND a chosen isolation lever. If two writers share a file, stop and merge them into one worker.

### Phase 2: Configure isolation

Declarative agent (preferred):

```yaml
# .claude/agents/implementer.md
---
description: Implements one bead end to end
model: claude-opus-4-8
isolation: worktree
background: true
---
```

Sparse checkout for large monorepos (`.claude/settings.json`):

```json
{ "worktree": { "baseRef": "fresh", "sparsePaths": ["packages/api", "packages/shared"] } }
```

Audit/cleanup hooks (`.claude/settings.json`) — `WorktreeCreate` / `WorktreeRemove` also provide VCS-agnostic isolation outside git:

```json
{ "hooks": {
  "WorktreeCreate": [{ "hooks": [{ "type": "command", "command": "echo \"$(date -u) created $CLAUDE_WORKTREE_PATH\" >> .claude/worktree-audit.log" }] }],
  "WorktreeRemove": [{ "hooks": [{ "type": "command", "command": "echo \"$(date -u) removed $CLAUDE_WORKTREE_PATH\" >> .claude/worktree-audit.log" }] }]
} }
```

**Checkpoint:** `claude agents` lists the isolated teammate; settings parse without error. Sparse paths cover every file each worker needs.

### Phase 3: Run + coordinate

Spawn workers. For mid-session isolation, `EnterWorktree { name: "bead-ag-123" }` creates `.claude/worktrees/bead-ag-123` on a new branch and switches CWD into it. Layer **agent-mail file reservations** on top when swarm members might still converge on a shared path — worktrees stop the filesystem clobber, agent-mail prevents two workers *choosing* the same file.

**Checkpoint:** Each worker reports its worktree path/branch. No two report the same branch.

### Phase 4: Merge + clean up

1. Each worker commits inside its worktree.
2. Merge branches back in a deterministic order (resolve any real conflicts — these are the legitimately-shared files you should have caught in Phase 1).
3. `ExitWorktree { action: "remove" }` per worker (or `keep` to preserve). If it refuses, inspect the listed changes, then re-invoke with `discard_changes: true` only after operator confirmation. Manual fallback: `git worktree remove <path> && git branch -d <branch>`.

**Checkpoint:** `git worktree list` shows only the canonical tree; `git branch` has no orphaned worker branches; audit log matches.

## Output Specification

**Format:** repo state + a short merge/cleanup report (markdown to stdout or the session log).
**Filename:** optional audit trail at `.claude/worktree-audit.log` (created/removed events); no other file is mandated.
**Structure:** the report names each worker's worktree path + branch, the merge order applied, conflicts resolved, and the final `git worktree list` / `git branch` state proving zero orphans.

## Quality Rubric

- [ ] Every parallel writer had a disjoint file set defined BEFORE spawn.
- [ ] Isolation lever matched the spawn shape (declarative `isolation: worktree` preferred over shell choreography).
- [ ] `worktree.baseRef` was chosen deliberately (`fresh` vs `head`), not defaulted blindly.
- [ ] Sparse paths (if a monorepo) cover all files each worker touches.
- [ ] No `EnterWorktree` create-nesting; existing-worktree entry used `path`.
- [ ] Cleanup verified: `git worktree list` + `git branch` show no orphans; no work silently discarded.
- [ ] No `claude -p` used for any worker.

## Examples

**Fan-out across three beads:** define three implementer teammates with `isolation: worktree`, assign each a disjoint package, spawn, merge in bead-dependency order, remove worktrees.

**Monorepo subtree work:** set `worktree.sparsePaths: ["services/billing"]` so each worker checks out only billing — faster, and structurally impossible to touch unrelated code.

**Mid-session hotfix isolation:** operator says "do this in a worktree" → `EnterWorktree { name: "hotfix-payments" }` → fix → commit → `ExitWorktree { action: "remove" }`.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `EnterWorktree` rejected | Already in a worktree session (with `name`) | Use `path` to switch into an existing worktree, or `ExitWorktree` first |
| Worker can't see local changes | `worktree.baseRef: fresh` branches from `origin` | Set `baseRef: head`, or push changes first |
| `ExitWorktree remove` refuses | Uncommitted/unmerged work present | Inspect listed changes; commit/merge, or confirm then `discard_changes: true` |
| Merge conflicts despite isolation | Two workers edited the same file | Phase-1 ownership failure — combine those tasks next time |
| `path` entry rejected | Path not in `git worktree list` for this repo | Only registered worktrees of the current repo are enterable |
| Isolation ignored in non-Claude runtime | Tool unavailable | Fall back to `WorktreeCreate` hook or manual `git worktree add` |

## See Also / References

- Claude Code feature contract (AgentOps shared ref: `../shared/references/claude-code-latest-features.md`) — §2 Agent Definitions, §3 Worktree Isolation, §4 Hooks (the authoritative feature set; re-verify against the upstream changelog before relying on a flag).
- `cc-hooks` skill — authoring `WorktreeCreate` / `WorktreeRemove` and other governance hooks.
- `agent-mail` skill — file reservations that complement worktrees for swarm coordination.
- `ntm` / `vibing-with-ntm` skills — spawning the parallel workers that consume this isolation.
- Upstream docs: `https://code.claude.com/docs/en/sub-agents`, `https://code.claude.com/docs/en/cli-reference`.
