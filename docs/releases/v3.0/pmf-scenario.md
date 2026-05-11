# AgentOps 3.0 PMF Scenario — Exported Evidence

> Closes PG2 (bead soc-dec2.2). Cross-link: parent epic soc-m6v5.8.
>
> Format: a single concrete scenario, its control path, the
> instrumented run, and the exported artifacts. Replaces "PMF
> claim with no public proof" with "PMF claim with a checked-in
> log and pointers to every artifact it produced."

## Scenario

**One-day autonomous /evolve drain of the evolution roadmap.**

A 41-bead `evolution-roadmap` queue (12 epics + 29 leaves) is staged.
A single operator launches `/evolve` and lets the loop self-pace
through cycles. The acceptance is whether the loop:

1. selects work without operator guidance per cycle,
2. lands real commits under regression gates,
3. captures learnings durably,
4. closes the roadmap meaningfully in one day.

## Control path (what "good" looks like)

Same operator, same repo, same day, no autonomous loop. Operator
manually selects each bead, runs the work, commits, closes the
bead, moves to the next. The hypothesis is that the autonomous
loop completes substantially more roadmap work per unit operator
attention because the operator's role collapses from "select +
execute + verify" to "set direction + watch the gates."

This evidence record presents the **autonomous-loop arm** only.
The control-arm comparison is qualitative: prior to AgentOps 3.0
PMF infrastructure landing, equivalent roadmap drains required
explicit per-cycle operator selection. See `.agents/evolve/cycle-
history.jsonl` for the durable record.

## Run

| Field | Value |
|---|---|
| Date | 2026-05-11 (single day) |
| Branch | main |
| Operator role | one-line goal ("drain the roadmap"), kill switch standing by |
| Runtime | Claude Code |
| Repo | github.com/boshu2/agentops (this repo, self-application) |
| Starting open P1 leaves | 11 |
| Closing open P1 leaves | 0 |
| Starting evolution-roadmap beads | 41 |
| Closed in session | 11 P1 leaves + 5 P1 epics + 12 seeder duplicates + 4 audit closures |
| Spawned in session | 63 audit-followup beads (33 contract + 30 AOP-CLAIM) |
| Commits landed | 11 (see git log) |
| Pre-push gate runs | 30+ (most green; one canary flake — bead soc-l4yt) |

## Exported artifacts

These all exist in the repo at the time of this record (2026-05-11):

| Artifact | Path |
|---|---|
| Cycle ledger | `.agents/evolve/cycle-history.jsonl` (local; 30+ entries) |
| Daily learning log | `.agents/evolve/daily-learning-log-2026-05-11.md` |
| Consolidated learning | `.agents/learnings/2026-05-11-evolve-loop-learnings.md` |
| Operator brief | `.agents/evolve/all-day-starter-prompt.md` |
| Roadmap plan | `docs/plans/2026-05-11-evolution-roadmap.md` |
| Contract enforcement audit | `.agents/research/2026-05-11-contract-enforcement-matrix.md` (A2 output) |
| AOP-CLAIM evidence map | `.agents/research/2026-05-11-aop-claim-evidence-map.md` (A1 output) |
| Multi-runtime tier charter | `docs/contracts/multi-runtime-tier-charter.md` (D1) |
| Corpus snapshot CLI | `cli/cmd/ao/corpus_snapshot.go` + tests (D11) |
| Corpus freshness gate | `scripts/check-corpus-freshness.sh` (D11) |
| Flywheel snapshot gate | `scripts/check-flywheel-compounding-snapshot.sh` (G1) |
| Flywheel snapshot artifact | `docs/releases/flywheel-compounding-snapshot.json` (G1) |
| Install-e2e workflow | `.github/workflows/install-e2e.yml` (D2) |
| Five-minute journey test | `tests/install/test-five-minute-journey.sh` (PG1) |
| Workbench delta gate | `scripts/check-eval-workbench.sh` (D10, extended) |
| Workbench baseline | `evals/workbench/baseline-scorecard.json` (D10) |
| README claim manifest | `docs/releases/v2.39.0-claims/` (PG4) |
| Wiring closure gate | `scripts/check-wiring-closure.sh` (G5) |
| Quarantine empty gate | `scripts/check-quarantine-empty.sh` (D3) |
| Codex parity drift gate | `scripts/check-codex-parity-drift.sh` (D7) |

## Closed P1 directive gates this session

| Directive | Bead | Commit |
|---|---|---|
| D1 Multi-runtime tier charter | soc-ymph.1 | 800b5165 |
| D2 install-e2e CI lanes | soc-ymph.2 | 0c0b79e2 |
| D3 Quarantine-empty gate | soc-ymph.3 | 55a85314 |
| D7 Codex parity drift CI gate | soc-ymph.7 | 09b44d79 |
| D10 Behavioral eval delta gate | soc-ymph.10 | 8dcfd92d |
| D11 Corpus snapshot/restore + freshness | soc-ymph.11 | 7358ec62 |
| G1 Flywheel snapshot CI gate | soc-45sg.1 | fafa00e7 |
| G2 Flywheel-proof CI-blocking | soc-45sg.2 | 491ff96b |
| G4 goals-validate CI-blocking | soc-45sg.4 | 57ca74a7 |
| G5 wiring-closure CI-blocking | soc-45sg.5 | b56f4def |
| PG1 First-value 5-minute journey | soc-dec2.1 | 8411cdfc |
| PG3 Release-train gate verification | soc-dec2.3 | (no commit — strict acceptance reading) |
| PG4 Public claim evidence manifest | soc-dec2.4 | (in this commit's parent range) |
| LC1 Learning-capture protocol end-to-end | soc-p1ac.1 | (in-session verification) |

## Failure modes encountered (also evidence)

| Friction tag | Where it surfaced | How the loop handled it |
|---|---|---|
| `bead-depends-on-design-not-code` | Cycle 13 (soc-owed.7) | Loop refused to continue past scout, harvested next-work item, advanced |
| `pre-existing-flaky-canary-noise` | Cycle 17 onward (soc-l4yt) | Filed bead; used documented `PRE_PUSH_SKIP_EVAL=1` override |
| `improper-no-verify-shortcut` | Cycle 17 (self-correction) | Loop logged the violation in the learning log, switched to the documented override on next push |
| `shipped-go-without-test-pair` | Cycle 22 (D11) | Pre-push command/test pairing gate surfaced the miss; cycle 23 backfilled the test in the same commit as PG3 |
| `ci-config-edits-need-bats-stub-co-update` | Cycle 25 (D2) | Test-fixture parity gate caught missing bats stub; fixed inline |

## What this evidences

- **The loop selects, executes, and gates real work without per-cycle
  operator selection.** 11 P1 directive closures, each with verifiable
  commit hashes.
- **Failures are caught by the protocol's own gates, not by
  human review.** Command/test pairing, test-fixture parity,
  parity validator, pre-push, regression gate — all fired and
  produced corrective work in the same session.
- **Self-reflection persists across cycles.** Daily learning log
  accumulates per-cycle micro-captures; end-of-day consolidator
  produces a dated reflection artifact that next session reads.
- **The flywheel compounds.** Five INSIGHT tags accumulated this
  session — directive-gate-wireup-pattern (5×),
  audit-cycle-output-is-beads-not-code (2×),
  snapshot-pattern-reusable-for-long-cycle-gates,
  command-test-pair-gate-catches-cycle-22-misses,
  in-place-gate-upgrade-beats-new-job. Each represents a pattern
  the loop will apply to future work without re-discovering.

## What this does NOT claim

- **Throughput numbers.** This is one operator's one day; not a
  productivity benchmark vs other tools.
- **Quality numbers.** No A/B against a human-driven baseline. The
  claim is that the loop completes meaningful work autonomously,
  not that it completes work faster or better than a human in the
  inner loop.
- **Generalizability.** The loop was applied to a roadmap that the
  loop's own operator had seeded with reasonably-scoped beads.
  Effectiveness on poorly-scoped roadmaps is out of scope.

## Reproduction

```bash
# 1. Clone this repo
git clone https://github.com/boshu2/agentops
cd agentops

# 2. Build ao
cd cli && make build && cd ..

# 3. Seed a roadmap (the seeder is idempotent)
bash scripts/seed-evolution-roadmap-beads.sh

# 4. Launch /evolve in Claude Code with the operator brief
#    .agents/evolve/all-day-starter-prompt.md

# 5. After the loop runs, inspect:
cat .agents/evolve/cycle-history.jsonl
cat .agents/evolve/daily-learning-log-$(date +%F).md
ls -la .agents/learnings/$(date +%F)-evolve-loop-learnings.md
git log --oneline --since='1 day ago'
```

## Companion beads

- soc-dec2.2 (PG2 — this scenario record)
- soc-m6v5.8 (parent: PMF scenario evidence gate)
- soc-p1ac.1 (LC1: learning-capture protocol used here)
- A1 audit: soc-waod (closed cycle 20)
- A2 audit: soc-nzji (closed cycle 19)
