# Plan: Close PMF Blockers

**Goal:** Remove the seven identified blockers between AgentOps and product-market fit.

**Date:** 2026-05-27

**Epic:** pmf-blockers

**Applied findings:** none (first plan against this goal)

---

## Context

AgentOps has shipped the four-layer architecture (bookkeeping, context compiler,
validation gates, knowledge flywheel) across 73 skills, a Go CLI, and 4 runtime
adapters. Traction is 328 stars. The project's own workbench A/B shows
Δ=+0.0000 — no measurable lift at v1 difficulty. The README is 416 lines, the
quickstart has 9 conditional branches, and the activation moment requires
multiple sessions. The vendor-eclipse thesis is honest but unfocused.

This plan targets the three highest-leverage gaps first (prove lift, shrink
time-to-aha, narrow ICP), then the four supporting gaps (surface area,
compounding payoff, vocabulary, distribution).

## Boundaries

- Do not change the four-layer architecture or the skills runtime contract.
- Do not delete skills — restructure discoverability around them.
- Keep the sovereignty thesis but move it below the fold.
- All changes must pass existing CI (`scripts/pre-push-gate.sh --fast`).

## Baseline Audit

| Metric | Current | Target |
|--------|---------|--------|
| Workbench A/B Δ | +0.0000 (12/12 both legs) | Δ ≥ +0.15 on v2 tasks |
| README length | 416 lines | ≤ 150 lines above the fold |
| Quickstart conditional branches | 9 | 1 (happy path) + fallback |
| Time install → first verdict | untimed (est. 15-20 min) | < 5 min measured |
| ICP messaging | "solo dev + orchestrator + quality-first" | 1 primary persona |
| Skill discoverability layers | flat list of 73 | 3 tiers: start-here / power / specialist |
| Internal vocabulary on front page | CDLC, RPI, σρ>δ, Meadows, Brownian | 0 (moved to docs/) |

---

## Issues

### PMF-1: Ship workbench v2 with realistic tasks

**Priority:** P0 — blocks all credibility claims

**Description:** The current 12-task workbench uses off-by-one bugs and simple
validators that any agent solves without AgentOps. Design and ship 8-12 v2 tasks
where AgentOps' hook layer is differentiating: multi-file refactors that need
prior-session learnings, security reviews that benefit from prevention rules,
implementation tasks where a pre-mortem would have caught a known pitfall. Run
skill-on vs skill-off A/B and publish the Δ.

**Files to modify:**
- `evals/workbench/tasks/` — new v2 task directories
- `evals/workbench/suite-workbench-behavioral-v2.json` — new suite definition
- `evals/workbench/components/` — possibly a new component with richer history
- `PRODUCT.md` — update Evidence section with v2 Δ
- `README.md` — add Δ number above the fold

**Acceptance criteria:**
- [ ] 8+ v2 tasks exist with setup/score scripts
- [ ] `make -C evals/workbench verify` passes for v2 suite
- [ ] Skill-on leg scores measurably higher than skill-off (Δ ≥ +0.15)
- [ ] Δ published in README.md above the fold
- [ ] `eval-skill-delta` CI gate covers v2 suite

**Dependencies:** none

**Test levels:** L2 (eval scoring scripts), L1 (task setup/teardown)

---

### PMF-2: Build a 90-second `ao demo` experience

**Priority:** P0 — blocks activation

**Description:** Replace the current multi-step onboarding with a single `ao demo`
command that: (a) clones/creates a small scratch repo with seeded `.agents/`
history, (b) runs a task that triggers a `/vibe` or `/council` verdict the user
wouldn't have produced themselves, (c) shows the verdict in the terminal with a
clear "this is what AgentOps added" callout. The demo must complete in under 90
seconds and require zero prior setup beyond `ao` being installed.

**Files to modify:**
- `cli/cmd/ao/demo.go` — new or rewritten demo command
- `cli/cmd/ao/demo_test.go` — tests
- `examples/demo/` — scratch repo template with seeded `.agents/` artifacts
- `README.md` — replace "See It Work" section with `ao demo` instructions

**Acceptance criteria:**
- [ ] `ao demo` runs end-to-end in < 90 seconds on a cold start
- [ ] Output includes a verdict/finding the user did not produce
- [ ] No dependencies beyond `ao` binary and `git`
- [ ] Demo works offline (no network calls after initial install)
- [ ] README "See It Work" section updated

**Dependencies:** none

**Test levels:** L2 (demo integration test), L1 (template validation)

---

### PMF-3: Narrow ICP messaging to sovereignty persona

**Priority:** P1 — blocks word-of-mouth clarity

**Description:** The README and PRODUCT.md currently pitch three personas (solo
dev, orchestrator, quality-first maintainer) equally. The sovereignty thesis
(cross-runtime corpus, local-first, operator-owned scheduling) is the only
wedge Anthropic cannot absorb. Rewrite the README to lead with the sovereignty
value prop for teams that need constrained-network discipline, audit trails, and
cross-runtime portability. Move the "what if Anthropic ships native X" defense
from PRODUCT.md into the README as confident positioning, not anxious hedging.

**Files to modify:**
- `README.md` — rewrite above-the-fold pitch, reorder sections
- `PRODUCT.md` — sharpen Target Personas (primary vs secondary)
- `docs/index.md` — align mkdocs landing page

**Acceptance criteria:**
- [ ] README leads with sovereignty value prop in first 3 sentences
- [ ] One primary persona identified (teams needing audit + portability)
- [ ] Secondary personas acknowledged but not equal-weighted
- [ ] "What stays after vendor X ships native Y" is assertive, not defensive
- [ ] README above-the-fold is ≤ 150 lines

**Dependencies:** PMF-1 (need the Δ number for the README)

**Test levels:** L0 (doc linting), manual review

---

### PMF-4: Simplify quickstart to single happy path

**Priority:** P1 — blocks time-to-aha metric

**Description:** The current `/quickstart` skill has 9 conditional branches
covering every possible state combination. Replace with a single happy-path flow
(git repo exists + ao installed → run one command → see result) and a short
fallback ("install ao first, then re-run"). Move the elaborate state detection
into `ao doctor` where it belongs.

**Files to modify:**
- `skills/quickstart/SKILL.md` — simplify to 1 happy path + 1 fallback
- `cli/cmd/ao/doctor.go` — absorb the detailed state-detection logic
- `skills/quickstart/references/getting-started.md` — simplify

**Acceptance criteria:**
- [ ] `/quickstart` has ≤ 3 conditional branches
- [ ] Happy path completes in < 60 seconds of user time
- [ ] `ao doctor` covers all former quickstart state-detection cases
- [ ] Existing quickstart eval/smoke tests still pass

**Dependencies:** PMF-2 (demo is the new "first thing to run")

**Test levels:** L2 (quickstart smoke), L1 (doctor state detection)

---

### PMF-5: Create three-tier skill discoverability

**Priority:** P1 — blocks learnability

**Description:** Replace the flat 73-skill catalog with three tiers:
- **Start Here (5-7 skills):** `/rpi`, `/council`, `/vibe`, `/research`, `/quickstart`, `/status`
- **Power (10-15 skills):** `/evolve`, `/dream`, `/plan`, `/implement`, `/crank`, `/swarm`, `/forge`, `/compile`, `/pre-mortem`, `/post-mortem`
- **Specialist (50+ skills):** everything else

Surface the tiers in `README.md`, `docs/SKILLS.md`, and the `/quickstart` output.
Update `SKILL-TIERS.md` if the classification system already supports this.

**Files to modify:**
- `skills/SKILL-TIERS.md` — add user-facing tier column
- `docs/SKILLS.md` — restructure by tier instead of flat list
- `README.md` — show only Start Here tier in Skills section
- `skills/quickstart/SKILL.md` — reference Start Here skills only

**Acceptance criteria:**
- [ ] SKILL-TIERS.md has a `user_tier` column (start-here / power / specialist)
- [ ] docs/SKILLS.md renders three sections
- [ ] README Skills table has ≤ 7 entries with "full catalog" expandable
- [ ] `scripts/sync-skill-counts.sh` still passes

**Dependencies:** none

**Test levels:** L0 (doc validation), CI parity checks

---

### PMF-6: Move internal vocabulary below the fold

**Priority:** P2 — blocks first-impression clarity

**Description:** Remove CDLC, RPI, Brownian Ratchet, σρ > δ, Meadows leverage
points, Knowledge OS lineage, and dK/dt from `README.md` and the mkdocs landing
page. Keep these in their dedicated docs (`docs/cdlc.md`,
`docs/brownian-ratchet.md`, `docs/the-science.md`, `PRODUCT.md` Lineage
section). The README should use plain English: "your repo remembers what worked"
instead of "the knowledge flywheel achieves escape velocity when σρ > δ."

**Files to modify:**
- `README.md` — replace jargon with plain language
- `docs/index.md` — same treatment
- Link to theory docs for readers who want depth

**Acceptance criteria:**
- [ ] README.md contains zero instances of: CDLC, σρ, dK/dt, Brownian, Meadows
- [ ] All removed concepts are still accessible via links to docs/
- [ ] No information is deleted, only relocated
- [ ] Doc validation scripts pass

**Dependencies:** PMF-3 (do messaging rewrite first, then vocabulary cleanup)

**Test levels:** L0 (grep for removed terms)

---

### PMF-7: Add distribution surface beyond power-user channels

**Priority:** P2 — blocks funnel growth

**Description:** The current distribution is GitHub raw URLs, brew tap, and
Claude plugin marketplace. Add: (a) a 2-minute demo GIF or asciicast in the
README, (b) VS Code marketplace listing for the skills, (c) a "try without
installing" section that shows what `/council` output looks like on a real PR.
These are top-of-funnel investments — each should take < 1 day.

**Files to modify:**
- `README.md` — add demo GIF/asciicast
- `examples/demo-output/` — checked-in example verdict output
- `.github/workflows/` or `scripts/` — asciicast recording script (optional)

**Acceptance criteria:**
- [ ] README has a visual demo (GIF, asciicast, or screenshot)
- [ ] At least one "example output" artifact checked into `examples/`
- [ ] Demo visual renders correctly on GitHub
- [ ] No broken image links in README

**Dependencies:** PMF-2 (record the demo output from `ao demo`)

**Test levels:** L0 (link validation)

---

## Execution Order (Waves)

### Wave 1 (no dependencies — can run in parallel)

| Issue | Rationale |
|-------|-----------|
| PMF-1: Workbench v2 | Unblocks credibility; generates the Δ number |
| PMF-2: `ao demo` | Unblocks activation path; generates demo output |
| PMF-5: Skill tiers | Independent restructuring of existing catalog |

### Wave 2 (depends on Wave 1)

| Issue | Blocked by | Rationale |
|-------|------------|-----------|
| PMF-3: ICP messaging | PMF-1 | Needs the Δ number for the README rewrite |
| PMF-4: Quickstart simplification | PMF-2 | Demo replaces quickstart as first action |
| PMF-7: Distribution surface | PMF-2 | Records demo output for README GIF |

### Wave 3 (depends on Wave 2)

| Issue | Blocked by | Rationale |
|-------|------------|-----------|
| PMF-6: Vocabulary cleanup | PMF-3 | Messaging rewrite sets the tone first |

## File Dependency Matrix

| Issue | File | Access | Notes |
|-------|------|--------|-------|
| PMF-1 | evals/workbench/tasks/* | write | New v2 tasks |
| PMF-1 | evals/workbench/suite-*.json | write | New suite def |
| PMF-1 | PRODUCT.md | write | Evidence section |
| PMF-1 | README.md | write | Δ number |
| PMF-2 | cli/cmd/ao/demo.go | write | Demo command |
| PMF-2 | cli/cmd/ao/demo_test.go | write | Tests |
| PMF-2 | examples/demo/ | write | Template |
| PMF-2 | README.md | write | See It Work |
| PMF-3 | README.md | write | Full rewrite |
| PMF-3 | PRODUCT.md | write | Persona sharpening |
| PMF-3 | docs/index.md | write | Landing page |
| PMF-4 | skills/quickstart/SKILL.md | write | Simplify |
| PMF-4 | cli/cmd/ao/doctor.go | write | Absorb detection |
| PMF-5 | skills/SKILL-TIERS.md | write | Tier column |
| PMF-5 | docs/SKILLS.md | write | Restructure |
| PMF-5 | README.md | read | Reference only |
| PMF-6 | README.md | write | Vocabulary |
| PMF-6 | docs/index.md | write | Vocabulary |
| PMF-7 | README.md | write | Demo visual |
| PMF-7 | examples/demo-output/ | write | Example output |

**Conflict note:** README.md is written by PMF-1, PMF-2, PMF-3, PMF-6, and PMF-7.
Wave ordering resolves this: PMF-1/PMF-2 touch different sections in Wave 1,
PMF-3 does the full rewrite in Wave 2 (incorporating PMF-1's Δ and PMF-2's
demo), PMF-6 and PMF-7 do targeted edits in Wave 3 after the rewrite lands.

---

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical enforcement | PASS | Each issue has grep-able or measurable acceptance criteria |
| PR-002: External validation | PASS | PMF-1 is literally an external validation workbench |
| PR-003: Feedback loops | PASS | PMF-1 Δ feeds PMF-3 messaging; PMF-2 demo feeds PMF-7 distribution |
| PR-004: Separation | PASS | Each issue is independently deployable |
| PR-005: Process gates | PASS | Existing CI gates preserved; new eval gate added |
| PR-006: Cross-layer consistency | PASS | README, PRODUCT, docs/SKILLS all updated in coordinated waves |
| PR-007: Phased rollout | PASS | 3-wave structure with explicit dependencies |

## Next Steps

1. `/pre-mortem` this plan to pressure-test the wave structure
2. `/crank` Wave 1 (PMF-1, PMF-2, PMF-5 in parallel)
3. Measure: time `ao demo` end-to-end, run workbench v2 A/B, count skill tiers
