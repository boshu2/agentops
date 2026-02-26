# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Session Start (Zero-Context Agent)

If you spawn into this repo without context, do this first:

1. Open `docs/README.md` then `docs/INDEX.md` to get the current doc map.
   - New contributors should also read `docs/newcomer-guide.md` for a practical repo orientation.
2. Identify your task domain:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, `cli/docs/COMMANDS.md`
   - Skill behavior: `skills/<name>/SKILL.md`
   - Hook/gate behavior: `hooks/hooks.json` + `hooks/*.sh`
   - Validation/release/security flows: `scripts/*.sh` + `tests/`
3. Use source-of-truth precedence when docs disagree:
   1. Executable code and generated artifacts (`cli/**`, `hooks/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
   2. Skill contracts and manifests (`skills/**/SKILL.md`, `hooks/hooks.json`, `schemas/**`)
   3. Explanatory docs (`docs/**`, `README.md`)
4. If you find conflicts, follow the higher-precedence source and call out the mismatch explicitly in your report/PR.

## Installing/Updating Skills

Use the [skills.sh](https://skills.sh/) npm package to install AgentOps skills for any agent:

```bash
# Claude Code: use Claude plugin install path (not npx)
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Other agents (for example Cursor): install only selected skills
npx skills@latest add boshu2/agentops -g -a <agent> -s <skill-name> -y
npx skills@latest add boshu2/agentops -g -a cursor -s quickstart -y

# Update all installed skills
npx skills@latest update
```

## Quick Reference

```bash
# Issue tracking
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git

# CLI development
cd cli && make build  # Build ao binary
cd cli && make test   # Run tests
cd cli && make lint   # Run linter

# Validation (run before pushing)
scripts/ci-local-release.sh     # Full local release gate (runs everything)
scripts/validate-go-fast.sh     # Quick Go validation (build + vet + test)
```

## CI Validation — Passing the Pipeline

All pushes to `main` and PRs run `.github/workflows/validate.yml`. **Run checks locally before pushing.** The summary job gates on all checks except security-toolchain-gate (non-blocking).

### Local Pre-Push Checklist

Run these before every push. If any fail, CI will fail too.

```bash
# 1. Skill integrity (most common failure)
bash skills/heal-skill/scripts/heal.sh --strict

# 2. Doc-release gate (skill counts, link validation)
./tests/docs/validate-doc-release.sh

# 3. ShellCheck
find . -name "*.sh" -type f -not -path "./.git/*" -print0 | xargs -0 shellcheck --severity=error

# 4. Markdownlint
git ls-files '*.md' | xargs markdownlint

# 5. Go build + tests (if cli/ changed)
cd cli && make build && make test

# 6. Contract compatibility
./scripts/check-contract-compatibility.sh

# 7. Hook/docs parity
bash scripts/validate-hooks-doc-parity.sh

# 8. CI policy/docs parity
bash scripts/validate-ci-policy-parity.sh

# 9. Plugin structure (symlinks, manifests)
./scripts/validate-manifests.sh --repo-root .
find skills -type l  # must be empty — zero symlinks allowed

# Full gate (runs everything above and more):
scripts/ci-local-release.sh
```

### CLI Skill-Map Refresh

After changing `ao` command usage in any of these locations, refresh [`docs/cli-skills-map.md`](docs/cli-skills-map.md):

- `skills/*/SKILL.md`
- `skills-codex/*/SKILL.md`
- `hooks/*.sh`
- `hooks/hooks.json`

Process:
1. Update this map from current sources.
2. Run `bash scripts/validate-hooks-doc-parity.sh`.
3. Run `bash tests/docs/validate-doc-release.sh` and `bash tests/docs/validate-skill-count.sh` before pushing.

### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **cli-docs-parity** | `cli/docs/COMMANDS.md` matches `ao --help` output | Adding a CLI command without running `scripts/generate-cli-reference.sh` |
| **cli-integration** | Built CLI runs integration command matrix and hook lifecycle smoke tests | CLI command behavior drift not covered by unit tests |
| **contract-compatibility-gate** | INDEX.md contract links resolve; schemas are valid JSON; orphan contracts fail unless allowlisted | Adding a contract file without cataloguing it in `docs/INDEX.md` or allowlist governance |
| **doc-release-gate** | Skill counts match across SKILL-TIERS.md, PRODUCT.md, README.md, INDEX.md; link validation | Adding/removing a skill without running `scripts/sync-skill-counts.sh` |
| **embedded-sync** | `cli/embedded/` matches source files in `hooks/`, `lib/`, `skills/` | Editing hooks without running `cd cli && make sync-hooks` |
| **go-build** | `ao` binary builds; tests pass with `-race`; embedded hooks in sync; Go complexity budget | New function exceeds cyclomatic complexity 25 |
| **hook-preflight** | All hooks have kill switches, no unsafe eval, timeouts present | Using `eval` or backtick substitution in hooks |
| **memrl-health** | MemRL feedback loop wiring and health checks | Broken ingestion/feedback loop wiring |
| **plugin-load-test** | No symlinks anywhere in the repo; manifests valid; plugin structure correct | Creating symlinks instead of real file copies |
| **security-scan** | No hardcoded secrets or dangerous patterns (`curl\|sh`, `rm -rf /`) | Hardcoded API keys or passwords in non-test files |
| **security-toolchain-gate** | Semgrep, gosec, gitleaks, etc. | Non-blocking (`continue-on-error: true`) |
| **shellcheck** | All `.sh` files pass ShellCheck at error severity | Unquoted variables, missing `set -euo pipefail` |
| **skill-integrity** | Every `references/*.md` file is linked from SKILL.md; no dead refs, dead xrefs, or missing scripts | Adding a reference file without linking it in SKILL.md |
| **skill-schema** | SKILL frontmatter conforms to schema | Missing/invalid frontmatter fields in SKILL.md |
| **smoke-test** | Skill frontmatter valid; no placeholders; no TODOs in SKILL.md files | Leaving `TODO` or placeholder emails in SKILL.md |
| **validate-ci-policy-parity** | AGENTS CI table and blocking policy match workflow summary enforcement | Docs say non-blocking/required but workflow differs |
| **validate-hooks-doc-parity** | Scoped docs avoid stale hook-count claims vs runtime `hooks/hooks.json` | Runtime hook contract changed but docs were not updated |

### Key Constraints Agents Must Follow

**No symlinks.** The plugin-load-test rejects ALL symlinks. If you need the same file in multiple skill `references/` dirs, **copy the file** — do not symlink.

**Skill counts must be synced.** When adding or removing a skill directory, run:
```bash
scripts/sync-skill-counts.sh
```
This updates counts in SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md.

**Every reference file must be linked.** If a file exists in a skill's `references/` directory, the skill's SKILL.md must link to it via markdown link or Read instruction. Run `heal.sh --strict` to check.

**Embedded hooks must stay in sync.** After editing anything in `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`, run:
```bash
cd cli && make sync-hooks
```

**CLI docs must stay in sync.** After adding/changing CLI commands or flags, run:
```bash
scripts/generate-cli-reference.sh
```

**Contracts must be catalogued.** When adding files to `docs/contracts/`, add a corresponding entry in `docs/INDEX.md`. The contract gate discovers files dynamically but checks for orphans.

**Go complexity budget.** New or modified functions must stay under cyclomatic complexity 25 (warning at 15). The check only flags new/worsened violations, not legacy ones.

**No TODOs in SKILL.md files.** The smoke test greps for `TODO` and `FIXME` in `skills/*/SKILL.md`. Use issue tracking (`bd`) for follow-up work instead.

## Releasing

Standard release flow:

1. Run `scripts/ci-local-release.sh` to validate
2. Tag and push: `git tag v2.X.0 && git push origin v2.X.0`
3. GitHub Actions runs GoReleaser — builds binaries, creates release, updates Homebrew tap
4. Upgrade locally: `brew update && brew upgrade agentops`

For retagging (rolling post-tag commits into an existing release):

```bash
scripts/retag-release.sh v2.13.0
```

This moves the tag to HEAD, pushes, rebuilds the GitHub release, updates the Homebrew tap, and upgrades locally. One command, no manual steps.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
