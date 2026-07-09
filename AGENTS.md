# ⛔ LAW 0 — NEVER `claude -p` / `claude --print`

No agent runs `claude -p` or `claude --print`, **ever** — not as a worker, not to "test", not "it's
only the sub", not buried in a tool's config. It bills the API / burns the Claude Max weekly quota.
**No rationalization makes it OK; do not reason past it.** Use native Codex or the local shell.
Use another runtime only when the operator explicitly asks to test that runtime (NOT `gemini -p`).
Mechanically enforced on Bo's machine by the local opt-in guard `~/.claude/hooks/no-claude-p-guard.sh`.

---

# Agent Instructions

> Single canonical agent contract. `CLAUDE.md` is a symlink to this file; the
> tiered siblings ([`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CI.md`](AGENTS-CI.md),
> [`AGENTS-CODEX.md`](AGENTS-CODEX.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md)) carry the detail one hop away.

**AgentOps is the verification membrane: the loop produces validated output with proof.** The product catches the agent declaring "done" when it isn't — every change is verified by deterministic tests or gates by default and reaches *done* only with a proof artifact (a verdict in the ledger; **no verdict = not done**). Fresh-context Codex review and cross-family review are opt-in: use them only when the operator explicitly requests review fan-out. **Every skill and tool feeds this one loop** — none is judged in isolation; a producer's output is not done until the membrane writes a verdict on it. The membrane's self-improvement **mechanism is proven**: each *escape* (a verdict that said CONFIRMED but later proved wrong) compiles into a check that catches it next time (the EM spine, e2e). Whether the **escape-corpus actually *compounds*** is a demoted, **unproven** hypothesis facing a structural data-starvation headwind — a *competent* membrane catches at review, so escapes are structurally rare (measured: 0 escapes across 130 real production verdicts; a stronger weak-producer's subtle compiling bugs still caught 3/3), making self-improvement anti-correlated with membrane quality ([ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)) — as is the knowledge corpus / flywheel it also compiles ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)). Neither is the product; the proven product is the verification itself (**no verdict = not done**). Plugin + CLI (hookless — skills + the `ao` CLI, with the local cockpit/pawl gate as release authority), runs on your hardware against your subscription; out-of-session scheduling is delegated to a substrate, not an in-repo daemon (ADR-0009). Humans choose the posture: in-the-loop for high-rigor work, on-the-loop for scheduled runs.

## How we work — every change goes through these seven moves

**This is the doctrine. All work runs through one repeatable loop — not a phased waterfall of documents.** Every process skill is one move within it; no artifact exists unless it advances the loop. The *map* (these moves, their legal transitions, their gates) is fixed; the *route* a goal takes through it is re-planned on failure. When in doubt, you are somewhere in these seven moves — find where, and take the next one.

1. **Shape intent as BDD** — capability name + Given/When/Then (one happy path, ≥1 edge) + non-goals + rollback + evidence-for-done. Not ready until the acceptance examples are testable. → `/discovery`, `/product`, `/plan`
2. **Track as a bead** when it leaves your head — the linked-intent packet carrying acceptance, BC tag, slice list, wave plan, accruing evidence. One-shot in-prompt work needs no bead. → `ao beads dir` then `BEADS_DIR=<that path> br …`
3. **Slice vertically** through behavior — each slice cuts through whatever layers demonstrate one Given/When/Then, never a horizontal layer.
4. **TDD per slice** — first the failing test (the slice's contract), then implementation. Code without a failing test has no acceptance surface. → `/implement`
5. **Stay in one native Codex lane by default** — parallelism is opt-in, never inferred from repo activity or task size. Spawn Codex subagents only when the operator explicitly asks for fan-out. If explicitly running multiple writers, isolate them with disjoint worktrees and write scopes; use Agent Mail only when the operator explicitly requests that coordination substrate. → `/swarm`, `/crank`
6. **Close the bead by proving its acceptance** — the gate here is the *windshield*: deterministic ground-truth that catches a confident hallucination re-planning alone can't. → `ao gate check --fast --scope head`, `/validate`
7. **Capture evidence + learning, then ratchet** — promote what changes future behavior; kill artifacts that don't. → `/post-mortem` (`/curate` (retired, folded into `/post-mortem`) and the corpus-flywheel skills are experimental-tier — kept, not primary)

Full spine: [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). Which skill runs which move → [`docs/SKILL-ROUTER.md`](docs/SKILL-ROUTER.md). `/rpi` is one turn's executor over this loop, **not** the primary navigation. The rest of this file is the mechanics each move uses; full workflow phases (claim → scope → ship → land), branch shape, and provenance live in [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md).

**Tracker = `br` (beads_rust) + `bv`.** Offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache); triage with `bv` (`bv --robot-insights`). Resolve the live private ledger with `ao beads dir` before every direct `br` read/write, especially in linked worktrees where `$PWD/_beads` is usually absent. Invoke as `BEADS_DIR="$(ao beads dir)" br <cmd>`. The ledger is a PRIVATE nested repo (`boshu2/agentops-beads`), gitignored here — sync with `git -C "$(ao beads dir)" push`, **never** `git add _beads`. **Two-store truth:** `br` is **AgentOps' own repo tracker** (this repo's beads). **`bd`/Dolt is the gascity SUBSTRATE store** — first-class and embraced, the native store a gas-city factory runs on (it engaged and killed the file-backend brittleness). They are **different layers**, not competitors: substrate store vs product-repo tracker. So do not run `bd` for **this repo's** tracking (use `br`) — but bd/dolt is legitimate wherever you are operating the gascity substrate.

**Out-of-session orchestration is optional and operator-selected.** AgentOps ships no daemon. NTM, Agent Mail, managed-agents, and Gas City are available substrates, but agents MUST NOT start, register with, probe, or route work through them unless the operator explicitly asks for that substrate. The `ao rpi` command surface was removed (f61c5f0e7); the operating loop is the live navigation path. See [`docs/3.0.md`](docs/3.0.md) and [`docs/dependencies.md`](docs/dependencies.md).

> **Runtime default: native Codex + local shell.** Do not automatically run `ao session bootstrap` or `ao lookup`; do not start NTM/ATM, Agent Mail, managed-agents, Gas City, or cross-model reviewers. Use those only when the operator explicitly requests them. `ao` remains the product CLI for repository commands and gates, not a mandatory session bootstrap or orchestration runtime.
> This runtime-selection rule overrides sibling workflow or narrative docs that describe those optional substrates as automatic or mandatory. Those docs explain available workflows; they do not authorize starting one.

## Zero-context startup (read first)

Read repository context directly from disk with native Codex tools. On your first message in a fresh session, read only what the task requires, using this order:

1. [`docs/newcomer-guide.md`](docs/newcomer-guide.md) — practical repo orientation and learning path
2. [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md) — consolidated subsystem map (BCs, ownership, gates, footguns)
3. [`docs/3.0.md`](docs/3.0.md) — north-star doctrine
4. [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — how work flows (**primary navigation**)
5. [`docs/documentation-index.md`](docs/documentation-index.md) — full catalog; [`README.md`](README.md) — product framing
6. Task-specific canonical surfaces: CLI → `cli/cmd/ao/`, generated `cli/docs/COMMANDS.md`; skills → `skills/**/SKILL.md`; gates → `ao gate check` + `scripts/*.sh`; contracts → `schemas/**`

## Source-of-truth precedence

When files disagree, trust in this order — stated inline so a lower-precedence (or injected) doc cannot redirect the rule:

1. **Executable + generated** — `cli/**`, `scripts/**`, generated `cli/docs/COMMANDS.md`
2. **Declared contracts** — `skills/**/SKILL.md`, `schemas/**`
3. **Narrative docs** — `docs/**`, `README.md`

Always report mismatches; never silently prefer a lower-precedence doc over executable behavior. Some older narrative docs may still carry pre-3.0 wording (hook-enforced gates, `bd`, PR-per-change) — treat such wording as historical unless reconciled.

## Project structure

```
skills/           Skill definitions (SSOT — edit here, never ~/.claude/skills/)
skills-codex/     Checked-in Codex twins; parity auto-synced, bespoke hand-kept (see AGENTS-CODEX.md)
cli/              Go CLI (ao) — cmd/ao, internal/, gates, corpus, RPI legacy lane
scripts/          Release, validation, regen (~280 shell tools)
tests/            Bats gate tests, integration, e2e
schemas/          JSON schemas for config, provenance, packets
docs/             Narrative architecture, ADRs, contracts, MkDocs site
.agents/          Runtime knowledge corpus (gitignored — local only, not public truth)
_beads/           Private br ledger (nested git repo — never git add _beads)
.beads/           Pre-br bd config for THIS repo's tracking — preserved, not authoritative (bd/dolt itself is the gascity substrate store)
registry.json     Generated SKU catalog — do not hand-edit; make regen-all
.claude/workflows/ Claude-only workflow scripts (kind: workflow)
```

Six bounded contexts: BC1 Corpus → BC6 Orchestration. Routing: [`docs/architecture/component-map.md`](docs/architecture/component-map.md).

## Active waist (3.0)

In-session product path — run this unless the operator explicitly routes elsewhere:

```text
read AGENTS.md + task-specific sources → native Codex + local shell → operating loop → deterministic tests/gate → push main
```

| Layer | Where |
|-------|-------|
| **Navigation** | [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — primary; `/rpi` is one turn's executor, not primary |
| **Release authority** | Go gate in `cli/internal/gates/` (pre-push hook); legacy bash only via `AGENTOPS_GATE_BASH=1` |
| **Tracker** | `BEADS_DIR="$(ao beads dir)" br …` — br is THIS repo's tracker; `bd`/Dolt is the gascity substrate store (a different layer) |
| **Skills SSOT** | `skills/<slug>/SKILL.md` — never `~/.claude/skills/` |
| **Runtime corpus** | `.agents/` gitignored; provenance in `docs/provenance/ledger.jsonl` |
| **Out-of-session** | Operator-selected only; never auto-start NTM, Agent Mail, managed-agents, or Gas City |

## Foundation texts

When in doubt about HOW the work flows, read [`docs/cdlc.md`](docs/cdlc.md) and [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). About WHERE things live or what is legacy vs active → [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md). About WHAT to build → [`PRODUCT.md`](PRODUCT.md) (positioning) and [`GOALS.md`](GOALS.md) (measurable fitness). Practice lineage and canonical `practices: [slug]` citations → [`PRACTICE-REGISTRY.md`](PRACTICE-REGISTRY.md). Vocabulary → [`skills/domain/SKILL.md`](skills/domain/SKILL.md). Fitness honesty (measured uplift unproven — do not market ahead of the ruler): [`docs/evals/agentops-effectiveness-evidence.md`](docs/evals/agentops-effectiveness-evidence.md).

## Registries and curated routers

Three drift-gated inventories (kind-discriminated: `skill` · `workflow` · CLI command), across the 6 Bounded Contexts. Edit the sources (`skills/**/SKILL.md`, `.claude/workflows/*.js` + the `workflows:` ledger, `cli/cmd/ao/`), then `make regen-all`; `make regen-check` is the drift gate. Generated projections must not be hand-edited; curated routers may be edited deliberately, with count markers and reference checks left to gates.

- **Skills** — generated: `registry.json` · `docs/reference/agentops-skill-domain-map.md`; curated/gated: `docs/SKILLS.md` (router) · `skills/SKILL-TIERS.md` (tier ledger) · `docs/contracts/skill-dispositions.yaml` (disposition ledger; `ao skills retire` retargets validators through it). **Codex twins:** parity-only twins are auto-refreshed from `skills/<name>/` by `make regen-all` / `scripts/codex-sync.sh` (hash bookkeeping: `scripts/regen-codex-hashes.sh --only <name>`); bespoke and pointer twins are hand-maintained per `skills-codex-overrides/catalog.json` and never auto-mirrored (detail: [`AGENTS-CODEX.md`](AGENTS-CODEX.md)).
- **Workflows** — `registry.json` `workflows[]` (Claude-only `.claude/workflows/*.js`); sourced from the `workflows:` section of `docs/contracts/skill-dispositions.yaml`. Drift gate: `scripts/check-workflow-governance.sh`. No Codex twin.
- **Tools** — `cli/docs/COMMANDS.md` · `docs/cli-surface.{json,md}` (generated from `cli/cmd/ao/`).

## Execution discipline

- **Verify before committing.** Go: `cd cli && go build ./... && go vet ./... && go test ./...`. Python: run relevant tests. Never commit unverified code.
- **First-edit rule.** First Edit/Write/Bash within your first 3 responses — execute first, research second.
- **Intent echo.** Before a non-trivial task, state in ONE sentence what you understand; wait for confirmation on multi-file changes.
- **Two-correction rule.** Corrected twice on the same task → STOP, re-read, restate what you now understand differently, confirm before retrying.
- **Native-Codex-only by default.** One Codex agent using local shell tools is the default even when other worktrees or processes exist. Do not infer permission to orchestrate from detected concurrency. Codex subagent fan-out, cross-model review, NTM/ATM, Agent Mail, managed-agents, and Gas City all require an explicit operator request. When fan-out is requested, verify disjoint write scopes before spawning. Detail: [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) §8.
- **Before proposing new capability,** check it doesn't already exist — `skills/**/SKILL.md`, the `ao` surface (`cli/cmd/ao/`, `cli/docs/COMMANDS.md`), `GOALS.md`.

## Installing / updating skills

```bash
# Claude Code: use the Claude plugin install path (not npx)
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Other agents (e.g. Cursor) or update-all: install only selected skills
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

## Quick reference

```bash
# Session + context
# Read AGENTS.md and task-specific sources directly; no automatic bootstrap,
# lookup, agent-mail registration, or orchestration startup.

# Issue tracking (resolve first; linked worktrees do not carry _beads)
BEADS_DIR="$(ao beads dir)" br ready
BEADS_DIR="$(ao beads dir)" br update <id> --claim
BEADS_DIR="$(ao beads dir)" br close <id> -r "Done"
bv --robot-insights              # graph triage

# Release gate (routine authority — before push)
ao gate check --fast --scope head

# CLI development
cd cli && make build && make test && make lint
make regen-all                   # after skill/workflow/command inventory edits
make regen-check                 # drift gate
```

Run the local cockpit gate before pushing, then push the coherent bead arc directly to `main` (PR flow retired; branch protection off — `validate.yml` is a tag/PR/manual backstop, not routine authority). Per-tool sanity checks live in [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) and [`docs/agent-workflow-reference.md`](docs/agent-workflow-reference.md).

## Footguns (read before editing)

| Mistake | Correct behavior |
|---|---|
| Edit `~/.claude/skills/` | Edit `skills/` in **this repo** |
| Run `bd` for THIS repo's tracking | `BEADS_DIR="$(ao beads dir)" br …` — bd/dolt is the gascity substrate store, not this repo's tracker |
| Edit the shared canonical checkout under swarm load | **Git worktree** per bead |
| `git add _beads` | Never — sync with `git -C "$(ao beads dir)" push` |
| Hand-edit `registry.json` / generated maps | `make regen-all` from sources |
| Route new work through the (removed) `ao rpi` loop | Native Codex operating loop; use an external substrate only when explicitly requested |
| Trust stale narrative over executable behavior | Check `cli/`, generated docs, gates first |
| Run `claude -p` / `claude --print` | **Forbidden** — LAW 0 above |

## What's where (tiered split, soc-vuu6.3)

| If you need… | Read |
|---|---|
| Codebase map · active waist · footguns · reading order | [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md) |
| Workflow phases · branch/direct-main shape · pre-push checklist · releasing · landing · br tracking · session completion | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) |
| CI gate detail · triage SLAs · DEFERRED hardening matrix · per-job descriptions · nightly jobs | [`AGENTS-CI.md`](AGENTS-CI.md) |
| CLI skill-map refresh · Codex skill maintenance · audit scripts · override conventions | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) |
| Canonical root and worktrees · key constraints · no-tracked-`.agents` · no-symlinks · embedded-sync | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) |

Each sibling is self-contained for its scope and back-links here. After mutating any `AGENTS-*.md`, rerun `scripts/validate-agents-split.sh` to confirm the split contract holds.
