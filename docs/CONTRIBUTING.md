# Contributing to AgentOps

AgentOps is the operational layer for coding agents. Contributions are welcome across docs, runtime behavior, CLI reliability, tests, and skills.

If you want the fastest path to a meaningful first contribution, start here:

- [Create Your First Skill](create-your-first-skill.md)
- [SKILL-API.md](SKILL-API.md)
- [testing-skills.md](testing-skills.md)

## Getting Started

### Prerequisites

- GitHub account
- Git
- A supported runtime if you want to test installs end to end (`Codex`, `Claude Code`, `OpenCode`, or another skill-install target)
- Comfort editing Markdown and YAML frontmatter

### Fork and Clone

```bash
git clone https://github.com/YOUR_USERNAME/agentops.git
cd agentops

# Optional, but useful for local workflow testing
bash scripts/install-dev-hooks.sh
```

## High-Leverage Ways To Contribute

You do not need to add a brand-new skill to make a good contribution.

Useful contribution paths:

- Docs clarity: tighten README, guides, examples, or onboarding
- Skill quality: improve an existing `SKILL.md`, references, or validation scripts
- Runtime reliability: hooks, install paths, and lifecycle behavior
- CLI ergonomics: help text, JSON output, workflow plumbing
- Validation and CI: tests, parity checks, and failure-proofing

## First Contribution In 30 Minutes

1. Pick one narrow improvement: a typo, broken link, unclear instruction, or stale command example.
2. Create a branch: `git checkout -b docs/first-contribution-fix`
3. Make the change.
4. Run the narrowest relevant check.
5. Open a PR with before/after context.

## Adding A Skill

Use [Create Your First Skill](create-your-first-skill.md) for the full walk-through.

The short version:

1. Create `skills/your-skill-name/`.
2. Add a current `SKILL.md` with `skill_api_version: 1`.
3. Keep the entry point lean; put deeper material in `references/` only when needed.
4. Link every `references/*.md` file from `SKILL.md`.
5. Run the local gates before opening the PR.

## Local Validation

At minimum, run:

```bash
# Skill structure and reference integrity
bash skills/heal-skill/scripts/heal.sh --strict

# Docs, links, and skill-count consistency
bash tests/docs/validate-doc-release.sh
```

**If you add or remove a skill directory, you must run:**

```bash
scripts/sync-skill-counts.sh
```

This updates the skill count across `SKILL-TIERS.md`, `PRODUCT.md`, `README.md`, `docs/SKILLS.md`, `docs/ARCHITECTURE.md`, and `using-agentops/SKILL.md`. The `doc-release-gate` CI job fails if counts drift, so skipping this step will block your PR. If you're unsure whether your change affects counts, run the script anyway — it's idempotent when counts are already in sync.

If you touched Codex-facing behavior or checked-in Codex artifacts, also run:

```bash
bash scripts/audit-codex-parity.sh --skill your-skill-name
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

Before pushing, the recommended fast gate is:

```bash
scripts/pre-push-gate.sh --fast
```

### Working On The Docs Site

The published site at [boshu2.github.io/agentops](https://boshu2.github.io/agentops/) is built with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/), not Jekyll. If your change touches anything under `docs/`, the top-level `README.md`, `CHANGELOG.md`, or `skills/**/SKILL.md`, verify the site still builds:

```bash
# Strict build (what CI runs). First run creates .venv-docs/ and installs
# the pinned toolchain from requirements-docs.txt (uv preferred, pip fallback).
scripts/docs-build.sh --check

# Live-reload dev server at http://127.0.0.1:8000
scripts/docs-build.sh --serve
```

MkDocs-specific expectations:

- Every internal link must resolve. `mkdocs build --strict` fails on unresolved relative links; `tests/docs/validate-links.sh` catches the same class without a Python toolchain.
- Skill pages and the CLI reference are **generated at build time** from `skills/**/SKILL.md` and `cli/docs/COMMANDS.md` respectively — do not hand-author `docs/skills/*.md` or `docs/cli/commands.md`.
- Navigation is declared in `mkdocs.yml` under `nav:`. New top-level docs need an entry there.

Python toolchain is required only for local preview and the strict build. If your dev machine can't install Python, set `PRE_PUSH_SKIP_MKDOCS=1` to bypass the MkDocs check in the pre-push gate; CI will catch it.

## Opening The PR

Make the PR easy to review. Include:

- what changed
- why the existing behavior or docs were not enough
- what checks you ran locally
- any follow-up work you intentionally did not include

Good PR titles:

- `docs: clarify first skill contribution path`
- `feat: add <skill-name> skill`
- `fix: tighten codex lifecycle guidance`

## Review Expectations

Maintainers will look for:

- current frontmatter and taxonomy usage
- linked references and working docs paths
- matching runtime/story across docs and shipped artifacts
- validation evidence
- no secrets, symlinks, or dangerous shell patterns

## Release Timing

AgentOps ships when the repo state justifies it. There is no fixed cadence.

Maintainer notes:

- Keep `[Unreleased]` in `CHANGELOG.md` current.
- Prefer coherent release bundles over random patch piles.
- Draft releases are acceptable for validating packaging before public promotion.

## Code of Conduct

### Our Standards

Positive behavior:
- Be respectful and inclusive
- Provide constructive feedback
- Collaborate openly
- Welcome newcomers
- Share knowledge generously

Unacceptable behavior:
- Harassment or discrimination
- Trolling or insulting comments
- Personal or political attacks
- Publishing others' private information
- Other unprofessional conduct

### Enforcement

Violations may result in:
1. Warning from maintainers
2. Temporary ban from contributing
3. Permanent ban from project

Report issues to: fullerbt@users.noreply.github.com

## Getting Help

Useful places to orient:

- [README.md](https://github.com/boshu2/agentops/blob/main/README.md)
- [docs/INDEX.md](INDEX.md)
- [docs/SKILLS.md](SKILLS.md)
- [docs/SKILL-API.md](SKILL-API.md)
- [docs/testing-skills.md](testing-skills.md)
- [AGENTS.md](https://github.com/boshu2/agentops/blob/main/AGENTS.md)

For examples, browse existing skills under `skills/`.

## License

By contributing to this project, you agree that your contributions will be licensed under the Apache License 2.0.

---

Questions? Open an issue or email fullerbt@users.noreply.github.com
