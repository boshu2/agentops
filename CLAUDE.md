# AgentOps Skills Repository

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
| `scripts/prune-agents.sh` | Clean up bloated .agents/ directory |

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
