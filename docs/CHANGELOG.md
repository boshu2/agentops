# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.40.0] - 2026-05-13

### Added

- **Practice-citation derivation graph** — `PRACTICE.md` becomes the derivation root for every primitive in the repo. A new `practice-citation derivation graph + advisory CI gate` enforces that every skill, hook, eval suite, CLI command, and schema declares its `practices:` lineage. Twelve backfill passes (`pass-1` through `pass-12`, four `pass-12` waves across `cli/`) reached **756/756 declared** primitives — full repo coverage. Touches `skills/`, `hooks/`, `schemas/`, `evals/agentops-core/`, and `cli/` via `// practice:` comment carriers.
- **Three-gap supergate (E5 epic)** — `scripts/check-three-gap-supergate.sh` consolidates three release-blocking gates into one entry point with Gap 1 `--strict-coverage` opt-in, wired into `scripts/pre-push-gate.sh` and the `validate-three-gap-supergate` CI workflow. Closes `soc-m47k` and three child gaps.
- **Contract enforcement gates** — `contracts-structural-floor` (covering all 38 contracts), plus dedicated CI-blocking gates for `factory-admission`, `finding-registry`, `factory-yield-ledger`, `flywheel-compounding-snapshot`, `wiring-closure`, `goals-validate`, `flywheel-proof`, `quarantine-empty`, and `contract-canaries` (`check 24e`). `feat(gate) --two-pass mode` and local-scope default added.
- **Behavioral eval workbench** — 12 eval tasks, first suite, three fixture components (Go CLI, Python FastAPI, DevOps scripts). Live-agent eval suite with 3 workbench cases, `scripts/eval-agent-harness.sh` wired into the eval CLI, and the suite expanded to all 12 workbench tasks.
- **Eval CI gates** — `eval-skill-delta` CI gate + nightly schedule template, `eval-workbench-verify` gate (GOALS.md Directive 10), head-to-head delta gate against the baseline scorecard (D10). Run records upload as CI artifacts for triage; Python venv bootstrapped in `agentops-eval-advisory`.
- **Eval harness primitives** — `context_comprehension` dimension, CDLC identity field, observability feedback loop, and industry-proven eval patterns added to the agent harness.
- **Unified registry** — Phase 1 added `scripts/generate-registry.sh` + `registry.json` source-of-truth. Phase 2 added a job type, CLI surface, and CI gate (`registry-check`).
- **Daemon factory-admission lane** — `feat(daemon): add factory admission job specs` + executor + lane validators, paired with `feat(gates): enforce factory-admission contract`.
- **Daemon RPI agent-update events** — `RPIRunExecutor` emits `agent_update.criterion_verdict` per wave checkpoint (`soc-awx8`) and phase-boundary `agent-update` events (`soc-y0ct.2`).
- **Daemon scheduling + executors** — wired skill schedules through the daemon, registered eval and planning job types, added a CLI fallback RPI executor.
- **Nightly automation upgrades** — admission-aware morning digest with durability; manual-PR-only landing policy for evolve; fail-closed execute preflight for source mutation; blocker matrix + main CI baseline artifacts; daemon-submitted dream runs; L3 rehearsal scenario + operator runbook; scheduler install helper + updated runbook; PR digest generator from structured run state.
- **Five-minute first-value install gate (PG1)** — first-time-user journey is now a release-blocking install proof.
- **Corpus snapshot/restore + freshness gate (D11)** — durable corpus state with a CI-blocking freshness check.
- **Hook commit guards** — `feat(hooks)` warns on commits with code-without-test (P2), missing sibling-pattern citation (P3), and missing fitness-delta (P4) in commit messages.
- **Goals additions** — `code-driven` vs `runtime-artifact` summary split; `SKIP` exit code 77 + flywheel-compounding dormant precondition; `AffectsFiles` sidecar for the open-PR blocker matrix.
- **New CLI surfaces** — `ao session spawn` (template-driven session launch) and `ao feedback-loop --drain` (clear unfed citation backlog, epic `soc-sx99` W3.1).
- **Skills framework expansions** — `skill-builder` + `skill-auditor` pair (epic `soc-9bak`, #237); `ao quickstart` + demo surface for `ao schedule` and `ao daemon` (epic `soc-sx99` W2.2); tracer-bullet shape for the `skills/domain` ubiquitous-language corpus; scoped evidence on `/crank` and `/implement` bead closures.
- **Schedule starter template** — tracked stock starter at `docs/templates/`.
- **Scripts** — `corpus-stats.sh` + derived `PRODUCT.md` evidence (epic `soc-sx99` W3.3); `evolve-update-session-state.sh` derives session-state from cycle-history tail.
- **`security` domain in eval-suite manifests** — `$defs/domain` enum in `schemas/eval-suite.v1.schema.json` now accepts `security` alongside the existing eight domains. Paired updates land in `cli/internal/eval/coverage.go` (`DefaultCoverageDomains`) and `cli/cmd/ao/cobra_commands_test.go` (`evalCoverageDomains`) so schema, production default, and test fixture stay in lock-step. `ao eval coverage` will report `security` as a missing required domain until a security-domain manifest is authored.

### Changed

- **Three-layer product model adopted across surfaces** — README, `PRODUCT.md`, `docs/index.md`, and downstream positioning docs aligned to the three-layer (substrate / assurance / bookkeeping) framing with the software-factory + TSMC + in/on-the-loop framing. Eight `docs(positioning)` commits cover the thesis polish, lineage cleanup, link to `vs-compound-engineer.md`, and the closure of `soc-yjzp.9` with empirical Δ=0.
- **`/release` skill refactored under the size limit + non-HEAD cut support** — `skills/release/SKILL.md` and the Codex twin shrank from 545 lines to a 141-line flow index, with the detail extracted to `references/release-workflow-detail.md`. A new `references/release-cut-and-bump.md` documents the `release/v<ver>` branch pattern for cutting at a non-HEAD SHA. (This is the commit that prepared the v2.40 cut itself.)
- **RPI lifecycle sharpened** — criterion contract + isolation enforcement + daemon executor swap (`soc-bcrn`, #255). The factory claim ledger reconciled via Wave 1A-D (`soc-e4ulx`, #264).
- **CI workflow path-filters** — added path-filter conditions to 28 CI jobs (the remaining tail); bats path-filter wired so `.bats` changes gate bats-test jobs; `bats` prints captured output on failure.
- **CI loop boundaries** — `feat(ci): enforce inner/middle/outer loop boundaries` (#230).
- **Codex parity drift** — `feat(ci): wire check-codex-parity-drift as CI-blocking + pre-push gate (D7)`.
- **Skill consolidation** — 12 standalone skills consolidated into `beads`, `review`, `doc`, and `research`; JSM quality playbook slices absorbed in two waves.
- **Documentation reorg** — `docs(claims)` introduced a public evidence manifest for v2.39 README claims (PG4); `docs(parity)` updated four times; `docs(cdlc)`, `docs(readme)`, `docs(eval)`, `docs(release)`, `docs(positioning)` swept across the catalog.
- **Dependencies bumped** — `chore(deps)` updates for `mkdocs-material` v9.7.6, `mkdocs-section-index` v0.3.12, `mkdocs-literate-nav` v0.6.3, `mkdocs-include-markdown-plugin` v7, `mkdocs-git-revision-date-localized-plugin` v1.5.1, `mkdocs-gen-files` v0.6.1, `linkchecker` v10.6.0, `pymdown-extensions` v10.21.2, Python 3.14, `dorny/paths-filter` v4. `deps(go)`: `pgregory.net/rapid` minor bump in `cli/` plus a go-minor-patch group bump (#268).

### Fixed

- **`fix(ci)`** — registry non-determinism + pre-push goals-validate fallback; structural CI failures from cycles 45-47; `registry.json` knowledge_stores and schedules must match gitignored-state regen (epic `soc-sx99`); refreshed registry generation; bumped CLI surface counts after `patterns repair-filenames` addition; close practice-provenance validator gaps; `--single-pass` on pre-push-gate tests broken by `--two-pass` default.
- **`fix(eval)`** — six fixes covering eval canary refresh, advisory job stabilization, run-record upload reliability, and miscellaneous workbench-suite drift.
- **`fix(hooks)`** — reconcile stale `.agents/.gitignore` deny-all with parent allowlist (`soc-rv5p`, #263); prove `pre-push` transmits refs (#254).
- **`fix(codex)`** — migrate deprecated hooks flag; remove dead Claude-specific references and runtime markers from the codex research skill.
- **`fix(cli/rpi)`** — three correctness fixes around RPI scoping and isolation enforcement.
- **`fix(daemon)`** — bound daemon RPI GasCity sessions; close unused-variable in `generate-registry.sh`; refresh CI registry after worker-spec additions.
- **`fix(flywheel-lifecycle)`** — survive sparse corpus in Stage 5.
- **`fix(next-work)`** — add `dream-degraded` to the source enum.
- **`fix(nightly)`**, **`fix(harvest)`**, **`fix(audit-truth)`**, **`fix(evolve)`**, **`fix(parity)`**, **`fix(worktree)`**, **`fix(coverage)`**, **`fix(skills)`**, **`fix(quickstart)`**, **`fix(scripts)`**, **`fix(docs)`**, **`fix(gates)`**, **`fix(pre-push)`**, **`fix(heal,lint)`**, **`fix(rpi)`** — a long tail of single-file fixes touching individual gates, scripts, and surfaces during the nightly close-loop cycles.
- **GitHub eval advisory setup** — the `agentops-eval-advisory` job now installs the deterministic canary toolchain (`jq`, `ripgrep`, `bats`, `bd`, and `gocyclo`) and initializes a disposable bd database before running `scripts/eval-agentops.sh --fast`, matching the local environment expected by the public canaries.

### Internal

- **RPI loop pass consumption** — 10 `chore(rpi)` consumption commits drove the practice-citation backfill epic through 12 passes, closing the `cli/` design pass and exhausting the schemas/hooks/evals pools en route to `756/756 declared`.
- **Tests** — `test(cmd/ao)`: beads citation-verify functions, beads human-formatter functions, `runBeads*` graceful-degradation paths, `TestSanitizeDaemonSkillInvokeArtifactName` 0%-coverage hole closed.
- **Refactors** — `refactor(daemon)` extracted routing-lane validators to drop cyclomatic complexity (two commits); `refactor(dream)` promoted probe shapes to the registry.
- **Codex artifact hashes** regenerated after enum updates and after merges.
- **Skill counts synced** to 71 after the consolidation passes.
- **Two nightly autonomous runs landed** — `Nightly 2026-05-06` (3 productive cycles, +1 code-driven goal, fitness 94.29 → 100.00, #235) and `Nightly 2026-05-07` (3 productive cycles, +0 code-driven goals already 100, 2 audit-truth regressions fixed).
- **Drain open `next-work.jsonl`** — six PRs bundled into one branch (epic `soc-xlw8`, #266).

## [2.39.0] - 2026-05-04

### Added

- **AgentOps daemon runtime** - landed opt-in `agentopsd` as the local control plane for durable jobs, queueing, worker execution, projections, health/readiness/status, event tailing, mutation tokens, and product soak proofs. New CLI surfaces include `ao daemon run`, `ready`, `status`, `events tail`, `jobs list/show/submit/wait/cancel`, `service install`, and `soak`.
- **Daemon-backed workflows and scheduling** - RPI, Dream, wiki/forge, and plans now have migration paths into the daemon via explicit daemon flags, `plans.projection`, worker policies, and projections. `.agents/schedule.yaml`, `ao schedule add/list/run/remove`, daemon `--schedule-file`, cron validation, backpressure controls, schedule mutation routes, and recurrence ledger events make recurring work a daemon-owned primitive.
- **Worker and factory substrate** - added AgentWorker contracts, GasCity API/SSE adapters, CLI fallback workers, process cleanup, quarantine, Linux cgroup caps, routing policy guardrails, scoped mutation tokens, factory lifecycle projections, worktree ownership contracts, validation state, manual merge disposition, and yield ledgers.
- **CLI command expansion** - added `ao agents inspect/lint/doctor`, `ao skills check`, `ao scope`, `ao eval task/cleanup/suite/coverage/baseline-audit`, `ao pool reindex`, `ao watch`, and daemon-backed plan synchronization surfaces.
- **Deterministic eval platform** - added eval runtime adapters, scorecards, coverage reports, baseline A/B, context-packet A/B, retrieval/file-backed backends, public canary suites, and contract canaries.
- **Dream and work-queue metadata** - added finding-generator sidecars, aggregation, external-watchlist output, end-user coverage fitness gates, and first-class next-work `status`, `requires`, `dedup_key`, and routing metadata.
- **`.agents/` write-surface governance** - catalogued repo memory write surfaces, added lint/smoke coverage, introduced the no-tracked-`.agents` policy, and added operator docs for working with runtime state.
- **Harvest and knowledge surfaces** - harvest now recurses into nested artifact directories, emits real rig metadata, and gained native Go extraction support.

### Changed

- **CLI architecture refactor** - reorganized large parts of the Go CLI around focused internal packages for daemon state, worker execution, GasCity, schedule parsing, eval, path resolution, lifecycle, harvest, OpenClaw, LLM wiki execution, safety, and quality checks.
- **Daemon migration model** - foreground RPI, Dream, plans, and wiki/forge paths remain compatible, while daemon submission/read paths move runtime ownership toward the durable ledger and rebuildable projections.
- **Hook runtime and context flow** - re-architected hooks around JIT context and a managed runtime backend, added Claude/Codex PreToolUse output parity checks, quieted Codex session-start behavior, and added edit/write hash-audit hooks.
- **Path resolution** - moved `.agents` and repo-state path logic toward shared Go/shell resolvers (`cli/internal/paths`, `lib/ao-paths.sh`) and migrated representative CLI commands and hooks to the new helpers.
- **Codex runtime packaging** - refreshed Codex skills for GPT-5.5, aligned native plugin metadata with the marketplace schema, regenerated manifests and hashes, converted remaining skill references to `$skill` notation, tightened native hook installation, and reduced skill-catalog context footprint.
- **Release and CI governance** - CI/local gates now cover daemon product proofs, contract canaries, eval baselines, command/test pairing, Codex runtime sections, pre-push wiring, release audit artifacts, Windows smoke, advisory policy, and nightly knowledge-cycle dedupe.
- **Docs and operator contracts** - added or expanded daemon, scheduling, control-plane, GasCity, OpenClaw, local compute routing, operator guide, and release governance documentation.
- **Bootstrap behavior** - `/bootstrap` now recommends installing `bd` instead of attempting automatic installation.

### Fixed

- **Daemon durability and state machines** - fixed fsync propagation, orphan temp sweeps, snapshot directory sync, idempotency-key dedupe, queue cancellation, recurrence recomputation, projection deep-copy/nil-safety, claimed job queue depth, terminal projection precedence, and store/reconcile context cancellation.
- **Daemon API and execution hardening** - capped request bodies, bounded event limits, normalized wait timeout errors, rejected bad cursors and malformed schedule payloads, hardened wiki source-path containment and schedule name traversal, tolerated oversized ledger lines, preserved Dream log permissions, and routed Dream/wiki execution through the foreground supervisor.
- **CLI hygiene** - fixed JSON output validity, UTF-8 truncation, schedule prompt behavior under tests, command catalog drift, Cobra docs conformance, hidden command surface checks, command/test pairing, and temp-directory walk-up behavior.
- **Hook/runtime drift** - repaired Codex skill chaining defaults, native hook manifest installation, noisy session-start output, hook output schema, intent-echo bypass for team runners, lifecycle guard coverage, and pre-push hook coverage.
- **Security and scanner false positives** - closed the harvest TOCTOU path with `os.OpenRoot`, excluded safe regexp literals from broad secret scans, split secret-regex construction so release gates do not flag their own patterns, and removed pipe-to-shell patterns from the ripgrep builder.
- **CI/nightly/release blockers** - repaired eval advisory fixtures, baseline-audit drift-only behavior, Windows smoke paths, shellcheck/pre-push gate regressions, knowledge-cycle dormant-corpus handling, release audit validation, and the canary count drift found during final v2.39.0 validation.

## [2.38.0] - 2026-04-22

### Added

- **Strict Delegation Contract** for `/rpi`, `/discovery`, and `/validation` — top-level orchestrator skills now declare strict sub-skill delegation as the default. Each skill points to the new canonical reference `skills/shared/references/strict-delegation-contract.md` which documents the contract, anti-pattern rationalizations, and supported compression escapes (`--quick`, `--fast-path`, `--no-retro`, `--no-forge`, `--skip-brainstorm`, `--no-scaffold`, `--no-behavioral`, `--allow-critical-deps`). There is no `--full` flag — strict delegation is always on.
- **Orchestrator Compression Anti-Pattern learning** at `docs/learnings/orchestrator-compression-anti-pattern.md`, surfaced through the orchestrator skill contracts. Includes detection phrases, corrective actions, and rationalizations to reject.
- **Orchestrator-owned step markers** in `skills/crank/SKILL.md` (STEP 3a.3, STEP 6.5 slop-scan, STEP 8.7) plus an "Inline Work Policy" footer documenting which steps are intentionally inline vs delegated.
- **MkDocs Material documentation site** — Pages site rebuilt on MkDocs Material (slate dark palette). Skill catalog and CLI reference are generated at build from `skills/*/SKILL.md` and `cli/docs/`. `mkdocs build --strict` wired into the pre-push gate (Check 25a). New dedicated pages for hooks, schemas, and upgrading, plus an expanded glossary. Theme tuned to the agentops-showcase terracotta-on-near-black palette; landing page leads with the primary use case and a headline skills table; flywheel diagram ASCII art realigned; doctrine back-links added to `12factoragentops.com`.
- **Shell completion for enumerated-value flags** — `ao <cmd> --<flag> <TAB>` now suggests the valid values instead of falling back to file completion. Covers `ao --output` (`json`/`table`/`yaml`), `ao seed --template`, `ao goals init --template`, `ao inject --format` and `--session-type`.
- **`ao doctor` stale-reference scan** now covers `skills-codex/*/SKILL.md` and `skills/*/references/*.md` in addition to the primary skill docs, catching drift in Codex mirrors and skill reference content.
- **Nightly close-loop throughput alarm** — the dream close-loop gate now fails loudly when `ingested > 0 && promoted == 0`, replacing the silent zero-throughput mode that previously masked citation-gate deadlocks.

### Removed

- Archived AO↔Olympus bridge integration: removed `docs/ol-bridge-contracts.md`, `docs/architecture/ao-olympus-ownership-matrix.md`, MemRL policy contracts, `skills/*/scripts/ol-*.sh`, `cli/cmd/ao/inject_ol_test.go`, and associated CLI types (`OLConstraint`, `gatherOLConstraints`, `.ol/` directory collector). Olympus predecessor's useful patterns live on inside `ao`.
- Lowercase `docs/index.md` duplicate removed after the MkDocs migration canonicalized the landing page.

### Changed

- **`--no-lifecycle` in `/discovery` renamed to `--no-scaffold`** for semantic clarity — the flag controls STEP 4.5 scaffold auto-invocation only, not broader lifecycle checks. `--no-lifecycle` is honored as a deprecated alias through v2.40.0; when both flags are passed, they are equivalent. Other skills (`/crank`, `/validation`, `/implement`, `/evolve`) retain `--no-lifecycle` with its existing lifecycle-skill-invocation semantics.
- **`/discovery` flags table** expanded: `--auto` is now explicitly documented (was transitively honored but undocumented); `--interactive` scope clarified ("research + plan gates, not pre-mortem").
- **`/validation` flags table** expanded: `--complexity=<level>` syntax formalized to match `/rpi` and `/discovery`; `--interactive` scope documented.
- **`/rpi` `--interactive` flag** scope note added: applies to discovery (research + plan) and validation (Gate 1, Gate 2); does NOT override pre-mortem or vibe council autonomy.
- **ASCII fast-path performance sweep** across rune-aware truncation call sites in `cli/` (`TruncateText`, `TruncateRunes`, `truncateForError`, plus goals/pool/search/rpi/parser call sites) — ASCII inputs now skip the full UTF-8 rune scan.
- **Compile and overnight internals refactored** — `runCompile` split into phase + preflight helpers; article scan, inbound count, and prune extracted from `repair`; dream packet corroboration split per source epic; dream yield emptiness guard extracted into a dedicated helper. No behavior change; lower cyclomatic complexity and tighter test surfaces.
- **Skills-codex DAG bodies converted to `$skill` notation** for the Codex runtime.
- **GitHub Actions bumped** — `actions/upload-pages-artifact 3→4`, `actions/deploy-pages 4→5`, `actions/configure-pages 5→6`, plus docs.yml workflow action versions aligned.
- **Precommit hook prefers local Go** when available.
- **Skills backfill pass** — docs, validators, and lint synced across all skills; Codex drift surface reset to no-op.

### Fixed

- **Orchestrator compression vulnerability** — a live compression was observed 2026-04-19 where `/rpi` was invoked but phases were inlined instead of delegated. This release **documents** the anti-pattern (forged learning + loud skill text), **scaffolds** future enforcement (shared contract reference used by all 6 orchestrator skills), and explicitly **defers** runtime hook enforcement to a follow-up initiative. It does not mechanically prevent compression yet — the durable fix depends on `ao inject` surfacing the forged learning on future session starts. See `.agents/research/2026-04-19-rpi-skill-dag-audit.md` for the audit and `.agents/plans/2026-04-19-rpi-dag-hardening.md` for the remediation plan.
- **Close-loop promotion deadlock** — `flywheel` close-loop auto-promotion and the loop-dominance signal are unblocked; citation-gate cycles no longer silently zero throughput.
- **Overnight findings router** now emits schema-compliant `next-work` v1.3 enums — valid `claim_status=available` (was `pending`), severity collapsed to `high` for `critical` and `blocker` inputs — with a build-time guard that fails on future drift.
- **Quality stale-refs scan** skips rename-doc lines so it no longer false-positives on deliberate rename notes.
- **Release and compile gates** — `go-complexity-ceiling` self-heals a missing `gocyclo`; `compile-*` gates fall back to Dream defrag preview when the primary path is unavailable.
- **Proof-run Phase 2** calls `pool ingest` before `close-loop` so the downstream stage has input to consume.
- **Hooks** — `git-worker-guard` narrowed to avoid false blocks on selective flags; test-hook harness tolerates environments without a locally built `ao`.
- **Scripts** — `goal-staleness`, `pillar-coverage`, `goal-quality`, and `bootstrap-maturity` now skip cleanly after the `GOALS.yaml → GOALS.md` migration and preserve existing JSONL maturity with compact output.
- **CI** — resolved 7 failures from the MkDocs rebuild, committed the forged-learning artifacts, and regenerated the codex shared hash.
- **Docs markdownlint** — unresolved `+ 9 findings).` continuation in the cross-disk harvest plan now reads as prose instead of tripping MD004.

## [2.37.2] - 2026-04-15

### Added

- **Swarm evidence validation** — AgentOps now ships a swarm-evidence schema and validator, and wires that proof surface into validation and release gates.
- **Lead-only worker git guard** — worker sessions now have an explicit lead-only git guard in the hook chain, reducing accidental write authority in multi-agent runs.
- **Compile and harvest operator controls** — `ao compile` adds runtime preference plus `--reset` and `--repair` controls, while harvest now reports excluded low-confidence candidates and top near-misses.

### Changed

- **Release and pre-push validation** — local release, pre-push, and command coverage gates now validate more of the hook, evidence, and Codex runtime surface before publish.
- **Codex/runtime artifacts and docs** — compile, evolve, post-mortem, swarm, and related runtime docs and artifacts were decomposed and synchronized to better match shipped behavior.
- **Flywheel backlog bookkeeping** — next-work aggregates, consumed markers, and enum normalization were cleaned up so carry-forward work is recorded consistently.

### Fixed

- **Pre-mortem gate ambiguity** — the crank pre-mortem gate now denies ambiguous state by default instead of failing open.
- **CLI and shell reliability edges** — `ao rpi serve --run-id` now accepts legacy 8-hex IDs, `ao mine --dry-run` emits a single clean JSON payload, and bash invocations are sanitized to bypass unsafe shell aliases.
- **Compile, harvest, and release drift** — compile repair defaults, malformed frontmatter salvage, YAML parse error surfacing, CI fixture drift, shellcheck drift, and Codex artifact metadata drift were corrected.

## [2.37.1] - 2026-04-15

### Added

- **Dream morning packets** — Dream can now emit ranked morning work packets with evidence, target files, exact follow-up commands, and queue/bead handoff metadata.
- **Dream yield telemetry and long-haul corroboration** — overnight reports now record packet-confidence telemetry and can trigger a bounded long-haul corroboration pass when the first pass produces weak morning output.

### Changed

- **Dream decision flow** — overnight runs now prefer cheaper evidence corroboration before slower council fan-out, so strong runs stay short and extended runtime is reserved for genuinely weak output.

### Fixed

- **Headless Claude Dream council** — Dream now uses Claude's working JSON output contract for headless council runs and normalizes the returned envelope before validation.
- **Dream close-loop and report surfaces** — overnight runs now write real close-loop callbacks and post-loop report artifacts instead of leaving placeholder `pending` steps.
- **Retrieval ratchet release gate fallback** — the retrieval-quality release check now falls back to checked-in eval data when a local manifest is absent.

## [2.37.0] - 2026-04-14

### Added

- **Windows install and smoke coverage** — `scripts/install-ao.ps1` adds a first-class Windows install path, and the blocking `windows-smoke` gate exercises PowerShell install, local `ao doctor`, and Windows-sensitive Go packages.
- **Compile command** — `ao compile` makes knowledge compilation a first-class CLI surface with docs and tests.
- **Local LLM forge pipeline** — `ao forge` can now redact, summarize, structurally review, and queue transcript-derived wiki pages with Dream worker integration.
- **Dream curator and evolve sub-cycle** — Dream gained a local curator adapter plus `ao evolve --dream-first|--dream-only`, allowing overnight knowledge passes to feed the daytime improvement loop.
- **`.agents` wiki surfaces** — INDEX, LOG, wiki directories, and search integration formalize `.agents/` as a Karpathy-style knowledge wiki with index-first navigation.
- **Operational quality surfaces** — beads audit/cluster commands, swarm preflight advice, status quality signals, retrieval eval queries, and a retrieval-quality CI ratchet broaden release-time proof.

### Changed

- **Knowledge scoring and search behavior** — inject now deduplicates by content hash, boosts indexed pages, weights stability, and search can pull Dream vault and wiki sources with stronger local recall.
- **Overnight and RPI internals** — overnight, lifecycle, search, inject, harvest, and RPI flows were decomposed into smaller helpers while tightening proof paths, mixed-mode provenance, and worktree cleanup.
- **Public framing and contributor docs** — README, philosophy, planning/post-mortem docs, and reference surfaces now better match the context-compiler and operational-layer story.

### Fixed

- **Windows overnight liveness** — Windows process checks no longer rely on Unix `signal(0)` semantics.
- **Dream RunLoop status invariants** — live-tree hash coverage now exercises every terminal RunLoop status, and `degraded` reflects the current rollback semantics.
- **Release retag safety** — release tooling now preserves annotated tags, validates audit artifact manifests and refs, and cancels stale reruns before duplicate publish attempts.
- **Post-mortem and closure audits** — metadata links, evidence-only closure packets, parser-path handling, and closure packet evidence modes were normalized.
- **Codex and runtime reliability** — same-thread lifecycle restart, root-scoped fallback reads, JSON config writes, bridge contract validation, and next-work proof-path handling were hardened.

## [2.36.0] - 2026-04-11

### Added

- **Evolve operator command** — `ao evolve` now exposes the v2 autonomous improvement loop directly in the CLI, including `--max-cycles`, `--queue`, `--beads-only`, `--quality`, `--compile`, and strict-quality passthrough flags.
- **Autodev program contract** — root `PROGRAM.md` gives evolve/autodev a repo-local operating contract with mutable and immutable scope, validation commands, escalation policy, and stop conditions.
- **Beads stale-scope tooling** — `ao beads verify|lint|harvest` adds first-class stale-citation checks for bead-driven planning and RPI recovery.
- **RPI discovery artifacts** — RPI can now persist and consume discovery artifacts, with tests and docs covering the `--discovery-artifact` path.
- **Dream RunLoop invariant coverage** — `TestRunLoop_LiveTreeHashInvariant_AllStatuses` locks the `IsCorpusCompounded()` and live-tree mutation invariant across deterministically reproducible terminal statuses.
- **Dream failed-summary contract coverage** — regression tests now lock the `finalizeOvernightSummary` contract for MEASURE consecutive-failure halts and persisted iteration history.
- **Dream operator mode** — `ao overnight start|run|report|setup` adds a private overnight lane with shared `dream.*` config, keep-awake defaults, scheduler/bootstrap guidance, council-ready runner packets, and DreamScape-style morning summaries
- **Nightly live retrieval proof** — the dream-cycle now runs `ao retrieval-bench --live --json`, emits retrieval proof in nightly summaries, and keeps a visible artifact trail for flywheel health
- **Pattern-to-skill drafts** — repeated patterns can now generate review-only skill drafts under `.agents/skill-drafts/` during flywheel close-loop
- **Fresh-repo onboarding welcome** — new session-start routing helps first-time repos enter discovery, implementation, or validation without needing the full RPI lane first
- **Docs-site and contribution proof surfaces** — GitHub Pages navigation, comparison pages, behavioral-discipline guidance, strategic-doc validation patterns, and a first-skill guide expand the public proof surface

### Changed

- **RPI wave recovery integrated** — recovered RPI wave work landed across Dream, council, stale-scope planning, discovery artifacts, CI hardening, and Codex runtime surfaces.
- **Council `--mixed` strict contract documented** — `skills/council/references/cli-spawning.md` documents that `/council --mixed` requires Codex CLI and emits a hard error instead of silently falling back to Claude-only.
- **Plan and pre-mortem skill bodies decomposed** — focused reference files now carry the detailed pre-decomposition, scope-mode, mandatory-check, output, wave-matrix, and task-creation guidance while keeping the top-level skills within lint budgets.
- **Bead-input pre-flight wired into planning skills** — `/plan` and `/pre-mortem` invoke `ao beads verify <bead-id>` for full-complexity, aged, or prior-session bead inputs before decomposition or validation.
- **Operational-layer framing** — README, onboarding, docs, comparisons, and linked surfaces now consistently explain AgentOps as bookkeeping, validation, primitives, and flows for coding agents
- **Dream runtime positioning** — the public GitHub nightly is now documented as a proof harness, while `ao overnight` is documented as the private local compounding engine
- **Codex default path** — native hooks, install copy, runtime smoke coverage, and checked-in Codex artifacts are aligned around the native-plugin path on supported Codex versions
- **Validation guidance** — behavioral-discipline and strategic-doc review are now first-class references alongside code review and runtime validation

### Fixed

- **Windows Codex installer** — Codex installation now has a Windows path instead of assuming Unix shell behavior.
- **golangci-lint v2 contract** — the local lint wrapper and CI configuration now pin the v2 behavior expected by the repository.
- **security-toolchain-gate CI** — deterministic fixture generation in `cli/internal/overnight/fixture/gen_fixture.go` is annotated as a non-cryptographic seeded-random use, avoiding a false-positive semgrep blocker.
- **Recovered RPI validation blockers** — validation drift from the recovered RPI wave was cleared before retagging the release.
- **Stale-scope reference placement** — shared stale-scope validation guidance now lives under `skills/shared/references/` so `heal.sh --strict` can resolve it consistently.
- **Release and CI drift** — resolved docs-site Liquid/frontmatter issues, headless runtime smoke portability problems, pre-push shim test drift, and compile-skill headless command drift caught during release prep
- **Codex install and artifact drift** — fixed stale slash-command references, refreshed checked-in artifact metadata, added a Codex compile wrapper, and corrected plugin/marketplace mismatches exercised by smoke coverage
- **Runtime proof stability** — promoted Codex runtime smoke into the blocking smoke path and fixed related shellcheck and install-surface rough edges

### Removed

- **DevOps-rooted tagline** — public framing no longer leads with the old DevOps-layer tagline; the Three Ways lineage remains supporting doctrine instead of the category label

## [2.35.0] - 2026-04-07

### Added

- **Codex native hooks** — AgentOps hooks now install natively into Codex CLI v0.115.0+ via `~/.codex/hooks.json`; 8 hooks wired (session-start, inject, flywheel-close, prompt-nudge, quality-signals, go-test-precommit, commit-review, ratchet-advance); installer enables the `hooks` feature flag, migrates deprecated `codex_hooks` configs, and upgrades from hookless fallback to native hook runtime
- **Knowledge compiler skill** — renamed athena → `/compile` with Karpathy-style incremental compilation, pluggable LLM backend (`AGENTOPS_COMPILE_RUNTIME=ollama|claude`), interlinked markdown wiki output at `.agents/compiled/`
- **App struct dependency injection** — `App` struct carries `ExecCommand`, `LookPath`, `RandReader`, `Stdout`, `Stderr` seams; gc bridge, events, executor, context relevance, tracker health, and stream modules accept injected dependencies instead of mutable package-level vars
- **Test shuffle in CI** — `-shuffle=on` added to `validate.yml` and `Makefile` test targets, exposing and fixing 6 ordering-dependent tests (cobra flag leaks, maturity var leaks, env var leaks)

### Changed

- **CLI internal extraction (waves 5-13)** — business logic extracted from `cmd/ao` monolith into 15 `internal/` domain packages (`rpi`, `search`, `context`, `quality`, `goals`, `lifecycle`, `bridge`, `forge`, `mine`, `plans`, `knowledge`, `storage`, `pool`, `taxonomy`, `worker`) using Options struct pattern for dependency injection
- **Goals test migration** — 7 goals test files moved from `cmd/ao` to `internal/goals` as external test package (`goals_test`) with `t.Parallel()` and direct `goals.Run*()` calls replacing cobra command wiring
- **Test isolation** — `resetCommandState` now saves/restores 10 maturity globals; `resetFlagChangesRecursive` resets flag values to defaults; RPILoop and toolchain tests clear `AGENTOPS_RPI_RUNTIME*` env vars via `t.Setenv`

### Fixed

- **Defrag test flag leak** — `TestDefragOutputDirFlag` used `cmd.Flags().Lookup("output")` which matched the root persistent `--output` flag; changed to `cmd.LocalFlags().Lookup("output")`
- **Goroutine leak false positive** — `TestRunGoals_GoroutineLeak` used `goleak.VerifyNone` which caught goroutines from parallel tests; switched to `goleak.IgnoreCurrent()` to only detect leaks within the test itself
- **Secret scan false positives** — excluded `.gc/` directory and `Getenv`/`os.Environ` patterns from secret pattern scan
- **Codex skill validation** — added `output_contract` as valid schema key, `cross-vendor`/`knowledge` as valid tiers, fixed `$/` prefix in codex forge/post-mortem/scenario skills
- **Scenario CLI snippets** — replaced non-existent `--source`/`--scope` flags with valid `--status` variants

### Removed

- **Coverage percentage CI gates** — removed `coverage-ratchet` job, `check-cmdao-coverage-floor.sh`, `.coverage-baseline.json`, and associated BATS tests; percentage gates blocked CI during architectural refactors without catching bugs
- **`fire.go`** — FIRE loop (find-ignite-reap-escalate) superseded by gc sling + bead dispatch; `formatAge` helper moved to `inject_predecessor.go`
- **`rpi_workers.go`** — per-worker health display superseded by gc agent health patrol; `ao rpi workers` subcommand removed from CLI and docs

## [2.34.0] - 2026-04-05

### Added

- **Stage 4 Behavioral Validation** — new validation tier between council/vibe and production:
  - Holdout scenarios stored in `.agents/holdout/` with PreToolUse isolation hook preventing implementing agents from seeing evaluation criteria
  - Satisfaction scoring (0.0-1.0 probabilistic) in verdict schema v4, replacing boolean-only PASS/FAIL
  - Agent-built behavioral specs generated during `/implement` Step 5c
  - `/scenario` skill for authoring and managing holdout scenarios
  - `ao scenario init|list|validate` CLI commands (4 subcommands, 11 tests)
  - STEP 1.8 in `/validation` pipeline evaluating holdout scenarios + agent specs
  - `schemas/scenario.v1.schema.json` defining the holdout scenario format
- **Flywheel gate command** — `ao flywheel gate` checks readiness for retrieval-expansion work (research closure, rho threshold, holdout precision@K)
- **Citation confidence scoring** — `citationEventIsHighConfidence` with bucketed confidence (0/0.5/0.7/0.9) gates MemRL rewards on match quality
- **Retrieval bench refactor** — train/holdout splits, section-aware scoring (`scoreBenchSections`), manifest-based benchmark cases
- **Proof-backed next-work visibility** — `classifyNextWorkCompletionProof` unifies completed-run, execution-packet, and evidence-only-closure proof types; context explain and stigmergic packet now report proof-backed suppressions
- **Three-gap contract proof gates** — lifecycle gap mapping gates added to GOALS.md
- **Cross-vendor execution** — `--mixed` flag for Claude + Codex council judges
- **Gas City bridge** — gc as default executor for RPI phase execution with L1-L3 tests
- **149 L2 integration tests** — AI-native test shape ("L2 first, L1 always") validated at scale; coverage floor raised 78.8% → 81.0%
- **Test coverage hardening** — GPG commit-signing fixes, root-skip guards for containerized CI, 350+ lines of vibecheck detector/metrics tests, maturity.go empty-content bugfix

### Changed

- **Codex parity hook** — `codex-parity-warn.sh` now supports opt-in blocking mode via `AGENTOPS_CODEX_PARITY_BLOCK=1` (exit 2 instead of advisory)
- **12-factor doctrine** — compressed from 474 to 114 lines, reframed as supporting lens rather than product definition
- **Skill count** — 65 → 66 (added `/scenario`)
- **Research skill** — now persists reusable findings to `.agents/findings/registry.jsonl` with finding-compiler refresh
- **Closure integrity audit** — accepts durable closure packets without scoped-file sections as valid evidence
- **Proof-backed legacy entries** — `shouldSkipLegacyFailedEntry` uses `CompletionEvidence` field (proof-only, no heuristic fallback)
- **`readQueueEntries`** — returns all non-consumed entries; proof filtering is downstream via `shouldSkipLegacyFailedEntry`

### Fixed

- **6 CI failure categories** resolved in one commit (f1b83b25)
- **Cobra test registration** — `scenario` and `flywheel gate` added to expectedCmds
- **Citation feedback test** — assertion corrected for recorded confidence preference (0.5 not 0.7)
- **RPI hardening** — UAT version pre-flight, goals history filter, proof-backed suppression, fail-closed gates, cross-epic handoff contamination, bare ag- prefix guard
- **Branch consolidation** — 10 stale Codex branches analyzed, cherry-picked (9 commits, ~3,500 lines), and deleted; 25 orphaned worktrees pruned
- **git rerere enabled** — conflict resolution memory for future merges

## [2.33.0] - 2026-04-02

### Added

- **Backlog hygiene gates** — added `bd-audit.sh`, `bd-cluster.sh`, and Crank/Codex guidance for cleaning stale or mergeable beads before execution
- **Retrieval benchmarking and global scope** — added `ao retrieval-bench`, benchmark corpora, `--live`, `--global`, and nightly IR regression coverage
- **`/red-team` adversarial validation** — added a persona-based validation skill plus checked-in Codex runtime artifacts
- **Software factory operator lane** — added a CLI/operator surface and Claude factory startup routing for software-factory workflows
- **Flywheel maintenance utilities** — added global garbage purge tooling and nightly retrieval benchmarking for knowledge quality tracking

### Changed

- **Release policy** — removed the enforced release cadence gate so releases no longer block on a minimum wait between tags
- **Knowledge operator surfaces** — plan and validation now wire knowledge operator surfaces directly into execution flow
- **Proof and runtime docs** — goals, RPI docs, and contributor guidance now reflect the expanded proof surfaces and hookless runtime behavior

### Fixed

- **Codex artifact parity** — restored checked-in Codex parity for red-team and cleaned Codex runtime metadata/frontmatter drift across crank, forge, post-mortem, release, and swarm artifacts
- **Retrieval quality** — replaced exact-substring filtering with token-level matching and tuned penalty, deduplication, and OR-fallback behavior
- **Harvest metadata preservation** — promotion now preserves source metadata and fills missing maturity, utility, and type fields safely
- **Release tooling** — release artifact directories are created safely and audit artifacts now resolve against release tag names
- **Documentation and link drift** — repaired the post-mortem Codex link and aligned runtime docs around the newer startup and lifecycle flows

## [2.32.0] - 2026-04-01

### Added

- **Knowledge activation skill** — new `/knowledge-activation` skill and CLI surfaces for activating cross-domain knowledge at runtime, with operator surface consumption and ranked intelligence context
- **Session intelligence engine** — complete runtime engine with explainability, ranked context assembly, and trust policy enforcement
- **Runtime selection for `ao rpi serve`** — serve now supports explicit runtime selection for Claude and Codex execution modes
- **Quality signals hook** — new `quality-signals.sh` hook with test coverage for session quality telemetry
- **Pre-push gate expansion** — 9 checks migrated from CI-only to the local pre-push gate for faster feedback
- **Inject stability warnings and status dashboard** — closed 3 harvest items with signal tests and dashboard improvements

### Changed

- **README refresh** — product-minded rewrite with gain-framing and Strunk-style prose fixes
- **Philosophy doc** — new `docs/philosophy.md` and observations section added to README
- **Documentation alignment** — repo front doors and codex artifact guidance unified across entry points
- **Claude Code architecture lessons** — retry budgets, stability flags, quality signals, and orchestration patterns applied to skills
- **Homebrew formula** — updated to v2.31.0 with pre-built binaries

### Fixed

- **Post-mortem closure integrity** — normalized file parsing for closure integrity audits
- **CI reliability** — resolved CI failures across codex refs, test pairing, hook coverage, worktree handling, docs parity, hook portability, and codex lifecycle
- **Lookup nested scanning** — `ao lookup` now scans nested global knowledge directories correctly
- **Pre-push test stubs** — added test stubs for new pre-push checks, skip non-shell in shellcheck

### Dependencies

- Bumped `codecov/codecov-action` from 5 to 6
- Bumped `DavidAnson/markdownlint-cli2-action` from 22 to 23

## [2.31.0] - 2026-03-30

### Added

- **9 lifecycle skills** — bootstrap, deps, design, harvest, perf, refactor, review, scaffold, and test skills wired into RPI with auto-invocation and mechanical gates
- **`ao harvest`** — cross-rig knowledge consolidation extracts and catalogs learnings from sibling crew workspaces
- **`ao context packet`** — inspect stigmergic context packets for debugging inter-session handoff state
- **Hook runtime contract** — formal Claude/Codex/manual event mapping with runtime-aware hook tooling
- **Evidence-driven skill enrichment** — production meta-knowledge, anti-patterns, flywheel metrics, and normalization defect detection baked into 9 skill reference files
- **Research provenance** — pending learnings now carry full research provenance for discoverability and citation tracking
- **Context declarations** — inject, provenance, and rpi skills declare their context requirements explicitly
- **Goals and product output templates** — `/goals` and `/product` produce evidence-backed structured output

### Changed

- **Three-gap context lifecycle contract** — README, PRODUCT.md, positioning docs, and operational guides reframed around the context lifecycle model
- **Dual-runtime hook documentation** — runtime modes table and troubleshooting updated for Claude + Codex hook coexistence

### Fixed

- **CI reliability** — resolved 4 pre-existing CI failures, restored headless runtime preflight, repaired codex parity drift checks
- **`ao lookup` retrieval** — fixed retrieval gaps that caused lookup to return no results
- **Embedded sync** — using-agentops SKILL.md and `.agents/.gitignore` now written correctly on first session start
- **Closure integrity** — 24h grace window for close-before-commit evidence, normalized file parsing
- **Skill lint compliance** — vibe, post-mortem, crank, and plan skills trimmed or restructured to stay under 800-line limit
- **Codex tool naming** — added CLAUDE_TOOL_NAMING rule and fixed 5 Claude-era tool references in codex skills
- **ASCII diagram consistency** — aligned box-drawing characters across 23 documentation files
- **Fork exhaustion prevention** — replaced jq with awk in validate-go-fast to prevent fork bombs on large repos

## [2.30.0] - 2026-03-24

### Added

- **Codex hookless lifecycle support** — `ao codex` runtime commands, lifecycle fallback, and Codex skill orchestration now cover hookless sessions end to end
- **PROGRAM.md autodev contract** — Added a first-class `PROGRAM.md` contract for autodev flows and taught `/evolve` and related RPI paths to use it
- **Long-running RPI artifact visibility** — Mission control now exposes run artifacts and evaluator output so long-running RPI sessions are replayable and easier to inspect

### Changed

- **Codex runtime maintenance flow** — Refreshed Codex bundle hashes, lifecycle guards, runtime docs, and release validation coverage around the expanded Codex execution path

### Fixed

- **Codex RPI scoping and closeout** — Tightened objective scope, epic scope, closeout ownership, and validation gaps in the Codex RPI lifecycle
- **Release gate reliability** — Restored headless runtime coverage, runtime-aware Claude inventory checks, and release-gate coherence validation
- **Reverse-engineer repo hygiene** — Repo-mode reverse engineer now ignores generated and temp trees when identifying CLI and module surfaces

## [2.29.0] - 2026-03-22

### Added

- **Model cost tiers and config writes** — `ao config` can now assign per-agent models by cost tier and persist repo configuration changes directly
- **Search brokerage over session history and repo knowledge** — `ao search` now wraps upstream `cass` results with repo-local AgentOps artifacts by default
- **Reviewer and post-mortem reference packs** — Added model-routing, iterative-retrieval, confidence-scoring, write-time-quality, and conflict-recovery guidance across council, research, swarm, vibe, compile, and related skills

### Changed

- **Competitive comparison and CLI docs** — Refreshed comparison docs, release smoke coverage, and command documentation around the expanded search/config surface

### Fixed

- **Flywheel proof and citation loop** — Added deterministic proof fixtures, preserved exact research provenance, and made citation feedback artifact-specific so flywheel health reflects real closure state
- **Search alignment with forged session history** — Search now stays aligned with forged session artifacts and fallback behavior
- **Hook-launched validation** — Pre-push and release gates now isolate inherited git env/stdin correctly and cover newer hook scripts in integration tests
- **Codex council profile parity** — Source and checked-in Codex council docs are back in sync for the shared profile contract

## [2.28.0] - 2026-03-21

### Added

- **Node repair operator** — Crank now classifies task failures as RETRY (transient), DECOMPOSE (too complex), or PRUNE (blocked) with budget-controlled recovery
- **Knowledge refresh auto-trigger** — Lightweight compile defrag runs automatically at session end via new SessionEnd hook
- **Configurable review agents** — Project-level `.agents/reviewer-config.md` controls which judge perspectives council and vibe spawn
- **Three-tier plan detail scaling** — Plan auto-selects Minimal, Standard, or Deep templates based on issue count and complexity
- **Adversarial ideation** — Brainstorm Phase 3b stress-tests each approach with four red-team questions before user selection

### Fixed

- **Crank SKILL.md line limit** — Consolidated duplicate References sections to stay under 800-line skill lint limit
- **Codex skill parity** — Synced all five competitive features to skills-codex with reference file copies

## [2.27.1] - 2026-03-20

### Fixed

- **Flywheel golden signals always shown** — Golden signals were gated behind `--golden` flag, causing `ao flywheel status` to report "COMPOUNDING" while the hidden golden signals analysis showed "accumulating". Golden signals now compute and display by default.

## [2.27.0] - 2026-03-20

### Added

- **Flywheel golden signals** — Four derived health indicators (velocity trend, citation pipeline, research closure, reuse concentration) that distinguish knowledge compounding from noise accumulation; accessible via `ao flywheel status --golden`
- **Forge-to-pool bridge** — Forge auto-writes pending learnings as markdown to `.agents/knowledge/pending/` for close-loop pool ingestion
- **SessionStart citation priming** — `ao lookup` wired into SessionStart hook to close the citation gap between inject and session context
- **Skill catalog quality** — Improved descriptions, extraction patterns, and reference linking across skill catalog

### Fixed

- **`.agents/.gitignore` scope** — Replaced broad `!*/` pattern with explicit subdirectory list to prevent accidental tracking
- **Codex runtime skill parity** — Hardened Codex runtime skill discovery and validation
- **Codex install smoke tests** — Fixed test assertions for install path edge cases

### Changed

- **CLI reference docs** — Regenerated with updated date stamps

## [2.26.1] - 2026-03-16

### Fixed

- **RPI stops after Phase 2** — Restructured rpi, discovery, and validation orchestrator skills as compact DAGs with execution sequence in a single code block; eliminates LLM stopping between phases due to `###` section headings acting as natural breakpoints
- **Test grep patterns for DAG headings** — Updated `test-tuning-defaults.sh` to match new complexity-scaled gate headings after DAG restructure

### Changed

- **Goals reimagined** — GOALS.md rebuilt from first principles with fitness gate fixes
- **README progressive disclosure** — Lead with moats, collapse detail into expandable sections
- **CLI reference docs** — Regenerated with updated date stamps
- **Doctor + findings helpers** — Added CLI test coverage for extracted helpers

## [2.26.0] - 2026-03-15

### Added

- **BF6–BF9 test pyramid levels** — Regression (bug-specific replay), Performance/Benchmark, Backward Compatibility, and Security (in-test) bug-finding levels with language-specific patterns for Go and Python
- **Test pyramid decision tree expansion** — 4 new routing questions for BF6–BF9 in the "When to Use" guide
- **RPI phase mapping for BF6–BF9** — Bug fix → BF6 mandatory, hot-path → BF7 benchmark, format change → BF8 compat fixture, secrets → BF9 redaction tests
- **`regen-codex-hashes.sh`** — Manifest hash regeneration script for Codex skill maintenance

### Changed

- **Go standards** — Added benchmark tests (BF7), backward compat with `testdata/compat/` (BF8), regression test naming convention (BF6), security tests for path traversal (BF9)
- **Python standards** — Added Hypothesis property-based testing (BF1), `pytest-benchmark` patterns (BF7), backward compat with parametrized fixtures (BF8), regression test naming (BF6), secrets redaction tests (BF9)
- **Coverage assessment template** — Extended BF pyramid table from BF1–BF5 to BF1–BF9

### Fixed

- **Codex skill audit** — 60+ findings fixed across all 54 Codex skills; removed orphaned `claude-code-latest-features.md` and `claude-cli-verified-commands.md` references
- **Skill lint warnings** — Resolved all warnings in crank, rpi, recover skills
- **README skill references** — Corrected broken references and linked orphaned templates
- **Skill linter refs** — Fixed directory reference and backtick formatting in reverse-engineer-rpi
- **CHANGELOG sync hook** — Replaced broken awk extraction with sed; awk failed on em-dash UTF-8 content producing header-only syncs
- **Plugin version parity** — Added pre-commit check that warns when `.claude-plugin/` manifest versions don't match the release version

## [2.25.1] - 2026-03-15

### Fixed

- **Codex BF pyramid parity** — Synced BF1/BF2/BF4 bug-finding level selection into skills-codex implement, post-mortem, and validation skills
- **Codex Claude backend cross-contamination** — Removed orphaned `backend-claude-teams.md` files (Claude primitives: TeamCreate, SendMessage) from 4 Codex skills (council, research, shared, swarm)
- **Dead converter rule** — Removed stale sed substitution for `backend-claude-teams.md` rename in converter script
- **Swarm reference integrity** — Added Reference Documents section to swarm SKILL.md; updated validate.sh to check only Codex-native backend references

## [2.25.0] - 2026-03-14

### Added

- **L0–L7 test pyramid standard** — Shared reference doc (`standards/references/test-pyramid.md`) defining 8 test levels, agent autonomy boundaries (L0–L3 autonomous, L4+ human-guided), and RPI phase mapping
- **Test pyramid integration across RPI lifecycle** — Discovery identifies test levels, plan classifies tests by level, pre-mortem validates coverage, implement selects TDD level, crank carries `test_levels` metadata, validation audits coverage, post-mortem reports gaps
- **RPI autonomous execution enforcement** — Three-Phase Rule mandates discovery → implementation → validation without human interruption; anti-patterns table documents 7 failure modes
- **Evolve autonomous execution enforcement** — Each cycle runs a complete 3-phase `/rpi --auto`; anti-patterns table documents 6 failure modes; large work decomposed into sub-RPI cycles
- **Codex skill standard** — New `standards/references/codex-skill.md` with tool mapping, prohibited primitives, two-phase validation, DAG-first traversal, and prompt constraint boundaries
- **Codex-native overrides** — Durable overrides for crank, swarm, council that survive regeneration
- **DAG-based Codex smoke test** — `scripts/smoke-test-codex-skills.sh` validates 54 skills with dependency-ordered traversal
- **Codex skill API contract** — `docs/contracts/codex-skill-api.md` with conformance validator
- **Output contract declarations** — `output_contract` field on council, vibe, pre-mortem, research skills with canonical finding-item schema

### Changed

- **Codex converter rewrite** — Strips Claude primitives instead of mapping to unavailable tools; rewrites reference files through `codex_rewrite_text`
- **CI pipeline** — Removed codex skill parity check (skills-codex/ now manually maintained); fixed shellcheck and embedded sync issues

### Fixed

- **Converter primitive stripping** — Task primitives (TaskCreate, TeamCreate, SendMessage) properly stripped instead of mapped to non-existent Codex equivalents
- **Embedded hook sync** — Added missing `test-pyramid.md` and `codex-skill.md` to CLI embedded references
- **ShellCheck SC1125** — Fixed em-dash in shellcheck disable directive in smoke test script
- **Skill line limits** — Moved verbose autonomy rules to reference files to stay under tier-specific line budgets

## [2.24.0] - 2026-03-12

### Added

- **Error & rescue map template** — Pre-mortem Step 2.5 with 3 worked examples (HTTP, database, LLM)
- **Scope mode selection** — Pre-mortem Step 1.6 with 3-mode framework (Expand/Hold/Reduce) and auto-detection
- **Temporal interrogation** — Pre-mortem Step 2.4 walks implementation timeline (hour 1/2/4/6+) for time-dependent risks
- **Prediction tracking** — Pre-mortem findings get unique IDs (`pm-YYYYMMDD-NNN`) correlated through vibe and post-mortem
- **Finding classification** — Vibe separates CRITICAL (blocks ship) from INFORMATIONAL findings
- **Suppression framework** — Vibe loads default + project-level suppression patterns for known false positives
- **Domain-specific checklists** — Standards skill extended with SQL safety, LLM trust boundary, and race condition checklists, auto-loaded by vibe
- **RPI session streak tracking** — Post-mortem Step 1.5 shows consecutive session days and verdict history
- **Persistent retro history** — Post-mortem Step 4.8 writes structured JSON summaries to `.agents/retro/` for cross-epic trend analysis
- **Prediction accuracy scoring** — Post-mortem Step 3.5 scores HIT/MISS/SURPRISE against pre-mortem predictions
- **Commit split advisor** — PR-prep Phase 4.5 suggests bisectable commit ordering (suggestion-only)
- **Council finding auto-extraction** — Significant findings from WARN/FAIL verdicts staged for flywheel consumption

### Changed

- **Post-mortem examples condensed** — Verbose examples replaced with concise 4-mode summary to stay under skill line limit

## [2.23.1] - 2026-03-12

### Fixed

- Resolved all golangci-lint quality findings
- Synced embedded standards after skill audit fixes
- Synced Codex bundle after skill audit fixes
- Resolved audit findings across council, vibe, standards skills

## [2.23.0] - 2026-03-11

### Added

- **Discovery and validation phase orchestrators** — New `/discovery` and
  `/validation` skills decompose the RPI lifecycle into independently
  invocable phases (research+plan+pre-mortem and vibe+post-mortem)
- **Stigmergic packet scorecard** — Ranked scoring for flywheel knowledge
  packets so higher-utility learnings surface first
- **Pinned work queue** — `/evolve` gains a pinned work queue with blocker
  auto-resolution for directed improvement loops
- **Per-package coverage ratchet** — Pre-push gate enforces per-package
  coverage baselines that only move upward
- **Fast pre-push mode** — `--fast` flag for diff-based conditional checks,
  skipping unchanged packages
- **Standards auto-loading** — Go and Python coding standards injected
  automatically into `/crank` and `/swarm` workers
- **271 test functions** — Four internal packages (`pool`, `ratchet`,
  `resolver`, `storage`) brought to 100% coverage

### Changed

- **README restructured** — Extracted reference material into dedicated docs,
  reducing README from 679 to 472 lines
- **RPI skill refactored** — `/rpi` now delegates to `/discovery` and
  `/validation` phase orchestrators instead of inlining all phases
- **Go and Python test conventions** — Canonical standards enriched with
  assertion quality rules, naming conventions, and table-driven test guidance
- **Documentation alignment** — Lifecycle, flywheel, primitive chain, and
  positioning docs updated to reflect current architecture

### Fixed

- **Goal runner deadlock** — Fixed goroutine deadlock in goal runner and added
  job timeouts to prevent stalls
- **17 CLI bugs from deep audit** — Addressed goroutine leaks, race
  conditions, panics, buffer overflows, and nil-check inconsistencies
- **Session close reliability** — Resolved pre-existing session_close issues
  surfaced by vibe council review
- **~50 zero-assertion tests** — Upgraded smoke tests from no-op to
  behavioral assertions across cmd/ao and internal packages
- **Test file hygiene** — Merged `_extra_test.go` and `cov*_test.go` files
  into canonical `<source>_test.go` names
- **CI stability** — FIFO test skip on Linux, embedded skill sync, coverage
  ceiling adjustments, crank SKILL.md trimmed below 800-line limit
- **Auto-extract quality gate** — Added quality gate to prevent low-fidelity
  auto-extracted learnings from entering the knowledge store

## [2.22.1] - 2026-03-10

### Added

- **Repo-native redteam harness** — Added a packaged redteam pack and prompt
  runner to `security-suite` for repeatable repository-local security
  exercises
- **Findings management commands** — Added CLI commands for listing and
  managing saved findings from the terminal

### Changed

- **Closed-loop prevention validation** — Completed the end-to-end finding
  compiler and prevention-ratchet validation path so saved findings feed back
  into earlier planning and task validation more reliably
- **Runtime contract parity** — Localized shared Claude runtime reference
  packs into the source skills and regenerated Codex artifacts so source and
  generated bundles stay aligned

### Fixed

- **Finding metadata injection** — Exposed finding metadata consistently in
  inject output and JSON integrations after the merged findings work landed
- **Release gate regressions** — Restored goals/package coverage, learning
  coherence, and hook-fixture isolation so the local release gate matches the
  shipped tree again

## [2.22.0] - 2026-03-09

### Added

- **Finding registry** — Council findings are saved to a persistent registry
  and automatically fed back into planning and validation, so the same class
  of bug is caught earlier next time
- **Repo execution profiles** — `.repo-execution-profile.json` lets skills
  and runtimes adapt to each repository's validation gates, startup reads,
  and done-criteria
- **Headless team backend** — Multi-agent workflows can run non-interactively
  (e.g. in CI) with structured JSON output and automated validation

### Changed

- **Codex and embedded artifacts** — Synced generated Codex bundles, embedded
  standards references, and install artifacts after merging branch work
- **Validation feedback capture** — Recorded validation-cycle feedback into
  `.agents` learnings so tracked patterns match the shipped tree

### Fixed

- **Lookup findings** — Fixed `ao lookup` and inject scoring so findings
  render, cite, and score correctly after the branch merge
- **23 CLI bug fixes** — Fixed goroutine leaks, race conditions, panics,
  buffer overflows, missing error handling, and nil-check inconsistencies
- **Post-mortem evidence hardening** — Staged changes and worktree evidence
  are now captured durably so proof isn't lost during compaction or cleanup

## [2.21.0] - 2026-03-09

### Added
- Codex-first skill rollout across the full catalog with override coverage, generated-artifact governance, and install/runtime parity validation
- Claim-aware next-work lifecycle handling with contract parity checks for `/rpi` and follow-on flows
- Headless runtime skill smoke coverage and Codex backbone prompt validation in the release gate stack

### Changed
- Codex maintenance guidance, override coverage docs, and CLI-to-skills mapping to match the generated runtime model
- Release-prep validation flows for runtime smoke, Codex artifact sync, and release note generation

### Fixed
- Next-work queue mutation races by making claim/update handling concurrency-safe and per-item
- Codex prompt parity drift by syncing generated prompts and tightening override coverage gates
- Worktree Git resolution and vibe-check runtime environment handling
- Push/pre-push validation regressions and nested pre-push wrappers
- Streamed phase timeout cancellation so phased runtime tests and release gating terminate promptly

## [2.20.1] - 2026-03-07

### Fixed
- Codex install workflow now uses `~/.agents/skills` as the single raw skill home and stops recreating an AgentOps mirror in `~/.codex/skills`
- Native Codex plugin refresh now archives overlapping legacy `~/.codex/skills` AgentOps folders instead of repopulating them
- Codex install docs now consistently describe the `~/.agents/skills` workflow and the need for a fresh Codex session after install
- Codex skill conversion now preserves multiline YAML `description` fields correctly, fixing malformed generated metadata for skills such as Compile
- `ao doctor` now treats plugin-cache plus `~/.agents/skills` as the supported Codex layout and reports manifest drift with accurate wording

## [2.20.0] - 2026-03-05

### Added
- Flywheel loop closure — `ao session close --auto-extract` produces lightweight learnings and auto-handoff at session boundary
- Handoff-to-learnings bridge — `ao handoff` now extracts decisions into `.agents/learnings/` automatically
- Session-type scoring in `ao inject --session-type` — 30% boost for matching session context (career, debug, research, brainstorm)
- Identity artifact support — `ao inject --profile` surfaces `.agents/profile.md` in session context
- MEMORY.md auto-promotion in `ao flywheel close-loop` (Step 7) after maturity transitions
- Session-type detection in `ao forge` output metadata
- Production RPI orchestration engine — `ao rpi serve <goal>` with SSE streaming and auto mode
- Knowledge mining — `ao mine` and `ao defrag` commands for automated codebase intelligence
- Context declarations — `ao inject --for <skill>` reads skill frontmatter `context:` block for scoped retrieval
- Sections include allowlist and context artifact directories for skill-scoped injection
- `ao handoff` command for structured session boundary isolation
- Behavioral guardrails — 3-layer hook defense-in-depth (intent-echo, research-loop-detector, task-validation-gate)
- Context enforcement hook and run-id namespaced artifact paths
- Headless invocation standards and RPI phase runner
- Nightly CI compile job for automated knowledge warmup
- Coverage ratchet gate with BATS integration tests for shell scripts
- Fuzz targets, property tests, and golden file contracts for CLI
- Git worker guard, embedded parity gate, and swarm evidence validation hooks
- Release cadence gate warns on releases within 7 days of previous

### Changed
- Coverage floor raised to 84% for `cmd/ao`, average floor to 95%
- Complexity ceiling tightened to 20 (from 25)
- Default session-start hook mode switched from manual to lean
- Hard quality gate on injection — maturity + utility filter
- Post-mortem redesigned as knowledge lifecycle processor
- RPI god-file split — 1,363 lines reduced to 203 with structured handoff schema
- Legacy RPI orchestrator retired — serve now uses phased engine (-1,121 lines)
- Council V2 findings synthesized into agent instructions and skill contracts
- 10k LOC of coverage-padding tests deleted; 72 stale tests quarantined
- Skill hardening — web security controls across 5 skills, CSRF protection, crank pre-flight
- Session-end hook wires `ao session close --auto-extract` before existing forge pipeline

### Fixed
- Flywheel signal chain — confidence decay, close-loop ordering, glob errors
- Path traversal in context enforcement hook and frontmatter parsing
- Race condition in handoff consumption at session boundary
- `ao mine` stabilized — dedup IDs, error propagation, `--since` window, empty output guard
- Hook test assertions aligned with warn-then-fail ratchet pattern (strict env required)
- Pre-mortem gate exit code corrected to 2 in strict mode (was 1)
- RPI serve event pipeline and coherence gate hardened
- jq injection via bare 8-hex run IDs in serve classifier
- Goals parser edge cases — paired backtick strip and rune-aware truncation
- UTF-8 truncation across six functions converted to rune-safe slicing
- CORS headers and stale doc references cleaned up
- Cross-wave worktree file collisions prevented
- hookEventName added to hookSpecificOutput JSON schema

## [2.19.3] - 2026-02-27

### Changed
- README highlights `ao search` (built on CASS) — indexes all chat sessions from every runtime unconditionally; adds Second Brain + Obsidian vault section with Smart Connections local/GPU embeddings and MCP semantic retrieval

## [2.19.2] - 2026-02-27

### Fixed
- CHANGELOG retrospectively updated to document all v2.19.1 post-tag commits (skills namespace fixes were shipped but not recorded)

## [2.19.1] - 2026-02-27

### Fixed
- Quickstart skill rewritten from 275 lines to 68 lines — removes 90-line ASCII diagram and 50-line intent router that caused 3+ minute runtime; now outputs ~8 lines and completes in under 30 seconds
- `truncateText` edge case: maxLen 1–3 now returns `"..."[:maxLen]` instead of the original string unchanged
- Dead anti-pattern promotion functions removed from `ao maturity` (`promoteAntiPatternsCmd`, `filterTransitionsByNewMaturity`, `displayAntiPatternCandidates`, ~99 LOC)
- Windows file-lock and signal support — replace no-op `filelock_windows.go` with real `LockFileEx`/`UnlockFileEx` via kernel32.dll; extract `syscall.Flock` and `syscall.Kill` into platform-specific helpers so the binary compiles on Windows without POSIX-only syscalls
- `heal.sh` Check 7 false positive — script reference integrity check now strips URLs before pattern matching, preventing remote `https://…/scripts/foo.sh` references from being validated as local files
- Security gate `BLOCKED_HIGH` — three persistent findings resolved: gosec G118 false positive (context cancel func returned to caller), golangci-lint nolint syntax (space in `// nolint:` directive), radon double-counting `reverse_engineer_rpi.py` from `skills-codex/` copy
- 71 stale `ao know *` and `ao quality *` namespace references replaced across 17 `skills-codex/` SKILL.md files — agents running rpi/evolve/crank were invoking non-existent commands from the pre-flatten CLI namespace
- Three HIGH-severity stale command references fixed across `skills/` and `skills-codex/`: `ao flywheel status` → `ao metrics flywheel status`, `ao settings notebook update` → `ao notebook update`, `ao start seed/init` → `ao seed`/`ao init`

### Added
- Spec-consistency gate (`scripts/spec-consistency-gate.sh`) validates contract files before crank spawns workers
- Command-surface parity gate (`scripts/check-cmdao-surface-parity.sh`) ensures all CLI leaf commands are tested
- `scripts/post-merge-check.sh` now validates `go mod tidy` sync and blocks on symlinks
- `scripts/merge-worktrees.sh` now propagates file deletions and preserves permissions
- Post-mortem preflight script checks reference file existence before council runs
- Hooks.json preflight validates script existence
- Windows binaries added to GoReleaser and SLSA attestation subject list

### Changed
- Coverage floor raised 78% → 80% with CI enforcement gate; Codecov threshold aligned to 75%
- Six truncation functions converted to rune-safe Unicode slicing
- `truncateID` in pool.go delegates to shared `truncateText`
- Crank skill invokes spec-consistency gate before spawning workers
- Vibe skill carries forward unconsumed high-severity next-work items as pre-flight context
- Release skill warns on unconsumed high-severity next-work items
- next-work JSONL schema formalized to v1.2
- Skills installation switched from `npx skills` to native curl installer (`bash <(curl -fsSL …/install.sh)`)
- README updated with 5-command summary, compound effect section, and `/vibe` breakdown

## [2.19.0] - 2026-02-27

### Added
- `ao mind` command for knowledge graph operations.
- New RPI operator surfaces: normalized C2/event plumbing plus `ao rpi stream`, `ao rpi workers`, and tmux worker nudge visibility.
- Codex install/bootstrap improvements, including native `~/.codex/skills` install and one-line installer flow.
- Windows binaries added to GoReleaser build outputs.

### Changed
- CLI namespace migration completed and aligned across hooks, docs, integration tests, and generated command references.
- Codex skill system moved to regenerated modular layout with codex-specific overrides and runtime prompt tailoring.
- CI/release gates hardened (codex runtime sections, release e2e validation, parity checks, stricter policy enforcement).
- High-complexity CLI paths refactored (`runRPIParallel`, `runDedup`, `parseGatesTable`) to lower cyclomatic complexity.

### Fixed
- Multiple post-mortem remediation waves landed for CLI/RPI/swarm reliability and edge-case handling.
- Hook delegation and integration behavior corrected for flat command namespace.
- `heal.sh` false-positive behavior reduced and doctor stale-path detection improved.
- Skill/doc parity and cross-reference drift issues corrected across codex and core skill catalogs.

### Removed
- Legacy inbox/mail command surface and stale/dead skill references from active catalogs.

## [2.18.2] - 2026-02-25

### Fixed
- `ao seed` now creates `.gitignore` and storage directories — reuses `setupGitProtection`, `ensureNestedAgentsGitignore`, and `initStorage` from `ao init`
- `ao seed` text updated from stale `ao inject`/`ao forge` to current MEMORY.md + session hooks paradigm
- MemRL feedback loop closed — `ao feedback-loop` command wired, `ao maturity --recalibrate` dry-run guard added
- Quickstart skill updated to reference `ao seed` and current flywheel docs
- CLI reference regenerated after `ao feedback-loop` and seed help text changes

### Changed
- `.agents/` session artifacts removed from git tracking
- PRODUCT.md updated — Olympus section removed, value props and skill tier counts corrected
- GOALS.md coverage directive updated to measured 78.8% (target 85%)

## [2.18.1] - 2026-02-25

### Changed
- SessionStart hook default mode changed from `manual` to `lean` — flywheel injection now fires every session
- Auto-prune enabled by default (`AGENTOPS_AUTO_PRUNE` defaults to `1`, opt-out via `=0`)
- Anti-pattern detection threshold lowered from `harmful_count >= 5` to `>= 3`
- Eviction confidence threshold relaxed from `< 0.2` to `< 0.3`
- Maturity promotion threshold in `--help` text synced with code (`0.7` → `0.55`)

### Fixed
- Empty learnings no longer inflate flywheel metrics — extract prompt skips empty files, pool ingest rejects "no significant learnings" stubs
- `ao pool ingest` now runs automatically in session-end hook after forge (was manual-only)
- 8 stale doc/comment references to old thresholds updated across hooks, ENV-VARS.md, HOOKS.md, using-agentops skill
- 13 empty stub learnings removed from `.agents/learnings/`

## [2.18.0] - 2026-02-25

### Added
- `ao notebook update` command — compound MEMORY.md loop that merges latest session insights into structured sections
- `ao memory sync` command — sync session history to repo-root MEMORY.md with managed block markers for cross-runtime access (Codex, OpenCode)
- `ao seed` command — plant AgentOps in any repository with auto-detected templates (go-cli, python-lib, web-app, rust-cli, generic)
- `ao lookup` command — retrieve specific knowledge artifacts by ID or relevance query (two-phase complement to `ao inject --index-only`)
- `ao constraint` command family — manage compiled constraints (list, activate, retire, review)
- `ao curate` command family — curation pipeline operations (catalog, verify, status)
- `ao dedup` command — detect near-duplicate learnings with optional `--merge` auto-resolution
- `ao contradict` command — detect potentially contradictory learnings
- `ao metrics health` subcommand — flywheel health metrics (sigma, rho, delta, escape velocity)
- `ao context assemble` command — build 5-section context packet briefings for tasks
- Work-scoped knowledge injection: `ao inject --bead <id>` boosts learnings tagged with the active bead
- Predecessor context injection: `ao inject --predecessor <handoff-path>` surfaces structured handoff context
- Compact knowledge index: `ao inject --index-only` outputs ~200 token index table for JIT retrieval
- Learning schema extended with `source_bead` and `source_phase` fields for work-context tracking
- `ao extract --bead <id>` tags extracted learnings with the active bead ID
- Citation-to-utility feedback pipeline in flywheel close-loop (stage 5)
- Global `~/.agents/` knowledge tier for cross-repo learning sharing (0.8 weight penalty, deduped)
- Bead metadata resolver reads from env vars (`HOOK_BEAD_TITLE`, `HOOK_BEAD_LABELS`) or cache file
- Goal templates embedded in binary (go-cli, python-lib, web-app, rust-cli, generic) for `ao goals init --template` and `ao seed`
- Platform-specific process-group isolation for goal check timeouts (Unix: SIGKILL pgid, Windows: taskkill /T)
- SessionStart hook rewritten with 3 startup modes: lean (default), manual, legacy — via `AGENTOPS_STARTUP_CONTEXT_MODE`
- SessionEnd hook now gates notebook update and memory sync on successful forge
- Type 3 setup hook template: `hooks/examples/50-agentops-bootstrap.sh`
- Constraint compiler hook: `hooks/constraint-compiler.sh`
- Codex-native skill format (`skills-codex/`) with install and sync scripts for cross-runtime skill delivery
- Comprehensive cmd/ao test coverage push — 500+ tests across 5 waves reaching 79.2% statement coverage (13 untestable functions excluded)

### Changed
- SessionStart hook default mode changed from full inject to `lean` (extract + lean inject, shrinks when MEMORY.md is fresh)
- `ao flywheel close-loop` now applies ALL maturity transitions (not just anti-pattern)
- `ao hooks` generated config uses script-based commands instead of inline ao invocations
- `ao rpi` prefers epic-type issues before falling back to any open issue

### Fixed
- `truncateText` now uses rune-safe `[]rune` slicing to avoid breaking multi-byte UTF-8 characters
- `syncMemory` extracted from Cobra handler for testability
- `parseManagedBlock` detects duplicate markers and refuses to parse (prevents data loss)
- `readNLatestSessionEntries` warns on skipped unreadable session files
- `readSessionByID` detects ambiguous matches and returns error instead of first substring match
- `findMemoryFile` broad contains-fallback removed (was matching wrong projects)
- `pruneNotebook` iteration capped at 100 to prevent runaway loops
- `MEMORY_AGE_DAYS` sentinel initialized to -1 (was 0, causing false lean-mode activation when file missing)
- Lean-mode guard now requires `MEMORY_AGE_DAYS >= 0` before comparing freshness
- Memory sync moved inside forge success gate in session-end hook
- `ao search --json` returns `[]` (empty JSON array) when no results, instead of human-readable text
- `ao doctor` returns `DEGRADED` status for warnings without failures (previously only HEALTHY/UNHEALTHY)
- `ao rpi status` goroutine leak fix — signal channel properly cleaned up
- Inline rune truncation in `formatMemoryEntry` replaced with shared `truncateText`
- 6 new tests for dedup, ambiguity detection, iteration cap, duplicate markers
- Cobra pflag state pollution between test invocations — explicit flag reset in `executeCommand()` helper
- Goals validate.sh outdated checks and missing validate.sh for 7 skills
- 10 tech debt findings from ag-8km+ag-chm post-mortem (stale nudge, scanner, docs)
- ao binary codesigned with stable Mach-O identifier
- Hook integration tests updated — removed 8 stale standalone ao-* hook tests consolidated into session-end-maintenance.sh

## [2.17.0] - 2026-02-24

### Added
- GOALS.md (v4) OODA-driven intent layer — markdown-based goals format with mission, north/anti stars, and steerable directives
- `ao goals init` interactive GOALS.md bootstrap with `--non-interactive` mode
- `ao goals steer` command to add, remove, and prioritize directives
- `ao goals prune` command to remove stale gates referencing missing paths
- `ao goals migrate --to-md` converter from GOALS.yaml to GOALS.md format
- `ao goals measure --directives` JSON output of active directives
- `ao goals validate` reports format and directive count
- Format-aware `ao goals add` writeback (auto-detects md or yaml)
- Go markdown parser library with case-insensitive heading matching and round-trip rendering (26 tests)
- `/goals` skill rewritten with 5 OODA verbs (init/measure/steer/validate/prune)
- `/evolve` Step 3 rewritten with directive-based cascade for idle reduction

### Fixed
- `ao rpi` falls back to any open issue when no epic exists (#50)
- RPI phased processing tests added (~230 lines) for writePhaseResult, validatePriorPhaseResult, heartbeat, and registry directory

## [2.16.0] - 2026-02-23

### Added
- Evolve idle hardening — disk-derived stagnation detection, 60-minute circuit breaker, rolling fitness files, no idle commits
- Evolve `--quality` mode — findings-first priority cascade that prioritizes post-mortem findings over goals
- Evolve cycle-history.jsonl canonical schema standardization and artifact-only commit gating
- `heal-skill` checks 7-10 with `--strict` CI gate for automated skill maintenance
- 6-phase E2E validation test suite for RPI lifecycle (gate retries, complexity scaling, phase summaries, promise tags)
- Fixture-based CLI regression and parity tests
- `ao goals migrate` command for v1→v2 GOALS.yaml migration with deprecation warning (#48)
- Goal failure taxonomy script and tests

### Changed
- CLI taxonomy, shared resolver, skill versioning, and doctor dogfooding improvements (6 architecture concerns)
- GoReleaser action bumped from v6 to v7
- Evolve build detection generalized from hardcoded Go gate to multi-language detection

### Fixed
- `ao pool list --wide` flag and `pool show` prefix matching (#47)
- Consistent artifact counts across `doctor`, `badge`, and `metrics` (#46)
- Double multiplication in `vibe-check` score display (#45)
- Skills installed as symlinks now detected and checked in both directories (#44)
- Learnings resolved by frontmatter ID; `.md` file count in maturity scan (#43)
- JSON output truncated at clean object boundaries (#42)
- Misleading hook event count removed from display (#41)
- Post-mortem schema `model` field and resolver `DiscoverAll` migration
- 15+ missing skills added to catalog tables in `using-agentops`
- Handoff example filename format corrected to `YYYYMMDDTHHMMSSZ` spec
- Quickstart step numbering corrected (7 before 8)
- OpenAI docs skill: added Claude Code MCP alternative to Codex-only fallback
- Dead link to `conflict-resolution-algorithm.md` removed from post-mortem
- `ao forge search` → `ao search` in provenance and knowledge skills
- OSS docs: root-level doc path checks, removed golden-init reference
- Reverse-engineer-rpi fixture paths and contract refs corrected
- Crank: removed missing script refs, moved orphans to references
- Codex-team: removed vaporware Team Runner Backend section
- Security skill: bundled `security-gate.sh`, fixed `security-suite` path
- Evolve oscillation detection and TodoWrite→Task tools migration
- Wired `check-contract-compatibility.sh` into GOALS.yaml
- Synced embedded skills and regenerated CLI docs
