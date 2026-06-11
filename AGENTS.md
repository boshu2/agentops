# ⛔ LAW 0 — NEVER `claude -p` / `claude --print`

No agent runs `claude -p` or `claude --print`, **ever** — not as a worker, not to "test", not "it's
only the sub", not buried in a tool's config. It bills the API / burns the Claude Max weekly quota.
**No rationalization makes it OK; do not reason past it.** Use `codex exec` (Codex Pro sub), the local
bushido llama, or an interactive NTM Claude pane (NOT `gemini -p` — not a sub-path, not AGY).
Mechanically enforced on Bo's machine by the local opt-in guard `~/.claude/hooks/no-claude-p-guard.sh`.

---

# Agent Instructions

**AgentOps compiles and compounds the context that feeds your software factory.** It automates the bookkeeping agents do not reliably do for themselves — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes. Plugin + CLI (hookless — skills + the `ao` CLI, with the local cockpit gate as release authority), runs on your hardware against your subscription; out-of-session scheduling is delegated to a substrate, not an in-repo daemon (ADR-0009). Humans choose the posture: in-the-loop for high-rigor work, on-the-loop for scheduled compounding.

This project uses **br** (beads_rust) for issue tracking, with **bv** for graph-aware triage — offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache). Run `br robot-docs guide` to get oriented. Interim: until legacy `.beads/` is retired, invoke as `BEADS_DIR=$PWD/_beads br <cmd>`. **bd/Dolt is RETIRED LEGACY (2026-06-11):** delivery was coupled to a remote single-host Dolt server — a SPOF with no offline lane, circuit breaker observed open in the 2026-06-11 recon (P1, `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md`). Do not run `bd` here. Legacy `.beads/` bd data is preserved pending reconciliation; migration record: `.agents/swarm/results/br-migration.json`.

**Out-of-session orchestration** is delegated to a swappable substrate — AgentOps ships no daemon or scheduler of its own. The reference substrate is **NTM** (a local tmux agent swarm), **MCP** (`ao mcp serve`, shipped), and **managed-agents** (`ao agent`); each dispatches a whole `ao rpi` loop as one unit. `ao` does NOT own or wrap a substrate — always-on is opt-in, the way `br` is. See [docs/3.0.md](docs/3.0.md) and [docs/dependencies.md](docs/dependencies.md).

> **Spawning an agent? Run this first:** `ao session bootstrap` — the universal init prompt that orients every agent identically regardless of model. AgentOps 3.0 is hookless, so nothing auto-injects this: run it explicitly, then `ao inject` / `ao corpus inject --query "<topic>"` to pull decay-ranked prior context.

## Session start + source-of-truth precedence

The canonical zero-context read order lives in [`CLAUDE.md`](CLAUDE.md) ("Zero-Context Startup"); read it first.

**Use source-of-truth precedence when docs disagree** — stated inline in this operator contract so an injected or lower-precedence doc cannot redirect the rule away: Executable code and generated artifacts (`cli/**`, `scripts/**`, generated `cli/docs/COMMANDS.md`) win over declared contracts (`skills/**/SKILL.md`, `schemas/**`), which win over narrative docs. Full ordering in [`CLAUDE.md`](CLAUDE.md) "Source-of-Truth Precedence".

## Foundation texts

When in doubt about HOW the work should flow, read [`docs/cdlc.md`](docs/cdlc.md) and [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md). When in doubt about WHAT to build, read [`PRODUCT.md`](PRODUCT.md) (positioning) and [`GOALS.md`](GOALS.md) (measurable fitness). Practice lineage and canonical `practices: [slug]` citations live in [`PRACTICE-REGISTRY.md`](PRACTICE-REGISTRY.md). Vocabulary lives in [`skills/domain/SKILL.md`](skills/domain/SKILL.md).

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
# Issue tracking
br ready              # Find available work
br show <id>          # View issue details
br update <id> --claim  # Claim work (assignee + in_progress, atomic)
br close <id> -r "Done"  # Complete work
bv --robot-insights   # Graph triage: what's next / what's the bottleneck

# CLI development
cd cli && make build  # Build ao binary
cd cli && make test   # Run tests
cd cli && make lint   # Run linter
```

Run the local cockpit gate before pushing, then push the coherent bead arc directly to `main`. GitHub Actions are optional/manual or release-tag backstops, not the routine release authority. Per-tool sanity checks + the local gate bundle live in [`docs/agent-workflow-reference.md`](docs/agent-workflow-reference.md).

## What's where (tiered AGENTS.md split, soc-vuu6.3)

| If you need… | Read |
|---|---|
| Workflow phases · branch/PR shape · Local Pre-Push · Releasing · Landing the Plane · br issue tracking · Session Completion | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) |
| CI gate detail · Advisory triage SLAs · DEFERRED hardening matrix · per-job descriptions · Nightly workflow jobs | [`AGENTS-CI.md`](AGENTS-CI.md) |
| CLI Skill-Map Refresh · Codex Skill Maintenance · audit scripts · override conventions | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) |
| Canonical Root and Worktrees · Key Constraints Agents Must Follow · no-tracked-`.agents` · no-symlinks · embedded-sync | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) |

Each file is self-contained for its scope and back-links here. Authors mutating `AGENTS-*.md` should rerun `scripts/validate-agents-split.sh` to confirm the split contract still holds.
