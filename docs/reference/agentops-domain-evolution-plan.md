# AgentOps Domain Evolution Plan

This plan is the bridge between the BDD contract and future `/evolve` runs. It
does not authorize bulk rewrites. It creates the control plane that lets the
catalog be improved one vertical slice at a time.

AgentOps 3.0 is the target: an SDLC control plane and context compiler for LLM
agents. BDD/Gherkin expresses intent as observable behavior; DDD gives shared
names and bounded contexts; Hexagonal Architecture keeps runtime adapters out of
the core; TDD proves done locally; XP keeps slices small; CI/SRE/ADRs/provenance
make trust and memory repeatable.

## Inputs Used

| Input | Status |
|---|---|
| `soc-y5vh` | Live bead read; epic is 8/9 complete with `soc-y5vh.8` in progress. |
| `.agents/research/soc-y5vh.8-ao-loop-hypothesis-converged.md` | Not present in this working tree. |
| `.agents/plans/2026-05-16-ao-loop-hypothesis-converged.md` | Not present in this working tree. |
| `origin/main:PRODUCT.md` | Read after `git fetch`; product direction has moved ahead of local main. |
| `origin/main:GOALS.md` | Read after `git fetch`; Directive 12 is the governing loop-shape rule. |
| `origin/main:PROGRAM.md` | Read after `git fetch`; defines mutable scope and vertical-slice policy. |
| `origin/main:skills/evolve/SKILL.md` | Read after `git fetch`; includes current v2 `ao evolve` and loop-port direction. |
| `docs/plans/2026-05-12-rescope-evolve-and-architecture.md` | Read from `origin/main`; defines BC1-BC5 and port waves. |

Local freshness note: `git fetch` found local `main` diverged from
`origin/main` by 4 local-only commits and 3 remote-only commits. This plan uses
`origin/main` direction sources without merging, rebasing, or resetting the
dirty working tree.

## Execution Strategy

### Phase 0: Control Plane

Status: this patch.

- Write the Gherkin BDD acceptance contract.
- Map all 77 checked-in skills into domains.
- Create the hexagonal architecture target.
- Add a checker that proves every skill appears in the domain map.
- Bootstrap a local Codex skill that can orchestrate this program as a
  context-compiler evolution cycle, not a skill-pile rewrite.

### Phase 1: Local Bootstrap Proving

Use `/Users/bo/.codex/skills/agentops-evolution-bootstrap` and
`/Users/bo/.codex/skills/agentops-skill-factory`.

Done when:

- the local bootstrap skill validates with Codex `skill-creator`,
- it can regenerate or inspect the BDD/domain/architecture/control docs,
- it can score any AgentOps skill and choose the smallest next patch,
- it refuses JSM content copying and JSM mutation.

### Phase 1.5: CLI Orchestration Rehearsal

Use the `ao` CLI as the runner, but only after preflight proves the tree and
binary are safe.

Preflight:

```bash
git fetch --prune origin
git rev-list --left-right --count HEAD...origin/main
git status --short
bash scripts/check-worktree-disposition.sh
cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao autodev validate --file ../PROGRAM.md --json
ao evolve --help
ao rpi loop --help
ao loop --help 2>/dev/null || true
cd cli && go run ./cmd/ao loop --help
```

Current hazard: source may expose `ao loop append/history/verify` while the
installed `/Users/bo/go/bin/ao` does not. Do not run unattended against a stale
installed binary when the selected slice needs the newer CLI surface.

Rehearsal command:

```bash
ao factory start --goal "AgentOps 3.0 domain evolution"
ao evolve --dry-run --max-cycles 1 --repo-filter agentops --landing-policy off
```

First real command, after rehearsal:

```bash
ao evolve "Land BC3 Loop slice for soc-y5vh.8" \
  --max-cycles 1 \
  --repo-filter agentops \
  --lease \
  --ensure-cleanup \
  --auto-clean \
  --gate-policy best-effort \
  --landing-policy off
```

Landing stays manual or `/push`-driven until one unattended cycle is reviewed.
Only use `--landing-policy commit` or `--landing-policy sync-push` from a clean
synced task worktree with explicit operator authorization.

### Phase 2: Loop Spine Upgrade

Upgrade the highest-leverage BC3 skills first:

1. `evolve`
2. `rpi`
3. `discovery`
4. `plan`
5. `crank`
6. `validation`
7. `post-mortem`
8. `ratchet`

Each skill gets one small patch per cycle: usually `SELF-TEST.md`, sharper
trigger boundaries, or a reference split. For `evolve`, do not settle for text:
the repo implementation must follow `soc-y5vh.8` through typed loop ports.

### Phase 3: Factory Spine Upgrade

Upgrade BC4 skills so future skills scaffold to the new standard by default:

1. `skill-builder`
2. `skill-auditor`
3. `heal-skill`
4. `standards`
5. `converter`
6. `bootstrap`

Expected result: new and updated skills include domain metadata, self-tests,
small references, validation commands, and productization boundaries.

### Phase 4: Corpus, Validation, and Runtime Waves

Run domain-local waves only when write scopes are disjoint:

- BC1 Corpus: `compile`, `inject`, `flywheel`, `forge`, `harvest`, `dream`
- BC2 Validation: `council`, `vibe`, `pre-mortem`, `test`, `review`,
  `security-suite`, `release`
- BC5 Runtime: `hooks-authoring`, `scope`, `push`, `swarm`, `codex-team`

Candidate merge/cut reviews happen after core spines are stable. Do not delete
skills until a replacement workflow, migration note, and validation result exist.

## Per-Skill Evolution Loop

For each skill:

1. Read the skill and domain-map row.
2. Score it with the local skill factory.
3. Write or select one Gherkin acceptance row.
4. Choose one action: keep, update, refactor, merge-review, or cut-review.
5. Apply the smallest patch that improves the action's evidence.
6. Run skill-local validation plus the domain-evolution checker.
7. Record remaining gaps and move to the next skill.

## CLI and Hook Extension

After the skill catalog is domain-mapped, apply the same loop to the CLI and
hooks:

- CLI commands map to one bounded context and one port surface.
- Hooks map to Runtime adapters and Validation gates.
- Scripts map to either gate adapters, corpus adapters, or loop mechanics.
- New shell-only read paths are rejected when a typed port already exists.

## Stop Conditions

The evolution program is complete when:

- all skills have a domain, disposition, and validation evidence,
- all loop-spine skills have self-tests,
- `soc-y5vh.8` is closed with typed Loop-port acceptance evidence,
- merge/cut candidates have explicit replacement decisions,
- CLI and hooks have the same BC ownership map,
- local validation passes without relying on hidden `.agents` state.
