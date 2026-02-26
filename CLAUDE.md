# AgentOps Skills Repository

## Zero-Context Startup (Read First)

If this is your first message in a fresh session, orient in this order:

1. `docs/README.md` and `docs/INDEX.md` for navigation.
2. `docs/newcomer-guide.md` for a practical repo orientation and learning path.
3. `README.md` for product-level framing.
4. Task-specific canonical surfaces:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, generated `cli/docs/COMMANDS.md`
   - Skills behavior: `skills/**/SKILL.md`
   - Hooks/gates: `hooks/hooks.json` and `hooks/*.sh`
   - Contracts/schemas: `schemas/**`, `lib/schemas/**`

## Source-of-Truth Precedence

When files disagree, trust in this order:

1. Executable implementation and generated outputs (`cli/**`, `hooks/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
2. Declared contracts/manifests (`skills/**/SKILL.md`, `hooks/hooks.json`, `schemas/**`)
3. Narrative docs (`docs/**`, `README.md`)

Always report mismatches; do not silently pick a lower-precedence doc over executable behavior.

## Project Structure

```
cli/          Go CLI (ao binary) — cmd/ao, internal packages
skills/       Skill definitions (source of truth)
hooks/        Git/session hooks
lib/          Shared shell helpers
scripts/      Release, validation, and maintenance scripts
schemas/      JSON schemas for config/manifest
tests/        Integration and validation tests
bin/          Standalone shell tools
docs/         Documentation
```

## Critical: Skill File Locations

**Skills source of truth is `skills/` in THIS repo.**

When editing skills, ALWAYS edit the files under `skills/` in this repo. NEVER edit `~/.claude/skills/` directly — those are installed copies that get overwritten on `npx skills@latest update`.

```
CORRECT:  skills/evolve/SKILL.md          (this repo — source of truth)
WRONG:    ~/.claude/skills/evolve/SKILL.md (installed copy — do not edit)
```

## Building the CLI

```bash
cd cli && make build        # Build ao binary to cli/bin/ao
cd cli && make test         # Run tests
cd cli && make lint         # Run linter
cd cli && make sync-hooks   # Sync embedded hooks/skills into cli/embedded/
```

## Key Scripts

| Script | Purpose |
|--------|---------|
| `scripts/ci-local-release.sh` | Local release validation gate (run before releasing) |
| `scripts/retag-release.sh` | Retag existing release with post-tag commits |
| `scripts/extract-release-notes.sh` | Extract notes from CHANGELOG.md for GitHub release |
| `scripts/security-gate.sh` | Security scanning (semgrep, gosec, gitleaks) |
| `scripts/validate-go-fast.sh` | Quick Go validation (build + vet + test) |
| `scripts/sync-skill-counts.sh` | Sync skill counts across all docs after adding/removing skills |
| `scripts/generate-cli-reference.sh` | Regenerate CLI docs after changing commands/flags |
| `scripts/prune-agents.sh` | Clean up bloated .agents/ directory |

## CI Validation

All pushes to `main` run `.github/workflows/validate.yml` (14 jobs). **Run checks locally before pushing.**

### Quick Local Validation

```bash
# Minimum checks before any push:
bash skills/heal-skill/scripts/heal.sh --strict   # Skill integrity
./tests/docs/validate-doc-release.sh               # Skill counts + links
./scripts/check-contract-compatibility.sh           # Contract refs + JSON validity

# If you changed Go code:
cd cli && make build && make test

# If you changed hooks or lib/hook-helpers.sh:
cd cli && make sync-hooks

# Full gate (runs everything):
scripts/ci-local-release.sh
```

### Rules That Break CI

**No symlinks.** Ever. The plugin-load-test rejects all symlinks in the repo. If you need the same reference file in multiple skills, **copy** it.

**Skill counts must be synced.** Adding or removing a skill directory requires:

```bash
scripts/sync-skill-counts.sh
```

This updates SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md. Forgetting this fails the doc-release-gate.

**Every `references/*.md` must be linked in SKILL.md.** If a file exists in `skills/<name>/references/`, the skill's SKILL.md must contain a markdown link to it or a `Read` instruction referencing it. Use `heal.sh --strict` to check.

**Embedded hooks must stay in sync.** After editing `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`:

```bash
cd cli && make sync-hooks
```

**CLI docs must stay in sync.** After adding/changing CLI commands or flags:

```bash
scripts/generate-cli-reference.sh
```

**Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/INDEX.md`.

**Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15).

**No TODOs in SKILL.md.** Use `bd` issue tracking instead.

**No secrets in code.** CI greps for hardcoded passwords, API keys, tokens in non-test files.

## Release Pipeline

Releases are automated via GoReleaser + GitHub Actions:

1. **Normal release**: Tag triggers the workflow automatically
   ```bash
   git tag v2.X.0 && git push origin v2.X.0
   ```
2. **Retag release** (roll post-tag commits into existing release):
   ```bash
   scripts/retag-release.sh v2.X.0
   ```

The workflow builds cross-platform binaries, creates the GitHub release, updates the Homebrew tap (`boshu2/homebrew-agentops`), generates SBOM + security report, and attests SLSA provenance.

**Always run `scripts/ci-local-release.sh` before tagging.**

## Agent Goals

GOALS.md is the strategic intent layer consumed by `/evolve` and `/goals`:
- `ao work goals measure` — fitness gate checks
- `ao work goals measure --directives` — list strategic directives as JSON
- `ao work goals steer add/remove/prioritize` — manage directives
- `ao work goals init` — bootstrap GOALS.md interactively
- `ao work goals migrate --to-md` — convert GOALS.yaml → GOALS.md
