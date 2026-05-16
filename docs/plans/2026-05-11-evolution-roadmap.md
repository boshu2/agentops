---
id: plan-2026-05-11-evolution-roadmap
type: plan
date: 2026-05-11
goal: Enumerate the full evolution work for /evolve to drain against the aspirational layer
detail_level: comprehensive
research_refs:
  - PRODUCT.md
  - GOALS.md
  - PRACTICE-REGISTRY.md
  - docs/contracts/ (38 contracts)
  - docs/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md  # durable export of .agents/learnings/2026-05-11-... per soc-w6vh.5
---

# Evolution Road Map — 2026-05-11

## Purpose

The 13-cycle /evolve session on 2026-05-11 shipped the practice-citation epic (756/756) and surfaced a structural friction: /evolve's Step 3 ladder picks from `bd ready` or `next-work.jsonl` but neither captures the **largest source of work in this repo** — the gap between aspirational docs (`PRODUCT.md`, `GOALS.md`, `PRACTICE-REGISTRY.md`, `docs/contracts/`, `docs/code-map/`, `docs/plans/`) and what the code actually does.

This document enumerates that gap as concrete beads organized into evolution epics. /evolve drains the resulting `bd ready` queue instead of guessing.

The thesis from PRODUCT.md's "Desired State vs Current State" section:

> `PRODUCT.md` and `GOALS.md` are allowed to outpace the current repo. That is the point of goals: they define the desired state, not a frozen claim that every mechanism is already complete. ... `GOALS.md` is the setpoint, the repo is actual state, `ao goals measure` is the sensor, and `/evolve`, dream, validation gates, and follow-up issues are the reconcile loop.

This road map IS the reconcile queue, made explicit.

## Structure

Five evolution epics, three audit epics that generate inventory for follow-on beads, and one learning-capture epic that compounds the loop's self-improvement across days.

| Epic | Scope | Concrete beads |
|------|-------|----------------|
| **E1: Directive Closure** | Close the remaining gaps in GOALS.md's 11 directives | 11 |
| **E2: Roadmap Gate Promotion** | Move "Roadmap (declared, not yet enforced)" gates → CI-blocking | 5 |
| **E3: Known Product Gap Closure** | Close the 11 gaps in PRODUCT.md "Known Product Gaps" | 11 |
| **E4: Four-Layer Polish** | For each product layer (bookkeeping, compiler, gates, flywheel), close the highest-impact missing capability | 4 |
| **E5: Three-Gap Contract Surface** | Each of the three context-lifecycle gaps gets a measurable proof path | 3 |
| **A1: AOP-CLAIM Audit** | Inventory all 83 claim markers, separate verified vs unverified, generate beads for unverified | 1 audit + N follow-ups |
| **A2: Contract Enforcement Audit** | For each of 38 contracts in `docs/contracts/`, verify it has a `scripts/check-*.sh` enforcement gate | 1 audit + N follow-ups |
| **A3: Code-Map Drift Audit** | For each `docs/code-map/*.md`, diff claimed structure against actual file layout | 1 audit + N follow-ups |
| **LC: Learning Capture Loop** | Each /evolve cycle micro-captures 1 line; every 5th productive cycle reflects on patterns; each hard stop consolidates to a dated learning file; recurring frictions auto-file `evolve-improvement` beads under this label | 1 epic + 1 leaf + N auto-filed |

## E1: Directive Closure (11 beads)

Each directive in `GOALS.md` carries a "Progress" line. Where progress is incomplete, file a bead. Names use D1–D11 mapping to directive number.

### D1: Multi-runtime live execution proof
Tier S structural is green. Tier E live execution isn't a default gate. Build a CI lane that runs one canonical smoke against a real Claude Code, Codex, Cursor, OpenCode runtime — or document why this can't be a default gate (auth/cost) and freeze the tier system as the contract.
**Acceptance:** Either `tests/skills/test-runtime-*-live.sh` exist and run in CI, OR `docs/contracts/multi-runtime-tier-charter.md` documents why Tier E is opt-in with no further work expected.

### D2: End-to-end install execution in sandboxed CI
`tests/install/test-install-smoke.sh` validates syntax/structure. Real install execution against a clean env is documented as out-of-scope. Build a sandboxed CI job (Docker or GH Actions matrix) that runs `bash <(curl ...) install.sh` against a fresh container and verifies the resulting state.
**Acceptance:** `.github/workflows/install-e2e.yml` exists, runs install against ubuntu+macos containers, validates `ao --version` works post-install.

### D3: Quarantine empty enforcement
`tests/_quarantine/` currently has zero suites. Add a `goals-validate` gate that fails if `find tests/_quarantine -name '*.sh' -o -name '*.bats' | wc -l` > 0.
**Acceptance:** New gate in `GOALS.md`, weight 4, blocks on push if quarantine populated without explicit override.

### D4: Flywheel-lifecycle citation hard-fail
Stage 5 citation is currently soft-fail on sparse corpus. After corpus passes a populated threshold, flip to hard-fail.
**Acceptance:** `scripts/check-flywheel-lifecycle.sh` adds a `--strict` flag that hard-fails when corpus has ≥ 100 learnings AND citation density < threshold.

### D5: Complexity regression ratchet
CC ceiling green. Add a pre-commit hook variant that fails on any new function exceeding CC 18 (not just 20).
**Acceptance:** `hooks/go-complexity-precommit.sh` flips `--threshold` from 20 → 18 for new code paths.

### D6: Competitive freshness sweep
`scripts/check-competitive-freshness.sh` exists; comparison docs may be drifting. Audit `docs/comparisons/*.md`, refresh any > 45 days old, and ensure the gate is failing for drift.
**Acceptance:** All `docs/comparisons/vs-*.md` have `last_reviewed` within 45 days, gate is currently green.

### D7: Codex parity drift = 0
Gate exists. Audit current findings count and resolve to zero.
**Acceptance:** `bash scripts/check-codex-parity-drift.sh` returns 0 findings.

### D8: Dream end-user dogfood
`.agents/schedule.yaml.example` exists per `dream-end-user-coverage` gate. Verify the example is actually usable: a fresh `ao init --with-schedule` produces a runnable schedule.
**Acceptance:** `tests/install/test-dream-dogfood.sh` runs `ao init --with-schedule` in a temp dir and verifies the resulting `.agents/schedule.yaml` parses + has real-bodied job types.

### D9: Pattern-to-skill synthesis (v2)
Detection layer ships in v1. Synthesis (LLM-authored draft skill bodies, tier heuristics, on-disk drafts) is deferred. Build v2: `ao flywheel close-loop --synthesize` writes review-only `.agents/skill-drafts/*.md` files with full bodies.
**Acceptance:** A pattern with 3+ session evidence produces a draft skill body that passes `skill-frontmatter` gate and would-pass `skill-lint`.

### D10: Behavioral eval as default blocking gate
Workbench exists, A/B exists, scoring infrastructure verified. The skill-on vs skill-off delta isn't a default gate.
**Acceptance:** `eval-workbench-verify` gate in GOALS.md upgraded to also fail when `make -C evals/workbench head-to-head` produces a regression delta.

### D11: Corpus durability (snapshot/restore)
Tracked under `soc-rv5p`. Routine cleanup wipes most of `.agents/`. Build snapshot to durable storage + restore tooling + freshness gate.
**Acceptance:** `ao corpus snapshot` writes to a configurable path; `ao corpus restore` rehydrates from latest snapshot; `corpus-freshness` gate fires if snapshot is > 7 days old.

## E2: Roadmap Gate Promotion (5 beads)

GOALS.md's three-gap contract proof surface explicitly lists gates that are "declared, not yet enforced". Each is a bead to move it left.

### G1: flywheel-compounding → CI-blocking
**Tag:** long-cycle, corpus-state. Gate requires multi-session evidence. Design a corpus-state evidence model: a snapshot file in `.agents/proof/` that the CI can validate without running multi-session work.
**Acceptance:** `flywheel-compounding` weight stays 3, but moves to "Currently enforcing" column with a defined evidence-snapshot protocol.

### G2: flywheel-proof → CI-blocking
**Tag:** cross-session evidence. `scripts/proof-run.sh` exists but isn't invoked from automation that blocks merges. Wire into `.github/workflows/validate.yml` or pre-push.
**Acceptance:** `flywheel-proof` runs in CI for every push to main, blocks merge on failure.

### G3: compile-freshness → CI-blocking
**Tag:** runtime-artifact. Gate depends on `.agents/defrag/latest.json` being fresh. Design a CI mechanism that either generates the artifact in CI or stages a pre-computed one with a known-good hash.
**Acceptance:** `compile-freshness` is no longer skipped in CI; runs as `runtime-artifact-required` mode.

### G4: goals-validate → CI-blocking
Currently CI-not-gating. Wire into pre-push or validate.yml as blocking.
**Acceptance:** `goals-validate` moves to "Currently enforcing" column; any push that breaks `GOALS.md` validity is blocked.

### G5: wiring-closure → CI-blocking
`scripts/check-wiring-closure.sh` exists. Wire as blocking in CI.
**Acceptance:** `wiring-closure` in "Currently enforcing" column; any push with orphan scripts/skills/hooks blocks.

## E3: Known Product Gap Closure (11 beads)

PRODUCT.md "Known Product Gaps" table has 11 rows. Each is a bead. Mapping by impact statement.

| ID | Gap | Bead seed |
|----|-----|-----------|
| **PG1** | First-value path too diffuse | Build a 5-minute install→first-validated-flow journey with measurable checkpoints. Surface: README quickstart, `ao quickstart` CLI, install scripts UX, first `/rpi` experience. |
| **PG2** | 3.0 PMF scenario evidence pending | Resolve epic `soc-m6v5.8`: define scenario, control path, exported evidence under `docs/releases/` or `evals/workbench/results/`. |
| **PG3** | /validate + /curate consolidation | Resolve epic `soc-m6v5.9` (AgentOps 3.0 polished release train): skill-count, registry, codex artifact gates must pass. |
| **PG4** | Public launch claims need exported proof | Audit all `AGENTOPS-CLAIM-*` markers in README + landing pages, link each to `docs/releases/<version>/<claim-id>.md` evidence file. |
| **PG5** | Dream autonomy still maturing | Complete `/dream` full-loop autonomy: scheduled run → harvest → forge → close-loop → defrag → report without operator intervention. |
| **PG6** | Pattern-to-skill synthesis (v2) | See D9 above; this is the same work. Cross-link. |
| **PG7** | Multi-runtime proof tiered | See D1 above; cross-link. |
| **PG8** | Retrieval + worker knowledge propagation | Audit `ao inject` quality + verify worker context packets carry prevention/finding info. Specific: workers spawned by `/crank` should receive cited learnings, not just spec. |
| **PG9** | Behavioral eval live runtime at scale | See D10; cross-link. |
| **PG10** | High-assurance profile control mapping | Extend `docs/assurance-profile.md` with redaction, evidence export, supply-chain inputs, program-specific control mapping. |
| **PG11** | Context-compiler messaging sweep | Update `docs/comparisons/*.md` and skill-page intros to use "context compiler" framing consistently. |

## E4: Four-Layer Polish (4 beads)

For each of the four product layers, identify the single highest-impact "claimed but not delivered" capability and file a closure bead.

### L1: Bookkeeping
**Highest gap:** Citation log signal-to-noise. `.agents/ao/citations.jsonl` has ~3,867 entries but utility scoring (MemRL feedback) is weak — cited artifacts get small rewards.
**Bead:** Improve citation-event weighting: cited-then-followed (the agent acted on the cite) > cited-then-ignored (just read). Add a `follow_up_action` field; weight in retrieval scoring.

### L2: Context Compiler
**Highest gap:** Phase-scoped context packets are claimed but no test verifies that `ao context assemble` produces different packets per RPI phase.
**Bead:** Add `tests/scripts/test-context-phase-scoping.bats` that runs `ao context assemble --phase=research` vs `--phase=implement` and verifies token-budgeted differences.

### L3: Validation Gates
**Highest gap:** Council judges aren't tested against known-bad inputs. No fixture-based test of "council finds the planted bug".
**Bead:** Build `tests/council/test-planted-bug-detection.bats` with N planted bugs in fixtures, council must catch ≥ floor%.

### L4: Knowledge Flywheel
**Highest gap:** Dream cycle reports compounding metrics but no operator-facing dashboard shows the trend.
**Bead:** `ao flywheel dashboard` — single-screen view of σρ, δ, citation density, learning count over time. Markdown output.

## E5: Three-Gap Contract Surface (3 beads)

GOALS.md's three-gap contract proof surface table separates "Currently enforcing" from "Roadmap". Each gap gets one closure bead that drives the roadmap-column gates leftward in sequence.

### TG1: Gap 1 — Judgment validation
Currently enforced. Bead is to ratchet stricter: add `council-coverage` gate that verifies every PR-bound commit has either a `/pre-mortem` or `/vibe` verdict in `.agents/council/`.

### TG2: Gap 2 — Durable learning
`compile-no-oscillation` is enforced; `flywheel-compounding`, `flywheel-proof`, `compile-freshness` are roadmap. See G1, G2, G3 above. Bead is the integration: a single `gap-2-closure` super-gate that combines all four.

### TG3: Gap 3 — Loop closure
`release-cadence` is partial. `flywheel-proof`, `goals-validate`, `wiring-closure` are roadmap. See G2, G4, G5 above. Bead is the integration super-gate.

## A1: AOP-CLAIM Audit (1 audit bead + N follow-ups)

83 unique AOP-CLAIM-* markers across the corpus. Each is a verifiable claim. Audit cycle:

1. Grep all markers + the paragraph that follows each.
2. Classify into categories: PRODUCT, GOALS, README, BRIEF, COMP, DOCS, etc.
3. For each claim, identify the evidence file or test that backs it.
4. For unverified claims: file a child bead with title "Verify AOP-CLAIM-<id>" and acceptance "Evidence file at <path> OR claim removed from <source>".

**Audit deliverable:** `.agents/research/2026-05-11-aop-claim-evidence-map.md` — table of all 83 claims with verified-by column.

## A2: Contract Enforcement Audit (1 audit bead + N follow-ups)

38 contracts in `docs/contracts/`. Many claim mechanical enforcement; some are documentation-only. Audit cycle:

1. For each `docs/contracts/<name>.md`, search `scripts/check-*<name>*.sh` and `tests/contracts/*<name>*`.
2. Classify: enforced, partially enforced, documentation-only.
3. For unenforced contracts: file a child bead with title "Enforce <contract-name>" and acceptance "Gate script or test exists and is wired into CI."

**Audit deliverable:** `.agents/research/2026-05-11-contract-enforcement-matrix.md` — table of all 38 contracts with enforcement column.

## A3: Code-Map Drift Audit (1 audit bead + N follow-ups)

`docs/code-map/` has `agentopsd-codebase-map.md` and `eval-lid-primitives.md`. Audit cycle:

1. For each code map, extract claimed file structure / module layout / function names.
2. Diff against actual repo via `find`, `ls`, and `grep`.
3. Drift items: file paths that the map claims exist but don't, OR files in the repo that the map doesn't account for.

**Audit deliverable:** `.agents/research/2026-05-11-code-map-drift-report.md` — diff per code map.

## LC: Learning Capture Loop (1 epic + 1 leaf + N auto-filed)

The 13-cycle /evolve session on 2026-05-11 surfaced 6 frictions only because a human ran a manual post-mortem afterwards. That doesn't compound — the next 13-cycle session would surface the same 6 frictions plus new ones, and nobody would notice the recurrence. LC encodes the reflection inside the loop so each day compounds insight about the loop's own behavior, not just the work product.

Three layers:

**1. Per-cycle micro-capture (cheap, mechanical, every cycle):**

At end of each cycle, the loop appends one line to `.agents/evolve/daily-learning-log-YYYY-MM-DD.md` in the format:
```
- cycle N [result] work_ref: short note  [optional: FRICTION: tag-slug | INSIGHT: tag-slug]
```

The daily log accumulates a low-cost stream of observations. Friction tags use a stable kebab-case taxonomy (see `.agents/evolve/daily-learning-log.template.md`) so the same friction recurring across days is mechanically detectable.

**2. Every-5th-productive-cycle reflect (light, inline):**

When `PRODUCTIVE_THIS_SESSION % 5 == 0 && > 0`, before scheduling the next wakeup the loop:
- Reads the daily log entries since the last reflect
- Looks for the same FRICTION tag appearing 2+ times this session
- Surfaces the pattern inline in the cycle-history `note` field (no separate cycle, just an annotation)
- If a tag has appeared 3+ times in the session AND once already on a prior day, fast-path file an `evolve-improvement` bead immediately rather than waiting for end-of-day

**3. End-of-day consolidate (full reflection, automatic):**

When a hard stop fires (KILL/STOP/END_AT/all-evolution-beads-closed/dormancy), the loop runs:
```
bash scripts/evolve-capture-daily-learning.sh
```

This produces `.agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md` with:
- Counts (total/productive/scout/idle/regressed cycles)
- Cycle ledger for the day
- Per-cycle micro-captures
- Friction tags this session
- Cross-day recurring patterns (compared to prior daily learning files)
- Promotion candidates

The script ALSO auto-files `evolution-roadmap` + `evolve-improvement` labeled beads for any friction that recurred on 2+ days, with the title shape `LC-followup: Address recurring /evolve friction "<tag>"` and idempotency by title match.

**Acceptance:**
- `scripts/evolve-capture-daily-learning.sh` exists, is executable, and supports `--dry-run`.
- `.agents/evolve/daily-learning-log.template.md` documents the micro-capture format + tag taxonomy.
- The all-day starter prompt invokes the consolidator at every hard stop.
- After 2 days of /evolve runs, the recurring-friction auto-bead-filing path is exercised at least once (test scenario: same FRICTION tag in two daily log files → 1 new `LC-followup` bead).

**Why "capture cycle" vs "post-mortem-then-forget":**
Manual post-mortem on the 13-cycle session worked, but it required a human turn after the day to write the learnings. LC inverts that: the loop captures continuously and consolidates automatically, so day 2 starts with the lessons from day 1 already promoted into the work queue. The flywheel applies to the loop itself, not just to product work.

## Execution Order (for /evolve to drain)

Priority order, descending by "operator-visible impact per unit work":

1. **E1.D7** (codex parity = 0) — gate exists, just close drift items. Quick win.
2. **E1.D3** (quarantine gate) — add one gate, freeze a directive. Low effort.
3. **E2.G2, G4, G5** (wire flywheel-proof, goals-validate, wiring-closure as CI-blocking) — gates exist, just wire them.
4. **A2** (contract enforcement audit) — generates N more beads of similar shape.
5. **A1** (AOP-CLAIM audit) — generates N more beads.
6. **E3.PG3** (release-train consolidation) — clears a blocker on 3.0 launch.
7. **E1.D11** (corpus durability) — addresses the most visible product gap.
8. **E1.D1** (multi-runtime live execution) — closes the most-cited promise gap.
9. Everything else.

## Soft Targets

- **End of week:** E1.D7, E1.D3, E2.G2/G4/G5 closed. A1 + A2 audits completed.
- **End of month:** All A1+A2+A3 follow-up beads enumerated. 50% of E1 directives closed.
- **End of quarter:** E3 product gaps all have either closure or "won't fix" disposition.

## Operating Contract

Each /evolve cycle:
1. Picks the highest-priority unblocked bead from `bd ready` (this road map's beads have priority P1).
2. Runs `/rpi --auto --max-cycles=1` against it.
3. Logs cycle, commits, advances queue.
4. Schedules next wakeup.

When all of E1+E2+E3+E4+E5 close: the audits in A1/A2/A3 will have generated follow-up beads; /evolve switches to draining those.

When even those are empty: the loop has reconciled the entire aspirational surface against the code. That's the dormancy criterion the skill currently mis-models.

## Cross-References

- PRODUCT.md "Known Product Gaps" table — source for E3
- GOALS.md "Directives" section — source for E1
- GOALS.md "Three-Gap Contract Proof Surface" — source for E2 + E5
- GOALS.md "Gates" table — source for E2 priority ordering
- PRACTICE-REGISTRY.md slug registry — source for D9 acceptance criteria
- `docs/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md` — context for why this road map exists (durable export of the originally-local-only learning, per soc-w6vh.5)
