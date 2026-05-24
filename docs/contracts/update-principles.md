# Update Principles Contract

> **Status:** Draft
> **Decision:** Every commit demonstrates five operator-exemplar properties or is rejected at review.
> **Consumers:** every contributor; `scripts/check-update-principles.sh` (future).
> **Source exemplar:** commit `1b9d139c` (operator-pushed to `nightly/2026-05-12`, cherry-picked to main as `f62295f7` on 2026-05-12).

Source for these principles is observation, not abstraction. Cycles 44-51 demonstrated the inverse — text-only patches, multi-concern commits, no drift tests, narrative quality claims, dirty branch points — and produced 32 commits of red CI invisible to the operator. The exemplar commit `1b9d139c` demonstrated the inverse-inverse: a single 2-file change with 4 bats tests, sibling-pattern citation, and a measured fitness delta. The five principles below codify what made the exemplar a teaching moment.

## The five principles

Every commit MUST demonstrate ALL FIVE:

### 1. Single concern

One bug or feature per commit. If the commit message body has more than one bullet describing what changed at the code level, split the commit.

**Counter-example (cycle 48):** "Fix 3 CI failures (markdownlint, supergate, registry)." Even though the failures were related (all surfaced by the same operator audit), they should have landed as three commits. Composite commits mask which fix caused which gate result.

**Exemplar (`1b9d139c`):** "three-gap-supergate SKIPs compile-health on greenfield." One concern. One gate. One commit.

### 2. Drift-blocking test included

If the commit fixes behavior, a test in the same commit must fail when that behavior regresses. A one-shot operator probe (`AGENTS_DIR=/tmp/no-such-dir bash …`) is verification, not a drift-block. A bats test that runs on every push is.

**Counter-example (cycle 48 supergate fix):** verified locally with one probe. No regression test. Subsequent cycles couldn't tell if the SKIP path still worked.

**Exemplar (`1b9d139c`):** four bats tests covering all paths (greenfield SKIP, overnight PASS, canonical PASS, empty-tree SKIP). Any future change that breaks the SKIP semantic fires a CI failure.

### 3. Sibling-pattern citation

Every fix names the precedent it follows. New shapes need explicit rationale; copying an existing shape needs the citation to be explicit.

**Counter-example (cycle 51 Step 1.5 healing-first classifier):** added as new text in SKILL.md with no reference to similar gating in other skills.

**Exemplar (`1b9d139c`):** commit message says "matching the Gap 1 council-coverage SKIP shape." Reviewers can verify the pattern is reused, not reinvented.

### 4. Fitness delta in the commit message

Measured outcome, not narrated outcome. If the change can't be measured, it's suspect. The fitness delta is one or two numbers, paired (before → after).

**Counter-example (cycle 46 PG4 promotion):** "promotes claim to PG4 strong-verify." No measurement. Was the claim previously verified by 0 gates, 1, 5? Reader cannot tell.

**Exemplar (`1b9d139c`):** "Code-driven fitness: 134/139 → 139/139." Reader sees exactly what improved.

Acceptable fitness delta forms:
- Gate count: `N/M → M/M`
- Test count: `X passing → X+Y passing`
- Coverage: `A% → B%`
- Bead count: `K open → K-N open`
- Goals score: `S → S+1`
- Any other numerical pair the change actually moves

If the change is unmeasurable, that itself is a signal: it's either a structural change with no observable consequence (suspect) or a refactor that needs to be paired with a behavioral test (back to principle 2).

### 5. Branched from a clean point

Even a single-commit fix should not carry unrelated context. If main is red or noisy, branch from the most recent clean commit and apply the fix in isolation.

**Counter-example (cycles 46-47 PG4 promotions):** shipped onto a CI-red branch (had been red for ~30 commits). The promotion evidence files cited gates that were running red at write time.

**Exemplar (`1b9d139c`):** branched from `dcdb016b` (the commit immediately before the cycle 44 noise). The fix shipped on a clean base; readers see the change without unrelated cycle-44-51 noise.

## Enforcement plan

The five principles are voluntary until codified. Codification path:

| Principle | Enforcer | Status |
|---|---|---|
| 1. Single concern | `scripts/check-single-concern.sh` reads commit diff, fails if > N files touched without explicit `[multi-concern]` tag | TODO (BC3 epic, separate cycle) |
| 2. Drift-blocking test | `check-test-pair-on-commit.sh` checks for added `*_test.go` / `*.bats` paired with modified `*.go` / `*.sh` | TODO (BC3 epic, separate cycle) |
| 3. Sibling-pattern citation | lint commit body for `matching … pattern` / `sibling …` / `following the … shape` phrasing | TODO (BC3 epic, separate cycle) |
| 4. Fitness delta | regex `/[0-9]+\/[0-9]+ → [0-9]+\/[0-9]+/` or similar numerical-pair pattern in commit body | TODO (BC3 epic, separate cycle) |
| 5. Clean branch point | `git log --since` check on the commit's first-parent base — already partially enforced by `pre-push-gate.sh` worktree-disposition lane | partial |

Each principle's enforcer ships as its own commit (each cycle demonstrates principle 1).

## Sibling contracts and consumers

- **Pattern follows:** existing contract shape in `docs/contracts/factory-claim-ledger.md`, `docs/contracts/finding-registry.md`, etc.
- **Catalog entry:** `docs/documentation-index.md` (this contract is added there in the same commit).
- **Validates against:** `scripts/check-contracts-structural-floor.sh` — minimum-bar gate that every contract has a top-level heading, frontmatter consumers list, and body ≥ 200 chars.
- **Future strong-enforcement gates:** see the table above.

## Anti-claim

Not claiming these principles are exhaustive. The five are the minimum bar. Repo-specific principles (e.g., "every skill edit produces a hypotheses.jsonl entry," soc-z8rt.6 → BC3.3) layer on top, contract-by-contract. The five here are the floor.

Not claiming the exemplar commit is the only valid shape. A bug fix can be one file (no bats if there's nothing to bats); a refactor can omit fitness delta if it's truly behavior-preserving (but then principle 2's drift-blocking test becomes mandatory to prove behavior preservation). The principles compose; they aren't a literal template.

## Companion artifacts

- Rescope plan: `.agents/plans/2026-05-12-rescope-evolve-and-architecture.md` (operator review pending; tracked outside `.agents/` once filed under bd epics).
- Bounded-context inventory: `.agents/research/2026-05-12-bounded-contexts-and-ports.md`.
- Source post-mortem: `.agents/post-mortems/2026-05-12-evolve-session-improvement-postmortem.md`.

## Cycle log

- 2026-05-12 cycle 52: contract written as the first concrete output of rescope Wave 1.
