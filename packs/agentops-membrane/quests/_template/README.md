# Quest template (`quests/_template/`)

The scaffold the **planner** instantiates to turn a one-line ask into a shaped,
default-FAIL quest — the "shape intent as a BDD acceptance contract" move
(operating-loop move 1) that stock Gas City structurally lacks. It ships as
**pack content**, so any city that imports `agentops-membrane` gets it for free.

## The shape

| File | Role | Who authors it |
|---|---|---|
| `CONTRACT.md` | The **ruler**. Numbered, default-FAIL Given/When/Then acceptance clauses the membrane close door judges the diff against. Read from `main` only; the builder never sees it. | **planner** (fills the clauses from the ask) |
| `test.sh` | Executable acceptance harness. One assertion per contract clause; exits **nonzero** until every clause passes. Starts red against the placeholder. | planner scaffolds → **builder** implements against it |
| `impl.sh` | Placeholder implementation. Every entrypoint returns `NOT_IMPLEMENTED` so the quest starts red. | **builder** only (planner never edits impl code — RBAC) |

## How a quest is born

```bash
# the mechanical half (deterministic; the planner invokes this):
membrane/scaffold-quest.sh <slug> [--ask "<one-line ask>"]
#   → copies _template/ → quests/<slug>/, substitutes {{QUEST}}/{{SLUG}}/{{ASK}},
#     inits a git repo with a `main` branch carrying the skeleton.
# the judgment half (the planner does this by hand):
#   → author CONTRACT.md's numbered default-FAIL clauses from the ask
#   → create the quest bead, hand off to the builder lane
```

Then the native build/close loop takes over:

```bash
gc sling agentops-membrane.builder <quest-bead-id> --on membrane-quest \
  --var quest=<slug> --var task="<build task>"
```

The builder implements `impl.sh` in its worktree until `./test.sh` is green; the
membrane close gate (`membrane/close-gate.sh`) routes the diff + `CONTRACT.md` to
≥2 cross-family reviewers and fail-closes on anything but a deterministic
CONFIRMED.

## The default-FAIL invariant

Every acceptance clause starts `[ ]` (FAILING) and every `test.sh` assertion
starts red against the placeholder `impl.sh`. This is deliberate: a contract that
is green before any work is a contract that can't fail, and a clause that can't
fail is decoration, not acceptance. Correct-by-construction: `scaffold-quest.sh`
is unit-proven (`tests/intake.bats`) to always emit ≥2 default-FAIL clauses and a
`test.sh` that exits nonzero on the placeholder.
