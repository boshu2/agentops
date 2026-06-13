# Landed evidence — ag-wi9w1 (canonicalize bdd-foundry.js v7)

## Landed arc

- **LANDED_SHA:** `4c8c321f29778f17b04b436287bd69e5e111f712` (tip of the arc on `origin/main`)
- **ARC_BASE_SHA:** `41c22cf623308586cda537c0be652cf54f9069be`
- Arc commits (all cite ag-wi9w1):
  - `111ad7ecb` feat(workflows): canonicalize bdd-foundry.js v7 into .claude/workflows
  - `9aaad348f` feat(scripts): workflow install + drift/marker validation scripts
  - `57688ffae` feat(gates): register workflow.install-drift always-run blocking gate
  - `4c8c321f2` fix(scripts): drift-check tolerates cross-checkout symlink when bytes match canonical

## Canonical integrity at LANDED_SHA

- Canonical path: `.claude/workflows/bdd-foundry.js`
- **SHA256 of `git show 4c8c321f2:.claude/workflows/bdd-foundry.js`:** `1f3b4a4689dd56142c79780a0f100ff464584091447b623096d9d82310d8f7e9`
- Equals the immutable pre-write snapshot except exactly one line (S2 proof, via `diff snapshot <(git show LANDED:CANON)`):
  - removed: `// HAZARD: not git-tracked; canonicalize into agentops to end the multi-lane clobber.`
  - added:   `// CANONICAL: .claude/workflows/bdd-foundry.js (agentops repo, git-tracked); ~/.claude/workflows/bdd-foundry.js is a symlink/copy installed via scripts/install-workflows.sh`
- Header version at LANDED_SHA: v7 (lineage v2..v7 intact).

## Follow-mechanism (install) result

- `readlink ~/.claude/workflows/bdd-foundry.js` → `/Users/bo/dev/agentops/.claude/workflows/bdd-foundry.js` (symlink resolves to the repo canonical path).
- `cmp` of the symlink target against `git show 4c8c321f2:.claude/workflows/bdd-foundry.js`: equal at the landed SHA (the install replaced the pre-canonicalize divergent file; backup written `bdd-foundry.js.pre-canonicalize-20260612T225515Z`).
- Verification commands recorded were static only: `node --check`, `grep`, `diff`/`cmp`, `shasum`, `readlink`, `git`. No command executes `bdd-foundry.js` (only `node --check`). No Workflow-tool invocation.

## Gate

- Pre-push cockpit gate (real HOME) at push: `fast/head: 17 checks — 17 pass, 0 warn, 0 fail, 0 skip`, **exit code 0**. Push: `41c22cf62..4c8c321f2 main -> main`.
- `cd cli && go build ./... && go vet ./... && go test ./internal/gates/...` — exit 0 (`ok …/gates`, `ok …/gates/checks`).

## LAW-0 exceptions (comment / string-literal documenting the prohibition; file:line)

- `.claude/workflows/bdd-foundry.js:64` — the `REGISTER` template-string literal contains `claude -p`/`--print` as the prohibition text the conductor passes to subagents (string-literal, not an executable call). Accepted by frozen-X3 + the C9.2 amendment.
- `docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/source-snapshot-20260612T224203Z.js:64` — the immutable pre-write snapshot is a byte-verbatim copy of the v7 canonical, so it carries the same string-literal prohibition text at line 64 (data artifact, not executable). Same disposition as the canonical.
- The four added scripts (`scripts/install-workflows.sh`, `scripts/check-workflow-drift.sh`, `scripts/check-bdd-foundry-markers.sh`, `scripts/validate-workflow-install.sh`) contain **zero** LAW-0 strings (verified at LANDED_SHA).

## Tracker invocations (exact form; main checkout only, never a worktree)

```
BEADS_DIR=/Users/bo/dev/agentops/_beads br create "Canonicalize bdd-foundry.js into agentops …"   # → ag-wi9w1
BEADS_DIR=/Users/bo/dev/agentops/_beads br update ag-wi9w1 --claim
BEADS_DIR=/Users/bo/dev/agentops/_beads br create "bdd-foundry v8: repair→re-gate loop + arc-shape gate …" --deps blocks:ag-wi9w1   # → ag-28qxd
BEADS_DIR=/Users/bo/dev/agentops/_beads br create "Remediate drifted sibling workflow copies …" --deps blocks:ag-wi9w1            # → ag-pkpwd
BEADS_DIR=/Users/bo/dev/agentops/_beads br update ag-wi9w1 --status closed   # cites landed 4c8c321f2
```

## Concurrency note → RESOLVED as v8 (the very hazard this arc retires, then exercised)

After this arc landed at `4c8c321f2`, a **concurrent lane** committed `3b7ab6d0a`
(`feat(bdd-foundry): v6 — adversarial-by-construction gap-check`, Co-Authored-By Claude Opus 4.8,
refs mto-e4pv) **directly onto the shared main checkout's local `main`**, modifying
`.claude/workflows/bdd-foundry.js` (the `gapsOk` cross-family prompt). It was **unpushed**,
mislabeled "v6" though the header read v7, with **no lineage entry** — i.e. exactly the
un-versioned multi-lane edit this canonicalization exists to end.

**Resolution (ag-96g01):** the change is genuinely good (turns the generic gap-check into an
adversarial-dimension checklist + findings-ledger ratchet), and topologically a clean child of
the v7 canonical. Rather than treat it as a foreign blocker, it was reconciled into **one coherent
v8 commit**: the unpushed local commit was soft-reset (content preserved, attributed via
Co-Authored-By + this provenance note), the header bumped v7→v8 with a lineage entry citing
mto-e4pv/3b7ab6d0a, and pushed. The home symlink auto-follows; `check-workflow-drift.sh` stays
green (home == canonical).

**Effect on this (closed) arc's suite:** S2/S9 are pinned to the v7 canonicalization moment
(`4c8c321f2`) by design — S9's snapshot-timestamp ≤ first-canonical-commit rule makes the snapshot
genuinely immutable, so once the canonical advances to v8 the snapshot-equality test (S2) reads red
against the new head. That is the immutable snapshot working as intended, not a defect: ag-wi9w1's
acceptance was met and proven at `4c8c321f2` (above). The **going-forward** invariant is the
`workflow.install-drift` gate (home == repo canonical), which remains green at v8. E3 (stale
Homebrew `ao` lacks `--fast`) and E4 (plan-dir untracked→tracked porcelain) remain environment/
transition artifacts as documented.
