# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Session Start (Zero-Context Agent)

If you spawn into this repo without context, do this first:

1. Read `docs/newcomer-guide.md` first for a practical repo orientation.
2. Open `docs/README.md` then `docs/INDEX.md` to get the current doc map.
3. Identify your task domain:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, `cli/docs/COMMANDS.md`
   - Skill behavior: `skills/<name>/SKILL.md`
   - Hook/gate behavior: `hooks/hooks.json` + `hooks/*.sh`
   - Validation/release/security flows: `scripts/*.sh` + `tests/`
4. Use source-of-truth precedence when docs disagree:
   1. Executable code and generated artifacts (`cli/**`, `hooks/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
   2. Skill contracts and manifests (`skills/**/SKILL.md`, `hooks/hooks.json`, `schemas/**`)
   3. Explanatory docs (`docs/**`, `README.md`)
5. If you find conflicts, follow the higher-precedence source and call out the mismatch explicitly in your report/PR.

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

## CI Validation — Passing the Pipeline

All pushes to `main` and PRs run `.github/workflows/validate.yml`. **Run checks locally before pushing.** The summary job gates on all checks except security-toolchain-gate (non-blocking), doctor-check (non-blocking), and check-test-staleness (non-blocking).
Blocking policy list (must match the validate summary failset): every job in the CI table below except jobs marked `(non-blocking)`, including `codex-runtime-sections`.

### Local Pre-Push Checklist

Run `scripts/pre-push-gate.sh --fast` for a smart conditional gate that only checks what changed. Or run individual checks below. If any fail, CI will fail too.

```bash
# Recommended: smart conditional gate
scripts/pre-push-gate.sh --fast

# Or individual checks:

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

# 9. Worktree disposition
bash scripts/check-worktree-disposition.sh

# 10. Plugin structure (symlinks, manifests)
./scripts/validate-manifests.sh --repo-root .
find skills -type l  # must be empty — zero symlinks allowed

 # 11. Headless runtime skill smoke (local Claude/Codex sessions; skips missing CLIs)
 bash scripts/validate-headless-runtime-skills.sh

 # 12. Codex-first override coverage (full skill catalog is classified and covered)
 bash scripts/validate-codex-override-coverage.sh

 # 13. Codex semantic parity audit (generated skills still match Codex-native tool/runtime semantics)
 bash scripts/audit-codex-parity.sh

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

### Codex Skill Maintenance

Codex is a first-class runtime in this repo.

- `skills/<name>/SKILL.md` is the canonical behavior contract.
- `skills-codex-overrides/<name>/` is the Codex-specific tailoring layer.
- `skills-codex-overrides/catalog.json` is the machine-readable treatment map for the full catalog.
- `skills-codex/<name>/` is the checked-in Codex runtime artifact. It is manually maintained, while the legacy manifest/marker files remain part of the validation contract.

When a skill change affects Codex behavior, phrasing, orchestration, or UX:

1. Update the source skill under `skills/` when the shared contract changes.
2. Update `skills-codex/<name>/SKILL.md` directly when the Codex runtime copy needs to change, or update `skills-codex-overrides/<name>/` when the Codex experience should differ from Claude.
   - Prompt/operator-layer changes belong in `skills-codex-overrides/<name>/prompt.md`.
   - Durable Codex-only body rewrites belong in `skills-codex-overrides/<name>/SKILL.md`.
3. Run the semantic audit if the checked-in Codex body looks suspicious:
   ```bash
   bash scripts/audit-codex-parity.sh
   # or target one skill
   bash scripts/audit-codex-parity.sh --skill <name>
   ```
4. Validate the checked-in Codex artifacts:
   ```bash
   bash scripts/audit-codex-parity.sh
   bash scripts/validate-codex-override-coverage.sh
   bash scripts/validate-codex-generated-artifacts.sh --scope worktree
   bash scripts/validate-codex-backbone-prompts.sh
   bash scripts/validate-headless-runtime-skills.sh
   ```

Think of `skills/` as the shared contract, `skills-codex-overrides/` as the durable Codex-only tailoring layer, and `skills-codex/` as the checked-in Codex artifact shipped to users.

### Canonical Root and Worktrees

This repo has a canonical root worktree. It owns the common `.git` directory and must remain a non-disposable anchor.

- Keep the canonical root clean and attached to `main`.
- Do not use the canonical root as scratch space for task work.
- Create task branches in linked worktrees and do the actual edits there.
- Every foreign worktree must end the session as `merged`, `preserved`, `exported`, or `deleted`.
- Preserve unfinished branch work on `codex/preserve-*` when it is not ready to merge.
- Run `bash scripts/check-worktree-disposition.sh` before push and session close.

### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **cli-docs-parity** | `cli/docs/COMMANDS.md` matches `ao --help` output | Adding a CLI command without running `scripts/generate-cli-reference.sh` |
| **cli-integration** | Built CLI runs integration command matrix and hook lifecycle smoke tests | CLI command behavior drift not covered by unit tests |
| **codex-runtime-sections** | Required Codex runtime sections and ordering remain valid; CI also enforces generated-artifact parity, backbone prompts, and headless runtime smoke in this job | AGENTS/runtime guidance changes drift from required Codex runtime section rules or Codex artifact/runtime checks stop matching the shipped local gate stack |
| **contract-compatibility-gate** | INDEX.md contract links resolve; schemas are valid JSON; orphan contracts fail unless allowlisted | Adding a contract file without cataloguing it in `docs/INDEX.md` or allowlist governance |
| **doc-release-gate** | Skill counts match across SKILL-TIERS.md, PRODUCT.md, README.md, INDEX.md; link validation | Adding/removing a skill without running `scripts/sync-skill-counts.sh` |
| **doctor-check** | `ao doctor` runs without error on built binary | Non-blocking (`continue-on-error: true`) |
| **embedded-sync** | `cli/embedded/` matches source files in `hooks/`, `lib/`, `skills/` | Editing hooks without running `cd cli && make sync-hooks` |
| **go-build** | `ao` binary builds; tests pass with `-race`; embedded hooks in sync; Go complexity budget | New function exceeds cyclomatic complexity 25 |
| **hook-preflight** | All hooks have kill switches, no unsafe eval, timeouts present | Using `eval` or backtick substitution in hooks |
| **learning-coherence** | Learning files have valid frontmatter and are not garbage/hallucinated | Auto-extracted learnings with no recognized fields or boilerplate content |
| **markdownlint** | Markdown style/lint rules pass for repository docs | Docs formatting regressions not caught by link checks |
| **memrl-health** | MemRL feedback loop wiring and health checks | Broken ingestion/feedback loop wiring |
| **plugin-load-test** | No symlinks anywhere in the repo; manifests valid; plugin structure correct | Creating symlinks instead of real file copies |
| **security-scan** | No hardcoded secrets or dangerous patterns (`curl\|sh`, `rm -rf /`) | Hardcoded API keys or passwords in non-test files |
| **security-toolchain-gate** | Semgrep, gosec, gitleaks, etc. | Non-blocking (`continue-on-error: true`) |
| **shellcheck** | All `.sh` files pass ShellCheck at error severity | Unquoted variables, missing `set -euo pipefail` |
| **skill-dependency-check** | Skill `metadata.dependencies` entries resolve to existing skills | Declaring a skill dependency that no longer exists |
| **skill-integrity** | Every `references/*.md` file is linked from SKILL.md; no dead refs, dead xrefs, or missing scripts | Adding a reference file without linking it in SKILL.md |
| **skill-lint** | Skill line limits, required sections, Claude feature coverage | Judgment-tier skill exceeds 600 lines; missing `## Examples` in user-facing skill |
| **skill-schema** | SKILL frontmatter conforms to schema | Missing/invalid frontmatter fields in SKILL.md |
| **smoke-test** | Skill frontmatter valid; no placeholders; no TODOs in SKILL.md files | Leaving `TODO` or placeholder emails in SKILL.md |
| **validate-ci-policy-parity** | AGENTS CI table and blocking policy match workflow summary enforcement | Docs say non-blocking/required but workflow differs |
| **validate-hooks-doc-parity** | Scoped docs avoid stale hook-count claims vs runtime `hooks/hooks.json` | Runtime hook contract changed but docs were not updated |
| **bats-tests** | BATS integration tests for shell scripts pass | Hook or script behavioral regression |
| **check-test-staleness** | Detects stale/abandoned test files | Non-blocking (`continue-on-error: true`) |
| **coverage-ratchet** | Coverage baseline doesn't regress vs `.coverage-baseline.json` | New code drops package coverage below floor |
| **file-manifest-overlap** | No file path conflicts between workers/skills | Two skills claim the same output file |
| **json-flag-consistency** | All `--json` flags produce valid JSON with consistent format | Missing `--json` support on a new command |

### Key Constraints Agents Must Follow

**No symlinks.** The plugin-load-test rejects ALL symlinks. If you need the same file in multiple skill `references/` dirs, **copy the file** — do not symlink.

**Skill counts must be synced.** When adding or removing a skill directory, run:
```bash
scripts/sync-skill-counts.sh
```
This updates counts in SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md.

**Every reference file must be linked.** If a file exists in a skill's `references/` directory, the skill's SKILL.md must link to it via markdown link or Read instruction. Run `heal.sh --strict` to check.

**Codex checked-in artifacts are manually maintained, with legacy generated provenance metadata.** If `skills-codex/` still contains Claude-era primitives (`TaskCreate`, `TaskList`, `Tool: Task`), Claude backend refs, or duplicated runtime rewrites, run:
```bash
bash scripts/audit-codex-parity.sh --skill <name>
```
Then update the canonical source and, when the Codex runtime copy itself must change, patch `skills-codex/<name>/` and/or add a durable override under `skills-codex-overrides/<name>/`. Finish by running `bash scripts/refresh-codex-artifacts.sh --scope worktree` and re-running the audit. Do not use the deprecated `scripts/sync-codex-native-skills.sh` workflow.

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

**Validate before proposing.** Before suggesting a new capability or safeguard, verify it doesn't already exist: check `ao rpi serve --help`, `hooks/hooks.json`, `GOALS.md`, and existing SKILL.md files. Three suggested features in our March 2026 validation review were already implemented.

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
   bd vc status  # Optional visibility check; bd auto-sync handles JSONL import/export
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches, and validate worktree disposition
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
- NEVER leave a foreign branch-attached worktree without a recorded disposition

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs with git:

- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

<!-- END BEADS INTEGRATION -->
