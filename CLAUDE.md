# AgentOps Skills Repository

## Zero-Context Startup (Read First)

If this is your first message in a fresh session, orient in this order:

1. `docs/newcomer-guide.md` for a practical repo orientation and learning path.
2. `docs/README.md` and `docs/INDEX.md` for navigation.
3. `README.md` for product-level framing.
4. Task-specific canonical surfaces:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, generated `cli/docs/COMMANDS.md`
   - Skills behavior: `skills/**/SKILL.md`
   - Hooks/gates: `hooks/hooks.json` and `hooks/*.sh`
   - Contracts/schemas: `schemas/**`, `lib/schemas/**`
5. `.agents/AGENTS.md` for knowledge store navigation (search on demand, don't pre-load).

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

When editing skills, ALWAYS edit the files under `skills/` in this repo. NEVER edit `~/.claude/skills/` directly — those are installed copies that get overwritten on `bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)`.

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
| `scripts/pre-push-gate.sh` | Smart pre-push validation (use `--fast` for diff-based checks) |
| `scripts/ci-local-release.sh` | Local release validation gate (run before releasing) |
| `scripts/retag-release.sh` | Retag existing release with post-tag commits |
| `scripts/extract-release-notes.sh` | Extract notes from CHANGELOG.md for GitHub release |
| `scripts/security-gate.sh` | Security scanning (semgrep, gosec, gitleaks) |
| `scripts/validate-go-fast.sh` | Quick Go validation (build + vet + test) |
| `scripts/sync-skill-counts.sh` | Sync skill counts across all docs after adding/removing skills |
| `scripts/generate-cli-reference.sh` | Regenerate CLI docs after changing commands/flags |
| `scripts/audit-codex-parity.sh` | Audit generated `skills-codex/` for semantic drift that simple rewrites miss |
| `scripts/regen-codex-hashes.sh` | Regenerate manifest/marker hashes after changing skills-codex/ files |
| `scripts/prune-agents.sh` | Clean up bloated .agents/ directory |

## CI Validation

All pushes to `main` run `.github/workflows/validate.yml` (24 jobs). **Run checks locally before pushing.**

### Quick Local Validation

```bash
# Recommended: smart conditional gate (only checks relevant to changed files):
scripts/pre-push-gate.sh --fast

# These checks are now included in the pre-push gate (no need to run separately):
# bash skills/heal-skill/scripts/heal.sh --strict   # → check 12
# ./tests/docs/validate-doc-release.sh               # → check 25
# ./scripts/check-contract-compatibility.sh           # → check 26

# If you changed Go code:
cd cli && make build && make test

# If you changed hooks or lib/hook-helpers.sh:
cd cli && make sync-hooks

# If you changed skills-codex/ files:
scripts/regen-codex-hashes.sh

# Full gate (all 33 checks, ~3min):
scripts/pre-push-gate.sh

# Release gate (runs everything including security):
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

**Codex skills are manually maintained.** Edit `skills-codex/<name>/SKILL.md` directly or add a durable override in `skills-codex-overrides/<name>/`. The sync script (`sync-codex-native-skills.sh`) is deprecated — it overwrites manual edits. To audit for drift:

```bash
bash scripts/audit-codex-parity.sh --skill <name>
```

**Embedded hooks must stay in sync.** After editing `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`:

```bash
cd cli && make sync-hooks
```

**CLI docs must stay in sync.** After adding/changing CLI commands or flags:

```bash
scripts/generate-cli-reference.sh
```

**Codex maintenance flow.** For Codex-specific skill changes:

```bash
# 1. Edit skills-codex/<name>/SKILL.md directly, or add override in skills-codex-overrides/<name>/
# 2. Audit for drift
bash scripts/audit-codex-parity.sh --skill <name>
# 3. Validate artifacts
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

**Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/INDEX.md`.

**Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15).

**No TODOs in SKILL.md.** Use `bd` issue tracking instead.

**No secrets in code.** CI greps for hardcoded passwords, API keys, tokens in non-test files.

## Testing Rules

- **No coverage-padding tests.** Tests that use trivial `!= ""` or `!= nil` assertions solely to inflate coverage metrics are banned. Every test must assert behavioral correctness, not just presence. If a function's coverage is low, write a real test or accept the metric.
- **No `cov*_test.go` naming convention.** Test files must be named after the source file they test (e.g., `goals_test.go` not `cov15_goals_init_test.go`).

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
- `ao goals measure` — fitness gate checks
- `ao goals measure --directives` — list strategic directives as JSON
- `ao goals steer add/remove/prioritize` — manage directives
- `ao goals init` — bootstrap GOALS.md interactively
- `ao goals migrate --to-md` — convert GOALS.yaml → GOALS.md

## AgentOps Workflow (RPI)

```
Research → Plan → Implement → Validate
    ↑                            │
    └──── Knowledge Flywheel ────┘
```

### Session & Swarm Constraints

- **Multi-phase work:** Route through `ao rpi` — it enforces 90-min phase timeouts and 10-min stall detection. Raw sessions have neither.
- **Validation overhead is by design:** Pre-mortem + vibe cost 3-5x implementation time. This ratio prevents bug rework — do not shortcut.
- **Before spawning workers:** Verify no file overlap across the wave (see swarm SKILL.md pre-flight). File collisions are the #1 swarm failure mode.
- **Before proposing new capability:** Check `ao rpi serve --help`, `hooks/hooks.json`, and `GOALS.md` first.

### Execution Discipline

- **Produce artifacts, not just plans.** When asked to research, plan, or investigate, always produce actionable output (code changes, tests, or concrete files) within the session. Do not spend an entire session only planning unless explicitly told to "just plan."
- **Verify before committing.** After modifying Go files, run `go test ./...` and `go vet ./...` before committing. After modifying Python files, run relevant tests. Never commit code that hasn't been verified.
- **Execute first, research second.** When asked to run tests or execute something, start running within the first 2-3 messages. If research is needed, do it concurrently or after initial execution — not instead of it.
- **Parallel agent caution.** When working with parallel agents, worktrees, or swarm workers: avoid using git worktrees unless explicitly requested, watch for linter side-effects and import errors from partial changes, and verify the base branch is up-to-date before starting parallel work.
- **First-Edit Rule.** Your first Edit, Write, or executable Bash call MUST happen within your first 3 responses. If the user asked you to DO something, start doing it immediately. Research while doing, not instead of doing. If you reach response 3 without producing output, stop and act on what you know.
- **Intent Echo.** Before starting any non-trivial task, state in ONE sentence what you understand the user wants: "I understand you want me to [action] [scope] [constraint]." Wait for confirmation before proceeding with multi-file changes. This is especially important for removal, refactoring, scope changes, or requests with "just" or "only."
- **Two-Correction Rule.** If the user corrects your approach twice on the same task: STOP, re-read the original request, state what you now understand differently, and ask "Is this what you mean?" Do not attempt a third approach without explicit confirmation.
