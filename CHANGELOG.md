# Changelog

All notable changes to the AgentOps marketplace will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.13.2] - 2026-02-21

### Added

- **Claude Code best-practices alignment** — All 53 skills now use appropriate frontmatter: `disable-model-invocation` on dangerous skills, `user-invocable: false` on background skills, `allowed-tools` on read-only skills, `model: haiku` on lightweight skills, `context: fork` on heavy skills.
- **Plugin agents directory** — New `agents/code-reviewer.md` and `agents/researcher.md` distributed via the plugin for use as Claude Code subagents.
- **Hook event coverage** — Adopted Claude Code v2.1.47–v2.1.50 hook events; extracted 10 inline hook commands into named scripts under `hooks/`.
- **MemRL packet v1 lifecycle** — Added packet lifecycle contract with canonical identity joins for the memory reinforcement learning subsystem.
- **Safe worktree GC** — Added worktree garbage collection with serialized session-end maintenance to prevent concurrent cleanup races.
- **Proactive improvement agenda** — `/retro` and `/post-mortem` now require a structured improvement proposal with every retrospective.

### Changed

- **Local CI release gate parallelized** — Restructured `ci-local-release.sh` into phased parallel execution with new `--fast` flag (~30s vs ~100s+).
- **Skill structure reference v2.0.0** — Updated `skill-structure.md` to document all 10 Claude Code frontmatter fields with upstream source attribution.
- **Evolve skill split** — Reduced `skills/evolve/SKILL.md` from 778 to 493 lines by extracting cycle-history reference.
- **Hook CPU guardrails** — Bounded batch feedback loops and hardened hook robustness for faster local CI.
- **Auto-promotion hardening** — Hardened promotion pipeline with legacy capture migration support.
- **Cached-memory flow** — Aligned learn-ingest and staged promotion flows for knowledge flywheel consistency.

### Fixed

- **CI false positives** — Excluded `.tmp/` from secret and dangerous-pattern scans; fixed dangerous-pattern scan for CI test fixtures.
- **CLI reference generation** — Made UTC-stable to prevent timezone-dependent diffs.
- **Hook tech debt** — Addressed post-mortem findings on hook reliability and install paths.
- **RPI stale cleanup** — Hardened stale-run detection and startup state persistence.
- **Reverse-engineer docs** — Improved detection and validation for reverse-engineer-rpi workflows.

### Removed

- **Stale docs/anthropic-skills-guide.md** — Deleted local copy of upstream Claude Code docs that caused false pre-mortem failures. Replaced with link to `code.claude.com/docs/en/skills`.

## [2.13.1] - 2026-02-21

### Fixed

- **Hotfix release** — Bump plugin manifest and marketplace versions to trigger binary rebuild and marketplace update.

## [2.13.0] - 2026-02-20

### Added

- **Context-guard workflow protections** — Added assignment mapping plus proactive handoff and one-shot recovery markers to reduce stale-session drift and improve recovery behavior.
- **Claude team context enhancements** — Expanded `ao` support for Claude team context handling and related docs.
- **Security-suite primitives** — Added `/security-suite` for composable binary assurance (static/dynamic collection, contract capture, baseline drift, and policy gating).
- **RPI stale-run cleanup tooling** — Added stale run detection, `ao rpi cleanup`, and configurable worktree cleanup mode.
- **Plan skill symbol-level specs** — `/plan` now generates file inventory tables, exact function references with line numbers, inline code blocks, named test functions, and verification procedures for implementation-ready specs.
- **Inline quick-start examples** — Added working examples to `/evolve` and `/vibe` skill docs for faster onboarding.
- **Destructive path tests** — Added test coverage for worktree path validation edge cases.
- **Skill lint references dir** — Added `references/` directory for skill lint tooling.

### Changed

- **Install guidance priorities** — Claude marketplace/plugin install is now the first install path, with `npx skills` positioned as the cross-agent path for Codex/Cursor/OpenCode.
- **Homebrew update consistency** — Standardized upgrade messaging to `brew update && brew upgrade agentops` across docs/release notes and improved installer fallback messaging.
- **Repository hygiene** — `.agents/` artifacts are now fully ignored to keep generated local state out of tracked source history.
- **Release gating model** — Release validation moved to local CI (`scripts/ci-local-release.sh`) while GitHub Actions release workflow is now publisher-only.
- **Validate script compatibility** — Updated validate scripts for tier renames and ruff output format changes.
- **npx skills troubleshooting** — Added troubleshooting guidance for `npx skills update` issues.

### Fixed

- **Claude plugin marketplace compatibility** — Removed unsupported `"$schema"` key in plugin manifest so `claude plugin install agentops@agentops-marketplace` succeeds.
- **RPI phased reliability** — Hardened stream startup/fallback behavior and resolved related validation regressions.
- **Hook/runtime regressions** — Fixed hook-coverage compile regressions and restored install symbols used by `ao init`/`ao doctor`.
- **CLI quality debt** — Addressed lint/staticcheck regressions and reduced `runInit` complexity below CI threshold.
- **RPI cleanup reliability** — Persisted terminal metadata and validated stale-run worktree paths before cleanup.
- **Worktree path validation** — Tightened path validation to prevent destructive operations on invalid worktree paths.

## [2.12.1] - 2026-02-20

### Fixed

- **Claude plugin marketplace install compatibility** — Removed unsupported `"$schema"` key from `.claude-plugin/plugin.json`, which caused `claude plugin install agentops@agentops-marketplace` to fail manifest validation.

## [2.12.0] - 2026-02-20

### Added

- **RPI phased engine enhancements** — Added executor backend routing (swarm-first), deterministic queue semantics, run-scoped artifact contracts, step aliases, and stricter lifecycle/failure propagation.
- **MemRL policy foundation** — Added deterministic MemRL policy package and AO export contract.
- **Validation and security gates** — Added changed-scope Go race pre-push gating, markdownlint baseline CI gating, and blocking gitleaks security gate integration.
- **Flywheel/knowledge coverage** — Added ao-free fallbacks, cross-repo knowledge wiring, and stronger flywheel consistency diagnostics.

### Changed

- **RPI model alignment** — Standardized docs and tests around the 3-phase RPI model.
- **Hook install defaults** — `ao init --hooks` now defaults to full 8-event coverage, with explicit minimal mode available.
- **Docs and routing guidance** — Expanded README/skill routing guidance and reverse-engineering documentation.

### Fixed

- **Hook runtime/install regression** — `chain-parser.sh` is now shipped with installs, and hook sourcing degrades gracefully if helper files are missing.
- **RPI reliability** — Fixed dry-run preflight behavior, stream watchdog timeout handling, and pre-mortem default mode fallback.
- **Knowledge/search correctness** — Research now searches all knowledge locations; retrieval effectiveness calculation was corrected.
- **Security/CI stability** — Fixed gitleaks install path, resolved semgrep false positives, and re-enabled the blocking security gate.
- **CLI correctness** — Fixed `ao doctor` plugin path checks, pool promotion filename-collision handling, and schema test repo-pollution behavior.

## [2.11.0] - 2026-02-18

### Added

- **Embedded hooks** — ao binary now ships hooks via Go `embed`, eliminating external hook file dependencies at runtime.
- **reverse-engineer-rpi enhancements** — Comparison report generation, binary mode self-test assertions, binary CLI surface spec with registry enrichment, and `--help` capture for Go CLI detection.
- **Codex/PR/OSS skills** — New `pr-research`, `pr-plan`, `pr-implement`, `pr-validate`, `pr-prep`, `pr-retro`, `oss-docs`, and `openai-docs` skills.
- **`/security` skill** — Continuous security scanning and release gating with `security-gate.sh`.

### Fixed

- **`ao pool auto-promote`** — Defaults threshold corrected to 7 days.
- **`ao pool ingest`** — Processed files now moved to `processed/` directory.
- **`ao doctor` search index** — Fixed wrong path and remediation command for `checkSearchIndex`.
- **`doctor_test.go`** — Search index test now uses `IndexDir`/`IndexFileName` constants instead of hardcoded paths.
- **oss-docs** — Fixed broken links in reference template.
- **Markdown standards** — Fixed example links that broke the doc-release link checker.

## [2.10.2] - 2026-02-18

### Added

- **`--upstream-ref` flag** (reverse-engineer-rpi) — Pin repo clones to a specific commit, tag, or branch. Emits `clone-metadata.json` with resolved SHA for reproducibility.
- **Python CLI detection** (reverse-engineer-rpi) — Detects Click, argparse, and Typer CLIs via `pyproject.toml` (PEP 621 + Poetry) and `setup.cfg` entry_points.
- **Go CLI detection** (reverse-engineer-rpi) — Detects Go CLIs via `go.mod` + `cmd/*/main.go` pattern with Cobra and stdlib flag framework detection.
- **Self-test improvements** (reverse-engineer-rpi) — Negative test for invalid mode, upstream-ref pinning test with `file://` protocol, multi-language graceful degradation test.

### Changed

- **Tuning defaults** — Sonnet default for judges and workers, opus reserved for leads. `--quick` default for all gates at low+medium complexity. Cap judges at 2, consolidate product to 1 judge, quick pre-mortem.
- **`ao rpi phased`** — Consolidated from 6 phases to 3.
- **Issue templates** — Professionalized docs, added glossary updates, modernized templates.
- **reverse-engineer-rpi SKILL.md** — Documented `--upstream-ref`, `clone-metadata.json` output, Python/Go CLI detection, pinned clone example.

### Fixed

- **`ao rpi phased` spawning** — Reliability fix for agent sessions.
- **Lint failures** — Resolved 3 lint failures, added lint hook and config tests.
- **Tuning validation** — Added 33-assertion tuning defaults validation suite.

## [2.10.1] - 2026-02-17

### Added

- **Backend reference docs** — Concrete tool call examples for each spawn backend: Claude native teams (`TeamCreate` + `SendMessage`), Codex sub-agents/CLI, background tasks fallback, and inline degraded mode. Shared at `skills/shared/references/backend-*.md`.

### Changed

- **README** — Reframed Flow as orchestration + single-piece flow, Feedback and Learning with Phoenix Project vibes, sell `ao rpi` phased as the killer feature. Streamlined install, condensed intro. Aligned tagline across all manifests.
- **Quickstart** — Added rpi/product/goals/evolve progression to Step 7.
- **Skills rewrite** — Runtime-agnostic capability contracts replace hardcoded tool names. Council, swarm, and research skills now load backend-specific references after detection.
- **Goals skill** — New `ao goals` CLI commands wired into the skills layer.

### Fixed

- **Cross-skill reference linting** — `lint-skills.sh` now resolves `skills/*/references/` paths from repo root instead of failing on cross-skill references.
- **Swarm validation** — Updated `validate.sh` to check for backend reference documentation pattern instead of requiring inline `TeamCreate`/`spawn_agent` strings.
- **Council context explosion** — Banned background tasks, patched v2.10.0 release notes.
- **Cache eviction** — Three-wave implementation: passive tracking with decay write-back, composite eviction criteria with cache health goals, SessionEnd hook wiring.
- **Stars badge** — Fixed shields.io social variant rendering.

## [2.10.0] - 2026-02-17

### Added

- **`/brainstorm` skill** — Structured idea exploration before `/plan`. Four phases: assess clarity, understand the idea, explore approaches, capture design decisions. Output goes to `.agents/brainstorm/`.
- **`/heal-skill` skill** — Automated skill hygiene checker. Detects missing frontmatter, name mismatches, unlinked references, dead references, and empty directories. Supports `--check` (report) and `--fix` (auto-repair) modes.
- **`/converter` skill** — Cross-platform skill converter pipeline (parse → convert → write). Codex target produces `SKILL.md` + `prompt.md` with 1024-char description truncation. Cursor target produces `.mdc` rule files with 100KB budget-fitting and optional `mcp.json` for MCP-referencing skills. Test target dumps raw SkillBundle for inspection.
- **`/readme` skill** — Gold-standard README generation with council validation.
- **Named council perspectives** — `--perspectives="a,b,c"` for free-form judge viewpoints, `--perspectives-file=<path>` for YAML-defined perspectives with focus descriptions. Perspectives override presets when both are specified.
- **`doc-review` council preset** — Four perspectives for documentation quality review: clarity-editor, accuracy-verifier, completeness-auditor, audience-advocate.
- **PRODUCT.md** — Product definition document with mission, personas, value props, and roadmap.
- **`reverse-engineer-rpi` skill** — Reverse-engineer a product into feature inventory, registry, code map, and specs. Supports repo mode, binary mode (with `--authorized` guard), and security audit mode.

### Changed

- **README rewritten** — Lead with the problem ("agents forget between sessions"), acknowledge prior art, add trust block near install, collapse depth with details blocks. Terminal transcripts instead of assertions.
- **Skill count** — 39 → 42 skills (32 user-facing, 10 internal). All counts synchronized across 8 files with CI validation.
- **Council SKILL.md trimmed** — Consolidated named perspectives section to stay under 550-line limit. Preset details moved to `references/personas.md`.
- **Council context budget** — Judges write full analysis to output files, send only `{verdict, confidence, file}` via message. Consolidation runs inline as team lead (no chairman agent). Debate R2 sends verdict summaries instead of full JSON.
- **Background task ban** — `Task(run_in_background=true)` replaced with foreground `Task()` across council, swarm, research, shared, SKILL-TIERS, and validation-contract. Causes Claude instability. `Bash(run_in_background=true)` for Codex CLI processes unchanged.
- **`reverse-engineer-rpi`** — Increased 1:1 fidelity for CLI and artifact surface matching.

### Fixed

- **Skill count validator** — Excluded `.tmp` directories from `find` count to prevent test artifacts inflating totals.
- **`brainstorm` trigger collision** — Added `brainstorm` overlap between `/brainstorm` and `/council brainstorm` to alias collision allowlist.
- **`readme` skill missing from SKILL-TIERS.md** — Added to user-facing skills table.
- **Validation test failures** — Fixed heal-skill example text triggering false-positive dead-ref detection, council line count exceeding limit.
- **Council context explosion** — Spawning 7+ judges flooded lead's context window via SendMessage. Judges now send minimal 4-line completion signals.

## [2.9.3] - 2026-02-16

### Changed

- **Quickstart skill menu** — Added 10 missing skills (`/release`, `/rpi`, `/evolve`, `/status`, `/recover`, `/product`, `/codex-team`) to the onboarding orientation. New users now see all 26 user-facing skills.
- **Using-agentops skill catalog** — Updated Available Skills table from 17 to all 26 user-facing skills with natural language triggers for each.
- **Install instructions** — All `brew install` commands now include the Homebrew tap prefix (`boshu2/agentops`) so new users don't hit "formula not found". Updated `ao hooks install` references to `ao init --hooks`.
- **Troubleshooting** — Fixed incorrect hooks.json path (was `.claude-plugin/hooks.json`, now `hooks/hooks.json`).

### Fixed

- **Release pipeline empty body** — GoReleaser created an "untagged" release with empty body when a draft release already existed for the tag. Workflow now deletes any existing release before GoReleaser runs.
- **Release skill skip draft creation** — `/release` no longer creates a draft GitHub Release via `gh release create`. GoReleaser handles release creation in CI; local drafts conflict with it.
- **Goal count test** — `validate-goal-count.sh` no longer crashes when README doesn't contain a hardcoded goal count.

## [2.9.2] - 2026-02-16

### Added

- **`ao init` consolidated entry point** — Single command to set up a repo for AgentOps: creates all `.agents/` directories, manages `.gitignore` (with `--stealth` for `.git/info/exclude`), and optionally registers hooks via `--hooks`. Idempotent, dry-run capable. 12 unit tests.
- **Auto-gitignore `.agents/`** — Session-start hook automatically adds `.agents/` to `.gitignore` and creates a nested deny-all `.agents/.gitignore`. Dangerous-git-guard warns when `.agents/` files are staged.

### Fixed

- **Symlink file-read in standards-injector** — Rejects symlinked standards files and verifies resolved path stays within `skills/standards/references/`.
- **Non-git source directory for hook install** — `ao hooks install --full` now errors (not warns) when source directory is not a git root.
- **Command injection in task-validation-gate** — Blocked newlines and quotes in restricted command execution.
- **Dir list sync** — `ao init` and `session-start.sh` now create identical directory sets (added `.agents/plans/` and `.agents/rpi/`).
- **Version badge** — README badge updated from 2.9.0 to 2.9.2.
- **Release notes quality** — `extract-release-notes.sh` now requires a CHANGELOG entry (no more commit-dump fallback) and supports curated highlights from `.agents/releases/`.
- **Release pipeline crash in CI** — `extract-release-notes.sh` failed with `set -euo pipefail` when `.agents/releases/` directory didn't exist (gitignored). Added `|| true` to the `find` pipeline.

### Changed

- **CONTRIBUTING.md** — Added `ao init` to contributor setup instructions.

## [2.9.1] - 2026-02-16

### Added

- **Knowledge cache core** — `ao index` for citation-backed search index, `ao metrics cite-report` for citation health, `ao maturity --expire` for stale artifact cleanup. 530+ new test assertions.
- **`/update` skill** — One-command skill reinstall (`npx skills@latest add boshu2/agentops --all -g`).
- **`/evolve` enhancements** — Added references for artifacts and examples.

### Changed

- **README** — Added town planning example to See It Work section.
- **License** — Corrected from MIT to Apache-2.0 in goreleaser and brew formula.
- **Goals fitness** — Trimmed skills, hardened tests, improved docs across 6 epics.
- **.agents/ directory** — Now gitignored; removed tracked session data and PII.

### Fixed

- **Hook security hardening** — Removed `npx` and `bash` from task-validation-gate allowlist (closed RCE vector), sourced hook-helpers.sh from plugin install dir instead of repo root (closed confused-deputy code execution), removed kill switch self-advertising from push-gate and pre-mortem-gate stderr, added `git restore` to dangerous-git-guard blocklist.
- **Code injection via crafted filenames** — Hardened prescan.sh filename handling.
- **Stale counts** — Corrected GOALS.yaml and README skill/goal counts.
- **INDEX.md exclusion** — Excluded from learning-format-compliance goal check.

## [2.9.0] - 2026-02-16

### Added

- **`/recover` skill** — Post-compaction context recovery. Detects in-progress RPI and evolve sessions, loads knowledge, shows recent work and pending tasks. Behavioral validation with 6 checks.
- **`ao vibe-check` CLI command** — Full vibecheck Go package: timeline parser, 5 health metrics, 4 detectors (stagnation, thrashing, drift, regression), orchestrator, and Cobra command with `--json`/`--markdown`/`--since`/`--full` flags. 60+ tests. Session-end hook auto-captures vibe snapshots.
- **`ao rpi status` CLI command** — Dashboard showing RPI phase progress, current state, and recent activity.
- **`scripts/sync-skill-counts.sh`** — Single-source-of-truth skill count updater. Derives counts from disk + SKILL-TIERS.md, patches all 11 hardcoded locations across 6 files. Supports `--check` mode for CI dry-run. Integrated into doc-release-gate CI.
- **`next-work.jsonl` schema v1.2** — Added `target_repo` field for cross-rig work harvesting.
- **Git worktree isolation for `ao rpi phased`** — Each phase runs in its own worktree, preventing cross-phase contamination.
- **Test infrastructure expansion** — CLI smoke tests, skill invocation tests, hook-chain tests, structural assertions, CLI coverage to 12 commands, shared test colors, binary caching, dynamic counts, integration test wiring.
- **GOALS.yaml restructure** — Goal-count goal, release size cap, goal-count validator script.

### Changed

- **v3 roadmap** — Retro trajectory analysis documented future direction based on evolve session learnings.
- **README** — Added worktree isolation to Phased RPI section.

### Fixed

- **Crank context explosion** — Replaced per-wave vibe gate (9,407 lines loaded per wave) with lightweight inline acceptance check. Worker results now write to disk instead of flooding orchestrator context.
- **Skill count drift** — Updated all 11 locations for /recover addition (35 total: 25 user-facing, 10 internal). Created sync script to prevent recurrence.
- **Shellcheck compliance** — Added directive to colors.sh (SC2148).
- **CHANGELOG goal count** — Fixed active drift (42→44).
- **Worktree merge retry** — Pre-merge dirty check now retries for parallel worktree merges.
- **CLAUDECODE env var** — Stripped from `ao rpi phased` child sessions to prevent interference.

## [2.8.0] - 2026-02-15

### Added

- **`ao rpi phased` command** — Solves the context window problem with `/rpi`. Currently `/rpi` runs all 6 phases in one session — the context fills up and you rely on compaction being lossless (it mostly is, since real state lives in plans and beads issues, but you're hoping). `ao rpi phased` runs each phase in its own fresh Claude session. The Go CLI carries state between phases via filesystem artifacts (goal, verdicts, phase summaries), so each session starts clean with exactly the context it needs. Three modes: hands-free (`ao rpi phased "goal"`), interactive with human gates at research/plan (`--interactive`), or resume from any phase (`--from=crank`). Also supports `--fast-path` for small epics, `--max-retries` for retry loops with structured findings. Per-phase token budgets prevent compaction. Runnable by a human from the terminal or by an agent — it's a CLI command, not a skill. 33 unit tests.
- **Wire 10 ao CLI commands into skills/hooks** — `ao maturity --scan` on SessionStart; `ao session-outcome`, `ao feedback-loop`, `ao task-sync`, `ao batch-feedback` on SessionEnd; `ao badge` and `ao task-status` in `/status`; `ao maturity --scan`, `ao promote-anti-patterns`, `ao badge` in `/flywheel`; `ao feedback-loop`, `ao session-outcome`, `ao temper validate` in `/post-mortem`; `ao task-feedback` in `/retro`. Every ao command now has an automated caller — only `ao rpi phased`, `ao search`, and `ao demo` remain human-typed.
- **3-layer compaction prevention tests** — Layer 1: `test-token-budgets.sh` statically checks per-skill and SessionStart token budgets without running the CLI. Layer 2: `test-no-compaction.sh` runs 3 scenarios asserting no `compact_boundary` events and <60% context utilization. Layer 3: reusable `assert_context_under_60pct` helper parsing `stream-json` peak token usage.
- **.agents/ directory standardization** — RPI phase summaries now use dated filenames to prevent silent overwrites between cycles. `.agents/README.md` rewritten documenting all 25+ directories with purpose, writers, and retention policies. New `scripts/prune-agents.sh` enforces per-directory retention (dry-run default). SessionStart hook warns when `.agents/` exceeds 500 files.

### Changed

- **README CLI section** — Reframed: the ao CLI is plumbing invoked by skills and hooks, not a human interface. Added automation map showing which skills/hooks call which commands. Added CASS search example to "See It Work" section.
- **CLAUDE.md** — Synced stale metadata, added 9 development pitfalls (verified mechanically, go test validation, TaskCreate ordering, lead-only commits, grep call sites, no hardcoded counts, full corpus validation, tight regex, file disjointness).
- **Crank SKILL.md slimmed** — Context-discipline goal added to GOALS.yaml.

## [2.7.1] - 2026-02-14

### Changed

- **ao search simplified** — Removed index-based search path; CASS now opt-in (`--cass`) instead of default, reducing complexity.
- **README and PRODUCT.md** — Aligned with context orchestration framing.
- **Architecture docs** — Updated framing, fixed broken links, added export-constraints stub.

### Fixed

- **Crank pre-mortem gate** — Epics with 3+ issues now require pre-mortem evidence before cranking, codifying the pattern validated across 7 consecutive epics.
- **Integration test coverage** — Added tests for `ao doctor`, `ao pool`, `ao search`, and `ao status` commands.

## [2.7.0] - 2026-02-13

### Added

- **ao CLI improvements (ag-z9ok)** — 11-issue epic: generic worker pool for concurrent file processing, inverted search index, shared table formatter, CASS as default search mode, pool pagination (--offset/--limit), progress indicators for inject/search, dry-run flags for destructive pool operations, expanded doctor (9 health checks), unified status + flywheel display.
- **README overhaul** — Dedicated ao CLI section, adoption path diagram, personas, 4 pillars framework, competitive comparison, /evolve expansion.
- **Deliberation consensus alignment** — 4 pillars, new mission statement, 44 fitness goals in GOALS.yaml.

### Changed

- **`/evolve` runs forever by default** — Removed 10-cycle cap; loop runs until kill switch or stagnation (3 idle cycles). Use `--max-cycles=N` for explicit cap.
- **Terminology alignment** — Replaced 'autonomous' with 'hands-free'/'independently' across reference docs.
- **Council verdict schema v2** — Judges now emit FIX/WHY/REF fields per finding, env manifest, flywheel validation.

### Fixed

- **GOALS.yaml reliability** — Fixed check patterns for tier and schema validation, annotated infrastructure goal sections, fixed fragile find pattern.
- **Learning-format compliance** — Widened check to all files, backfilled 56 legacy learnings to match format.
- **Vibe council findings** — Restored CASS default (reverted by parallel agent), added pool pagination CLI flags, replaced dead tabwriter import with shared formatter.
- **Consolidation fallback** — Aligned fallback behavior + updated packet schema_version example.
- **Structured failure feedback** — Retry loops now include formatted findings for actionable guidance.

## [2.6.0] - 2026-02-13

### Added

- **Micro-epic fast path for `/rpi`** — Issues with 3 or fewer children skip `/pre-mortem` and execute sequentially without `/swarm`, reducing overhead for small epics.
- **Examples + Troubleshooting for all 34 skills** — Every skill now includes usage examples and common troubleshooting guidance, following the Anthropic official skills guide structure.
- **`--test-first` scaffolding** — `/crank --test-first` wave model with spec-first TDD templates, RED gate enforcement, and `test-first-mode.md` reference.
- **Headless Codex team-runner** — `lib/scripts/team-runner.sh` spawns parallel Codex agents with JSONL event watching, structured output via `worker-output.json` and `team-spec.json` schemas.
- **5 mission-fitness goals** — GOALS.yaml expanded with mission-aligned fitness targets; stub `ao export-constraints` command added.

### Changed

- **Anthropic skills guide alignment** — All 34 skills restructured to match the official Anthropic skills guide format (YAML frontmatter, triggers, progressive disclosure).
- **AO-OL bridge contracts updated** — Bridge contracts document aligned with Olympus codebase; ownership matrix and next-steps plan added.

### Fixed

- **Mandatory plan audit + scoped lint detection** — `/plan` now enforces audit step; lint troubleshooting detection scoped to avoid false positives.
- **Vibe findings remediation** — Fixed shell quoting, validation edge cases, cleanup ordering, and portability issues across hook and test scripts.

## [2.5.1] - 2026-02-12

### Added

- **100% behavioral validate.sh coverage** — All 34 skills now have validate.sh scripts with behavioral contracts (~280 total checks across structural, semantic, and mode-preservation dimensions).
- **RPI large-repo context windowing** — `scripts/rpi/` shard generator, progress tracker, bounded shard runner, and `context-window-contract.sh` for deterministic full-file coverage under bounded context budgets.
- **Incident runbook** — `docs/INCIDENT-RUNBOOK.md` with consumer recovery procedures, emergency kill switches, rollback escalation levels, and root cause analysis workflows.
- **cmd/ao test coverage boost** — 37 new test functions across 12 files (cmd/ao 25.1%→30.5%, overall average 85.7%→86.1%).
- **Hook integration tests** — 100 tests across all 12 hook scripts for error recovery, auto-handoff, and kill switch enforcement.
- **GOALS.yaml expanded to 20 goals** — Added `behavioral-skill-contracts`, `hook-tests-pass`, and `incident-runbook-exists`.

### Changed

- **`--test-first` flag passthrough** — `/rpi --test-first` and `/evolve --test-first` now pass the flag through to `/crank` for spec-first TDD across the full lifecycle.
- **Evolve full-fitness regression gate** — Step 5 re-checks ALL goals (not just target) after each cycle. Multi-commit revert using cycle start SHA. Continuous fitness values in snapshots.
- **4 large skills split into references/** — Progressive disclosure extraction for inbox, trace, rpi, and evolve.
- **Goals schema docs** — `goals-schema.md` updated with continuous values format.

### Fixed

- **Hook preflight expanded** — Now validates all 12 hooks (was partial). Fixed false positive detection for kill switches.
- **SKILLS.md completeness** — Added 12 missing skill entries, fixed README skill count.
- **Smoke test false positive** — Fixed ratchet coverage test that was masking real failures.

## [2.5.0] - 2026-02-12

### Added

- **`/evolve` skill** — Autonomous fitness-scored improvement loop. Measures `GOALS.yaml` fitness, picks the worst-scoring goal, runs `/rpi` to fix it, re-measures. Compounds via the knowledge flywheel — each cycle loads learnings from all prior cycles. Includes kill switches (`~/.config/evolve/KILL`, `.agents/evolve/STOP`), `--dry-run` mode, and harvested-work queue from `next-work.jsonl`.
- **`/product` skill** — Interactive interview that generates a filled-in `PRODUCT.md` covering mission, personas, value propositions, and competitive landscape. New `product` and `developer-experience` council presets.
- **`/crank --test-first` mode** — Spec-first TDD wave model. Specs and tests are written in early waves (GREEN), implementation follows. Includes contract templates, BLOCKED recovery, RED gate enforcement, strict category validation, and TaskCreate examples reference.
- **Pre-mortem gate hook** — `pre-mortem-gate.sh` enforces pre-mortem validation before `/crank` execution. Integrated with smoke tests.
- **Hook error recovery and auto-handoff** — All hook failure paths now include error recovery suggestions and auto-handoff context. Shared `lib/hook-helpers.sh` with `write_failure()`, `schema_version`, and ISO timestamps. Dual-output pattern applied to all failure paths.
- **Spec-completeness judge** — New council judge validates specs for completeness with cross-cutting constraint injection. Domain-neutrality check included.
- **`/plan` SDD boundaries** — Solution Design Document patterns with conformance checks, new `sdd-patterns.md` reference.
- **`/post-mortem` checkpoint policy** — Preflight validator with metadata verification reference.
- **GOALS.yaml fitness suite** — 11 goals for automated repo health measurement. Release pre-flight manifest check.
- **Hook integration tests** — 10 new test cases for error recovery and auto-handoff hooks. Smoke test for `--test-first` flow (286 lines).

### Changed

- **`/research` runtime-agnostic** — Works across Claude and Codex runtimes. 6-tier discovery enrichment for exploration prompts.
- **Domain-neutral language** — Replaced AgentOps-specific branding with domain-neutral terms across all skills.
- **Flywheel taxonomy** — Renamed `skill-enhancement` → `process-improvement` across the knowledge flywheel.
- **`/evolve` teardown** — Auto-runs `/post-mortem` on the full evolution session before writing summary. Added `post-mortem` as explicit dependency.
- **Skills source-of-truth pattern** — `CLAUDE.md` now documents that this repo is the canonical source for skills.

### Fixed

- **Skills badge count** — Updated to 34 to match actual skill count.
- **Pre-mortem judge count** — Corrected comment from 3 → 4.
- **Release pipeline idempotency** — Deletes existing assets before publish to avoid conflicts.

## [2.4.0] - 2026-02-11

### Added

- **Knowledge flywheel wiring across RPI skills** — `/plan`, `/pre-mortem`, and `/vibe` now search `ao` for prior learnings before council dispatch (`ao search` with guard + fallback). `/plan` and `/pre-mortem` record ratchet progress after completion. Session-start hook runs `ao extract` to close the forge-extract loop.
- **Retro remediation hardening sweep** — JSON Schema validation for plugin and hooks manifests (`schemas/`), Dependabot config for Go + GitHub Actions, release workflow stub, hook preflight validator, expanded hook integration tests (170+ lines), doc-release validation script.
- **`ao forge markdown` subcommand** — Forge now supports mining `.md` files in addition to `.jsonl` transcripts (256+ LOC in `forge.go`).
- **Task-validation-gate command allowlist** — Replaced `eval()` shell injection vector with allowlist-based execution; only `go`, `pytest`, `npm`, `npx`, `make`, `bash` permitted.
- **Self-perpetuating knowledge flywheel** — Post-mortem now synthesizes process improvement proposals from retro learnings (Step 5.5), always suggests a ready-to-copy next `/rpi` command (Step 7), and harvests `process-improvement` items alongside tech-debt (Step 8). `/rpi` Step Final always shows the flywheel section with the next suggested cycle. Every run ends by pointing at the next one — the system grows knowledge with each use.

### Changed

- **README onboarding and engagement** — Expanded install flow with plugin adoption path, swarm/Ralph loop backend support clarification, engagement funnel improvements, issue routing via CONTRIBUTING.md and GitHub issue templates.
- **Smoke and e2e test improvements** — Timeout shims, environment variable documentation in `docs/reference.md`, CI workflow matrix improvements.

### Fixed

- **8 vibe WARN findings from retro remediation** — CI validate workflow permissions, schema `additionalProperties` consistency, hook preflight script exit codes, smoke test portability, tasks-sync error handling.
- **Command injection in task-validation-gate.sh** — Fixed `eval()` on untrusted metadata (shipped alongside `ao forge markdown` in `acc49f8`).

### Removed

- **Multi-agent preflight script** — Removed `scripts/multi-agent-preflight.sh` (268 lines) and all distributed-mode references to it from crank, implement, and swarm skills. Superseded by native team pre-flight in each skill.
- **Low-value vibe fixes reverted** — Removed env var docs and timeout shim additions that added complexity without proportional value.

## [2.3.0] - 2026-02-11

### Added

- **Runtime-native spawn backend contract** — `skills/shared/SKILL.md` now defines backend selection order for orchestration skills: `spawn_agent` → `TeamCreate` → `Task(run_in_background=true)`, plus operation mapping for spawn/wait/retry/cleanup.
- **Codex sub-agent execution reference flows** — `skills/swarm/references/local-mode.md` and `skills/council/references/cli-spawning.md` now include concrete `spawn_agent`, `wait`, `send_input`, and `close_agent` patterns.

### Changed

- **Swarm/Council/Crank/Codex-Team runtime universality** — `skills/swarm/SKILL.md`, `skills/council/SKILL.md`, `skills/crank/SKILL.md`, and `skills/codex-team/SKILL.md` now document runtime-native orchestration (Codex sub-agents or Claude teams with fallback).
- **Council validator generalized** — `skills/council/scripts/validate-council.sh` now validates judge counts without Claude-specific assumptions.
- **Core docs updated for Codex sub-agents** — `README.md`, `docs/reference.md`, and `docs/ARCHITECTURE.md` now describe runtime-native backend selection and coordination behavior.
- **Swarm validator updated** — `skills/swarm/scripts/validate.sh` now validates runtime backend selection and cleanup lifecycle documentation.

### Fixed

- **Release skill doc-link integrity** — Fixed `skills/release/SKILL.md` changelog link template to satisfy doc link validation.

## [2.2.1] - 2026-02-10

### Added

- **`ao pool ingest`** — New command to ingest `.agents/knowledge/pending/*.md` learnings into `.agents/pool/` candidates with stable IDs and preserved timestamps.
- **`ao flywheel close-loop`** — New orchestration command chaining: `pool ingest → auto-promote (promote) → promote-anti-patterns → store --categorize`.
- **Codex session-end equivalent** — `~/.codex/agentops/.codex/agentops-codex session-end` for manual close-loop on Codex (since Codex has no native Stop hook).

### Changed

- **`ao pool auto-promote`** — Added `--promote` mode to stage and promote eligible silver/gold candidates into `.agents/learnings/` / `.agents/patterns/`.
- **`ao store` indexing** — Added `--categorize` to capture `category` and `tags` metadata in index entries.
- **Codex bootstrap/install docs** — Updated `.codex/agentops-bootstrap.md` and `.codex/INSTALL.md` with session-end workflow and usage.

### Fixed

- **Stop hook close-loop wiring** — `hooks/hooks.json` Stop hook now executes flywheel close-loop and logs failures to `.agents/ao/hook-errors.log`.
- **Codex compatibility fallback** — `agentops-codex session-end` falls back to repo-local `go run ./cmd/ao` when system `ao` is missing/outdated.

## [2.2.0] - 2026-02-10

### Added

- **RPI Gate 4 iteration loop** — `/rpi` now loops after post-mortem failure instead of stopping, feeding failure context back into the retry
- **Standards security and documentation sections** — Security Practices and Documentation Standards added to Go, Python, Rust, JSON, YAML language references
- **Skill tier frontmatter** — All 32 skills now declare `tier:` in YAML frontmatter for taxonomy validation
- **Council validation script** — `skills/council/scripts/validate-council.sh` (308 lines) validates schema, presets, personas, and output format
- **Skill linting** — `tests/skills/lint-skills.sh` validates tier, triggers, and tool declarations across all skills
- **5 SKILL.md extractions into references/** — Council, crank, implement, swarm, and vibe moved detailed content into progressive-disclosure reference files, keeping SKILL.md lean
- **Documentation validation test suite** (ag-o80) — Three CI-ready scripts preventing doc drift: link validation, skill count assertion, CLI reference generation
- **INDEX.md, GLOSSARY.md, workflows guide, CLI reference** (ag-qmd) — 83-link documentation index, 30-term glossary, workflow decision matrix, 35-command CLI reference
- **Hook execution integration tests** (ag-ab6) — 26 tests in `tests/hooks/test-hooks.sh` covering prompt-nudge, push-gate, and task-validation-gate (JSON injection resistance, kill switches, allowlist enforcement)
- **Skill dependency validation** — `validate-skill.sh` and `run-all.sh` now check declared dependencies exist and report health summary
- **Standards gap fills** (ag-ab6) — TypeScript Testing, Python Security, Rust Security + Documentation, Go Documentation sections added
- **Quickstart monorepo detection** — Shallow scan with trigger paths and dirty-tree-first recent file selection for better onboarding heuristics

### Changed

- **README intro rewrite** — New "What Is AgentOps" section, prerequisites table, 3 install options ordered by ease, FAQ updated for multi-agent compatibility (Claude Code, Codex CLI, Cursor, Open Code)
- **install.sh agent gate softened** — Hard `exit 1` on missing `claude` CLI replaced with warning that checks `claude`/`codex`/`cursor` and continues
- **Vibe skill restructured** — Excess inline content moved to `references/` for progressive disclosure
- **Project tagline** updated to describe full product value

### Fixed

- **Security: command injection in task-validation-gate.sh** — Replaced `/bin/sh -c "$cmd"` with allowlist-based array execution; only `go`, `pytest`, `npm`, `npx`, `make`, `bash` permitted
- **Security: regex injection in task-validation-gate.sh** — Changed `grep -q` to `grep -qF` for literal pattern matching
- **Security: JSON injection in prompt-nudge.sh** — Replaced `printf` JSON construction with `jq -n --arg` for safe escaping
- **stop-team-guard** only blocks when tmux panes are alive (no longer blocks on dead panes)
- **bd doctor warnings** — Sync, config, hooks, gitignore issues resolved
- **Skill count accuracy** — 33→32 across ARCHITECTURE.md, README.md, CLAUDE.md, SKILL-TIERS.md, marketplace.json
- **Vibe flag documentation contradiction** — Clarified default is `--deep` (3 judges), users can override with `--quick` or `--mixed`
- **Phantom `ao export-constraints`** marked as FUTURE in ol-bridge-contracts.md
- **Dual schema confusion in push-gate.sh** — Added CANONICAL/LEGACY comments for `gate`/`status` vs `step`/`locked` fields
- **7 doc inaccuracies** across README.md, SKILL-TIERS.md, ARCHITECTURE.md, profiles docs
- **5 broken links** in meta-observer README replaced with inline descriptions
- **CLAUDE.md skill count** and **SKILL-TIERS.md header count** corrected to match actual (22 user-facing, 10 internal)
- **curl-pipe pattern** removed from install.sh comment to pass security scan

### Removed

- **Deprecated `/judge` skill** — `skills/judge/` directory removed; all references updated to `/council`
- **Stale repo root files** — REBRAND-COORDINATION.md, RELEASE-NOTES.md, prompts/witness_prompt.txt
- **Olympus integration section** removed from README (bridge documented in ol-bridge-contracts.md)

## [2.1.0] - 2026-02-09

### Added

- **Public-ready polish** (ag-yxj) — Two-wave epic making AgentOps ready for external users
  - `scripts/install.sh` — One-liner automated install script
  - `cli/cmd/ao/doctor.go` — Health check command validating Go, skills, hooks, ao CLI (298 LOC)
  - `docs/troubleshooting.md` — Comprehensive troubleshooting guide covering hooks, skills, push gate, phantom commands (200 LOC)
  - `skills/quickstart/` — Pre-flight checks, error handling, `references/getting-started.md` and `references/troubleshooting.md`
  - `skills/status/SKILL.md` — Expanded dashboard with colored output, wave progress, and suggested next actions
  - `skills/council/SKILL.md` — Standalone usage section positioning council as an independent PR validator
  - 3 validation scripts — `council/scripts/validate.sh`, `crank/scripts/validate.sh`, `swarm/scripts/validate.sh`
  - 8 files with phantom commands quarantined with `FUTURE` markers
  - `MOLECULES.md` — `NOT YET IMPLEMENTED` header added
- **Rust standards** (ag-cn3) — Full two-tier Rust language support
  - `skills/standards/references/rust.md` — Tier 1 quick reference (cargo fmt/clippy, error handling, ownership, unsafe)
  - `skills/vibe/references/rust-standards.md` — Tier 2 deep reference (1,118 lines: project structure, ownership patterns, error handling, trait system, concurrency, unsafe code, testing)
- **Common standards catalog** (ag-eba) — Cross-language universal patterns as single source of truth
  - `skills/standards/references/common-standards.md` — 17KB covering error handling, testing, security, documentation, code organization
  - Updated JIT loading order: vibe-coding.md → common-standards.md → language standards
- **Standards coverage gaps filled** (ag-eba) — Critical sections added across 6 Tier 2 files
  - TypeScript: Testing Patterns (Jest/Vitest, React Testing Library, MSW)
  - Python: Security Practices (eval/exec, pickle, SQL injection, SSRF)
  - Rust: Security Practices + Documentation Standards
  - Go + Shell: Documentation Standards
  - JSON + YAML: Anti-Patterns, Code Quality Metrics, Prescan rules
- **Cross-references in language files** (ag-eba) — 5 language Tier 2 files link to common-standards.md
- **Codex integration tests** (ag-3b7) — Empirical validation of Codex CLI native features
  - `tests/codex/test-schema-validation.sh` — 10 assertions for verdict.json schema
  - `tests/codex/test-structured-output.sh` — 13 assertions for `--output-schema` conformance
  - `tests/codex/test-sandbox-mode.sh` — 4 assertions for `-s read-only` + `-o` capture
  - `tests/codex/test-codex-review.sh` — 4 assertions for `codex review --uncommitted`
  - `tests/codex/run-all.sh` — Runner with pass/fail/skip summary

### Fixed

- **Model selection consistency** — Replaced hardcoded model versions with `"opus"` short name in crank and swarm skills
- **verdict.json schema** — All properties now listed in `required` arrays (OpenAI structured output API requirement)
- **vibe SKILL.md codex review** — Fixed to use `> file 2>&1` redirect instead of unsupported `-o` flag

## [2.0.1] - 2026-02-09

### Added

- **Codex CLI native features** (ag-uj4) — Enhanced cross-vendor capabilities across council, vibe, and codex-team skills
  - `--output-schema` support in `/council` — Codex judges produce structured JSON verdicts via `skills/council/schemas/verdict.json`
  - `-s read-only` sandbox for Codex judges — safer execution with CLI-level output capture
  - `codex review --uncommitted` in `/vibe` (Step 2.5) — fast diff-focused review before council validation
  - `--add-dir` cross-project support in `/codex-team` — tasks spanning multiple repos
  - `--json` JSONL monitoring guidance in `/codex-team` — detect stuck agents, track token usage
  - Sandbox levels table in `/codex-team` — read-only, workspace-write, full-access guidance
  - Pre-flight `--output-schema` test with graceful fallback to `--full-auto` + markdown
- **Olympus (OL) CLI bridge** (ag-q7n) — Bidirectional integration with the Olympus context orchestration platform
  - `ao inject` discovers OL constraints from `.ol/constraints/quarantine.json` when present
  - `/vibe` runs `ol validate stage1` before council in Olympus projects (auto-FAIL on validation failure)
  - `/swarm --from-wave <json>` reads `ol hero hunt` output, creates tasks, ratchets completion back to OL
  - Bridge contracts spec at `docs/ol-bridge-contracts.md`
- **`ol harvest --format=ao`** — OL harvest outputs AO-compatible learning markdown (in OL repo)

## [2.0.0] - 2026-02-09

### Added

- **`/rpi` lifecycle skill** — One command, full lifecycle. Research → Plan → Pre-mortem → Crank → Vibe → Post-mortem. Give it a goal, get production code with multi-model validation at every gate. Autonomous by default — zero human prompts. Failed validations trigger retry loops with failure context, not escalation.
- **`--auto` is now the default for `/rpi`** — Just type `/rpi "goal"` and walk away. Use `--interactive` if you want human gates at research and plan phases. `--auto` flag kept for backwards compatibility.
- **`--auto` mode for `/research` and `/plan`** — Sub-skills now accept `--auto` to skip human approval gates. Enables fully autonomous `/rpi` pipeline.

### Changed

- **README rewritten** — New tagline: "Goal in, production code out." Restructured narrative flow, meaningful metric badges, progressive disclosure, expanded FAQ and examples.
- **Plugin description updated** — Reflects autonomous execution positioning.

### Fixed

- **README ao CLI docs accuracy** — Removed false claim that `ao inject` uses decay weighting by default (requires `--apply-decay` flag). Fixed `ao forge transcript` example to show it takes a file path argument.

## [1.9.1] - 2026-02-09

### Fixed

- **`ao hooks install` generates invalid format for Claude Code 2.1+** — Hook entries used old `{matcher, command: []}` format; now generates correct `{hooks: [{type: "command", command: "..."}]}` matcher format. Detection and merge logic handles both old and new formats for backwards compatibility.

## [1.9.0] - 2026-02-08

### Added

- **Flywheel automation CLI** (ag-m0r) — 4 new `ao` commands that replace fragile bash parsing in hooks with structured JSON output
  - `ao ratchet next` — Compute next RPI step from chain.jsonl state, with skill suggestion and lock detection
  - `ao flywheel nudge` — Single-command dashboard combining flywheel metrics, ratchet state, and pool status for session-start hook
  - `ao forge batch --extract` — Scan and process pending transcripts with forged-index deduplication and optional extraction trigger
  - `ao session close` — End-to-end session pipeline: forge transcript, extract learnings, measure flywheel velocity delta, report impact
- **Hook CLI integration** — `session-start.sh` and `ratchet-advance.sh` now use `ao` commands with version detection and full fallback paths for backward compatibility
- **Integration tests** — 836 lines covering full pipeline lifecycles (ratchet progression, forge batch dedup, session close, cross-command consistency)

## [1.8.0] - 2026-02-08

### Added

- **Flywheel automation hooks** (ag-oke) — RPI lifecycle hooks that nudge agents through the knowledge flywheel without forcing invocation
  - `ratchet-advance.sh` — PostToolUse hook detects `ao ratchet record` and suggests next RPI skill via additionalContext
  - `session-start.sh` ratchet resume — Reads chain.jsonl on new session, injects "RESUMING FLYWHEEL" directive with last step + artifact
  - `push-gate.sh` post-mortem gate — Blocks push when vibe exists but no post-mortem entry
  - `prompt-nudge.sh` nudge dedup — Flag-file coordination with 10-min TTL prevents triple-nudging across hooks
- **Safety and enforcement hooks** — 7 new hooks for agent guardrails
  - `dangerous-git-guard.sh` — Blocks force-push, reset --hard, clean -f, branch -D
  - `git-worker-guard.sh` — Blocks git commit/push for swarm workers (lead-only commit pattern)
  - `task-validation-gate.sh` — TaskCompleted hook validates metadata rules before accepting
  - `stop-team-guard.sh` — Prevents orphaned teams on session stop
  - `precompact-snapshot.sh` — Snapshots team state + git status before context compaction
  - `standards-injector.sh` — PreToolUse hook injects language-specific coding standards
  - `pending-cleaner.sh` — Archives stale pending queue entries at session start
- **Process improvements** — Metadata verification in `/crank`, wave vibe gate, pre-planning audit in `/plan`, ratchet pre-flight in `/implement`, ratchet post-flight in `/vibe`
- **`/release` skill** — GitHub Release draft with highlights + changelog (added in v1.7.0, enhanced with `--no-gh-release` and release notes generation)
- **README** — Reorganized skill catalog by category (orchestration, workflow, utilities, internal)

### Fixed

- Replace `eval()` with restricted execution in task-validation-gate.sh (command injection vulnerability)
- Add missing kill switch (`AGENTOPS_HOOKS_DISABLED`) to standards-injector.sh
- Remove `set -euo pipefail` from session-start.sh (hooks must fail open)
- Fix `--format=oneline` → `-o json` for `ao ratchet status` (flag doesn't exist)
- Fix relative `.agents/ao` path → `git rev-parse --show-toplevel` for correct root detection
- Replace PostToolUse comment-checker with PreToolUse standards-injector (better timing)
- Add failure logging to all ao commands in hooks.json (was bare `|| true`)

### Changed

- **README** — Visual-first rewrite with accurate skill examples
- **Docs** — Reference and L4 updated for native teams + lead-only commits

### Removed

- `comment-checker.sh` — Replaced by standards-injector.sh

## [1.7.1] - 2026-02-07

### Added

- **Hooks system** — 7 hook event types (SessionStart, SessionEnd, PreToolUse, UserPromptSubmit, TaskCompleted, Stop, PreCompact) with 10 scripts covering flywheel pipeline fixes, RPI enforcement gates, and agent team safety
- **Push gate** (`push-gate.sh`) — Blocks `git push`/`git tag` when vibe check not completed. Gates on RPI ratchet state. Cold start = no enforcement.
- **Ratchet-aware nudges** (`prompt-nudge.sh`) — Injects one-liner reminders based on prompt keywords vs ratchet state (e.g., "run /vibe before pushing")
- **Task validation gate** (`task-validation-gate.sh`) — TaskCompleted hook validates metadata.validation rules (files_exist, content_check, tests, lint, command) before accepting task completion
- **Git worker guard** (`git-worker-guard.sh`) — Blocks `git commit`/`push`/`add --all` for swarm workers, enforcing lead-only commit pattern
- **Dangerous git guard** (`dangerous-git-guard.sh`) — Blocks force-push, reset --hard, clean -f, checkout ., branch -D. Suggests safe alternatives.
- **Stop team guard** (`stop-team-guard.sh`) — Prevents orphaned teams by checking for active configs before session stop
- **PreCompact snapshot** (`precompact-snapshot.sh`) — Snapshots team state + git status before context compaction for recovery
- **Standards injector** (`standards-injector.sh`) — PreToolUse hook injects language-specific coding standards before Write/Edit
- **Pending cleaner** (`pending-cleaner.sh`) — Archives stale pending queue entries (>2 days) at session start
- **Ratchet pre-flight** in `/implement` — Checks pre-mortem status before coding starts
- **Ratchet post-flight** in `/vibe` — Records vibe completion in ratchet chain on PASS/WARN
- **README** — Reorganized skill catalog by category (orchestration, workflow, utilities, internal)

### Fixed

- Replace `eval()` with restricted execution in task-validation-gate.sh (command injection)
- Add missing kill switch to standards-injector.sh
- Remove `set -euo pipefail` from session-start.sh (hooks must fail open)
- Fix `--format=oneline` to `-o oneline` for ao ratchet status
- Fix relative `.agents/ao` path to use `git rev-parse --show-toplevel`
- Replace PostToolUse comment-checker with PreToolUse standards-injector
- Add failure logging to all ao commands in hooks.json (was bare `|| true`)

### Removed

- `comment-checker.sh` — Replaced by standards-injector.sh

## [1.7.0] - 2026-02-07

### Added

- **`bin/ralph`** — Full RPI loop script (Goal → Plan → Pre-mortem → Crank → Vibe → Post-mortem → PR). Each phase gets a fresh Claude context window (Ralph Wiggum pattern). Features: `--dry-run`, `--skip-pre-mortem`, `--branch`, `--spec` for acceptance criteria, `--resume` for checkpoint/resume, `--max-budget` and `--phase-timeout` for gutter detection.
- **`/codex-team` skill** — Spawn parallel Codex execution agents from Claude. Claude orchestrates task decomposition, Codex agents execute independently via `codex exec --full-auto`. Includes pre-flight checks, canonical command form, prompt guidelines, and fallback to `/swarm`.
- **`/codex-team` file-conflict prevention** — Team lead analyzes file targets before spawning: same-file tasks merge into one agent, dependent tasks sequence into waves with context injection, different-file tasks run in parallel. The orchestrator IS the lock manager.
- **`/release` skill** — General-purpose release workflow: pre-flight validation (`--check` mode), changelog generation from git history, semver suggestion from commit classification, version bump detection across ecosystems (package.json, pyproject.toml, Cargo.toml, go files), release commit, annotated tag, and optional draft GitHub Release. Boundary at the git tag — everything local and reversible.

### Crank All Epics (ag-kmk, ag-e0g, ag-ow7)

Autonomous execution of 19 issues across 3 epics in 3 waves (18 parallel workers, ~16 minutes, 100% success rate, zero retries).

#### Skills Hardening (ag-kmk + ag-e0g)

- **Structured SendMessage envelope** — JSON envelope schema for machine-parseable team communication (completion, blocked, retry, progress, verdict types)
- **Conflict resolution algorithm** — Severity escalation, deduplication, quorum rules, verdict computation, contradiction handling (`.agents/specs/conflict-resolution-algorithm.md`)
- **Pre-flight checks** — Added to vibe, post-mortem, and crank: verify git repo, files exist, epic has children before spawning agents
- **Native-teams fallback bundle** — Per-capability degradation table in `skills/shared/SKILL.md`; graceful fallback when specific team features unavailable
- **Race-free swarm claiming** — Mayor assigns tasks before spawning; workers only transition status (no concurrent claim races)
- **Anti-anchoring debate protocol** — R2 judges restate their R1 position before reading others' verdicts; weak flip detection in consolidation
- **Lead-only commit policy** — Workers write files, never `git commit`; team lead validates and commits per wave
- **Validation contract updated** — Default validation checks worker-reported artifacts, not commit history
- **Reaper cleanup pattern** — Graceful team shutdown sequence (shutdown_request → wait → TeamDelete) with configurable timeouts
- **Agent timeout and partial completion** — Worker timeout (180s), quorum handling for partial council results

#### ao CLI Quality (ag-ow7)

- **Shell completion** — `ao completion bash|zsh|fish` for tab-complete support
- **`ao forge batch`** — Scan and process pending transcripts in bulk with deduplication
- **`ao pool batch-promote`** — Bulk promote pending candidates meeting criteria (age >24h, cited, not duplicate)
- **File splits** — `ratchet.go` (1001→11 files), `metrics.go` (930→5 files), `inject.go` (891→5 files) for maintainability

### Fixed

- **Batch dedup false positives** — Replaced naive 80-char and 200-char prefix truncation in `normalizeForDedup` and `normalizeContent` with SHA256 content hashing. Distinct learnings with similar openings are no longer silently deduplicated.
- **Batch command test coverage** — Added `batch_forge_test.go` and `batch_promote_test.go` covering dedup logic, promotion criteria, citation counting, transcript discovery, and content loading.
- **Council findings** — Vibe always uses `--deep` (3 judges), debate enabled in post-mortem, auto-escalation when preset specifies more perspectives than judge count.

### Changed

- **Council redesign** — Independent judges by default (no perspective labels); diversity from sampling, not personality. Specialized presets (`code-review`, `plan-review`, `retrospective`) used by wrapper skills (`/vibe`, `/pre-mortem`, `/post-mortem`).
- **Codex model updated to `gpt-5.3-codex`** — All references across council, shared, and SKILL-TIERS updated from `gpt-5.3` to `gpt-5.3-codex` (canonical Codex model name).
- **README rewrite** — 479→149 lines. Leads with value prop, moved architecture and deep docs to `docs/reference.md`.

## [1.6.0] - 2026-02-06

### Adoption Improvements

Driven by council analysis (3 judges + 6 explorers) and pre-mortem validation (2 judges, unanimous WARN → fixes applied).

#### README Overhaul

- **Tagline reframed** — "DevOps for AI agents" → "A knowledge flywheel for AI coding agents — your agent remembers across sessions." Leads with the differentiator (knowledge compounding), not the analogy
- **Tier table added** — Tier 0 (skills only) through Tier 3 (cross-vendor consensus) with graduation triggers. Uses "Tier" naming to avoid collision with existing L1-L5 learning path
- **What This Is reframed** — Flywheel narrative leads ("each session feeds the next"), ASCII diagram preserved in `<details>` block
- **Quick Start rewritten** — Self-contained with commands and context. `/quickstart` offered as optional guided tour (not primary path, due to known slash-command discoverability bug)
- **CLI Reference expanded** — MemRL retrieval, confidence decay, provenance tracking, escape velocity metrics. Leads with capabilities, not LOC count
- **"Why Agents Need DevOps" → "Why Agents Need This"** — Consistent with tagline reframe

#### Tier/Level Disambiguation

- **`docs/levels/README.md`** — Added "Tiers vs Levels" section explaining the two axes: Tiers (0-3) = what tools you install, Levels (L1-L5) = what concepts you learn. Cross-references README tier table
- **`skills/quickstart/SKILL.md`** — Added graduation hints to Step 7 based on detected CLI state (ao, beads presence). Natural language, not formal tier labels

### Native Teams Migration

**The big idea:** Council judges and swarm workers are no longer fire-and-forget background agents. They now spawn as teammates on native teams (`TeamCreate` + `SendMessage` + shared `TaskList`), enabling real-time coordination without re-spawning.

#### Council

- **Judges spawn as teammates** on a `council-YYYYMMDD-<target>` team instead of independent `Task(run_in_background=true)` calls
- **Debate R2 via SendMessage** — judges stay alive after R1 and receive other judges' full verdicts via `SendMessage`. No more re-spawning fresh R2 instances with truncated R1 summaries. Result: zero truncation loss, no spawn overhead, richer debate
- **Team cleanup** — `shutdown_request` each judge + `TeamDelete()` after consolidation
- **Communication rules** — judges message team lead only (prevents anchoring); no peer-to-peer, no TaskList access
- Updated architecture diagram with Phase 1a (Create Team) and Phase 3 (Cleanup)
- R2 output files unchanged (`.agents/council/YYYY-MM-DD-<target>-claude-{perspective}-r2.md`)

#### Swarm

- **Team-per-wave** — each wave creates a new team (`swarm-<epoch>`), preserving Ralph Wiggum fresh-context isolation
- **Workers as teammates** — workers join the wave team, claim tasks via `TaskUpdate`, and report via `SendMessage`
- **Retry via SendMessage** — failed workers receive retry instructions on their existing context (no re-spawn needed within a wave)
- **Workers access TaskList** — workers can claim and update their own tasks (previously Mayor had to reconcile everything)
- Step 5a added: team cleanup (`shutdown_request` workers + `TeamDelete`) after each wave

#### Crank

- **Diagram updated** to show swarm's team-based execution flow (`TeamCreate` per wave, `SendMessage` for reporting, `TeamDelete` after wave)
- Separation of concerns clarified: Crank = beads-aware orchestration, Swarm = team-based parallel execution

#### Shared

- **Native teams fallback** added to CLI availability/fallback table: if `TeamCreate` unavailable, fall back to `Task(run_in_background=true)` fire-and-forget
- Fallback degrades gracefully: council loses debate-via-message (reverts to R2 re-spawn with truncation), swarm loses retry-via-message (reverts to re-spawn)

### Hardening (ag-3p1)

Fixes from council validation of the native teams migration:

- **Codex model pre-flight** — council now tests model availability (not just CLI presence) before spawning Codex agents. Catches account-type restrictions (e.g. gpt-5.3-codex on ChatGPT accounts) and degrades to Claude-only
- **Debate fidelity marker** — debate reports include `**Fidelity:** full | degraded` so users know if `--debate` ran with full-context native teams or truncated fallback
- **Explicit R2 timeout** — `COUNCIL_R2_TIMEOUT` env var (default 90s) replaces vague "idle too long" with concrete timeout and fallback-to-R1 instruction
- **TeamDelete() documentation** — clarified that `TeamDelete()` targets the current session's team context; concurrent team scenarios (e.g. council inside crank) documented

### Simplification

Pre-release council validation (2 judges, unanimous WARN) identified over-engineering. Refactored before shipping:

- **Council task types 5 → 3** — merged critique→validate, analyze→research. Keeps validate, research, brainstorm
- **Removed `--perspectives-file`** — presets and `--perspectives="a,b,c"` cover all current use cases. Bring back when someone asks
- **Agent hard cap: MAX_AGENTS=12** — prevents resource bombs from `--mixed --deep --explorers=N` combinations. Pre-flight check errors if exceeded
- **`--debate` restricted to validate mode** — brainstorm and research don't produce PASS/WARN/FAIL verdicts; combining with --debate now errors instead of producing "awkward outputs"
- **`--debate` documented as Ralph exception** — judges intentionally persist across R1/R2 within one atomic invocation. Bounded, documented, justified
- **Distributed mode gated as experimental** — swarm and crank distributed mode (tmux + Agent Mail) labeled experimental. Local mode (native teams) is the recommended path
- **Crank validation simplified** — collapsed triple validation (per-task + per-issue + batched) to double (trust swarm + final batched vibe). Per-issue layer was redundant

### Documentation

- Added official Skills installer instructions: `npx skills@latest add boshu2/agentops --all -g`
- Added agent-scoped install example: `npx skills@latest add boshu2/agentops -g -a codex -s '*' -y`
- Clarified that session hooks are Claude Code plugin functionality (skills remain portable)

## [1.3.1] - 2026-02-01

### Documentation Reality Check

Swarm documentation updated to match tested behavior:

- **TaskCreate API**: Removed invalid `blockedBy` parameter from examples. Dependencies require separate `TaskUpdate(addBlockedBy=[...])` call
- **Terminology**: "crank loops" → "atomic agents" (agents don't loop internally)
- **Monitoring**: Replaced `TaskOutput` polling with automatic `<task-notification>` pattern
- **Agent isolation**: Documented that agents cannot access TaskList/TaskUpdate - Mayor must reconcile
- **Mayor reconciliation**: Added explicit verify → update status → spawn next wave step
- **Prompts**: Simplified from complex loop instructions to atomic task format

Meta-learning: Task decomposition matters. 6 "independent" doc tasks weren't independent - they shared a file. Consolidated to 2 truly parallel tasks.

## [1.3.0] - 2026-02-01

### Pure Claude-Native Swarm

**The big idea:** Why depend on tmux, external scripts, or complex tooling when Claude Code has everything we need built-in?

The `/swarm` skill now uses pure Claude Code primitives:
- `TaskCreate` / `TaskUpdate` / `TaskList` for state management
- `Task(run_in_background=true)` for spawning background agents
- `<task-notification>` for completion callbacks

No tmux sessions. No external scripts. No beads dependency. Just Claude Code.

### Ralph Wiggum Pattern

This release documents WHY the architecture works, based on the [Ralph Wiggum Pattern](https://ghuntley.com/ralph/):

```
Ralph's bash loop:          Our swarm:
while :; do                 Mayor spawns Task → fresh context
  cat PROMPT.md | claude    Mayor spawns Task → fresh context
done                        Mayor spawns Task → fresh context
```

**Key insight:** Each `Task(run_in_background=true)` spawn creates a fresh process with clean context. Making demigods loop internally would cause context to accumulate and degrade - violating Ralph's core principle.

The loop belongs in Mayor (orchestration). Fresh context belongs in demigods (work).

### Changed

- **`/swarm` skill** - Complete rewrite:
  - Removed tmux dependency
  - Removed external script requirements
  - Pure Task tool orchestration
  - Added Ralph Wiggum pattern documentation
  - Wave execution via `blockedBy` dependencies

- **L4-parallelization docs** - Modernized:
  - Updated from `/implement-wave` to `/swarm`
  - Added Ralph Wiggum pattern explanation
  - Demo uses TaskList/TaskUpdate flow

### Technical Details

The swarm loop:

1. Mayor calls `TaskList()` to find ready tasks (pending, no blockers)
2. For each ready task, Mayor spawns: `Task(run_in_background=true, ...)`
3. Claude sends `<task-notification>` when each agent completes
4. Mayor calls `TaskUpdate(status="completed")` for finished tasks
5. This unblocks dependent tasks → next wave becomes ready
6. Repeat until all tasks complete

Each demigod has fresh context. Mayor maintains state via TaskList. Files/commits persist work across spawns.

## [1.2.0] - 2026-01-31

### Parallel Wave Execution

**The big idea:** When you have multiple issues that can run in parallel (no dependencies between them), why run them one at a time?

Before v1.2.0, `/crank` executed issues sequentially - finish one, start the next. Fine for small epics, but painfully slow when you have 10 independent tasks that could run simultaneously.

Now `/crank` detects **waves** - groups of issues with no blockers - and executes them in parallel using subagents. Each issue gets its own isolated agent. Results flow back to the main session.

```
Before (sequential):
  issue-1 → done → issue-2 → done → issue-3 → done
  Time: 3x

After (parallel waves):
  Wave 1: [issue-1, issue-2, issue-3] → 3 subagents in parallel → all done
  Time: 1x
```

**Why max 3 agents per wave?** Context management. Each subagent returns results that accumulate in your session. We tested higher parallelism - context explodes on complex issues. 3 is the sweet spot: meaningful speedup without blowing your context budget.

### How Waves Work

Waves emerge naturally from beads dependencies:

1. **`/plan`** creates issues with `blocks` dependencies
2. Issues with NO blockers = Wave 1 (run in parallel)
3. Issues blocked by Wave 1 = Wave 2 (run after Wave 1 completes)
4. **`bd ready`** returns the current wave - all unblocked issues
5. **`/crank`** takes the wave and dispatches up to 3 subagents

The dependency graph IS your execution plan. No separate "wave configuration" needed.

### Full Pipeline

```
/research → understand the problem
     ↓
/plan → decompose into issues with dependencies
     ↓         (waves form automatically)
/crank → execute waves in parallel
     ↓         Wave 1: [a, b, c] → 3 agents
     ↓         Wave 2: [d, e] → 2 agents
     ↓         Wave 3: [f] → 1 agent
     ↓
/post-mortem → extract learnings
```

### What's Next: Olympus

This parallel wave model is designed for **single-session work** - one Claude session spawning subagents. It's the foundation for something bigger.

**Olympus** (coming soon) will handle true multi-session orchestration: separate Claude sessions, persistent workers, direct context management instead of subagent nesting. The beads dependency graph persists across sessions - that's the ratchet that survives context resets.

### Changed

- **`/crank` skill** - Parallel wave execution:
  - Added `MAX_PARALLEL_AGENTS = 3` limit per wave
  - Step 4 now dispatches subagents in parallel via Task tool
  - FIRE loop updated to show wave model
  - `bd ready` explicitly documented as "returns current wave"

- **`/plan` skill** - Explicit wave formation:
  - Step 7 now shows how to create `blocks` dependencies
  - Added explanation of how waves form from dependencies
  - Clarified that `bd ready` returns parallelizable work

- **L4 implement-wave docs** - Updated max from 8 to 3 agents per wave

### Technical Details

The key instruction for `/crank`:

> **All Task calls for a wave MUST be in a single message to enable parallel execution.**

When Claude sends multiple Task tool calls in one message, they execute concurrently. Sequential messages = sequential execution. This is how we get parallelism without external orchestration.

## [1.1.0] - 2026-01-26

### Added
- **Agent Farm** (`/farm` skill) - Parallel multi-agent execution:
  - `ao farm validate` - Pre-flight checks with cycle detection
  - `ao farm start --agents N` - Spawn N agents + witness in tmux sessions
  - `ao farm status` - Check farm progress and agent states
  - `ao farm stop` - Graceful shutdown with process cleanup
  - `ao farm resume` - Resume incomplete farm from metadata
- **Witness monitoring** - Background observer for agent farm:
  - `ao witness start` - Start witness process
  - `ao witness stop` - Stop witness
  - `ao witness status` - Check witness state
- **Agent messaging** - Communication between agents:
  - `ao inbox` - View messages from agents
  - `ao mail send --to <agent> --body <message>` - Send message to agent
- **Serial agent spawn** with 30s stagger (rate limit protection)
- **Circuit breaker** - Stops farm if >50% agents fail
- `prompts/witness_prompt.txt` - Witness agent prompt template

### Changed
- Updated `using-agentops` skill documentation to include `/farm`
- Bumped skill count to 22

## [0.4.0] - 2026-01-25

### Changed
- **Repository restructure** - Professional polish for cleaner organization:
  - Reduced root directories from 22 to 13
  - Consolidated `levels/`, `profiles/`, `reference/`, `templates/`, `workflows/` into `docs/`
  - Renamed `shared/` to `lib/`
  - Deleted `mail/` (empty) and `agents-archived/` (56 obsolete agents)

- **README rewrite** - Minimal and approachable (47 lines vs 350):
  - One install command, 4 key skills, "want more?" section
  - Moved all details to `docs/PLUGINS.md`
  - Progressive disclosure: start simple, discover more as needed

- **Plugin description** - Simplified from verbose to concise:
  - Old: "Complete Knowledge OS for Claude Code - Research/Plan/Implement workflow..."
  - New: "Plugin kits for Claude Code: RPI workflow, validation, multi-agent orchestration"

### Added
- **Thin commands** - 4 command files that delegate to skills:
  - `commands/research.md` → `solo-kit:research`
  - `commands/plan.md` → `core-kit:formulate`
  - `commands/execute.md` → `core-kit:crank`
  - `commands/validate.md` → `vibe-kit:vibe`

- **Session hooks** - `hooks/` directory with:
  - `hooks.json` - SessionStart hook configuration
  - `session-start.sh` - Creates `.agents/` directories, outputs context

- **Multi-platform support**:
  - `.codex/setup.md` - Codex installation instructions
  - `.opencode/setup.md` - OpenCode installation instructions

- **RELEASE-NOTES.md** - User-friendly version highlights

- **docs/PLUGINS.md** - Complete plugin catalog moved from README

- **Marketplace cleanup** - Removed email from author fields, use GitHub username instead

## [0.3.1] - 2026-01-24

### Changed
- **Standardized .agents/ paths** (core-kit v0.2.1, pr-kit v0.1.1) - All skills now use relative `.agents/` paths:
  - Removed `~/gt/.agents/<rig>/` pattern in favor of portable `.agents/`
  - Removed "Phase 0: Rig Detection" sections from all skills
  - Skills affected: research, plan, formulate, product, pre-mortem, retro, post-mortem, implement
  - PR skills affected: pr-research, pr-plan, pr-implement, pr-retro
  - Gas Town-specific skills (gastown-kit, dispatch-kit) retain their specialized paths

- **README mermaid diagrams** - Replaced ASCII art with GitHub-native mermaid:
  - RPI Workflow diagram: `/research → /pre-mortem → /formulate → /crank → /post-mortem`
  - Plan → Crank handoff diagram with pre-mortem and post-mortem
  - Upgrade Path diagram

### Added
- **RAG Formatting Standard** (domain-kit) - New reference for knowledge artifacts:
  - `standards/references/rag-formatting.md` - 200-400 char sections, frontmatter conventions
  - Knowledge Artifact Detection section in standards SKILL.md
  - No `confidence` column rule (query-time, not storage-time)

- **RAG references added** (core-kit) - Knowledge-producing skills now reference RAG standard:
  - research, plan, formulate, pre-mortem, retro, post-mortem

### Fixed
- **retro skill** - Removed incorrect "Confidence" column from Discovery Provenance template:
  - Confidence/relevance are query-time metrics, not storage-time
  - Added reference to RAG formatting standard

## [0.2.3] - 2026-01-24

### Fixed
- **Plugin JSON uniformity** - Standardized all 14 plugin.json files:
  - Added `$schema` to all plugins (was missing from all)
  - Added `license: "MIT"` to 9 plugins that were missing it
  - Added `keywords` array to all plugins for discoverability
  - All plugins now have identical field structure

## [0.2.2] - 2026-01-24

### Added
- **marketplace-release skill** (core-kit v0.1.2) - New skill for releasing Claude Code plugins:
  - Complete release workflow documentation
  - Version bumping guidance
  - Update propagation explanation
  - Common pitfalls and anti-patterns
  - Context mode reference (inline vs fork)

## [0.2.1] - 2026-01-24

### Fixed
- **Marketplace plugin skills** - Applied `context: inline` fix to distributed plugins:
  - `core-kit/crank` (v0.1.1) - Epic execution now sees conversation context
  - `vibe-kit/vibe` (v0.1.2) - Validation now sees conversation context
  - `general-kit/vibe` (v0.1.2) - Validation now sees conversation context
  - Users who install from marketplace now get the fix

## [0.2.0] - 2026-01-24

### Fixed
- **Skill context mode** - Changed `context: fork` to `context: inline` for conversation-aware skills:
  - `vibe` - Now has access to chat context for inferring validation targets
  - `crank` - Now can identify epics mentioned in conversation
  - `pre-mortem` - Now can analyze specs discussed in chat
  - `post-mortem` - Now can identify completed epics from conversation
  - Root cause: `context: fork` creates isolated execution without conversation history
  - See `.agents/patches/2026-01-24-skill-context-inline.md` for details

## [0.1.3] - 2026-01-21

### Added
- **Two-Tier Standards Architecture** - JIT loading strategy for language standards:
  - **Tier 1** (slim refs, ~4-5KB): Always loaded via standards skill
  - **Tier 2** (deep standards, ~15-25KB): Loaded with `--deep` flag
  - Languages: Python, TypeScript, Shell, Go, YAML, JSON, Markdown

- **domain-kit v0.1.1** - Tier 1 slim references:
  - `standards/references/python.md` - Quick reference, common errors, prescan checks
  - `standards/references/typescript.md` - Strict mode, ESLint, type patterns
  - `standards/references/shell.md` - Required flags, shellcheck, error handling
  - `standards/references/go.md` - Error patterns, interfaces, concurrency
  - `standards/references/yaml.md` - yamllint, Helm/Kustomize patterns
  - `standards/references/json.md` - Formatting, JSONL, schema validation
  - `standards/references/markdown.md` - AI optimization, structure, tables

- **vibe-kit v0.1.1** - Tier 2 deep standards:
  - `vibe/references/python-standards.md` - Full complexity patterns, compliance grading
  - `vibe/references/typescript-standards.md` - Discriminated unions, branded types
  - `vibe/references/shell-standards.md` - ERR traps, security patterns
  - `vibe/references/go-standards.md` - Custom errors, thread-safe patterns
  - `vibe/references/yaml-standards.md` - Full Helm/Kustomize conventions
  - `vibe/references/json-standards.md` - Configuration patterns, tooling
  - `vibe/references/markdown-standards.md` - AI-agent optimization principles

- **general-kit v0.1.1** - Tier 2 deep standards (zero-dependency version):
  - Same 7 `*-standards.md` files as vibe-kit
  - Standalone operation without beads integration

### Changed
- **vibe SKILL.md** (vibe-kit, general-kit) - Added "Two-Tier Standards Loading" documentation:
  - Explains Tier 1 vs Tier 2 loading behavior
  - Documents `--deep` flag for comprehensive audits
  - Usage examples for different scenarios

### Design Decisions
- **Progressive disclosure**: Tier 1 gives quick answers, Tier 2 provides comprehensive audit capability
- **Context efficiency**: Default validation stays under 40% context budget
- **Portable**: general-kit has same deep standards for zero-dependency environments

## [0.1.2] - 2026-01-20

### Added
- **Tiered Architecture** - Scalable plugin system from solo developer to multi-agent orchestration:
  - **Tier 1**: solo-kit (any developer, any project)
  - **Tier 2**: Language kits (plug in based on project)
  - **Tier 3**: Team workflows (beads-kit, pr-kit, dispatch-kit)
  - **Tier 4**: Multi-agent orchestration (crank-kit, gastown-kit)

- **solo-kit v0.1.2** - Foundation for any developer:
  - 7 skills: `/research`, `/vibe`, `/bug-hunt`, `/complexity`, `/doc`, `/oss-docs`, `/golden-init`
  - 2 agents: `code-reviewer`, `security-reviewer` (read-only review specialists)
  - Hooks: auto-format on save, console.log/debug warnings, git push review, debug audit on session end
  - Zero external dependencies - works with any project

- **python-kit v0.1.2** - Python language support:
  - Standards skill with `references/python.md`
  - Hooks: ruff format, ruff check, mypy type checking

- **go-kit v0.1.2** - Go language support:
  - Standards skill with `references/go.md`
  - Hooks: gofmt, golangci-lint, P13/P14 error handling checks

- **typescript-kit v0.1.2** - TypeScript/JavaScript support:
  - Standards skill with `references/typescript.md`
  - Hooks: prettier, tsc type checking, `any` type warnings

- **shell-kit v0.1.2** - Shell scripting support:
  - Standards skill with `references/shell.md`
  - Hooks: shellcheck, `set -euo pipefail` enforcement

- **ARCHITECTURE-PROPOSAL.md** - Documents the tiered architecture design and migration path

### Changed
- **README.md** - Major update with tiered architecture:
  - Added tiered install instructions
  - Added upgrade path diagram (solo-kit → language-kit → beads-kit → crank-kit → gastown-kit)
  - Clarified legacy plugins and migration targets

- **Argument Inference** - Enhanced `/crank` and `/vibe` to semantically infer targets:
  - `/crank creating beads` now extracts "beads" keyword and searches for matching epic
  - `/vibe the auth changes` now validates auth-related files from git diff
  - Priority: conversational keywords > beads/git discovery > ask user
  - Updated in: core-kit/crank, vibe-kit/vibe, general-kit/vibe

### Skill Counts
| Kit | Skills | Agents | Tier |
|-----|--------|--------|------|
| solo-kit | 7 | 2 | 1 |
| python-kit | 1 (standards) | - | 2 |
| go-kit | 1 (standards) | - | 2 |
| typescript-kit | 1 (standards) | - | 2 |
| shell-kit | 1 (standards) | - | 2 |

---

## [0.1.1] - 2026-01-20

### Added
- **general-kit v1.0.0** - Standalone plugin with zero dependencies:
  - `/research`, `/vibe`, `/vibe-docs`, `/bug-hunt`, `/complexity`, `/validation-chain`
  - `/doc`, `/oss-docs`, `/golden-init`
  - 4 expert agents: security-expert, architecture-expert, code-quality-expert, ux-expert
- **standards library skill** (domain-kit) - Language-specific validation rules:
  - Python, Go, TypeScript, Shell, YAML, Markdown, JSON references
  - OpenAI platform standards (prompts, functions, responses, reasoning, GPT-OSS)
- **Context inference** for vibe and crank skills - Auto-detect targets from conversation
- **Natural language triggers** - Skills activate from intent, not just slash commands

### Changed
- **README overhaul**:
  - Added ASCII art logo and workflow diagrams
  - "Just Talk Naturally" section showing intent-based triggering
  - "The Killer Workflow: Plan → Crank" section with Shift+Tab + /formulate pattern
  - Clarified this provides plugins FOR beads/gastown, not built on them
  - Added OpenCode compatibility section
- **vibe skill** - Now references standards library for language-specific validation
- **validation-chain skill** - Added standards dependency
- **vibe-docs skill** - Added standards dependency

### Fixed
- **Standards dependencies** - Added missing `standards` skill dependency to:
  - vibe (vibe-kit, general-kit)
  - validation-chain (vibe-kit, general-kit)
  - vibe-docs (vibe-kit, general-kit)
- **Vibe findings** - Addressed quality findings across all plugins
- **Cross-skill references** - Test validator now handles relative paths correctly
- **Personal identifiers** - Removed from public plugin files

---

## [0.1.0] - 2026-01-19

### Added
- **Unix Philosophy Restructure** - Plugins reorganized into 8 focused kits:
  - **core-kit v1.0.0** - Workflow: research, plan, formulate, product, implement, implement-wave, crank, retro
  - **vibe-kit v2.0.0** - Validation only: vibe, vibe-docs, validation-chain, bug-hunt, complexity (+ 4 expert agents)
  - **docs-kit v1.0.0** - Documentation: doc, oss-docs, golden-init
  - **beads-kit v1.0.0** - Issue tracking: beads, status, molecules
  - **dispatch-kit v1.0.0** - Orchestration: dispatch, handoff, roles, mail
  - **pr-kit v1.0.0** - OSS contribution: pr-research, pr-plan, pr-implement, pr-validate, pr-prep, pr-retro
  - **gastown-kit v1.0.0** - Gas Town: crew, polecat-lifecycle, gastown, bd-routing
  - **domain-kit v1.0.0** - Reference knowledge: 18 domain skills (languages, development, security, etc.)

### Changed
- **vibe-kit** - Slimmed down from 23 skills to 5 focused validation skills
- **gastown plugin** - Replaced by gastown-kit (Gas Town specific) + pr-kit (contribution workflow)
- **Main README** - Updated with Unix philosophy structure, recommended combinations, clearer skill guidance
- **Core kit README** - Added decision trees for implement vs crank vs implement-wave

### Removed
- **gastown plugin** - Split into gastown-kit and pr-kit for better modularity

### Fixed
- **vibe-kit missing skills** - Restored vibe and vibe-docs skills that were lost during restructure

### Consolidated
- **domain-kit v1.1.0** - Consolidated from 18 to 17 skills:
  - Removed `doc-curator` (redundant with docs-kit/doc)
  - Consolidated 7 `base/` utilities (audit-diataxis, audit-onboarding, audit-workflow, cleanup-deprecated, cleanup-docs, cleanup-plans, cleanup-repo) into single `maintenance` skill

### Skill Counts (Final)
| Kit | Skills |
|-----|--------|
| core-kit | 8 |
| vibe-kit | 5 |
| docs-kit | 3 |
| beads-kit | 3 |
| dispatch-kit | 4 |
| pr-kit | 6 |
| gastown-kit | 4 |
| domain-kit | 17 |
| **Total** | **50** |

---

### Previous Unreleased

#### Added
- **vibe-kit v1.1.0** - New skills added:
  - `implement-wave` - Parallel execution of multiple issues
  - `complexity` - Code complexity analysis using radon/gocyclo
  - `doc` - Documentation generation and validation
  - `oss-docs` - OSS documentation scaffolding (README, CONTRIBUTING, SECURITY)
  - `golden-init` - Repository initialization with Golden Template
  - `molecules` - Workflow templates and formula TOML patterns
- **Skills sync** - All skills updated to match latest local versions:
  - beads, bug-hunt, dispatch, implement, research, vibe, vibe-docs (vibe-kit)
  - All 18 gastown plugin skills updated

### Fixed
- **Painted doors removed** - Cleaned up non-functional references:
  - Removed empty `references/` directories (bug-hunt, implement, pr-research, pr-retro)
  - Fixed pr-research template reference to point to inline section

### Changed
- **Commands deprecated** - Commands directory marked as deprecated in favor of skills
  - Added deprecation notice to commands/INDEX.md
  - Added migration guide pointing to skill equivalents
  - Commands maintained for legacy compatibility only
- **vibe-kit plugin.json** updated to version 1.1.0 with new skills

### Previous Unreleased

- **vibe-check Integration** in session-management plugin
  - `/session-start` now captures baseline metrics via `vibe-check session start`
  - `/session-end` now captures session metrics and failure patterns via `vibe-check session end`
  - Automatic failure pattern detection (Debug Spiral, Context Amnesia, Velocity Crash, Trust Erosion, Flow Disruption)
  - Session entries in `claude-progress.json` now include metrics and retro blocks
  - `@boshu2/vibe-check` npm package added as plugin dependency
- **vibe-coding Plugin** added with commands:
  - `/vibe-check` - Run vibe-check analysis
  - `/vibe-level` - Declare vibe level for session
  - `/vibe-retro` - Run vibe-coding retrospective
- **constitution Plugin** added with:
  - laws-of-an-agent skill
  - context-engineering skill
  - git-discipline skill
  - guardian agent
- SECURITY.md with vulnerability reporting process
- CONTRIBUTING.md with comprehensive plugin submission guidelines
- CHANGELOG.md for version tracking
- CODE_OF_CONDUCT.md for community standards
- GitHub Actions CI/CD pipeline for automated validation
- GitHub issue templates for plugin submissions and bug reports
- GitHub PR template for structured contributions
- Test suite infrastructure with validation scripts
- Makefile for common development tasks
- ARCHITECTURE_REVIEW.md with comprehensive compliance analysis

### Changed
- Updated repository structure to follow GitHub best practices
- Enhanced documentation for better discoverability

### Security
- Established security policy and vulnerability reporting process
- Added automated security scanning (Dependabot, CodeQL)

## [1.0.0] - 2025-11-10

### Added
- Initial marketplace structure with `.claude-plugin/marketplace.json`
- Three core plugins:
  - **core-workflow**: Universal Research → Plan → Implement → Learn workflow
  - **devops-operations**: DevOps and platform engineering tools
  - **software-development**: Software development for Python, JavaScript, Go
- External marketplace references:
  - aitmpl.com/agents (63+ plugins)
  - wshobson/agents (open source collection)
- Comprehensive README with quick start guide
- Apache 2.0 license
- Plugin structure following Anthropic 2025 standards
- 12-Factor AgentOps integration in all agents
- Token budget estimation for plugins

### Agents (11 total)
- **core-workflow** (4 agents):
  - research-agent: Research phase with JIT context loading
  - plan-agent: Planning phase with detailed specifications
  - implement-agent: Implementation phase with validation
  - learn-agent: Learning extraction for continuous improvement
- **devops-operations** (3 agents):
  - devops-engineer: DevOps automation specialist
  - deployment-engineer: Deployment and release management
  - cicd-specialist: CI/CD pipeline expert
- **software-development** (3 agents):
  - software-engineer: General software development
  - code-reviewer: Code quality and review
  - test-engineer: Testing and quality assurance

### Commands (14 total)
- **core-workflow** (5 commands):
  - /research: Start research phase
  - /plan: Create implementation plan
  - /implement: Execute approved plan
  - /learn: Extract learnings
  - /workflow: Full workflow orchestration
- **devops-operations** (3 commands):
  - /deploy-app: Deploy applications
  - /setup-pipeline: Configure CI/CD pipelines
  - /rollback: Rollback deployments
- **software-development** (3 commands):
  - /create-feature: Create new features
  - /refactor-code: Refactor existing code
  - /add-tests: Add test coverage

### Skills (9 total)
- **core-workflow**: Universal workflow patterns
- **devops-operations** (3 skills):
  - gitops-patterns: GitOps workflow patterns
  - kubernetes-manifests: Kubernetes resource templates
  - helm-charts: Helm chart best practices
- **software-development** (3 skills):
  - python-testing: Python testing patterns
  - javascript-patterns: JavaScript/TypeScript patterns
  - go-best-practices: Go language best practices

### Documentation
- Comprehensive README.md with installation instructions
- Plugin-level README files for each plugin
- Agent documentation with examples and anti-patterns
- AgentOps principles integration
- External marketplace references

## Version History

### Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version: Incompatible API changes
- **MINOR** version: New functionality (backwards-compatible)
- **PATCH** version: Bug fixes (backwards-compatible)

### Release Process

1. Update CHANGELOG.md with changes
2. Update version in `.claude-plugin/marketplace.json`
3. Update version in all plugin `plugin.json` files
4. Create git tag: `git tag -a v1.0.0 -m "Release v1.0.0"`
5. Push tag: `git push origin v1.0.0`
6. Create GitHub release with changelog excerpt

## Links

- [Repository](https://github.com/boshu2/agentops)
- [Issues](https://github.com/boshu2/agentops/issues)
- [Pull Requests](https://github.com/boshu2/agentops/pulls)
- [Security Policy](SECURITY.md)
- [Contributing Guidelines](CONTRIBUTING.md)
- [12-Factor AgentOps Framework](https://github.com/boshu2/12-factor-agentops)

## Community

### How to Stay Updated

- Watch this repository on GitHub
- Check this CHANGELOG regularly
- Follow [@boshu2](https://github.com/boshu2) on GitHub

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on:
- How to add plugins
- Testing requirements
- Submission process
- Code of conduct

### Support

- **Documentation**: Check README.md and plugin docs
- **Issues**: [GitHub Issues](https://github.com/boshu2/agentops/issues)
- **Discussions**: [GitHub Discussions](https://github.com/boshu2/agentops/discussions)

---

**Note:** This changelog is automatically updated with each release. See [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) for format guidelines.
