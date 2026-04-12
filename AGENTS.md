# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Session Start (Zero-Context Agent)

1. Read `docs/newcomer-guide.md` for repo orientation.
2. Open `docs/README.md` then `docs/INDEX.md` for the doc map.
3. Identify your task domain:
   - CLI: `cli/cmd/ao/`, `cli/internal/`, `cli/docs/COMMANDS.md`
   - Skills: `skills/<name>/SKILL.md`
   - Hooks/gates: `hooks/hooks.json` + `hooks/*.sh`
   - Validation/release: `scripts/*.sh` + `tests/`
4. Source-of-truth precedence: executable code > contracts/manifests > narrative docs. Report mismatches.

## Installing/Updating Skills

```bash
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Other agents (Cursor, etc.)
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

# Validation (run before pushing)
scripts/pre-push-gate.sh --fast     # Smart diff-based gate (recommended)
scripts/pre-push-gate.sh            # Full gate (all 33 checks, ~3min)
scripts/ci-local-release.sh         # Full release gate
```

## CI Validation

All pushes to `main` run `.github/workflows/validate.yml`. **Run `scripts/pre-push-gate.sh --fast` before pushing.** The summary job gates on all checks except security-toolchain-gate (non-blocking), doctor-check (non-blocking), and check-test-staleness (non-blocking).
Blocking policy list (must match the validate summary failset): every job in the CI table below except jobs marked `(non-blocking)`, including `codex-runtime-sections`.

See CLAUDE.md for the full list of rules that break CI.

### CI Jobs and What They Check

| Job | What it validates |
|-----|-------------------|
| **bats-tests** | BATS integration tests for shell scripts |
| **check-test-staleness** | Detects stale/abandoned test files |
| **cli-docs-parity** | `cli/docs/COMMANDS.md` matches `ao --help` output |
| **cli-integration** | Built CLI runs integration command matrix and hook lifecycle smoke tests |
| **codex-runtime-sections** | Required Codex runtime sections, artifact metadata parity, backbone prompts, override coverage, RPI contract, lifecycle guards, and headless runtime smoke |
| **contract-compatibility-gate** | INDEX.md contract links resolve; schemas are valid JSON; orphan contracts fail |
| **doc-release-gate** | Skill counts match across docs; link validation |
| **doctor-check** | `ao doctor` runs without error |
| **embedded-sync** | `cli/embedded/` matches source files in `hooks/`, `lib/`, `skills/` |
| **file-manifest-overlap** | No file path conflicts between workers/skills |
| **go-build** | `ao` binary builds; tests pass with `-race`; Go complexity budget |
| **hook-preflight** | All hooks have kill switches, no unsafe eval, timeouts present |
| **json-flag-consistency** | All `--json` flags produce valid JSON with consistent format |
| **learning-coherence** | Learning files have valid frontmatter and are not hallucinated |
| **markdownlint** | Markdown style/lint rules pass |
| **memrl-health** | MemRL feedback loop wiring and health checks |
| **plugin-load-test** | No symlinks anywhere; manifests valid; plugin structure correct |
| **security-scan** | No hardcoded secrets or dangerous patterns |
| **security-toolchain-gate** | Semgrep, gosec, gitleaks |
| **shellcheck** | All `.sh` files pass ShellCheck at error severity |
| **skill-dependency-check** | Skill `metadata.dependencies` entries resolve to existing skills |
| **skill-integrity** | Every `references/*.md` is linked from SKILL.md; no dead refs |
| **skill-lint** | Skill line limits, required sections, Claude feature coverage |
| **skill-schema** | SKILL frontmatter conforms to schema |
| **smoke-test** | Skill frontmatter, placeholder/TODO hygiene, runtime smoke scripts |
| **swarm-evidence** | Swarm evidence files and file manifests are valid |
| **validate-ci-policy-parity** | AGENTS CI table and blocking policy match workflow summary |
| **validate-hooks-doc-parity** | Docs avoid stale hook-count claims vs runtime `hooks/hooks.json` |
| **windows-smoke** | Windows PowerShell installer smoke, Codex plugin, `ao doctor` hints |

### Codex Skill Maintenance

- `skills/<name>/SKILL.md` — canonical behavior contract (shared source of truth)
- `skills-codex-overrides/<name>/` — Codex-specific tailoring layer
- `skills-codex/<name>/` — checked-in Codex runtime artifact (manually maintained)

When a skill change affects Codex behavior:
1. Update shared contract in `skills/` if it changed.
2. Update `skills-codex/<name>/SKILL.md` or `skills-codex-overrides/<name>/` for Codex-specific changes.
3. Audit: `bash scripts/audit-codex-parity.sh --skill <name>`
4. Validate: `bash scripts/validate-codex-override-coverage.sh`

## Session Completion

Work is NOT complete until `git push` succeeds.

1. Run quality gates if code changed.
2. Close/update issues: `bd close <id>` or `bd update <id> --status in_progress`
3. Push: `git pull --rebase && git push`
4. Verify: `git status` shows "up to date with origin"
5. Hand off context for next session.

**Rules:** Never stop before pushing. Never say "ready to push when you are" — push it yourself. Never leave a foreign-branch worktree without a recorded disposition.

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

Use `bd` for ALL task tracking. Do NOT use markdown TODOs or external trackers.

```bash
bd ready --json                    # Unblocked work
bd create "Title" -t bug -p 1 --json  # Create issue (types: bug, feature, task, epic, chore)
bd update <id> --status in_progress   # Claim work
bd close <id> --reason "Done"         # Complete work
```

Priorities: 0=critical, 1=high, 2=medium (default), 3=low, 4=backlog.

bd auto-syncs to `.beads/issues.jsonl` — no manual export needed.
<!-- END BEADS INTEGRATION -->
