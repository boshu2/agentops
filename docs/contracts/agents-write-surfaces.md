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
| `ao` | persistent | cli, hooks | runtime-state | Core runtime state: chain, citations, baselines, history, search index, factory state, hook-error logs |
| `archive` | persistent | cli | retention-archive | Archived/superseded artifacts with retention metadata |
| `archived-worktrees` | persistent | scripts | operator-preserve | Snapshot of preserved worktrees pending review |
| `briefings` | regenerated | scripts | generated-output | Compiled session-start briefings |
| `candidates` | rolling | cli | candidate-cache | Pre-promotion candidate observations |
| `compaction-snapshots` | rolling | hooks | hook-snapshot | Pre-compaction context snapshots |
| `compile` | regenerated | cli | generated-output | Intermediate compile state |
| `compiled` | regenerated | cli | generated-output | Compiled wiki output (subset; see `wiki/`) |
| `config` | persistent | cli, operators | operator-config | Rig identity (project/crew) - read by harvest path-prefix fallback |
| `constraints` | persistent | cli | generated-policy | Compiled constraint manifests |
| `context` | rolling | cli | run-scoped-cache | Per-run adhoc context injection paths keyed by run ID |
| `daemon` | persistent | cli | durable-ledger | Authoritative daemon ledger, queue/job projections, activation state, and consumer snapshots |
| `decisions` | persistent | operators, skills | decision-record | Durable decision records and review artifacts not owned by a single active skill |
| `defrag` | rolling | cli, scripts | maintenance-run-state | Defrag run state and dry-run reports |
| `evals` | persistent | cli, scripts | eval-evidence | Eval run outputs, promoted baselines, and suite execution state |
| `findings` | persistent | scripts, skills | promotion-inbox | Mined findings awaiting promotion |
| `git` | persistent | cli | git-cache | Git-derived state cached for the runtime |
| `handoffs` | persistent | cli | durable-replay-artifact | Content-addressed handoff artifacts keyed by sha256 for daemon job replay |
| `holdout` | persistent | cli, skills | scenario-store | Holdout scenarios stored outside the codebase view |
| `INDEX.md` | persistent | operators, scripts | corpus-index | Human-readable index for tracked `.agents/` knowledge surfaces |
| `knowledge` | persistent | cli | promoted-knowledge | Promoted knowledge artifacts |
| `learnings` | persistent | cli, skills | promoted-learning | Promoted learning artifacts |
| `ledger` | persistent | cli | append-only-ledger | Append-only audit ledger |
| `LOG.md` | persistent | operators, scripts | corpus-log | Human-readable change log for tracked `.agents/` knowledge surfaces |
| `memory` | persistent | cli | memory-rl-state | Memory-rl artifacts |
| `mine` | rolling | cli, skills | mining-inbox | Mined raw signal awaiting promotion |
| `nightly` | rolling | scripts | local-nightly-state | Private local nightly run digests, readiness snapshots, scheduler templates, and phase logs |
| `opencode-tests` | regenerated | scripts, tests | test-output | Opencode runtime test fixtures and outputs |
| `overnight` | rolling | scripts, skills | overnight-run-state | Overnight run state and morning packets |
| `packets` | rolling | cli | context-packet-cache | Source manifests and promoted packets feeding the context-explain surface |
| `patterns` | persistent | cli, skills | promoted-pattern | Promoted pattern artifacts |
| `planning-rules` | persistent | cli | generated-policy | Planning rules sourced from skills/contracts |
| `plans` | persistent | skills, scripts | planning-artifact | Planning artifacts |
| `playbooks` | persistent | cli | generated-playbook | Compiled playbook candidates |
| `pool` | persistent | cli | candidate-inbox | Idea pool / candidate inbox |
| `pre-mortem-checks` | persistent | skills | validation-artifact | Pre-mortem check templates and runs |
| `products` | persistent | skills | product-artifact | Product validation artifacts |
| `profile` | persistent | cli | profile-cache | Repo execution profile cache |
| `quarantine` | rolling | cli | failure-quarantine | Failed daemon worker payloads and retry/quarantine evidence for operator review |
| `releases` | rolling | scripts | release-evidence | Local CI release evidence |
| `retros` | persistent | skills | retro-artifact | Retrospectives |
| `schedule` | persistent | cli, scripts | schedule-store | Daemon schedule entries consumed by `ao schedule` and `agentopsd --schedule-file` |
| `schedule.yaml.example` | persistent | scripts, operators | schedule-example | Checked-in example schedule for daemon/runtime scheduling |
| `sessions` | rolling | cli, hooks | session-cache | Session transcripts and matches |
| `signals` | rolling | hooks | append-only-signals | Append-only quality signal log |
| `skill-drafts` | rolling | cli | generated-draft | Auto-generated SKILL.md drafts emitted by the ratchet (per-slug) |
| `skills` | persistent | cli | installed-skill-state | User-installed skill state (alt path under `~/.agents/skills/`) |
| `smoke-test` | regenerated | scripts, tests | test-output | Smoke test scratch dirs |
| `specs` | persistent | skills | spec-artifact | Specs gating ratchet steps |
| `swarm-role` | rolling | skills | coordination-state | Per-role swarm state |
| `synthesis` | persistent | skills | synthesis-artifact | Synthesis output |
| `tasks` | persistent | cli | task-fallback | Beads-optional task tracking fallback |
| `teams` | rolling | cli | coordination-state | Team coordination state |
| `tests` | regenerated | scripts, tests | test-output | Official local/CI test artifacts, including contract-canary run records |
| `triage` | persistent | operators, scripts | triage-artifact | Tracked triage packets and operator review notes not owned by a single active skill |
| `topics` | rolling | cli | topic-packet-cache | Topic-packets surface inputs |
| `wiki` | regenerated | cli | generated-output | Wiki source artifacts written by Dream / forge pipelines (sources/) |

### Skill-owned subdirs

Each active skill at `skills/<name>/SKILL.md` may write under
`.agents/<name>/`. The lint accepts any such reference automatically and
does not require an entry above. Removing a skill removes the implicit
permission for that subdir.

## Allowlist

<!-- BEGIN agents-write-surfaces-allowlist -->
ao
archive
archived-worktrees
briefings
candidates
compaction-snapshots
compile
compiled
config
constraints
context
daemon
defrag
evals
findings
git
handoffs
holdout
knowledge
learnings
ledger
memory
mine
nightly
opencode-tests
overnight
packets
patterns
planning-rules
plans
playbooks
pool
pre-mortem-checks
products
profile
quarantine
releases
retros
schedule
sessions
signals
skill-drafts
skills
smoke-test
specs
swarm-role
synthesis
tasks
teams
tests
topics
wiki
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
