# Candidate sweep — bdd-foundry.js (S10, pre-winner, pre-write)

> Executed 2026-06-12 (UTC 20260612T224135Z) BEFORE any snapshot or canonical write.
> Bead: ag-wi9w1. All commands run verbatim; outputs pasted verbatim below.

## Probe 1 — home workflows dir

```
$ ls -la ~/.claude/workflows/ | grep bdd-foundry
.rw-r--r--@ 28k bo 12 Jun 17:51 bdd-foundry.js
```

(One candidate: `/Users/bo/.claude/workflows/bdd-foundry.js`.)

## Probe 2 — repo-tracked workflows

```
$ git -C /Users/bo/dev/agentops ls-files '.claude/workflows/'
.claude/workflows/bead-crank.js
.claude/workflows/operating-loop.js
```

(bdd-foundry.js is NOT yet git-tracked — that is the hazard this arc retires. Siblings
bead-crank.js and operating-loop.js are tracked and untouched by this arc.)

## Probe 3 — every git-worktree path

```
$ git -C /Users/bo/dev/agentops worktree list --porcelain | awk '/^worktree /{print $2}' \
    | while IFS= read -r p; do [ -e "$p/.claude/workflows/bdd-foundry.js" ] && echo "HIT: $p"; done
(no output)
probed=168 worktree paths, hits=0
```

## Probe 4 — plan-dir *.js

```
$ ls docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/*.js
no matches found (no *.js in the plan dir at sweep time — the source snapshot is taken AFTER this sweep)
```

## Candidate table

| # | Path | Header (head -1) | Version | SHA256 |
|---|---|---|---|---|
| 1 | `/Users/bo/.claude/workflows/bdd-foundry.js` | `// ─── bdd-foundry v7 (2026-06-12) ───` | bdd-foundry v7 | `3bd5a4d051fb7dd4c3e7d77fd45595bf63140137509de14828e7155b82a58614` |

## WINNER

Candidate 1 — `/Users/bo/.claude/workflows/bdd-foundry.js` (bdd-foundry v7).

Rule applied: **single v7 source, no reconciliation needed** (E2 vacuous branch — exactly
one candidate exists across home dir, repo, all 168 worktree paths, and the plan dir; no
same-version divergent claimants, so no reconciliation.diff / disposition table is required).

## Tracker note (X5 support)

The arc bead is `ag-wi9w1`. All tracker writes happen from the main checkout only; the
verbatim `BEADS_DIR=/Users/bo/dev/agentops/_beads br …` invocation records (create/close)
are kept in `landed-evidence.md` (written post-land by the orchestrator — the implementing
worktree lane runs no tracker commands).
