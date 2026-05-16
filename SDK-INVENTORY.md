# AgentOps SDK Extraction Inventory

> Reverse-engineered from this repo's source tree on 2026-05-07. Bucketed for the SDK split.
> Status: **DRAFT** — for red-pen review, not yet a plan to execute.
>
> Premise (from prior conversation): AgentOps is currently one repo carrying three concerns at once. The state machine and the doctrine are good; the rest is opinion. The unlock is to extract a thin SDK so a personal harness and the opinionated product become two *instances* of the same spec.

## Layers

| Layer | What goes here | Output artifact |
|---|---|---|
| **SPEC** | Pure docs/schemas. The contract a fork conforms to. No code. | `12-factor-agentops/` (or similar) — schemas + doctrine docs |
| **RUNTIME** | Tiny binary. Atomic handoff ops, transcript scan, decay-ranked injection, gate runner, tracker adapter. | `ao-core` — small Go binary, ~10% of today's `ao` |
| **OPINIONATED** | Today's AgentOps. Full skill catalog, codex parity, ratchets, doc gates, CI scripts, factory metaphor. | `agentops` — batteries-included reference plugin |
| **DEAD** | Cut entirely. Project-specific cruft, deprecated paths, things native Claude Code now does. | (deleted) |

## Bucketing rules

- **SPEC** ⇐ schema, contract doc, naming convention, file-layout convention, format definition.
- **RUNTIME** ⇐ stateful primitive (atomic write, parse, scan, rank, dispatch); ≤1 obvious correct implementation.
- **OPINIONATED** ⇐ workflow choice, philosophical opinion, multiple defensible alternatives, fork-replaceable.
- **DEAD** ⇐ project-specific, unused, deprecated, or replaced by Claude Code native primitive.

## Top-level directory inventory

| Path | Bucket | Notes |
|---|---|---|
| `cli/` | mixed | Classified per package below. ~10% RUNTIME, rest OPINIONATED. |
| `skills/` (75) | mixed | Phase-skill *contracts* are SPEC; phase-skill *content* and the rest are OPINIONATED. |
| `skills-codex/` | OPINIONATED | Cross-runtime parity tax — only relevant to forks that want Codex. |
| `skills-codex-overrides/` | OPINIONATED | Same. |
| `hooks/` | mixed | Manifest *format* is platform-SPEC (Claude Code's); specific hook list is OPINIONATED. |
| `schemas/` (32) | mostly SPEC | Cleanest layer. ~70% SPEC, ~30% OPINIONATED-format. |
| `scripts/` (~150) | mostly OPINIONATED | CI/release/validation specific to AgentOps. |
| `lib/` | mixed | `chain-parser.sh` + `ao-paths.sh` → RUNTIME utility; `hook-helpers.sh` (29K) needs surgery. |
| `bin/` | OPINIONATED/DEAD | `ralph` (12K) — verify usage. |
| `agents/` | OPINIONATED | Subagent definitions; the layout is SPEC, content is fork-specific. |
| `plugins/` | OPINIONATED | Plugin manifests for AgentOps's own distribution. |
| `docs/` | OPINIONATED | Move SPEC-relevant docs (handoff, phase, CDLC) into SPEC layer. |
| `evals/` | OPINIONATED | Eval workbench is an opinion. |
| `tests/` | OPINIONATED | Tests AgentOps's specific behavior. |
| `examples/` | OPINIONATED | Reference examples. |
| `homebrew-tap/` | OPINIONATED | Distribution. |
| `.github/`, `.githooks/` | OPINIONATED | Project's CI + git hooks. |
| `.claude/`, `.codex/`, `.opencode/`, `.codex-plugin/`, `.claude-plugin/` | OPINIONATED | Plugin manifests for distribution. |
| `.agents/` | mixed | Layout convention is SPEC; corpus content lives in user fork. |
| Root `*.md` (`AGENTS.md`, `PRODUCT.md`, `PROGRAM.md`, `README.md`, `CHANGELOG.md`, `GOALS.md`, `MEMORY.md`, `CLAUDE.md`) | OPINIONATED | Every fork has its own. |
| `registry.json` (43K) | OPINIONATED-format | Format may belong in SPEC; content is fork-specific. |
| `goals-affects-files.yaml` | OPINIONATED | Gate-to-file mapping is opinion. |
| `Makefile`, `.goreleaser.yml`, `mkdocs.yml`, `.markdownlint.json`, `requirements-docs.txt` | OPINIONATED | Build/release/docs infra. |

## CLI subcommand inventory (`cli/cmd/ao/`)

535 files (incl. tests + `assets/`/`testdata/`). Many `cmd/ao/*.go` are thin Cobra wrappers around `cli/internal/*` packages — the real classification is the package. Grouped by capability.

| Capability | Files | Bucket | Notes |
|---|---|---|---|
| Context compiler | `context*.go`, `context_assemble*.go`, `context_packet*.go`, `context_ranked_intel*.go`, `context_relevance*.go`, `context_explain*.go`, `context_trust_policy.go` | **RUNTIME core** | Decay-ranked retrieval + token budgeting + phase-scoped packets. *The* primitive. |
| Compile pipeline | `compile.go` (21K) | RUNTIME | Mine→Grow→Defrag→Lint runner. Pipeline shape RUNTIME; specific stages may be OPINIONATED. |
| Defrag/dedup | `defrag.go`, `dedup.go` | RUNTIME | Generic corpus operations. |
| Corpus root | `corpus.go` | RUNTIME | Corpus root resolution. |
| Forge | `batch_forge.go` | RUNTIME-leaning | Transcript scan = RUNTIME; learning *format* = SPEC; tier *policy* = OPINIONATED. Needs split. |
| Curate | `curate.go`, `batch_promote.go` | OPINIONATED | Tier promotion rules. |
| Beads adapter | `beads.go` (26K), `beads_audit_cluster.go` | RUNTIME-as-adapter | Becomes one tracker adapter under SPEC's tracker-adapter contract. |
| Daemon | `agentopsd.go` (37K), `daemon_jobs.go`, `daemon_soak.go`, `daemon_fake_policy.go` | DEAD or OPINIONATED | Replaced by `/loop` + `/schedule` for personal harness. Keep in OPINIONATED for forks wanting off-API scheduling. |
| Autodev | `autodev.go` | DEAD | `/loop` replaces this natively. |
| Dream | `dream_subcycle.go` | OPINIONATED | Specific scheduled mode of flywheel; could be a `/loop` instance instead. |
| Evolve | `evolve.go` | OPINIONATED | Reconcile-against-GOALS.md loop. |
| Eval workbench | `eval.go`, `eval_suite.go`, `eval_task.go`, `eval_cleanup.go` | OPINIONATED | Eval workbench. |
| Goals | `goals/` referenced from CLAUDE.md | OPINIONATED | GOALS.md format + measure. |
| Agent registry | `agents.go`, `agents_doctor.go`, `agents_lint.go` | OPINIONATED | Format → SPEC; lint commands → OPINIONATED. |
| Codex export | `codex.go` (49K!), `codex_runtime.go`, `codex_*test.go` | OPINIONATED | Largest single chunk of opinion in the binary. |
| Constraint | `constraint.go` | OPINIONATED | Constraint compiler. |
| Contradict | `contradict.go` | OPINIONATED | Belief-contradiction scanner. |
| Doctor | `doctor.go` (19K) | OPINIONATED | Health check; every fork has its own shape. |
| Recover | `recover.go` (not in dir listing — check) | OPINIONATED | Session recovery. |
| Demo / badge | `demo.go`, `badge.go` | DEAD | Verify; both look unused. |
| Citations | `citation_namespace.go` | SPEC | Citation namespacing convention. |
| Identity | `canonical_identity.go` | RUNTIME utility | Small, deterministic. |
| Completion | `completion.go`, `completion_values.go` | RUNTIME utility | Shell completion. |
| Config | `config.go` | RUNTIME utility | Config file load. |
| App | `app.go` | RUNTIME utility | Cobra root. |
| Exit codes | `exit_codes.go` | SPEC | Exit code taxonomy. |

## CLI internal package inventory (`cli/internal/`)

| Package | Bucket | Notes |
|---|---|---|
| `context/` | **RUNTIME core** | The compiler engine: `assemble.go`, `budget.go`, `ranked_intel.go`, `summarize.go`, `trust_policy.go`. Most important RUNTIME asset. |
| `forge/` | RUNTIME (split) | `forge.go` + `runmine.go` — transcript-mining engine. Format → SPEC; tier policy → OPINIONATED. |
| `lifecycle/` | mixed (split) | `dedup.go`, `defrag.go`, `maturity.go` → RUNTIME. `close_loop.go`, `curate.go`, `feedback.go`, `temper.go` → OPINIONATED. `repo_readiness.go`, `seed.go`, `task_sync.go` → OPINIONATED. |
| `corpus/` | RUNTIME | Corpus model + ops. |
| `paths/` | RUNTIME utility | Path resolution. |
| `parser/` | RUNTIME utility | Parsing. |
| `resolver/` | RUNTIME utility | Resolution. |
| `formatter/` | RUNTIME utility | Output formatting. |
| `storage/` | RUNTIME | Storage backends. |
| `types/` | SPEC + RUNTIME | Type definitions. |
| `config/` | RUNTIME utility | Config. |
| `shellutil/` | RUNTIME utility | Shell helpers. |
| `search/` | RUNTIME | Corpus search. |
| `provenance/` | RUNTIME | Citation logger — primitive. |
| `goals/` | OPINIONATED | GOALS.md model. |
| `plans/` | OPINIONATED-format | Execution-packet format may be SPEC; planning model is OPINIONATED. |
| `knowledge/` | OPINIONATED | Tiered knowledge structure (learnings/patterns/rules). |
| `harvest/` | OPINIONATED | Flywheel harvest. |
| `mine/` | OPINIONATED | Compile mining stage. |
| `taxonomy/` | OPINIONATED | Knowledge taxonomy. |
| `quality/` | OPINIONATED | Quality signal model. |
| `vibecheck/` | OPINIONATED | Vibe-check semantics. |
| `ratchet/` | OPINIONATED | Brownian Ratchet impl. |
| `safety/` | OPINIONATED | Safety checks. |
| `scope/` | OPINIONATED | Scope guard. |
| `skillshealth/` | OPINIONATED | Skills health metrics. |
| `daemon/` | DEAD/OPINIONATED | Scheduling daemon engine. |
| `schedule/` | DEAD/OPINIONATED | Scheduler. |
| `overnight/` | OPINIONATED | Overnight orchestration. |
| `autodev/` | DEAD | `/loop` replaces. |
| `pool/` | OPINIONATED | Agent pool. |
| `worker/` | OPINIONATED/DEAD | Agent tool replaces most natively. |
| `agentworker/` | OPINIONATED/DEAD | Same. |
| `bridge/` | DEAD | Gas City bridge — project-specific. |
| `gascity/` | DEAD | Gas City integration — project-specific. |
| `openclaw/` | DEAD/OPINIONATED | Not core to SDK. |
| `llm/` | OPINIONATED | LLM client wrapper. |
| `llmwiki/` | OPINIONATED | LLM-wiki worker. |
| `wikiworker/` | OPINIONATED | Wiki worker. |
| `notebook/` | OPINIONATED | Context notebook. |
| `eval/`, `evalsubstrate/`, `bench/` | OPINIONATED | Eval workbench. |
| `rpi/` | mixed (split) | Phase chain runner → RUNTIME; phase semantics → SPEC; default content → OPINIONATED. |

## Skills inventory (`skills/` — 75)

The phase contract (entry criteria, handoff doc shape, exit criteria) for each phase skill IS the SPEC. The content is OPINIONATED reference. Every "phase skill" is dual-classified: **contract → SPEC, content → OPINIONATED.**

| Group | Skills | Bucket |
|---|---|---|
| Phase | `discovery`, `plan`, `research`, `implement`, `crank`, `validation`, `rpi` | SPEC (contract) + OPINIONATED (content) |
| Handoff | `handoff` | SPEC (artifact) + OPINIONATED (skill body) |
| Flywheel | `forge`, `harvest`, `evolve`, `dream`, `flywheel`, `retro`, `post-mortem`, `pre-mortem`, `provenance`, `trace`, `compile`, `inject`, `knowledge-activation` | OPINIONATED |
| Council | `council`, `vibe`, `red-team`, `pr-validate` | OPINIONATED |
| PR workflow | `pr-implement`, `pr-plan`, `pr-research`, `pr-prep`, `pr-retro` | OPINIONATED |
| Authoring | `readme`, `oss-docs`, `doc`, `product`, `design`, `brainstorm` | OPINIONATED |
| Tooling | `scaffold`, `bootstrap`, `update`, `converter`, `heal-skill`, `skill-auditor`, `skill-builder`, `standards`, `using-agentops`, `quickstart`, `status`, `recover`, `goals` | OPINIONATED |
| Domain | `security`, `security-suite`, `perf`, `complexity`, `refactor`, `test`, `bug-hunt`, `release`, `push`, `swarm`, `scope`, `scenario`, `ratchet`, `deps`, `codex-team`, `autodev`, `beads` | OPINIONATED |
| Niche | `grafana-platform-dashboard`, `llm-wiki`, `openai-docs`, `reverse-engineer-rpi`, `system-tuning` | OPINIONATED (some likely DEAD — verify usage) |
| Glue | `shared` | OPINIONATED |

**`SKILL.md` frontmatter format** → SPEC. Already an emerging cross-harness convention.

## Schemas inventory (`schemas/` — 32 files)

Cleanest split.

| Schema | Bucket | Notes |
|---|---|---|
| `phase.v1.schema.json` | SPEC | **Currently bead-coupled** ("legal state transitions for beads"). Abstract to tracker-agnostic. |
| `handoff.v1.schema.json` | SPEC | Core SDK primitive. |
| `verdict.v1.schema.json` | SPEC | Mentions "Athena". Rename or keep — open Q. |
| `learning.v1.schema.json` | SPEC | Mentions "Hephaestus". Same. |
| `briefing.v1.schema.json` | SPEC | Context briefing. |
| `bead.v1.schema.json` | SPEC (one tracker adapter) | Becomes one impl under generic tracker contract. |
| `finding.json` | SPEC | Finding artifact. |
| `rubric.v1.schema.json` | SPEC | Rubric format. |
| `skill-frontmatter.v1.schema.json` | SPEC | Skill manifest. |
| `plugin-manifest.v1.schema.json` | SPEC | Plugin manifest. |
| `hooks-manifest.v1.schema.json` | SPEC | Hook manifest (mirrors Claude Code's). |
| `memory-packet.v1.schema.json` | SPEC | Memory primitive. |
| `execution-packet.schema.json` | SPEC (candidate) | Execution packet format. |
| `agent-update.schema.json` | OPINIONATED | Agent registry update format. |
| `eval-run.v1.schema.json` | OPINIONATED | Workbench. |
| `eval-suite.v1.schema.json` | OPINIONATED | Workbench. |
| `evidence-only-closure.v1.schema.json` | OPINIONATED | Closure policy. |
| `factory-admission.v1.schema.json` | OPINIONATED | Factory metaphor. |
| `factory-work-order.v1.schema.json` | OPINIONATED | Factory metaphor. |
| `factory-yield.v1.schema.json` | OPINIONATED | Factory metaphor. |
| `quest.v1.schema.json` | OPINIONATED | Quest model. |
| `release-readiness.v1.schema.json` | OPINIONATED | Release gate. |
| `routing-policy.v1.schema.json` | OPINIONATED | Routing. |
| `scenario.v1.schema.json` | OPINIONATED | Holdout scenarios. |
| `schedule.schema.json` | OPINIONATED | Daemon scheduling. |
| `session-quality-signal.v1.schema.json` | OPINIONATED | Quality signal. |
| `swarm-evidence.schema.json` | OPINIONATED | Swarm evidence. |
| `worker-spec.v1.schema.json` | OPINIONATED | Worker spec. |
| `watch-event.v1.schema.json` | OPINIONATED | Watch event. |
| `remote-compute-target.schema.json` | OPINIONATED | Remote compute. |
| `remote-session-event.schema.json` | OPINIONATED | Remote session. |
| `codex-marketplace.v1.schema.json` | OPINIONATED | Codex-specific. |
| `codex-plugin-manifest.v1.schema.json` | OPINIONATED | Codex-specific. |

## Hooks inventory (`hooks/`)

`hooks.json` *manifest format* mirrors Claude Code's; that's the platform spec. The hooks AgentOps registers are OPINIONATED.

| Group | Examples | Bucket |
|---|---|---|
| Manifest format | `hooks.json` shape | SPEC (platform) |
| Session lifecycle utility | `session-start.sh`, `session-end-maintenance.sh` | RUNTIME utility |
| Compile pipeline | `compile-session-defrag.sh` | OPINIONATED |
| UX hooks | Removed from 3.0 runtime manifest; replaced by README/quickstart and execution-packet fields | RETIRED |
| Safety | `dangerous-git-guard.sh`, `edit-scope-guard.sh`, `edit-audit.sh`, `edit-knowledge-surface.sh`, `git-worker-guard.sh`, `lead-only-worker-git-guard.sh` | OPINIONATED |
| Workflow gates | `pre-mortem-gate.sh` (10K), `commit-review-gate.sh`, `holdout-isolation-gate.sh`, `task-validation-gate.sh` (30K!), `eval-verdict-compiler.sh` (10K), `factory-router.sh`, `finding-compiler.sh` (30K!) | OPINIONATED |
| Worktree | `worktree-cleanup.sh`, `worktree-setup.sh`, `subagent-stop.sh`, `stop-auto-handoff.sh`, `stop-team-guard.sh` | mixed (split: utility vs. gate logic) |
| Quality | `quality-signals.sh`, `write-time-quality.sh`, `context-guard.sh`, `context-monitor.sh` | OPINIONATED |
| CLI wrappers | `ao-extract.sh`, `ao-feedback-loop.sh`, `ao-flywheel-close.sh`, `ao-forge.sh`, `ao-inject.sh`, `ao-maturity-scan.sh`, `ao-ratchet-status.sh`, `ao-session-outcome.sh`, `ao-task-sync.sh`, `ao-agents-check.sh` | OPINIONATED |
| Codex parity | `codex-hooks.json`, `codex-parity-warn.sh`, `postedit-codex-refresh.sh` | OPINIONATED |
| Language-specific | `go-test-precommit.sh`, `go-vet-post-edit.sh`, `go-complexity-precommit.sh`, `skill-lint-gate.sh` | OPINIONATED |
| Misc | `precompact-snapshot.sh`, `pending-cleaner.sh`, `research-loop-detector.sh`, `standards-injector.sh`, `config-change-monitor.sh`, `citation-tracker.sh`, `constraint-compiler.sh` | OPINIONATED |

## Scripts inventory (`scripts/` — ~150)

Almost all OPINIONATED. They validate AgentOps-specific contracts, run AgentOps-specific gates, or build/release the AgentOps product itself.

| Group | Examples | Bucket |
|---|---|---|
| Validation gates | `validate-*.sh`, `check-*.sh`, `audit-*.sh` (~80 scripts) | OPINIONATED |
| Release tooling | `pre-push-gate.sh` (57K), `ci-local-release.sh` (30K), `release-smoke-test.sh` (24K), `retag-release.sh`, `extract-release-notes.sh`, `resolve-release-artifacts.sh` | OPINIONATED |
| Codex parity | `audit-codex-*.sh`, `validate-codex-*.sh`, `mirror-codex-references.sh`, `regen-codex-hashes.sh`, `register-new-codex-skill.sh`, `smoke-test-codex-skills.sh`, `test-codex-*.sh`, `export-claude-skills-to-codex.sh`, `lint-codex-native.sh` (~20 scripts) | OPINIONATED |
| Install | `install*.sh`, `install*.ps1` | OPINIONATED |
| Nightly/overnight | `nightly-*.sh`, `overnight-*.sh` | OPINIONATED (replaceable by `/loop`) |
| Eval | `eval-agent-*.sh`, `eval-agentops.sh` | OPINIONATED |
| Beads tooling | `bd-cluster.sh`, `bd-audit.sh`, `tasks-sync.sh`, `validate-bd-closeout-contract.sh` | OPINIONATED |
| Corpus | `corpus-stats.sh`, `bootstrap-maturity.sh` | OPINIONATED |
| Doc | `docs-build.sh`, `generate-cli-reference.sh`, `generate-index.sh`, `generate-registry.sh`, `extract-release-notes.sh` | OPINIONATED |
| Cleanup | `cleanup-global-agents.sh`, `purge-global-garbage.sh`, `prune-agents.sh` | OPINIONATED |
| Self-test | `test-*.sh`, `proof-run.sh`, `regression-bisect.sh` | OPINIONATED |

## Lib inventory (`lib/`)

| File | Bucket | Notes |
|---|---|---|
| `chain-parser.sh` | RUNTIME utility | Generic chain parsing. |
| `ao-paths.sh` | RUNTIME utility | Path resolution. |
| `hook-helpers.sh` (29K) | mixed (split) | Atomic write/JSON parse → RUNTIME; specific gate logic → OPINIONATED. **Biggest single split call.** |
| `skills-core.js` | OPINIONATED | JS for some web rendering. |

## High-confidence cuts (DEAD)

These belong nowhere — cut entirely:

- `cli/internal/gascity/` + `cli/internal/bridge/` — Gas City project-specific (CLAUDE.md flags these too).
- `cli/cmd/ao/badge.go`, `cli/cmd/ao/demo.go` — verify, but look unused.
- `cli/cmd/ao/dream_subcycle.go` — duplicates `/loop` semantics.
- `cli/internal/autodev/` + `cli/cmd/ao/autodev.go` — replaced by `/loop` + Plan mode.
- `cli/cmd/ao/agentopsd.go` (37K) + `daemon_*.go` + `cli/internal/daemon/` + `cli/internal/schedule/` — replaced by Claude Code `/loop` + `/schedule` for personal harness; keep in OPINIONATED for forks that want off-API scheduling, otherwise DEAD.
- ~20 `validate-codex-*.sh` / `audit-codex-*.sh` / `test-codex-*.sh` scripts — only kept by forks wanting Codex parity (move to `agentops` reference plugin, not SDK).
- Specialty skills with no recent use (`reverse-engineer-rpi`, `grafana-platform-dashboard`, `system-tuning`, `llm-wiki`, `openai-docs`) — verify; cut if zero invocations in last 30 days.
- Already-deprecated files flagged in CLAUDE.md: `rpi_loop_supervisor.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`, `rpi_workers.go`, `rpi_parallel.go`, `fire.go`.

## Splits (mixed files needing surgery)

Cannot be wholly bucketed; need extraction.

1. **`lib/hook-helpers.sh` (29K)** — extract atomic-ops + JSON helpers to RUNTIME; leave gate logic in OPINIONATED.
2. **`cli/internal/lifecycle/`** — extract `dedup.go`, `defrag.go`, `maturity.go` to RUNTIME; rest OPINIONATED.
3. **`cli/internal/forge/`** — extract transcript scanner to RUNTIME; learning *format* is SPEC (already in `schemas/learning.v1.schema.json`); tier policy stays OPINIONATED.
4. **`cli/internal/rpi/`** — phase chain runner → RUNTIME; phase semantics → SPEC; default content stays OPINIONATED.
5. **`schemas/phase.v1.schema.json`** — abstract from beads to tracker-agnostic; `bead.v1` becomes one tracker adapter.
6. **Phase skills (`discovery`, `implement`, `validation`, `rpi`, `handoff`)** — author a SPEC contract doc per skill capturing inputs/outputs/exit criteria; SKILL.md content stays OPINIONATED.
7. **`cli/internal/context/`** — split into ranking + retrieval (RUNTIME core) vs. trust policy + summarization formatting (could be either).
8. **`hooks/hooks.json`** — split into spec-shape doc (manifest format) + opinionated hook list.

## Open questions

Need Steve's call before splitting:

1. **Tracker abstraction.** Generic tracker-adapter interface (beads as one impl), or assume beads forever? Beads is *very* embedded — `bead.v1` references creep into `phase.v1`, hook helpers, hook gates.
2. **Mythology naming.** Drop "Athena" / "Hephaestus" in SPEC schemas, or keep? Renaming is breaking; keeping locks SPEC to AgentOps's flavor.
3. **Hooks in SDK or out?** Claude Code's hook taxonomy is platform spec. Should the SDK redocument it, or just point at Claude Code? (Same for Codex/Cursor if multi-runtime.)
4. **Runtime size.** If the personal harness only needs (atomic handoff write, transcript scan, beads call, gate runner), the runtime might be ~5 commands and ~1500 lines of Go — much smaller than 10% of current `ao`. Worth aiming small.
5. **Single-runtime SDK?** If yes, drop `skills-codex/` + `skills-codex-overrides/` + all `codex-*` scripts/schemas wholesale. ~20% of repo gone immediately.
6. **`.agents/` layout.** Is the dir structure (playbooks/, briefings/, learnings/, ratchets/, ao/citations.jsonl) part of the SPEC, or is each fork free? If SPEC, the personal harness inherits the corpus shape.
7. **GOALS.md / fitness gates.** SPEC the file format, or keep entirely OPINIONATED? Convergent question with `evolve.go` and `ratchet`.
8. **Plugin manifest naming.** Will `12-factor-agentops` be the doctrine repo and `agentops` keep being the reference plugin, or do names rotate?
9. **Self-curating skills (the personal-harness premise).** Where does the curator live? In the SDK runtime (since transcript scan is a RUNTIME primitive), or in the personal-harness plugin only?

## Next steps

In order:

1. **Steve red-pens this doc.** Resolves bucket disagreements + the 9 open questions.
2. **Author the SPEC doctrine** (`12-factor-agentops/` repo or `spec/` subtree): one-doc-per-factor (handoff, phase contract, hook events, tracker adapter, skill manifest, citation log, etc.). Pull in the SPEC schemas. This becomes the conformance contract.
3. **Cut DEAD bucket** from the OPINIONATED reference. Touches ~30-50 files.
4. **Extract the splits** (hook-helpers, lifecycle, forge, rpi, context, phase.v1) so RUNTIME code is its own subtree (`cli-core/` or new repo `ao-core`).
5. **Author the personal-harness plugin** as a separate small repo that conforms to the SPEC and uses the RUNTIME. Few skills, no parity tax, `/loop curate-skills` running.
6. **Fold AgentOps to "reference plugin" framing** — keep batteries-included for forks that want everything; document the personal-harness repo as the inverse demonstration.
