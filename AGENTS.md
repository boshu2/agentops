# Agent Instructions

**AgentOps compiles and compounds the context that feeds your software factory.** It automates the bookkeeping agents do not reliably do for themselves — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes. Plugin + CLI + scheduling daemon (hookless — skills + the `ao` CLI, with CI as the authoritative gate), runs on your hardware against your subscription. Humans choose the posture: in-the-loop for high-rigor work, on-the-loop for scheduled compounding.

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

**Gas City (`gc`)** is the optional out-of-session orchestration substrate that runs whole `ao rpi`/`ao evolve` loops (mayor + refinery agents over the reference City at `packs/agentops/`). `ao` does NOT wrap `gc` — it is a guided dependency, just like `bd`. See the [`using-gc`](skills/using-gc/SKILL.md) skill and [docs/dependencies.md](docs/dependencies.md).

> **Spawning an agent? Run this first:** `ao session bootstrap` — the universal init prompt that orients every agent identically regardless of model. AgentOps 3.0 is hookless, so nothing auto-injects this: run it explicitly, then `ao inject` / `ao corpus inject --query "<topic>"` to pull decay-ranked prior context.

## Session start + source-of-truth precedence

The canonical zero-context read order and the source-of-truth precedence rule live in [`CLAUDE.md`](CLAUDE.md) (sections "Zero-Context Startup" and "Source-of-Truth Precedence"). Read those first; this file does not duplicate them.

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
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work

# CLI development
cd cli && make build  # Build ao binary
cd cli && make test   # Run tests
cd cli && make lint   # Run linter
```

Push, let CI validate (it is the authoritative gate — no local omnibus gate). Per-tool sanity checks + the full release gate live in [`docs/agent-workflow-reference.md`](docs/agent-workflow-reference.md).

## What's where (tiered AGENTS.md split, soc-vuu6.3)

| If you need… | Read |
|---|---|
| Workflow phases · branch/PR shape · Local Pre-Push · Releasing · Landing the Plane · bd issue tracking · Session Completion | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) |
| CI gate detail · Advisory triage SLAs · DEFERRED hardening matrix · per-job descriptions · Nightly workflow jobs | [`AGENTS-CI.md`](AGENTS-CI.md) |
| CLI Skill-Map Refresh · Codex Skill Maintenance · audit scripts · override conventions | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) |
| Canonical Root and Worktrees · Key Constraints Agents Must Follow · no-tracked-`.agents` · no-symlinks · embedded-sync | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) |

Each file is self-contained for its scope and back-links here. Authors mutating `AGENTS-*.md` should rerun `scripts/validate-agents-split.sh` to confirm the split contract still holds.
