---
name: agy-project-worktree-permissions
description: |-
  Prove scoped project/worktree isolation on the AGY (Antigravity) image before a
  bead can join the quorum: pin each role to a non-overlapping --add-dir scope, a
  permission tier matched to author vs judge, and the dcg guard, then capture the
  isolation evidence.
  Triggers: agy, worktree, permissions, project.
practices:
- team-topologies
- continuous-delivery
hexagonal_role: supporting
consumes:
- agy-native
produces:
- agy-isolation-evidence
context_rel:
- kind: customer-of
  with: agy-native
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
  tier: cross-vendor
  dependencies: [agy-native, cc-worktree-isolation, dcg, beads-br]
  stability: experimental
  triggers:
  - agy
  - worktree
  - permissions
  - project
  - agy --add-dir scope
  - project/worktree isolation
  - scoped permission proof
  - author != judge isolation on AGY
output_contract: An AGY isolation-proof artifact — two roles (author, judge) pinned to non-overlapping --add-dir/worktree scopes with matched permission tiers, the dcg BeforeTool guard confirmed present, and a persisted record naming each scope, permission flag, and conversation id; gates cp-c6k.3.4 before the AGY image counts toward quorum.
---

# agy-project-worktree-permissions

Prove **scoped project/worktree isolation** on the **AGY/Antigravity image** before
its ticks count toward the cross-vendor quorum. This is the `cp-c6k.3.4` proof child
of `IMAGE-AGY.md`: the AGY image must show that concurrent roles run in
*non-overlapping* directory scopes with *permission tiers matched to role*, and that
the destructive-command guard survives auto-approve — not just that a bead closed.

AGY's write-isolation primitive is **directory scope, not spawned worktrees**:
`--add-dir <dir>` (repeatable) pins which paths a run may touch, and `--sandbox`
restricts the terminal. Project isolation comes from running each role against a
distinct repo/worktree path and never granting overlapping `--add-dir` scopes.
**Invoke `agy`; never rebuild it** (operator-side; see `agy-native` Rule 7).

## Overview / When to Use

Use this when you need to *prove* — with a persisted artifact, not prose — that an
AGY run kept its author and judge isolated. It is the gate that lets an AGY tick join
the quorum family. Triggered by: an AGY bead that needs a scoped-permission proof;
"prove project/worktree isolation on Antigravity"; closing `cp-c6k.3.4`; any AGY
fan-out where two roles could otherwise clobber each other.

This is a **proof skill**, narrower than `agy-native` (which drives the whole loop).
It composes: `agy-native` runs the tick; this skill asserts and records the isolation
invariants on that tick. It is the AGY analogue of `cc-worktree-isolation`, mapped
onto AGY's `--add-dir`/`--sandbox` model instead of Claude Code's `EnterWorktree`.

Verified AGY primitives this skill leans on (from `IMAGE-AGY.md` and `agy-native`):
- **Workspace scope:** `--add-dir <dir>` (repeatable) — the only write-isolation knob.
- **Permission tiers:** `--dangerously-skip-permissions` (author, auto-approve),
  default (judge, no auto-approve), `--sandbox` (full-auto only when confined).
- **Guard:** the `dcg` BeforeTool hook on `run_shell_command` in `~/.gemini/settings.json`.
- **Project isolation:** distinct repo/worktree paths per role; a git worktree per
  author when authors run concurrently.

## ⚠️ Critical Constraints

- **Rule 1 — non-overlapping scopes are the proof, not a nicety.** The author's
  `--add-dir` set and the judge's `--add-dir` set must share no path (no parent
  containing the other). A proof with overlapping scopes is a FAIL even if the bead
  closed. **Why:** overlapping write scope is a clobber/false-close path; isolation is
  the membrane being proven.
- **Rule 2 — match permission tier to role.** Author = `--dangerously-skip-permissions`
  with a **tight** `--add-dir`; judge = **default** (no auto-approve) with a
  read-mostly scope; full-auto only inside `--sandbox`. A judge that can auto-edit is a
  false-close path. **Why:** auto-approve is a blast-radius choice (`IMAGE-AGY.md`:
  "sandboxed defaults; no break-glass permission bypass in the normal image path").
- **Rule 3 — `dcg` guard stays on under auto-approve.** Confirm the `BeforeTool` hook
  on `run_shell_command` → `dcg` is present in `~/.gemini/settings.json` *before*
  trusting any `--dangerously-skip-permissions` run. **Why:** auto-approve would
  otherwise let destructive commands through; the guard is the floor.
- **Rule 4 — per-author git worktree when authors are concurrent.** Two authors
  touching the same repo get separate `git worktree` paths, each its own `--add-dir`.
  One repo working tree = one writer at a time. **Why:** prevents swarm races on the
  index/working tree.
- **Rule 5 — author != judge across contexts.** The scope/permission split is only
  real if the judge is a *separate* AGY conversation (no `-c`/`--continue` from the
  author). Record both `conversation_id`s. **Why:** a shared context defeats the
  isolation it claims (`agy-native` Rule 2).
- **Rule 6 — no break-glass in the image path.** The proof is invalid if isolation was
  achieved only by disabling the guard, by `--dangerously-skip-permissions` without a
  tight `--add-dir`, or by granting the repo root to every role. **Why:** that is the
  exact bypass `IMAGE-AGY.md` forbids for the normal image.
- **Rule 7 — operator-side; invoke-never-rebuild.** Do NOT write under `~/dev/agentops`
  as part of running a proof, do NOT git push agentops, do NOT re-author AGY. **Why:**
  AGY is the flywheel substrate (ACFS doctrine) — own a thin adapter.

## Workflow / Methodology

### Phase 1: Verify the image + guard are live
```bash
which agy && agy --version            # CLI present (IMAGE-AGY baseline: 1.0.6)
test -f "$HOME/.gemini/settings.json" # settings present
# Confirm the dcg BeforeTool guard on run_shell_command is wired:
grep -q 'run_shell_command' "$HOME/.gemini/settings.json" && \
  grep -q 'dcg' "$HOME/.gemini/settings.json"
```
**Checkpoint:** `agy` resolves and the `dcg`/`run_shell_command` guard is present
(Rule 3) before any auto-approve run.

### Phase 2: Lay out non-overlapping project/worktree scopes
Pin one path per role. When authors run concurrently against one repo, give each a
git worktree so no two writers share a working tree (Rule 4):
```bash
AUTHOR_DIR="$REPO"                                   # or a dedicated worktree
JUDGE_DIR="$REPO/evidence"                           # read-mostly slice — MUST NOT
                                                     # be a parent/child of AUTHOR_DIR
# For concurrent authors:
git -C "$REPO" worktree add "$REPO-wt-author-a" -b run/author-a
```
**Checkpoint:** the author scope and judge scope share no path; record both. If they
overlap, the proof is a FAIL (Rule 1) — re-slice before running.

### Phase 3: Author run — tight scope, auto-approve
```bash
agy --print --add-dir "$AUTHOR_DIR" --dangerously-skip-permissions \
  "Claim one ready bead via br. Implement only it within this scope. \
   Commit scoped. Write evidence as userFacing. Do NOT close it — a judge will."
```
**Checkpoint:** a scoped commit + evidence artifact exist; the bead is OPEN; the run
touched nothing outside `AUTHOR_DIR`.

### Phase 4: Judge run — separate context, read-mostly scope, no auto-approve
```bash
agy --print --add-dir "$JUDGE_DIR" \
  "Validate bead <id> against its evidence artifact ONLY. You did not author it. \
   Emit PASS/WARN/FAIL as a userFacing verdict. Do not edit code."
```
**Checkpoint:** the judge ran in a *different* conversation (Rule 5), default
permissions (no auto-approve, Rule 2), and a scope disjoint from the author's.

### Phase 5: Assert + persist the isolation proof
Record the isolation facts as the durable artifact that gates `cp-c6k.3.4`:
```bash
cat > "$REPO/evidence/agy-isolation-<bead>.md" <<EOF
author_scope:  $AUTHOR_DIR  (--dangerously-skip-permissions)
judge_scope:   $JUDGE_DIR   (default permissions)
scopes_disjoint: true
dcg_guard:     present (run_shell_command BeforeTool)
author_conversation_id: <id>
judge_conversation_id:  <id>
verdict: <PASS|WARN|FAIL>
EOF
```
**Checkpoint:** a persisted artifact names each scope, each permission tier, the guard
state, two distinct conversation ids, and the verdict. *That* is the proof — not the
fact that a bead closed.

## Output Specification

**Format:** an AGY isolation-proof artifact (markdown / JSON) plus the underlying
scoped commit and verdict.
**Filename / path:** `<repo>/evidence/agy-isolation-<bead>.md` (and/or a
`brain/*.md` with `userFacing:true`).
**Required fields:** `{ bead_id, author_scope, author_permission, judge_scope,
judge_permission, scopes_disjoint (true), dcg_guard (present),
author_conversation_id, judge_conversation_id, verdict }`.
**Pass condition:** `scopes_disjoint == true` AND `dcg_guard == present` AND
`author_conversation_id != judge_conversation_id` AND judge ran without auto-approve.

## Quality Rubric

- [ ] Author `--add-dir` and judge `--add-dir` share no path (Rule 1) — recorded.
- [ ] Author had auto-approve + tight scope; judge ran default, read-mostly (Rule 2).
- [ ] `dcg` BeforeTool guard on `run_shell_command` confirmed present (Rule 3).
- [ ] Concurrent authors each got a distinct git worktree (Rule 4).
- [ ] Author and judge ran in distinct conversations — two ids recorded (Rule 5).
- [ ] No break-glass: guard never disabled, no untightened auto-approve, no repo-root
      grant to every role (Rule 6).
- [ ] Nothing written under `~/dev/agentops`; no agentops push (Rule 7).
- [ ] A persisted isolation artifact exists with all required fields (Output Spec).

## Examples

- **Gate `cp-c6k.3.4`:** run one bead with `AUTHOR_DIR=$REPO`, `JUDGE_DIR=$REPO/evidence`,
  confirm disjoint, confirm the `dcg` guard, persist the artifact — the AGY image now
  satisfies the project/worktree isolation child and can join the quorum.
- **Concurrent author fan-out:** two authors on one repo get `repo-wt-author-a` and
  `repo-wt-author-b` worktrees, each its own `--add-dir`; the judge reads only
  `repo/evidence`. Three disjoint scopes, three conversations.
- **Cross-vendor isolated quorum:** author with `agy --print --model "Gemini 3.1 Pro"`
  in `AUTHOR_DIR`, judge with `agy --print --model "Claude Opus"` in `JUDGE_DIR` — two
  vendors, two scopes, no shared context.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Proof rejected: scopes overlap | author and judge `--add-dir` share a parent/child path | re-slice so the judge gets a disjoint read-mostly subdir (e.g. `evidence/`) |
| Judge edited code | judge ran with `--dangerously-skip-permissions` | drop to default permissions for the judge (Rule 2) |
| Destructive command ran under auto-approve | `dcg` BeforeTool hook missing/removed | restore the `run_shell_command`→`dcg` hook in `~/.gemini/settings.json` (Rule 3) |
| Two authors clobbered the index | shared working tree | give each author a `git worktree` + its own `--add-dir` (Rule 4) |
| Judge agreed too easily | same context reused (`-c`/`--continue`) | spawn a fresh AGY conversation; record both ids (Rule 5) |
| Isolation "passed" but guard was off | break-glass bypass | invalid proof — re-run with the guard on, tight scopes (Rule 6) |

## See Also / References

- **`IMAGE-AGY.md`** (`~/dev/control-plane/`) — the AGY image spec; this skill closes
  its `cp-c6k.3.4` project/worktree/scoped-permission child.
- Sibling AGY skills: `agy-native` (drives the whole loop; permission/scope Rules
  1–7), `agy-mcp-plugins` (least-privilege MCP/tool access), `agy-rules-workflows`
  (loop law), `agy-headless-evidence` (agentapi JSONL evidence).
- Cross-image analogue: `cc-worktree-isolation` (the Claude Code `EnterWorktree`
  version of this proof), `cc-subagents`, `agent-mail` (file reservations).
- Substrate: `dcg` (destructive-command guard), `beads-br` (br tracker).
- Official Antigravity docs: cli-plugins, hooks, sidecars (URLs in `IMAGE-AGY.md`).
