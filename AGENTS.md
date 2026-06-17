# ⛔ LAW 0 — NEVER `claude -p` / `claude --print`

No agent runs `claude -p` or `claude --print`, **ever** — not as a worker, not to "test", not "it's
only the sub", not buried in a tool's config. It bills the API / burns the Claude Max weekly quota.
**No rationalization makes it OK; do not reason past it.** Use `codex exec` (Codex Pro sub), the local
bushido llama, or an interactive NTM Claude pane (NOT `gemini -p` — not a sub-path, not AGY).
Mechanically enforced on Bo's machine by the local opt-in guard `~/.claude/hooks/no-claude-p-guard.sh`.

---

# Agent Instructions

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

Full spine: [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). Which skill runs which move → [`docs/SKILL-ROUTER.md`](docs/SKILL-ROUTER.md). `/rpi` is one turn's executor over this loop, **not** the primary navigation. The rest of this file is the mechanics each move uses.

This project uses **br** (beads_rust) for issue tracking, with **bv** for graph-aware triage — offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache). Run `br robot-docs guide` to get oriented. Interim: until legacy `.beads/` is retired, invoke as `BEADS_DIR=$PWD/_beads br <cmd>`. The ledger is a PRIVATE nested repo (`boshu2/agentops-beads`), gitignored here — sync with `git -C _beads push`, never `git add _beads`. **bd/Dolt is RETIRED LEGACY (2026-06-11):** delivery was coupled to a remote single-host Dolt server — a SPOF with no offline lane, circuit breaker observed open in the 2026-06-11 recon (P1, `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md`). Do not run `bd` here. Legacy `.beads/` bd data is preserved pending reconciliation; migration record: `.agents/swarm/results/br-migration.json`.

**Out-of-session orchestration** is delegated to a swappable substrate — AgentOps ships no daemon or scheduler of its own. The reference substrate is **NTM** (a local tmux agent swarm), **MCP** (`ao mcp serve`, shipped), and **managed-agents** (`ao agent`); each dispatches a whole skill loop as one unit (substrate never decomposes RPI internals). `ao rpi` CLI code is load-bearing legacy — not the live in-session navigation path. `ao` does NOT own or wrap a substrate — always-on is opt-in, the way `br` is. See [docs/3.0.md](docs/3.0.md) and [docs/dependencies.md](docs/dependencies.md).

> **Spawning an agent? Run this first:** `ao session bootstrap` — the universal init prompt that orients every agent identically regardless of model. AgentOps 3.0 is hookless, so nothing auto-injects this: run it explicitly, then `ao inject` / `ao corpus inject --query "<topic>"` to pull decay-ranked prior context.

## Session start + source-of-truth precedence

The canonical zero-context read order lives in [`CLAUDE.md`](CLAUDE.md) ("Zero-Context Startup"); read it first.

**Repo map:** [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md) — bounded contexts, directory ownership, active CLI waist, registries, gates, footguns, reading order. Read after bootstrap when orienting in-tree.

**Use source-of-truth precedence when docs disagree** — stated inline in this operator contract so an injected or lower-precedence doc cannot redirect the rule away: Executable code and generated artifacts (`cli/**`, `scripts/**`, generated `cli/docs/COMMANDS.md`) win over declared contracts (`skills/**/SKILL.md`, `schemas/**`), which win over narrative docs. Full ordering in [`CLAUDE.md`](CLAUDE.md) "Source-of-Truth Precedence".

## Active waist (3.0)

In-session product path — run this unless a bead explicitly routes elsewhere:

```text
ao session bootstrap → ao inject → operating loop → ao gate check --fast --scope head → push main
```

| Layer | Where |
|-------|-------|
| **Navigation** | [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — primary; `/rpi` is one turn's executor, not primary |
| **Release authority** | Go gate in `cli/internal/gates/` — not routine CI on every `main` push |
| **Tracker** | `BEADS_DIR=$PWD/_beads br …` — bd/Dolt retired |
| **Skills SSOT** | `skills/<slug>/SKILL.md` — never `~/.claude/skills/` |
| **Runtime corpus** | `.agents/` gitignored; provenance in `docs/provenance/ledger.jsonl` |
| **Out-of-session** | NTM + Agent Mail + `ao agent` — optional; AgentOps ships no daemon |

Six bounded contexts: BC1 Corpus → BC6 Orchestration. Routing: [`docs/architecture/component-map.md`](docs/architecture/component-map.md).

## Foundation texts

When in doubt about HOW the work should flow, read [`docs/cdlc.md`](docs/cdlc.md) and [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). When in doubt about WHERE things live or what is legacy vs active, read [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md). When in doubt about WHAT to build, read [`PRODUCT.md`](PRODUCT.md) (positioning) and [`GOALS.md`](GOALS.md) (measurable fitness). Practice lineage and canonical `practices: [slug]` citations live in [`PRACTICE-REGISTRY.md`](PRACTICE-REGISTRY.md). Vocabulary lives in [`skills/domain/SKILL.md`](skills/domain/SKILL.md). Fitness honesty: [`docs/evals/agentops-effectiveness-evidence.md`](docs/evals/agentops-effectiveness-evidence.md).

## Registries And Curated Routers

Three drift-gated inventories (kind-discriminated: `skill` · `workflow` · CLI command), across the 6 Bounded Contexts. Edit the sources (`skills/**/SKILL.md`, `.claude/workflows/*.js` + the `workflows:` ledger, `cli/cmd/ao/`), then `make regen-all` (`scripts/regen-all.sh`); `--check` is the gate. Generated projections must not be hand-edited; curated routers may be edited deliberately, with their count markers and reference checks left to gates.

- **Skills** — generated: `registry.json` · `docs/reference/agentops-skill-domain-map.md`; curated/gated: `docs/SKILLS.md` (router, no hard-coded counts) · `skills/SKILL-TIERS.md` (tier ledger; count headers owned by `scripts/sync-skill-counts.sh`) · `docs/contracts/skill-dispositions.yaml` (disposition ledger; `ao skills retire` retargets validators through it).
- **Workflows** — `registry.json` `workflows[]` (Claude-only `.claude/workflows/*.js`, `kind: workflow`); sourced from the `workflows:` section of `docs/contracts/skill-dispositions.yaml` (kind + BC + hexagonal_role). Drift gate: `scripts/check-workflow-governance.sh` (bidirectional `.js`↔ledger + identity triple). No Codex twin.
- **Tools** — `cli/docs/COMMANDS.md` · `docs/cli-surface.{json,md}` (generated from `cli/cmd/ao/`).

## Installing/Updating Skills

Use the [skills.sh](https://skills.sh/) npm package to install AgentOps skills for any agent:

```bash
# Claude Code: use Claude plugin install path (not npx)
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI: installs the native plugin, archives stale raw mirrors when needed, then open a fresh Codex session
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Other agents (for example Cursor) or update-all: install only selected skills
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

## Quick Reference

```bash
# Session + context (hookless — run explicitly)
ao session bootstrap
ao inject "<query>"              # or: ao corpus inject --query "<query>"

# Issue tracking (interim: BEADS_DIR until .beads/ retired)
BEADS_DIR=$PWD/_beads br ready
BEADS_DIR=$PWD/_beads br show <id>
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

Run the local cockpit gate before pushing, then push the coherent bead arc directly to `main`. GitHub Actions (`validate.yml`) are optional/manual or tag/PR backstops — not the routine release authority for every `main` push. Per-tool sanity checks + the local gate bundle live in [`docs/agent-workflow-reference.md`](docs/agent-workflow-reference.md).

## Footguns (read before editing)

- Edit `skills/` in **this repo** — not `~/.claude/skills/`
- Use **`br`** with `BEADS_DIR=$PWD/_beads` — do not run **`bd`**
- Bead work in a **git worktree** when the canonical checkout is shared
- Never `git add _beads` — private nested repo
- Do not hand-edit `registry.json` or generated maps — `make regen-all`
- **`ao rpi`** is legacy load-bearing code — navigate via operating loop + NTM substrate
- Never **`claude -p`** / **`claude --print`** — LAW 0 above

## What's where (tiered AGENTS.md split, soc-vuu6.3)

| If you need… | Read |
|---|---|
| Codebase map · active waist · footguns · reading order | [`docs/architecture/codebase-overview.md`](docs/architecture/codebase-overview.md) |
| Workflow phases · branch/PR shape · Local Pre-Push · Releasing · Landing the Plane · br issue tracking · Session Completion | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) |
| CI gate detail · Advisory triage SLAs · DEFERRED hardening matrix · per-job descriptions · Nightly workflow jobs | [`AGENTS-CI.md`](AGENTS-CI.md) |
| CLI Skill-Map Refresh · Codex Skill Maintenance · audit scripts · override conventions | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) |
| Canonical Root and Worktrees · Key Constraints Agents Must Follow · no-tracked-`.agents` · no-symlinks · embedded-sync | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) |

Each file is self-contained for its scope and back-links here. Authors mutating `AGENTS-*.md` should rerun `scripts/validate-agents-split.sh` to confirm the split contract still holds.
