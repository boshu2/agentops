# Evidence — M5: governance front-door admission guard (age-d16-self-hosting-route-nkr.6)

**Bead:** `age-d16-self-hosting-route-nkr.6` — "M5 — Governance front-door admission guard."
**Arc:** the kind-unified admission membrane — a newly-ADDED skill/workflow/loop cannot merge without front-door evidence. Depends on M3 (the acceptance membrane).

## Contract (the bead's scenarios)

1. Given a new skill/workflow/loop, When it is pushed, Then it CANNOT merge unless front-door evidence shows **bounded-context found + role assigned + acceptance run**.
2. Given enforcement, When it fires, Then it is **at the door** (`reconcile-pr.sh`), with `intake.sh` remaining the advisory early-warning.

## What existed (re-baseline, verified)

- `scripts/intake.sh` (104 lines) — an **advisory** free-text classifier (its own header: "advisory triage, NOT the pawl enforcement"). Pattern-matches a described action against a regex; does NOT check structured BC/role/acceptance evidence. Kept as the early-warning; the guard is net-new.
- The three evidence facts live in committed, greppable sources: role = `skills/<name>/SKILL.md` frontmatter `hexagonal_role:`; BC = the `docs/reference/agentops-skill-domain-map.md` row (`BC<n>`); acceptance = `skills/<name>/references/<name>.feature` or a `## Scenarios` block. Workflows carry BC+role in the `workflows:` block of `docs/contracts/skill-dispositions.yaml`.
- The live merge wall is the **pre-push Go gate** (`cli/internal/gates/checks/seed.go`); `reconcile-pr.sh` is the named door referenced by doctrine. The guard is wired into **both**.

## What M5 adds

`scripts/check-frontdoor-admission.sh` — a kind-unified, **added-only**, **fail-closed** admission guard. For every newly-added (`git --diff-filter=A` vs base, or injected via `--added`) skill / workflow / loop, it requires all three facts; any missing → BLOCKED (exit 1). Scope is added-only, so a routine edit is never blocked by a legacy unit that predates the guard.

Wired at two enforcement points:
- **Pre-push gate** (`seed.go`): a new gate `governance.frontdoor-admission` (Fast|Full, Blocking) matching `frontDoorPaths = {"skills/**", ".claude/workflows/**"}` — it fires exactly when a skill/workflow file changes.
- **The named door** (`reconcile-pr.sh`): after the pawl verdict authorizes and before the merge, the guard runs; an under-governed new unit yields `ADMISSION-HOLD` (no merge, no close), fail-closed.

Exit codes: 0 admitted / nothing-new · 1 BLOCKED · 2 usage.

## Acceptance test (fail-closed, offline)

`tests/scripts/check-frontdoor-admission.bats` — 15 cases; fixtures (a skills root, a domain-map table, a `workflows:` ledger) are built in a temp dir and the added set injected via `--added` (no git, deterministic). Covers: admit with full evidence (`.feature` or `## Scenarios`), block on each missing fact independently, workflows governed too, added-only pass, mixed-batch fail-closed, plus the refuter regressions below.

```
1..15  (all ok)
```

## Live enforcement points (real-data smoke)

- Real skill `forge` (role + BC row + real `.feature`) → **ADMIT**.
- Real workflow `ship-beads` (quoted `domain: "BC3 Loop"` + role + kind in the ledger) → **ADMIT**.
- A fabricated skill (no role / not in BC map / no acceptance) → **BLOCK**, naming all three.
- The gate fires only on `skills/**` / `.claude/workflows/**` changes; this M5 change adds neither, so it does not self-block. The pre-push gate ran **38 checks, 38 pass** (go.* fired on the `seed.go` change).

## Merge-to-main pawl (fresh-context refuter) — REFUTED ×3 → fixed

A fresh-context refuter (re-verified twice) caught **three** fail-OPEN holes my self-review missed — each one would have admitted an under-governed unit; all fixed and regression-locked:

1. **Base-degradation fail-open (the headline).** The git fallback silently degraded to `HEAD~1` when `origin/main` was unresolvable (shallow CI clone / fresh worktree / detached HEAD), so a bad unit added before `HEAD~1` was reported as "nothing to admit" (exit 0). Fixed: resolve the base via `git rev-parse --verify` up front and **FAIL-CLOSED** (exit 1, `error:unresolvable-base`) on an undetermined diff — the `HEAD~1`/two-dot rungs are removed; `--added` is the explicit bypass.
2. **Non-BC workflow domain admitted.** A workflow `domain:` of any non-empty string (`"not-a-bc"`) satisfied "BC found." Fixed: require `BC[1-6]`, symmetric with skills.
3. **Present-but-not-runnable acceptance.** A zero-byte `.feature` (or a bare `## Scenarios` header) satisfied "acceptance run." Fixed: require a non-empty `.feature` carrying a Scenario/Given, or a `## Scenarios` block with a Given/When/Then step.
4. **(Introduced by fix #2, caught on re-refute) inline-comment leak.** `wf_field` returned the whole line including a trailing `# comment`, so a comment containing a `BC2` token false-satisfied the BC check and comment text masked an empty role. Fixed: `wf_field` honors a quoted value, else strips at the first ` #`, then trims — locked by two comment-leak regressions.

Final re-verify: original fail-open reproduced-then-closed; no further fail-open / silent-defer / bypass found. Suite 15/15; `shellcheck` clean.

## Gate

- `shellcheck -x` clean on the guard + `reconcile-pr.sh`.
- `cd cli && go build/vet/test ./internal/gates/...` green (the new gate registers cleanly; no allowlist needed).
- `ao gate check --fast --scope head` — **38 checks, 38 pass, 0 warn, 0 fail**.
- Pre-existing, out-of-scope: `regen-all.sh --check` reports a `doc-release` failure unrelated to this change (release-message-freeze; touches no M5 file) — not gated by this push's scoped wall.

## Scope boundary

Non-goals held: did not re-litigate the intake schema; `intake.sh` stays advisory; no map regeneration; existing units never re-judged (added-only). Follow-up (noted, not this slice): in a CI context without `origin/main` fetched, the gate fail-closes on a skill/workflow change — the robust path is to have the Go gate pass its already-computed changed files via `--added` (the script already supports it).
