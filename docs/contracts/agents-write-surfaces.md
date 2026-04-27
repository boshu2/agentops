# `.agents/` Write Surfaces

> **Status:** Draft
> **Consumers:** `scripts/check-agents-write-surfaces.sh`, `/evolve`, future `ao agents` tooling
> **Linted by:** `scripts/check-agents-write-surfaces.sh`

This contract catalogues every top-level subdirectory under `.agents/` that
agentops production code (Go in `cli/` excluding tests, shell in `scripts/`,
`hooks/`, `lib/`) writes to or persists state under. It does **not** cover
skill-owned subdirs that follow the `.agents/<skill-name>/` convention —
those are validated dynamically against `skills/<skill-name>/SKILL.md`.

The lint that backs this contract reads the allowlist between the
`<!-- BEGIN agents-write-surfaces-allowlist -->` /
`<!-- END agents-write-surfaces-allowlist -->` markers below. Any
`.agents/<X>` literal in production code where `<X>` is neither in the
allowlist nor an active skill name fails the gate.

## Surfaces

| Subdir | Owner | Lifecycle | Purpose |
|---|---|---|---|
| `ao` | cli (`cli/internal/storage`, `cli/internal/ratchet`, hooks) | persistent | Core runtime state: chain, citations, baselines, history, search index, factory state, hook-error logs |
| `archive` | cli (curate, harvest, dedup) | persistent | Archived/superseded artifacts with retention metadata |
| `archived-worktrees` | scripts (`worktree-disposition`) | persistent | Snapshot of preserved worktrees pending review |
| `briefings` | scripts (compile, brief generators) | regenerated | Compiled session-start briefings |
| `candidates` | cli (ratchet `TierObservation`) | rolling | Pre-promotion candidate observations |
| `compaction-snapshots` | hooks (compaction recovery) | rolling | Pre-compaction context snapshots |
| `compile` | cli (compile pipeline) | regenerated | Intermediate compile state |
| `compiled` | cli (compile pipeline) | regenerated | Compiled wiki output (subset; see `wiki/`) |
| `config` | cli (.agents/.config) | persistent | Rig identity (project/crew) — read by harvest path-prefix fallback |
| `constraints` | cli (constraints subsystem) | persistent | Compiled constraint manifests |
| `context` | cli (`cli/cmd/ao/inject_context_paths.go`) | rolling | Per-run adhoc context injection paths keyed by run ID |
| `defrag` | cli (compile defrag), scripts | rolling | Defrag run state and dry-run reports |
| `evals` | cli eval runtime, scripts (`eval-agentops`) | persistent | Eval run outputs, promoted baselines, and suite execution state |
| `findings` | scripts, /forge | persistent | Mined findings awaiting promotion |
| `git` | cli (git-aware tooling) | persistent | Git-derived state cached for the runtime |
| `holdout` | cli (`/scenario`) | persistent | Holdout scenarios stored outside the codebase view |
| `knowledge` | cli (compile knowledge surface) | persistent | Promoted knowledge artifacts |
| `learnings` | cli (curate `TierLearning`), /post-mortem, /retro | persistent | Promoted learning artifacts |
| `ledger` | cli (audit ledger) | persistent | Append-only audit ledger |
| `memory` | cli (memory subsystem) | persistent | Memory-rl artifacts |
| `mine` | cli (compile mine), /forge | rolling | Mined raw signal awaiting promotion |
| `opencode-tests` | scripts (opencode runtime tests) | regenerated | Opencode runtime test fixtures and outputs |
| `overnight` | scripts (nightly dream cycle), /dream | rolling | Overnight run state and morning packets |
| `packets` | cli (`context_explain.go`, `context_packet_status.go`) | rolling | Source manifests and promoted packets feeding the context-explain surface |
| `patterns` | cli (curate), /post-mortem, /retro | persistent | Promoted pattern artifacts |
| `planning-rules` | cli (planning) | persistent | Planning rules sourced from skills/contracts |
| `plans` | /plan, scripts | persistent | Planning artifacts |
| `playbooks` | cli (knowledge-activation) | persistent | Compiled playbook candidates |
| `pool` | cli (`pool.PoolDir`) | persistent | Idea pool / candidate inbox |
| `pre-mortem-checks` | /pre-mortem | persistent | Pre-mortem check templates and runs |
| `products` | /product | persistent | Product validation artifacts |
| `profile` | cli (profile subsystem) | persistent | Repo execution profile cache |
| `releases` | scripts (`ci-local-release`) | rolling | Local CI release evidence |
| `retros` | /retro | persistent | Retrospectives |
| `sessions` | cli (`.agents/ao/sessions`), hooks | rolling | Session transcripts and matches |
| `signals` | hooks (quality-signals) | rolling | Append-only quality signal log |
| `skill-drafts` | cli (`cli/internal/ratchet/skill_drafts.go`) | rolling | Auto-generated SKILL.md drafts emitted by the ratchet (per-slug) |
| `skills` | cli (`doctor`, install) | persistent | User-installed skill state (alt path under `~/.agents/skills/`) |
| `smoke-test` | scripts (smoke harness) | regenerated | Smoke test scratch dirs |
| `specs` | /plan, /pre-mortem | persistent | Specs gating ratchet steps |
| `swarm-role` | /swarm | rolling | Per-role swarm state |
| `synthesis` | /plan, /forge, /compile | persistent | Synthesis output |
| `tasks` | cli (quickstart beads-optional mode) | persistent | Beads-optional task tracking fallback |
| `teams` | cli (`codex-team`, `swarm`) | rolling | Team coordination state |
| `topics` | cli (`context_explain.go`, `context_packet_status.go`) | rolling | Topic-packets surface inputs |
| `wiki` | cli (`dream_subcycle.go`, `forge.go`, `overnight.go`) | regenerated | Wiki source artifacts written by Dream / forge pipelines (sources/) |

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
defrag
evals
findings
git
holdout
knowledge
learnings
ledger
memory
mine
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
releases
retros
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
topics
wiki
<!-- END agents-write-surfaces-allowlist -->

## How to update

1. Add a new write surface to production code (Go, shell, or hook).
2. Add a row in the `## Surfaces` table above explaining owner / lifecycle / purpose.
3. Add the bare subdir name to the allowlist block.
4. Run `scripts/check-agents-write-surfaces.sh` and confirm it exits 0.
5. Add or update a regression test in `tests/scripts/check-agents-write-surfaces.bats` if the new surface introduces a new contract dimension (format, ownership rule, lifecycle).

## See also

- `docs/contracts/repo-execution-profile.md` — repo-local operating policy
- `PROGRAM.md` — autodev mutable/immutable scope
- `scripts/check-wiring-closure.sh` — broader registry-coverage gate
