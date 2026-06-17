# Recent-commits adversarial review — 181 commits, 2026-06-14→17

> 6-agent fan-out over disjoint commit slices (eval-ruler · D16 · dynamo/yield ·
> pawls/gate · provenance/mesh · doctrine/hooks+build-health), each applying
> correctness · **overclaim** · test-quality · regression lenses. Every HIGH below
> was independently re-verified at file:line by the orchestrator before landing here.
> Read-only review; fixes tracked as beads (this doc is the synthesis, not the fix).

## The convergent finding (5 of 6 lanes, independently)

**Real, well-engineered control machinery — overclaimed outcome.** Each lane's
headline commit ("enforced", "valid", "proven", "closes", "honest") overstates
because the thing is **inert on the live path**, its **evidence isn't git-auditable**,
or the **demo doesn't exercise the mechanism** it claims to prove. This is the
macro-scale form of this session's "first valid eval verdict was fragile" — and it is
the concrete answer to *"does AgentOps add process/control without improving
outcomes?"*: this sprint added a lot of control whose claims run ahead of what is
actually live/measured.

Build-health is otherwise GREEN: `go build/vet/test` clean; dead code is NOT
accumulating (the ~95 lint diagnostics are benign `unparam` test-helper noise).

## Verified HIGH findings

| # | Lane | Finding (verified) | Fix |
|---|---|---|---|
| H1 | pawls/gate | Cross-family pawl gate is wired ONLY to the PR-merge path (`reconcile-pr.sh`); `pre-push.local` has **0** pawl refs. ~94% of landings are push-to-main → **bypass the gate**. "enforced in the merge path" is false for how work lands. | bead age-58o (design call: wire into cockpit, or correct the claim) |
| H2 | D16 | `pawl-verdict.sh` jq schema-fallback `["…"]\|index(.disposition)` indexes an array with a string → exit 5 on **every valid verdict** where check-jsonschema/python absent — i.e. the minimal unattended host D16 targets. Accept-path **bricked** there; masked because this Mac has a validator. | bead age-anc (delegated fix) |
| H3 | provenance | `merge_sha` records pre-rewrite **local HEAD** (gate rewrites the commit) → **6 of 20** edges don't resolve on origin/main; one is a **branch name** stored as a SHA. Mesh join silently fails. "verdict half" has produced **0 real edges** (16/16 are the test fixture). | bead age-0tn |
| H4 | eval-ruler | The celebrated "first VALID corpus-A/B verdict" (age-9a9) rests on `scenario-ab-100-valid.json` which is **gitignored / not in the repo** — un-auditable, unreproducible. | bead age-wp1 |
| H5 | eval-ruler | The Seatbelt isolation is **never actually run** in tests — **0** `sandbox-exec` invocations; only the profile *string* is asserted. The intent-vs-enforcement gap that burned the sentinel. | bead age-wp1 |
| H6 | doctrine/build | **main is RED on the release/doc gate** (`tests/docs/validate-doc-release.sh`): 5 broken links. Blocks any tag/release. (Escaped because that gate is Full-tier, not `--fast`.) | **FIXED this review** (see below) |
| H7 | doctrine | `CLAUDE.md` Session-Constraints still mandated multi-agent "runs on NTM+AM substrate" — contradicts the just-landed single-agent-first default. | **FIXED this review** |

## MED / notable

- **dynamo** — gauge math is correct + the E-G admission gate is genuinely well-tested, but the "the loop closes / ratchet is honest" e2e's `L>0` comes from an explicit phase label; the thrown-away attempt-1 spend is mislabeled Productive — the rework-*attribution* it claims to prove never fires. C is never populated with real data. (bead age-vx0)
- **eval-ruler** — OOD verdict validity rests on an *unguessable answer key*, not on isolation forcing failure (the recorded control didn't time out; it answered without the sentinel). Generalizes only to truly-OOD keys. The gold-wiki `sanitize()` is pattern-based redaction (fail-open class) — don't credit it as a publish gate.
- **D16** — "the unattended self-hosting loop closes end-to-end" proves the organs **compose** under synthetic inputs (fake SHA, hardcoded timestamps, no live worker), not an actual unattended run; the "real producer artifact" verdict-sensor fixture is not producer-shaped (pr:0, no evidence). Honestly caveated in-file; the top-line claim overstates. (bead age-fau)
- **j39.2 (mine)** — "clears ADR-0002" is aspirational: a telemetry channel + pre-registered methodology, not realized evidence (guard ships inert; 0 data until installed + N≥30 fires). **FIXED this review** (reframed).

## Clean / solid (verified, not skimmed)
- pawl-verdict.sh fail-closed logic + 32 negative bats (blocks, not happy-path theater).
- E-G admission gate (real-writer round-tripped fixtures, exact-value assertions, no fail-open).
- recovery state-machine, assay tick, front-door admission guard (fail-closed holes genuinely closed).
- bead-id extraction (strict namespace, 9 cases); hash-chain/idempotency-given-stable-input.
- installed-skill-edit guard hook (fail-open paths reviewed clean).
- age-707/k8u/oe2 grading judges; the `thr<=0` fail-closed fix.

## Fixed in this review (landed)
- H6: the 5 broken doc-release links. The 4 CHANGELOG cross-doc links are NOT typos — they are root-relative by the byte-identical-sync invariant (gate #36: `CHANGELOG.md` == `docs/CHANGELOG.md`), so they resolve from the root copy and break one level deep in the docs/ mirror; per the existing pattern they're **allowlisted** in `tests/docs/broken-links-allowlist.txt` (rewriting them would break the root copy). The 1 public-doc→private-`.agents` seam link was **de-linked** (a genuine leak smell — removed, not suppressed). doc-release gate GREEN; changelog.sync GREEN.
- H7: `CLAUDE.md` multi-agent constraint reconciled to single-agent-first (opt-in escalation, not forced substrate).
- j39.2 overclaim reframed.

## Cross-cutting
- **168 agent worktrees** accumulated (`git worktree list`) — `git-worktree-rationalization` cleanup target.
- The recurring meta-lesson: a `feat(X): Y proven/enforced/closed` commit message is not evidence Y is live. Wire the gate to the real path, commit the evidence, make the demo exercise the mechanism — or soften the claim.
