# ⛔ LAW 0 — NEVER `claude -p` / `claude --print`

No agent runs `claude -p` or `claude --print`, **ever** — not as a worker, not to "test", not "it's
only the sub", not buried in a tool's config. It bills the API / burns the Claude Max weekly quota.
**No rationalization makes it OK; do not reason past it.** Use `codex exec` (Codex Pro sub), the local
bushido llama, or an interactive NTM Claude pane (NOT `gemini -p` — not a sub-path, not AGY).
Mechanically enforced on Bo's machine by the local opt-in guard `~/.claude/hooks/no-claude-p-guard.sh`.

---

# Agent Instructions

> Single canonical agent contract. `CLAUDE.md` is a symlink to this file; the
> tiered siblings ([`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CI.md`](AGENTS-CI.md),
> [`AGENTS-CODEX.md`](AGENTS-CODEX.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md)) carry the detail one hop away.

**AgentOps compiles and compounds the context that feeds your software factory.** It automates the bookkeeping agents do not reliably do for themselves — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes. Plugin + CLI (hookless — skills + the `ao` CLI, with the local cockpit gate as release authority), runs on your hardware against your subscription; out-of-session scheduling is delegated to a substrate, not an in-repo daemon (ADR-0009). Humans choose the posture: in-the-loop for high-rigor work, on-the-loop for scheduled compounding.

## How we work — every change goes through these seven moves

**This is the doctrine. All work runs through one repeatable loop — not a phased waterfall of documents.** Every process skill is one move within it; no artifact exists unless it advances the loop. The *map* (these moves, their legal transitions, their gates) is fixed; the *route* a goal takes through it is re-planned on failure. When in doubt, you are somewhere in these seven moves — find where, and take the next one.

1. **Shape intent as BDD** — capability name + Given/When/Then (one happy path, ≥1 edge) + non-goals + rollback + evidence-for-done. Not ready until the acceptance examples are testable. → `/discovery`, `/product`, `/plan`
2. **Track as a bead** when it leaves your head — the linked-intent packet carrying acceptance, BC tag, slice list, wave plan, accruing evidence. One-shot in-prompt work needs no bead. → `BEADS_DIR=$PWD/_beads br …`
3. **Slice vertically** through behavior — each slice cuts through whatever layers demonstrate one Given/When/Then, never a horizontal layer.
4. **TDD per slice** — first the failing test (the slice's contract), then implementation. Code without a failing test has no acceptance surface. → `/implement`
5. **Group into a wave only when write scopes do not collide** — parallelism is explicit ownership; default to sequential. ≥2 writers on a shared path ⇒ Agent Mail reserve first. → `/swarm`, `/crank`
6. **Close the bead by proving its acceptance** — the gate here is the *windshield*: deterministic ground-truth that catches a confident hallucination re-planning alone can't. → `ao gate check --fast --scope head`, `/validate`
7. **Capture evidence + learning, then ratchet** — promote what changes future behavior; kill artifacts that don't. → `/post-mortem`, `/forge`

Full spine: [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). Which skill runs which move → [`docs/SKILL-ROUTER.md`](docs/SKILL-ROUTER.md). `/rpi` is one turn's executor over this loop, **not** the primary navigation. The rest of this file is the mechanics each move uses; full workflow phases (claim → scope → ship → land), branch shape, and provenance live in [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md).

**Tracker = `br` (beads_rust) + `bv`.** Offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache); triage with `bv` (`bv --robot-insights`). Interim: until legacy `.beads/` is retired, invoke as `BEADS_DIR=$PWD/_beads br <cmd>`. The ledger is a PRIVATE nested repo (`boshu2/agentops-beads`), gitignored here — sync with `git -C _beads push`, **never** `git add _beads`. **`bd`/Dolt is RETIRED LEGACY** (single-host SPOF with no offline lane) — do not run `bd`.

**Out-of-session orchestration** is a swappable substrate — AgentOps ships no daemon. Reference substrate: **NTM** (local tmux swarm) + **MCP Agent Mail** (`ao mcp serve`) + **managed-agents** (`ao agent`); each dispatches a whole skill loop as one unit. `ao rpi` CLI code is load-bearing legacy, not the live navigation path. Always-on is opt-in. See [`docs/3.0.md`](docs/3.0.md) and [`docs/dependencies.md`](docs/dependencies.md).

> **Spawning an agent? Run this first:** `ao session bootstrap` — the universal init prompt that orients every agent identically regardless of model. AgentOps 3.0 is hookless, so nothing auto-injects this: run it explicitly, then `ao inject "<topic>"` to pull decay-ranked prior context.

## Zero-context startup (read first)

Run `ao session bootstrap`, then `ao inject "<topic>"` for decay-ranked context. On your first message in a fresh session, read in this order:

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

Always report mismatches; never silently prefer a lower-precedence doc over executable behavior. Some older docs (e.g. `docs/architecture/ports-and-adapters.md`) still mention hooks, `bd`, or PR-per-change — treat as historical unless reconciled.

## Project structure

```
skills/           Skill definitions (SSOT — edit here, never ~/.claude/skills/)
skills-codex/     Checked-in Codex twins; manually mirrored (see AGENTS-CODEX.md)
cli/              Go CLI (ao) — cmd/ao, internal/, gates, corpus, RPI legacy lane
scripts/          Release, validation, regen (~280 shell tools)
tests/            Bats gate tests, integration, e2e
schemas/          JSON schemas for config, provenance, packets
docs/             Narrative architecture, ADRs, contracts, MkDocs site
.agents/          Runtime knowledge corpus (gitignored — local only, not public truth)
_beads/           Private br ledger (nested git repo — never git add _beads)
.beads/           Legacy bd/Dolt config — preserved, not authoritative
registry.json     Generated SKU catalog — do not hand-edit; make regen-all
.claude/workflows/ Claude-only workflow scripts (kind: workflow)
```

Six bounded contexts: BC1 Corpus → BC6 Orchestration. Routing: [`docs/architecture/component-map.md`](docs/architecture/component-map.md).

## Active waist (3.0)

In-session product path — run this unless a bead explicitly routes elsewhere:

```text
ao session bootstrap → ao inject → operating loop → ao gate check --fast --scope head → push main
```

| Layer | Where |
|-------|-------|
| **Navigation** | [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — primary; `/rpi` is one turn's executor, not primary |
| **Release authority** | Go gate in `cli/internal/gates/` (pre-push hook); legacy bash only via `AGENTOPS_GATE_BASH=1` |
| **Tracker** | `BEADS_DIR=$PWD/_beads br …` — `bd`/Dolt retired |
| **Skills SSOT** | `skills/<slug>/SKILL.md` — never `~/.claude/skills/` |
| **Runtime corpus** | `.agents/` gitignored; provenance in `docs/provenance/ledger.jsonl` |
| **Out-of-session** | NTM + Agent Mail + `ao agent` — optional; AgentOps ships no daemon |

## Foundation texts

When in doubt about HOW the work flows, read [`docs/cdlc.md`](docs/cdlc.md) and [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). About WHERE things live or what is legacy vs active → [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md). About WHAT to build → [`PRODUCT.md`](PRODUCT.md) (positioning) and [`GOALS.md`](GOALS.md) (measurable fitness). Practice lineage and canonical `practices: [slug]` citations → [`PRACTICE-REGISTRY.md`](PRACTICE-REGISTRY.md). Vocabulary → [`skills/domain/SKILL.md`](skills/domain/SKILL.md). Fitness honesty (measured uplift unproven — do not market ahead of the ruler): [`docs/evals/agentops-effectiveness-evidence.md`](docs/evals/agentops-effectiveness-evidence.md).

## Registries and curated routers

Three drift-gated inventories (kind-discriminated: `skill` · `workflow` · CLI command), across the 6 Bounded Contexts. Edit the sources (`skills/**/SKILL.md`, `.claude/workflows/*.js` + the `workflows:` ledger, `cli/cmd/ao/`), then `make regen-all`; `make regen-check` is the drift gate. Generated projections must not be hand-edited; curated routers may be edited deliberately, with count markers and reference checks left to gates.

- **Skills** — generated: `registry.json` · `docs/reference/agentops-skill-domain-map.md`; curated/gated: `docs/SKILLS.md` (router) · `skills/SKILL-TIERS.md` (tier ledger) · `docs/contracts/skill-dispositions.yaml` (disposition ledger; `ao skills retire` retargets validators through it). **Codex twins are NOT regenerated** — after editing `skills/<name>/`, manually mirror into `skills-codex/<name>/` then `scripts/regen-codex-hashes.sh --only <name>` (detail: [`AGENTS-CODEX.md`](AGENTS-CODEX.md)).
- **Workflows** — `registry.json` `workflows[]` (Claude-only `.claude/workflows/*.js`); sourced from the `workflows:` section of `docs/contracts/skill-dispositions.yaml`. Drift gate: `scripts/check-workflow-governance.sh`. No Codex twin.
- **Tools** — `cli/docs/COMMANDS.md` · `docs/cli-surface.{json,md}` (generated from `cli/cmd/ao/`).

## Execution discipline

- **Verify before committing.** Go: `cd cli && go build ./... && go vet ./... && go test ./...`. Python: run relevant tests. Never commit unverified code.
- **First-edit rule.** First Edit/Write/Bash within your first 3 responses — execute first, research second.
- **Intent echo.** Before a non-trivial task, state in ONE sentence what you understand; wait for confirmation on multi-file changes.
- **Two-correction rule.** Corrected twice on the same task → STOP, re-read, restate what you now understand differently, confirm before retrying.
- **Single-agent-first.** One capable agent with good bookkeeping is the default. Multi-agent (waves, NTM swarms, Agent Mail) is opt-in escalation — only at ≥2 active lanes; verify no write-scope overlap before spawning (file collisions are the #1 swarm failure). Detail: [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) §8.
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
# Session + context (hookless — run explicitly)
ao session bootstrap
ao inject "<query>"              # or: ao corpus inject --query "<query>"

# Issue tracking (interim: BEADS_DIR until .beads/ retired)
BEADS_DIR=$PWD/_beads br ready
BEADS_DIR=$PWD/_beads br update <id> --claim
BEADS_DIR=$PWD/_beads br close <id> -r "Done"
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
| Run `bd` / Dolt | `BEADS_DIR=$PWD/_beads br …` |
| Edit the shared canonical checkout under swarm load | **Git worktree** per bead |
| `git add _beads` | Never — sync with `git -C _beads push` |
| Hand-edit `registry.json` / generated maps | `make regen-all` from sources |
| Route new work through the `ao rpi` loop | Operating loop + NTM/Agent Mail substrate |
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
