# `.agents/` Write Surfaces

> **Status:** Draft
> **Consumers:** `scripts/check-agents-write-surfaces.sh`, `/evolve`, future `ao agents` tooling
> **Linted by:** `scripts/check-agents-write-surfaces.sh`

This contract catalogues every top-level subdirectory under local repo-root
`.agents/` that agentops production code (Go in `cli/` excluding tests, shell
in `scripts/`, `hooks/`, `lib/`) writes to or persists state under. Repo-root
`.agents/` is runtime state, not a git persistence surface; it is ignored by
policy and guarded by `scripts/check-no-tracked-agents.sh`. This contract does
**not** cover
skill-owned subdirs that follow the `.agents/<skill-name>/` convention —
those are validated dynamically against `skills/<skill-name>/SKILL.md`.
The `ao session state` durable authority lives under `.ao/accepted` with its admission
ledger under `.ao/admissions/`; those paths are outside this `.agents/`
write-surface allowlist.

The lint that backs this contract reads the allowlist between the
`<!-- BEGIN agents-write-surfaces-allowlist -->` /
`<!-- END agents-write-surfaces-allowlist -->` markers below. Any
`.agents/<X>` literal in production code where `<X>` is neither in the
allowlist nor an active skill name fails the gate.

## Surfaces

Lifecycle vocabulary is restricted to `persistent`, `rolling`,
`regenerated`, `runtime-only`, and `ignored`. Allowed writers are restricted to
`cli`, `hooks`, `scripts`, `skills`, `operators`, and `tests`. The mutation
lane must name the intended write path; it cannot be blank or placeholder text.

| Surface | Lifecycle | Allowed writers | Mutation lane | Purpose |
|---|---|---|---|---|
| `ao` | persistent | cli | runtime-state | Core runtime state: chain, citations, baselines, history, search index, factory state |
| `archive` | persistent | cli | retention-archive | Archived/superseded artifacts with retention metadata |
| `briefings` | regenerated | scripts | generated-output | Compiled session-start briefings |
| `candidates` | rolling | cli | candidate-cache | Pre-promotion candidate observations |
| `canon` | persistent | cli | earned-knowledge | Team-knowledge canon: earned learnings plus the citation/verification attestation ledgers gating promotion |
| `compiled` | regenerated | cli | generated-output | Compiled wiki output (subset; see `wiki/`) |
| `config` | persistent | cli, operators | operator-config | Rig identity (project/crew) - read by harvest path-prefix fallback |
| `constraints` | persistent | cli | generated-policy | Compiled constraint manifests |
| `context` | rolling | cli | run-scoped-cache | Per-run adhoc context injection paths keyed by run ID |
| `daemon` | persistent | cli | durable-ledger | Legacy ledger, queue/job projection, and activation state written by the load-bearing legacy RPI lane (the standalone daemon surface itself was retired in 3.0) |
| `decisions` | persistent | operators, skills | decision-record | Durable decision records and review artifacts not owned by a single active skill |
| `defrag` | rolling | cli, scripts | maintenance-run-state | Defrag run state and dry-run reports |
| `duel` | rolling | cli, skills | plan-pawl-duel-state | Plan-pawl duel per-round judge verdicts read by `ao plan-pawl decide` (`--dir`); written by the duel skill (dual-pane-atm) |
| `evals` | persistent | cli, scripts | eval-evidence | Eval run outputs, promoted baselines, and suite execution state |
| `findings` | persistent | scripts, skills | promotion-inbox | Mined findings awaiting promotion |
| `git` | persistent | cli | git-cache | Git-derived state cached for the runtime |
| `goal-design` | persistent | skills, operators, tests | goal-design-packet | Schema-backed goal-design packets (`intent.md` + `driver.md`) under `.agents/goal-design/<slug>/`; generated packets stay local runtime state and must pass `scripts/check-goal-design-packet.sh` plus independent validation before driving work |
| `handoffs` | persistent | cli | durable-replay-artifact | Content-addressed handoff artifacts keyed by sha256 for job replay |
| `harvest` | persistent | cli | promotion-artifact | Cross-rig promotion sweep output (`.agents/harvest/latest.json`) written by `ao harvest`; formerly covered by the retired `harvest` skill (folded into `curate --mode=harvest`, cp-dxa) |
| `holdout` | persistent | cli, skills | scenario-store | Holdout scenarios stored outside the codebase view |
| `INDEX.md` | persistent | operators, scripts | corpus-index | Human-readable index for tracked `.agents/` knowledge surfaces |
| `knowledge` | persistent | cli | promoted-knowledge | Promoted knowledge artifacts |
| `land-queue` | rolling | scripts | land-serialization-state | Serialized land-queue state for the single-writer land lane (agentops-2pl.9): the file-queue (`requests.jsonl`), `claims`/`done`/`dead-letter`/`quarantine` side-files, gate/land logs, the `.lane.lock` singleton, and the no-actions `gh` runtime shim — written by `scripts/land-submit.sh` and `scripts/land-lane-run.sh`, read by `scripts/land-queue-next.sh` |
| `learnings` | persistent | cli, skills | promoted-learning | Promoted learning artifacts |
| `ledger` | persistent | cli | append-only-ledger | Append-only audit ledger |
| `LOG.md` | persistent | operators, scripts | corpus-log | Human-readable change log for tracked `.agents/` knowledge surfaces |
| `memory` | persistent | cli | memory-rl-state | Memory-rl artifacts |
| `mine` | rolling | cli, skills | mining-inbox | Mined raw signal awaiting promotion |
| `mto-handoff` | rolling | scripts | cross-factory-handoff | Inbound MTO→AgentOps recurrence handoff (`.agents/mto-handoff/recurrence.json`), written by the external MTO bridge (mt-olympus `efficacy-to-flywheel.sh`) and read by `scripts/assay/consume-mto-recurrence.sh` to emit a planning-rule on a recurrence tripwire |
| `nightly` | rolling | scripts | local-nightly-state | Private local nightly run digests, readiness snapshots, scheduler templates, and phase logs |
| `opencode-tests` | regenerated | scripts, tests | test-output | Opencode runtime test fixtures and outputs |
| `operator` | rolling | cli | operator-intents | Durable OperatorIntent records (halt, rescope, handoff) appended via the BC4 OperatorPort |
| `orchestration` | rolling | cli | orchestration-result | OrchestrationResult parity artifacts written by OrchestrationPort backends (beads-floor + tier results) |
| `overnight` | rolling | scripts, skills | overnight-run-state | Overnight run state and morning packets |
| `packets` | rolling | cli | context-packet-cache | Source manifests and promoted packets feeding the context-explain surface |
| `patterns` | persistent | cli, skills | promoted-pattern | Promoted pattern artifacts |
| `pawl` | persistent | scripts | service-state | Standing cross-family pawl-service session state (pane map + readiness, `session.json`) written by `scripts/pawl.sh up` and read by `route`/`health`/`down`; see `age-standing-pawl-service-ml8` |
| `pawl-evidence` | rolling | scripts | decision-record | Refuter review evidence — the codex cross-family review output proving the review actually ran — written per bead by `scripts/pawl-review.sh` (and the `/pre-land-refuters` flow) and read by `scripts/pawl-verdict.sh check`, which fail-closes if a refuter's `evidence` path is missing/empty |
| `pawl-review` | rolling | scripts | decision-record | Adversarial-review LINEAGE (`<bead>.adversarial.json`: the reviewed diff-hash + outcome) written by `scripts/pawl-review.sh` so `--converge` (the calibrated real-safety bar) can fail-closed unless a prior adversarial run covered the identical diff — preventing a skip of the adversarial pass (council C, age-cwo.8) |
| `pawl-verdicts` | persistent | scripts, skills | decision-record | Machine-checkable pawl verdicts (fresh-context default; multi-model opt-in; schema `schemas/pawl-verdict.v1.schema.json`) written by `/pre-land-refuters` via `scripts/pawl-verdict.sh write` and read by `scripts/reconcile-pr.sh` to gate merge-to-main (fail-closed: no CONFIRMED verdict, no merge) |
| `planning-rules` | persistent | cli | generated-policy | Planning rules sourced from skills/contracts |
| `plans` | persistent | skills, scripts | planning-artifact | Planning artifacts |
| `playbooks` | persistent | cli | generated-playbook | Compiled playbook candidates |
| `pool` | persistent | cli | candidate-inbox | Idea pool / candidate inbox |
| `pre-mortem-checks` | persistent | skills | validation-artifact | Pre-mortem check templates and runs |
| `products` | persistent | skills | product-artifact | Product validation artifacts |
| `profile` | persistent | cli | profile-cache | Repo execution profile cache |
| `provenance` | persistent | cli | legacy-ratchet-chain | Legacy ratchet provenance chain (`.agents/provenance/chain.yaml`) read/migrated by the ratchet CLI; formerly covered by the retired `provenance` skill |
| `proof` | persistent | scripts, operators | proof-evidence | Corpus-state and flywheel-compounding proof snapshots consumed by Roadmap-gate CI (GOALS.md G1) |
| `quarantine` | rolling | cli | failure-quarantine | Failed worker payloads and retry/quarantine evidence for operator review |
| `reconcile` | persistent | scripts, operators | reconciliation-artifact | Reconciliation engine artifacts: observation log aggregated from `factory-claim-ledger-strict (advisory)` CI runs, promotion-decision template, and related Wave-1E gate evidence (epic soc-e4ulx) |
| `releases` | rolling | scripts | release-evidence | Local CI release evidence |
| `retro` | persistent | cli | retro-artifact | Quick-capture learning index (`.agents/retro/index.jsonl`) written by the ratchet/index/init CLI; formerly covered by the retired `retro` skill (folded into `post-mortem --quick`, cp-bzj) |
| `retros` | persistent | skills | retro-artifact | Retrospectives |
| `schedule` | persistent | cli, scripts | schedule-store | Legacy schedule entries from the retired in-tree scheduler; out-of-session scheduling is now the substrate's job (NTM / MCP / managed-agents) |
| `schedule.yaml.example` | persistent | scripts, operators | schedule-example | Checked-in example schedule retained for the legacy/registry reader; out-of-session scheduling runs on the substrate (NTM / MCP / managed-agents) |
| `schedules` | rolling | cli | demo-schedule-example | Demo-scoped schedule files generated by `ao demo --quick --workdir` |
| `sessions` | rolling | cli | session-cache | Session transcripts and matches |
| `spawn` | rolling | skills, operators | spawn-evidence | Per-session spawn metadata JSON (e.g. `.agents/spawn/<session>.json`) written by ATM spawn checklists; formerly read by the now-removed `ao orchestrate verify` strong evidence tier (`ao orchestrate` was removed in 3.0 — the spawn-evidence surface is now exposed via the NTM + MCP Agent Mail substrate) |
| `signals` | rolling | cli | append-only-signals | Append-only quality signal log |
| `skill-drafts` | rolling | cli | generated-draft | Auto-generated SKILL.md drafts emitted by the ratchet (per-slug) |
| `skills` | persistent | cli | installed-skill-state | User-installed skill state (alt path under `~/.agents/skills/`) |
| `smoke-test` | regenerated | scripts, tests | test-output | Smoke test scratch dirs |
| `specs` | persistent | skills | spec-artifact | Specs gating ratchet steps |
| `synthesis` | persistent | skills | synthesis-artifact | Synthesis output |
| `tasks` | persistent | cli | task-fallback | Beads-optional task tracking fallback |
| `teams` | rolling | cli | coordination-state | Team coordination state |
| `tests` | regenerated | scripts, tests | test-output | Official local/CI test artifacts, including contract-canary run records |
| `triage` | persistent | operators, scripts | triage-artifact | Tracked triage packets and operator review notes not owned by a single active skill |
| `topics` | rolling | cli | topic-packet-cache | Topic-packets surface inputs |
| `validation` | persistent | scripts | proof-evidence | Local pre-push proof file (`.agents/validation/pre-push-success.tsv`) written by `scripts/pre-push-proof.sh`; formerly covered by the retired `validation` skill (folded into `validate --mode=post-impl`, cp-ki8) |
| `wiki` | regenerated | cli | generated-output | Wiki source artifacts written by Dream / forge pipelines (sources/) |
| `yield` | persistent | cli | durable-ledger | Dynamo yield instrument: append-only per-bead operational event stream (`yield-ledger.jsonl`) written by `ao yield emit`; read by `ao yield gauge` (ag-grcz3/ag-qzinh) |

### Skill-owned subdirs

Each active skill at `skills/<name>/SKILL.md` may write under
`.agents/<name>/`. The lint accepts any such reference automatically and
does not require an entry above. Removing a skill removes the implicit
permission for that subdir.

## Allowlist

<!-- BEGIN agents-write-surfaces-allowlist -->
ao
archive
briefings
candidates
canon
compiled
config
constraints
context
daemon
defrag
duel
evals
findings
git
goal-design
handoffs
harvest
holdout
knowledge
land-queue
learnings
ledger
memory
mine
mto-handoff
nightly
opencode-tests
operator
orchestration
overnight
packets
patterns
pawl
pawl-evidence
pawl-review
pawl-verdicts
planning-rules
plans
playbooks
pool
pre-mortem-checks
products
profile
proof
provenance
quarantine
reconcile
releases
retro
retros
schedule
schedules
sessions
spawn
signals
skill-drafts
skills
smoke-test
specs
synthesis
tasks
teams
tests
topics
validation
wiki
yield
<!-- END agents-write-surfaces-allowlist -->

## How to update

1. Add a new write surface to production code (Go, shell, or hook).
2. Add a row in the `## Surfaces` table above explaining owner / lifecycle / purpose.
3. Add the bare subdir name to the allowlist block.
4. Run `scripts/check-agents-write-surfaces.sh` and confirm it exits 0.
5. Run `scripts/check-no-tracked-agents.sh` and confirm no repo-root `.agents` path is tracked or staged for add/modify.
6. Add or update a regression test in `tests/scripts/check-agents-write-surfaces.bats` if the new surface introduces a new contract dimension (format, ownership rule, lifecycle).

## See also

- `docs/contracts/repo-execution-profile.md` — repo-local operating policy
- `PROGRAM.md` — autodev mutable/immutable scope
- `scripts/check-wiring-closure.sh` — broader registry-coverage gate
