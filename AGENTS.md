# Agent Instructions

**AgentOps compiles and compounds the context that feeds your software factory.** It automates the bookkeeping agents do not reliably do for themselves — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes. Plugin + CLI + scheduling daemon (hookless — skills + the `ao` CLI, with CI as the authoritative gate), runs on your hardware against your subscription. Humans choose the posture: in-the-loop for high-rigor work, on-the-loop for scheduled compounding.

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

> **Spawning an agent? Run this first:** `ao session bootstrap` (when [soc-vuu6.25](https://github.com/boshu2/agentops/issues?q=soc-vuu6.25) lands) — the universal init prompt that orients every agent identically regardless of model. Until then, follow the read order below.

## Session Start (Zero-Context Agent)

If you spawn into this repo without context, do this first:

1. Read `docs/newcomer-guide.md` first for a practical repo orientation.
2. Open `docs/index.md` (MkDocs landing) then `docs/documentation-index.md` (full catalog) to get the current doc map.
3. Identify your task domain:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, `cli/docs/COMMANDS.md`
   - Skill behavior: `skills/<name>/SKILL.md`
   - Validation/gate behavior: `.github/workflows/validate.yml` + `scripts/*.sh` (AgentOps 3.0 is hookless — CI is the authoritative gate)
   - Validation/release/security flows: `scripts/*.sh` + `tests/`
4. Use source-of-truth precedence when docs disagree:
   1. Executable code and generated artifacts (`cli/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
   2. Skill contracts and manifests (`skills/**/SKILL.md`, `schemas/**`)
   3. Explanatory docs (`docs/**`, `README.md`)
5. If you find conflicts, follow the higher-precedence source and call out the mismatch explicitly in your report/PR.

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

# Other agents (for example Cursor): install only selected skills
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)

# Update all installed skills
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

## Quick Reference

```bash
# Issue tracking
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd vc status          # Inspect Dolt state if needed (JSONL auto-sync is automatic)
bd dolt push          # Only if a real Dolt remote is configured

# CLI development
cd cli && make build  # Build ao binary
cd cli && make test   # Run tests
cd cli && make lint   # Run linter

# Validation (run before pushing)
scripts/pre-push-gate.sh --fast  # Smart conditional gate (only checks relevant to changed files)
bash scripts/install-dev-hooks.sh  # Activate repo-managed git hooks once per clone/worktree
scripts/ci-local-release.sh     # Full local release gate (runs everything)
scripts/validate-go-fast.sh     # Quick Go validation (build + vet + test)
```

## What's where (tiered AGENTS.md split, soc-vuu6.3)

| If you need… | Read | Owner area |
|---|---|---|
| Workflow phases · branch/PR shape · Local Pre-Push · Releasing · Landing the Plane · bd issue tracking · Session Completion | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) | How work flows |
| CI gate detail · Advisory triage SLAs · DEFERRED hardening matrix · per-job descriptions · Nightly workflow jobs | [`AGENTS-CI.md`](AGENTS-CI.md) | What CI checks |
| CLI Skill-Map Refresh · Codex Skill Maintenance · audit scripts · override conventions | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) | Codex parity rules |
| Canonical Root and Worktrees · Key Constraints Agents Must Follow · no-tracked-`.agents` · no-symlinks · embedded-sync | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) | Runtime constraints |

Each file is self-contained for its scope and back-links here for orientation. Authors mutating `AGENTS-*.md` should rerun `scripts/validate-agents-split.sh` to confirm the split contract still holds.

## Previously here, now in <X> (1-release deprecation footer)

The sections below moved out of `AGENTS.md` on 2026-05-20 (soc-vuu6.3). This footer stays for one release cycle so agents that hard-coded section anchors find the new home.

| Old anchor (this file) | New home |
|---|---|
| `## Workflow` and `### Phases / ### Branch + PR shape / ### Multi-agent discipline / ### Provenance / ### Doctrine altitudes / ### Source layer / ### CI tiers` | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md#workflow) |
| `## CI Validation — Passing the Pipeline` and subsections except those listed below | [`AGENTS-CI.md`](AGENTS-CI.md) |
| `### Local Pre-Push Checklist` | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md#local-pre-push-checklist) |
| `### CLI Skill-Map Refresh` and `### Codex Skill Maintenance` | [`AGENTS-CODEX.md`](AGENTS-CODEX.md) |
| `### Canonical Root and Worktrees` and `### Key Constraints Agents Must Follow` | [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) |
| `## Releasing` / `## Landing the Plane (Session Completion)` / `## Issue Tracking with bd (beads)` / `## Session Completion` | [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md) |
