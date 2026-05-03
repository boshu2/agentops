# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Session Start (Zero-Context Agent)

If you spawn into this repo without context, do this first:

1. Read `docs/newcomer-guide.md` first for a practical repo orientation.
2. Open `docs/index.md` (MkDocs landing) then `docs/documentation-index.md` (full catalog) to get the current doc map.
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

## CI Validation — Passing the Pipeline

All pushes to `main` and PRs run `.github/workflows/validate.yml`. **Run checks locally before pushing.** The summary job gates on all checks except agentops-eval-advisory (non-blocking), security-toolchain-gate (non-blocking), doctor-check (non-blocking), check-test-staleness (non-blocking), and swarm-evidence (non-blocking).
Blocking policy list (must match the validate summary failset): every job in the CI table below except jobs marked `(non-blocking)`, including the seven `validate-codex-*` and `validate-headless-runtime-skills` jobs (split from the previous aggregated `codex-runtime-sections` job, soc-ltp2).

#### Advisory Job Triage SLAs (post-merge advisory policy, soc-z7qq)

Advisory jobs run on every PR but their failure does NOT block merge. They surface a `(advisory)` suffix on the GitHub check name. Each advisory job has a triage SLA — when the job has been red for longer than its SLA, follow the escalation rule.

| Advisory job | Triage SLA | Escalation rule |
|---|---|---|
| **agentops-eval-advisory** | 7d | Release-blocking when stale: a failing eval-advisory older than 7d blocks the next `vX.Y.Z` tag until triaged. |
| **security-toolchain-gate** | 14d | Open a `bd` issue with label `ci-advisory`. Network/install flake (item 40) is mitigated by 3-attempt exponential-backoff retry on the install step; only persistent toolchain or scanner regressions count toward the SLA. |
| **doctor-check** | 30d | Open a `bd` issue tracking the stale CLI reference; prioritize when the next `cli/cmd/ao/**` PR lands. |
| **check-test-staleness** | none (info-only) | Read the report; no merge or release impact. Item 33 — drift signal, not a gate. |
| **swarm-evidence** | none (info-only) | Read the report; no merge or release impact. Item 34 — informational artifact validation. |

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is currently warn-only with a deferred promotion gate. Promotion criterion: `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**, where `baseline_p_at_5 = 0.30` is pinned in `docs/CI-CD.md` §"Retrieval-bench ratchet" until a durable non-`.agents` baseline artifact is introduced. The 14-consecutive-nightly observation window is intentionally observational — not yet wired into a counter — so flips to blocking remain a manual decision after the window is documented green.

#### DEFERRED CI Hardening (soc-mi17)

These CI 1-40 items are intentionally not being hardened in this wave. Revisit only when the named promotion trigger fires.

| Item | Current handling | Rationale | Promotion trigger |
|---|---|---|---|
| **1 — go-build error** | DEFER | Compilation breakage is developer hygiene; `cd cli && make build && make test` already exists in the local checklist. | Promote to FIX if a merged `main` commit reaches CI with the same build-class failure twice in 30 days despite local pre-push guidance. |
| **7 — cli-integration cascade** | DEDUPE/DEFER | Failures cascade from build/test root causes, primarily items 1 and 4. | Promote to FIX if `cli-integration` fails independently after items 1 and 4 are green for two consecutive affected runs. |
| **13 — contract-compatibility** | DEFER | The gate is doing its job; failures indicate real schema or catalog drift. | Promote to FIX if the same false-positive contract failure repeats twice in a quarter. |
| **14 — smoke-test Python 3.14** | DEFER | Rare flake; workflow pinning already narrows the surface. | Promote to FIX if the Python 3.14 smoke failure appears in two separate PRs or nightlies within 30 days. |
| **21 — GoReleaser publish failure** | DEFER | Release publish failures are covered by the `pre-tag-ci-validation` pattern and release discipline. | Promote to FIX if a publish failure recurs on two consecutive release attempts with the same root cause. |
| **22 — doc-release blocks publish** | DEDUPE/DEFER | This is a cascade from item 12 doc-release drift, now covered by pre-push gating. | Promote to FIX if publish is blocked by doc-release after item 12's local gate has passed on the release branch. |
| **23 — markdownlint** | DEFER | Rare and cheap to repair locally. | Promote to FIX if markdownlint failures occur more than twice in a quarter or block a release branch. |
| **24 — shellcheck** | DEFER | Rare and cheap to repair locally. | Promote to FIX if shellcheck failures occur more than twice in a quarter or block a release branch. |
| **27 — plugin-load-test manifest** | DEFER | Low failure rate and the gate catches real manifest/plugin-structure drift. | Promote to FIX if plugin-load-test reports a false positive twice in a quarter. |
| **30 — memrl-health degraded** | DEFER | Rare health signal; investigate when it actually fires. | Promote to FIX if `memrl-health` fires more than once per quarter. |
| **39 — nightly Static Validation** | DEFER | Nightly-only signal should be bundled with future nightly stabilization if the pattern persists. | Promote to FIX if static validation fails in 3 of 10 consecutive nightlies outside a known knowledge-cycle quarantine. |

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

 # 13. Codex RPI contract and lifecycle guard checks
 bash scripts/validate-codex-rpi-contract.sh
 bash scripts/validate-codex-lifecycle-guards.sh

 # 14. Codex semantic parity audit (generated skills still match Codex-native tool/runtime semantics)
 bash scripts/audit-codex-parity.sh

 # 15. AgentOps eval canaries
 scripts/eval-agentops.sh --fast

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
   bash scripts/validate-codex-rpi-contract.sh
   bash scripts/validate-codex-lifecycle-guards.sh
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
- Every surviving `codex/preserve-*` ref must have an entry in `docs/preserved-refs.tsv` with owner and retirement rule.
- Run `bash scripts/check-worktree-disposition.sh` before push and session close.

### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **agentops-eval-advisory** | Runs deterministic public AgentOps eval canaries and baseline comparisons when baselines exist | Non-blocking (`continue-on-error: true`); eval suite or scorecard regression until baselines are ratcheted |
| **agentops-eval-baseline-audit** | Runs `ao eval baseline-audit --root evals/agentops-core --json`; drift-only gate that fails on `stale_suite_hashes>0`. `policy_mismatch_count` is reported informationally (under the no-tracked-`.agents/` policy from `3f1566fd` baselines are operator-local, so fresh clones legitimately have missing_compare_baselines) | A promoted baseline's recorded suite SHA stops matching the current suite definition |
| **cli-docs-parity** | `cli/docs/COMMANDS.md` matches `ao --help` output | Adding a CLI command without running `scripts/generate-cli-reference.sh` |
| **cli-integration** | Built CLI runs integration command matrix and hook lifecycle smoke tests | CLI command behavior drift not covered by unit tests |
| **contract-compatibility-gate** | documentation-index.md contract links resolve; schemas are valid JSON; orphan contracts fail unless allowlisted | Adding a contract file without cataloguing it in `docs/documentation-index.md` or allowlist governance |
| **doc-release-gate** | Skill counts match across SKILL-TIERS.md, PRODUCT.md, README.md, documentation-index.md; link validation | Adding/removing a skill without running `scripts/sync-skill-counts.sh` |
| **doctor-check** | `ao doctor` runs without error on built binary | Non-blocking (`continue-on-error: true`) |
| **embedded-sync** | `cli/embedded/` matches source files in `hooks/`, `lib/`, `skills/` | Editing hooks without running `cd cli && make sync-hooks` |
| **go-build** | `ao` binary builds; tests pass with `-race`; embedded hooks in sync; Go complexity budget | New function exceeds cyclomatic complexity 25 |
| **hook-preflight** | All hooks have kill switches, no unsafe eval, timeouts present | Using `eval` or backtick substitution in hooks |
| **hook-output-schema-lint** | Hooks emit only the safely-portable PreToolUse output subset both Claude and Codex CLI accept | Using `hookSpecificOutput.updatedInput` (silently dropped by Codex CLI 0.128.0+) |
| **learning-coherence** | Learning files have valid frontmatter and are not garbage/hallucinated | Auto-extracted learnings with no recognized fields or boilerplate content |
| **markdownlint** | Markdown style/lint rules pass for repository docs | Docs formatting regressions not caught by link checks |
| **memrl-health** | MemRL feedback loop wiring and health checks | Broken ingestion/feedback loop wiring |
| **plugin-load-test** | No symlinks anywhere in the repo; manifests valid; plugin structure correct | Creating symlinks instead of real file copies |
| **pre-push-gate-wired** | `.githooks/pre-push` invokes `scripts/pre-push-gate.sh`; `git push --dry-run` smoke proves the hook actually fires | Editing the hook chain without re-running `scripts/check-pre-push-gate-wired.sh --dry-run-smoke` |
| **security-scan** | No hardcoded secrets or dangerous patterns (`curl\|sh`, `rm -rf /`) | Hardcoded API keys or passwords in non-test files |
| **security-toolchain-gate** | Semgrep, gosec, gitleaks, etc. | Non-blocking (`continue-on-error: true`) |
| **shellcheck** | All `.sh` files pass ShellCheck at error severity | Unquoted variables, missing `set -euo pipefail` |
| **skill-dependency-check** | Skill `metadata.dependencies` entries resolve to existing skills | Declaring a skill dependency that no longer exists |
| **skill-integrity** | Every `references/*.md` file is linked from SKILL.md; no dead refs, dead xrefs, or missing scripts | Adding a reference file without linking it in SKILL.md |
| **skill-lint** | Skill line limits, required sections, Claude feature coverage | Judgment-tier skill exceeds 600 lines; missing `## Examples` in user-facing skill |
| **skill-schema** | SKILL frontmatter conforms to schema | Missing/invalid frontmatter fields in SKILL.md |
| **smoke-test** | Repo smoke surface: skill frontmatter, placeholder/TODO hygiene, plus standalone Claude/Codex/OpenCode runtime smoke scripts and mocked headless runtime validation | Runtime install/bundle drift or placeholder/TODO regressions |
| **standards-injector-completeness** | Every `<lang>` mapped by `hooks/standards-injector.sh` has a matching `skills/standards/references/<lang>.md` | Adding a case branch without the reference file (the hook fails open silently) |
| **swarm-evidence** | Swarm evidence files and file manifests are valid | Non-blocking (`continue-on-error: true`); informational artifact validation only |
| **validate-ci-policy-parity** | AGENTS CI table and blocking policy match workflow summary enforcement | Docs say non-blocking/required but workflow differs |
| **validate-codex-backbone-prompts** | Codex backbone prompt files are present and well-formed | Backbone prompt file deleted, renamed, or shape regressed |
| **validate-codex-generated-artifacts** | Codex artifact metadata parity (manifests, markers, hashes) for the head commit | Codex artifact regen drift; missing or stale `skills-codex/` outputs |
| **validate-codex-lifecycle-guards** | Codex lifecycle guards (session/run boundaries, kill switches) remain wired | Lifecycle guard removed or runtime hook order changed without updating the guard |
| **validate-codex-override-coverage** | Every `skills-codex-overrides/<name>/` entry covers required override surfaces | Adding an override skill without the prompt or body coverage the runtime expects |
| **validate-codex-rpi-contract** | Codex RPI contract (phase prompts, transitions, output schema) matches runtime | RPI contract drift between Claude and Codex runtimes |
| **validate-codex-runtime-sections** | Required Codex runtime sections and ordering remain valid in shipped artifacts | AGENTS/runtime guidance changes drift from required Codex runtime section rules |
| **validate-headless-runtime-skills** | Headless runtime skill bundle smoke (mocked Claude/Codex/OpenCode runners) | Runtime install/bundle drift breaks headless skill execution |
| **validate-hooks-doc-parity** | Scoped docs avoid stale hook-count claims vs runtime `hooks/hooks.json` | Runtime hook contract changed but docs were not updated |
| **windows-smoke** | Native Windows PowerShell installer smoke, Codex plugin temp install, local `ao doctor` Windows hints, and focused Windows-sensitive Go tests | Windows install/plugin/runtime surfaces regress while Ubuntu CI stays green |
| **bats-tests** | BATS integration tests for shell scripts pass | Hook or script behavioral regression |
| **check-test-staleness** | Detects stale/abandoned test files | Non-blocking (`continue-on-error: true`) |
| **file-manifest-overlap** | No file path conflicts between workers/skills | Two skills claim the same output file |
| **json-flag-consistency** | All `--json` flags produce valid JSON with consistent format | Missing `--json` support on a new command |

### Nightly Workflow Jobs

`.github/workflows/nightly.yml` runs at 06:00 UTC daily and on `workflow_dispatch`.

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **cli-tests** | Go CLI tests with `-race` and coverage | Test regression in `cli/internal/**` |
| **static-validation** | Smoke, doc-release, and hooks/docs parity gates | Skill/doc drift slipping past pre-push |
| **retrieval-bench** | Synthetic + live corpus retrieval precision/coverage gates | P@3 < 0.67 or live coverage < 0.80 |
| **security-toolchain** | Full `security-gate.sh` (semgrep, gosec, gitleaks, trivy, hadolint) | Scanner findings or toolchain install flake |
| **knowledge-cycle** | Deduped compile + dream-cycle + Athena follow-up sharing one substrate (`scripts/nightly-knowledge-cycle.sh`); corpus-empty precondition skip per `f-2026-04-30-002`; single `nightly-knowledge-cycle` triage artifact replaces three (compile-report, dream-cycle-report, Athena) — `soc-2xmg` | Compile health gate fails, dream-cycle proof regresses, or substrate inputs missing |

**Knowledge-cycle precondition:** the `knowledge-cycle` job calls `scripts/nightly-knowledge-cycle.sh precondition` before any compile/dream/Athena stage. When `total_citations_in_window == 0 && total_artifacts > 0`, the cycle SKIPs every downstream stage with reason `corpus-dormant` rather than failing three separate jobs on the same dormant-corpus condition. Override with `NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1` for diagnostic runs. Static Validation (`#39` in the CI failure ranking) remains in its own `static-validation` job by design — see plan `2026-05-03-ci-failures-1-40-handling.md` §nightly-knowledge-cycle-dedupe.

### Key Constraints Agents Must Follow

**No symlinks.** The plugin-load-test rejects ALL symlinks. If you need the same file in multiple skill `references/` dirs, **copy the file** — do not symlink.

**Do not track repo-root `.agents/`.** `.agents/` is local agent runtime state for Codex, Claude, AgentOps, and other agents. It can churn and may contain sensitive session context. The pre-push and CI gates run `scripts/check-no-tracked-agents.sh`; use durable docs or release artifacts outside `.agents/` for anything that belongs in git history.

**Treat `ao codex start/stop` lifecycle shims as deprecated.** Do not use `ao codex start`, `ao codex stop`, `ao codex ensure-start`, or `ao codex ensure-stop` for routine validation, RPI closeout, or merge work. These commands are legacy lifecycle shims and can mutate local agent state. Prefer non-mutating validation commands, `bd` notes, and tracked docs/runbooks. If a targeted lifecycle task truly requires one of these commands, run it in an isolated/disposable worktree with isolated agent state, then verify `git status` and `git ls-files .agents` before committing.

**Merge older `.agents` branches defensively.** Branches created before the no-tracked-`.agents` policy may contain RPI packets, summaries, or runtime metadata under `.agents/`. During merge conflict resolution, keep `main`'s deletion of repo-root `.agents/*` unless the user explicitly asks to reintroduce a tracked artifact. After resolving conflicts, run `git ls-files .agents` and expect no output.

**Skill counts must be synced.** When adding or removing a skill directory, run:
```bash
scripts/sync-skill-counts.sh
```
This updates counts in SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md.

**Every reference file must be linked.** If a file exists in a skill's `references/` directory, the skill's SKILL.md must link to it via markdown link or Read instruction. Run `heal.sh --strict` to check.

**Codex checked-in artifacts are manually maintained, with manifest/marker provenance metadata used for validation.** If `skills-codex/` still contains Claude-era primitives (`TaskCreate`, `TaskList`, `Tool: Task`), Claude backend refs, or duplicated runtime rewrites, run:
```bash
bash scripts/audit-codex-parity.sh --skill <name>
```
Then update the canonical source and, when the Codex runtime copy itself must change, patch `skills-codex/<name>/` and/or add a durable override under `skills-codex-overrides/<name>/`. Finish by running `bash scripts/refresh-codex-artifacts.sh --scope worktree` and re-running the audit.

**Embedded hooks must stay in sync.** After editing anything in `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`, run:
```bash
cd cli && make sync-hooks
```

**CLI docs must stay in sync.** After adding/changing CLI commands or flags, run:
```bash
scripts/generate-cli-reference.sh
```

**Contracts must be catalogued.** When adding files to `docs/contracts/`, add a corresponding entry in `docs/documentation-index.md`. The contract gate discovers files dynamically but checks for orphans.

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
4. **PUSH TO REMOTE** - Git push is mandatory; bd push is conditional:
   ```bash
   git pull --rebase
   bd vc status
   bd dolt commit -m "tracker: <summary>"  # if tracker changes are pending
   bd dolt remote list
   bd dolt push  # only if a real Dolt remote is configured
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
- If `bd dolt push` says no remote is configured, do not treat that as a
  session failure. Record it as unavailable, then continue with the mandatory
  Git push. See [bd server-mode tracker closeout](docs/runbooks/bd-server-mode-closeout.md).

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
