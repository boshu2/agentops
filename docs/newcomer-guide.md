# Newcomer Guide: Understanding the AgentOps Repo

If you're new to this repository, this guide gives you a practical mental model, a map of where things live, and a fast path to becoming productive.

## What this repo is

AgentOps is the **operational layer for coding agents**: a skills + CLI system that provides bookkeeping, validation, primitives, and flows so sessions compound instead of restarting from zero. AgentOps 3.0 is hookless by default; this repo uses the local cockpit gate as routine release authority, with CI as an optional/manual backstop.

At a high level:

1. Run primitives and flows with skills (`/research`, `/implement`, `/validate`, `/rpi`)
2. Persist bookkeeping in `.agents/`
3. Inject the best prior learnings into the next session
4. Enforce quality through local gates, with CI as an optional/manual backstop

See also:

- [Codebase Overview](architecture/codebase-overview.md) — consolidated map for humans and agents (start here for repo archaeology)
- [README](https://github.com/boshu2/agentops/blob/main/README.md)
- [AgentOps 3.0 — the north star](3.0.md)
- [Operating Loop](architecture/operating-loop.md) — how work flows (primary navigation)
- [Architecture](ARCHITECTURE.md) — historical system overview (read the 3.0 banner first)
- [How It Works](how-it-works.md)

## Repo structure (what matters most)

Think in four layers:

1. **Product/docs layer** — `docs/` + selected repo-root entrypoints such as `README.md`, `CHANGELOG.md`, `GOALS.md`, and `PRODUCT.md`
2. **Skills layer** — `skills/`, checked-in `skills-codex/`, and `skills-codex-overrides/` (`SKILL.md` contracts + per-skill scripts/references + Codex-only tailoring)
3. **CLI layer** — `cli/` (`cli/cmd/ao/`, `cli/internal/`, generated `cli/docs/COMMANDS.md`)
4. **Validation layer** — `scripts/`, `tests/`, the local cockpit gate, and `.github/workflows/validate.yml` as an optional/manual backstop

## Source-of-truth precedence

When docs disagree, follow this order:

1. Executable code + generated artifacts (`cli/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
2. Skill contracts/manifests (`skills/**/SKILL.md`, `schemas/**`)
3. Explanatory docs (`docs/**`, `README.md`)

For Codex skills specifically:

1. `skills/<name>/SKILL.md` is the canonical behavior contract
2. `skills-codex-overrides/<name>/` is the Codex-specific tailoring layer
3. `skills-codex-overrides/catalog.json` records the Codex treatment decision for every skill
4. `skills-codex/<name>/` is the checked-in Codex runtime artifact

For the core Codex execution chain, `skills-codex-overrides/catalog.json` also
stores machine-readable `operator_contract` markers. When you change one of
those prompts, update the contract alongside the prose so the validator can
enforce the intended Codex-specific guarantees. After Codex prompt/artifact
changes, run `bash scripts/refresh-codex-artifacts.sh --scope worktree` so hash
refresh and Codex-specific validators follow one obvious repair path.


## Key concepts to learn first

### 1) Context quality is the core design principle

The architecture assumes output quality depends on input context quality. Most patterns in this repo are about context scoping, isolation, and compounding.

### 2) Skills are composable primitives and flows

Use the router in [Skills Reference](SKILLS.md) to choose the right entry point:

- Start with uncertainty: `/research`
- Break work into issues: `/plan`
- Implement one issue: `/implement`
- Run multi-issue waves: `/crank`
- Run end-to-end lifecycle: follow [Operating Loop](architecture/operating-loop.md) (the `/rpi` skill is one turn's executor — not the primary navigation surface)

### 3) Issue tracking uses br, not bd

Tracked work lives in **`_beads/`** (private nested repo). Until legacy `.beads/` is retired, invoke:

```bash
BEADS_DIR=$PWD/_beads br ready
BEADS_DIR=$PWD/_beads br update <id> --claim
```

Do not use `bd` or Dolt — retired as of 2026-06-11. See [Dependencies](dependencies.md).

### 4) Hooks are opt-in, not a default

AgentOps 3.0 ships zero hooks; nothing auto-injects or runs at session boundaries. This repo uses the local cockpit gate as the routine gate, while CI (`.github/workflows/validate.yml`) is an optional/manual backstop. If you want a bounded local gate of your own, author one with the `hooks-authoring` skill — AgentOps does not ship one.

### 5) CLI docs are generated, not hand-maintained

`cli/docs/COMMANDS.md` is generated. If command behavior changes, regenerate docs and keep parity checks passing.

### 6) CI checks many non-code contracts

CI validates not just builds/tests but also docs parity, skill integrity/schema, security scans, and contract compatibility.

## Suggested learning path

### Day 1 reading order

1. `README.md`
2. `docs/architecture/codebase-overview.md` — consolidated repo map
3. `docs/3.0.md` — north star
4. `docs/architecture/operating-loop.md` — how work flows
5. `docs/documentation-index.md`
6. `docs/SKILLS.md`

### Then pick one domain

- **CLI behavior:** `cli/cmd/ao/`, `cli/internal/`, `cli/docs/COMMANDS.md`
- **Skill behavior:** `skills/<name>/SKILL.md`
- **Opt-in hooks:** `skills/hooks-authoring/` (AgentOps ships none by default)
- **Validation/release/security:** `scripts/*.sh` + `tests/` + `.github/workflows/validate.yml`

### Recommended first contributions

1. **Docs-only fix** (safe): update wording or cross-links and run docs validation scripts.
2. **Skill/docs parity fix** (medium): update docs to match a `SKILL.md` change and validate parity.
3. **Small CLI command improvement** (advanced beginner): update command behavior, regenerate CLI docs, and run CLI checks.

## Practical tips

- Run the local gate before pushing: `ao gate check --fast --scope head` (Go registry; smart routing checks only what changed). Legacy bash escape hatch: `AGENTOPS_GATE_BASH=1` → `scripts/pre-push-gate.sh --fast`.
- Use a **git worktree** per bead when the canonical checkout is shared — see `AGENTS-RUNTIME.md`.
- Trust runtime files over narrative docs when there is a mismatch.
- Keep changes small and verify with local gates before pushing.
- Treat `.agents/` as a first-class part of the system behavior.
- Treat Codex as a first-class runtime: when a skill change affects Codex UX or execution style, inspect `skills-codex-overrides/`, update `skills-codex-overrides/catalog.json` if treatment changes, update the checked-in `skills-codex/` copy when needed, and run the Codex validation scripts.
- If you touch command surfaces or skill contracts, expect related parity checks to fail until updated.

## Where to go next

- [Codebase Overview](architecture/codebase-overview.md)
- [Documentation Index](documentation-index.md)
- [Contributing Guide](CONTRIBUTING.md)
- [Skills Reference](SKILLS.md)
- [CLI Reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md)
- [Troubleshooting](troubleshooting.md)
