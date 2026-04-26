# RFC 0001: Finding-Generator Parallelism

Status: accepted (Proposal 1 implemented 2026-04-26)
Date: 2026-04-26
Branch: `research/finding-generator-parallelism`

## Decision Record

**Proposal 1 — accepted and implemented.** The first slice landed in two commits:

- `1bd2f082` (PR #154) — wired the `mine.Run` adapter to write a per-run sidecar at `.agents/overnight/<run-id>/generator-results/mine-findings.json` from INGEST. Sidecar schema landed at `cli/internal/overnight/generator_sidecars.go:24-56` (`FindingGeneratorSidecar`, `FindingGeneratorCandidate`).
- `08e3a38e` (PR #155) — added `AggregateFindingGeneratorSidecars` (`cli/internal/overnight/generator_sidecars.go:225-302`) as the single serialized writer for `next-work.jsonl`, wired in as REDUCE stage 7 between `findings-router` and `inject-refresh` (`cli/internal/overnight/reduce.go:247-373`). Dedup uses both finding ID and a normalized `finding-generator|<generator>|<type|title|target>` `dedup_key`; on collision the higher-severity candidate wins (`preferGeneratorCandidate`). Soft-failed sidecars surface as degraded notes without blocking the iteration.

**Contract updates landed alongside:**
- `docs/contracts/dream-run-contract.md:208-249` — INGEST may run bounded in-process read-only generators concurrently; REDUCE remains the single serialized writer for `.agents/rpi/next-work.jsonl`.
- `skills/dream/SKILL.md:53-58` — anti-goals updated to mirror the contract.
- `PROGRAM.md` — adds `.agents/overnight/*/generator-results/*.json` as Dream-owned runtime state.

**Decision Ask #2 — resolved for `status`, `requires`, `dedup_key`; `goal_weight` deferred.**

- `dedup_key` is first-class on items the aggregator emits (`generatorNextWorkItem.DedupKey`, `cli/internal/overnight/generator_sidecars.go:324-336`) and the dedup loader reads it back across runs (`loadGeneratorNextWorkDedupState`, ibid. lines 368-422). It is now also a documented first-class field on `NextWorkItem.DedupKey` (`cli/internal/rpi/types.go`) and in the schema (`docs/contracts/next-work.schema.md`).
- `status` and `requires` are now first-class fields on `NextWorkItem` (`cli/internal/rpi/types.go`) and in the schema. The selector entry point `IsQueueItemSelectable` (`cli/internal/rpi/helpers.go`) routes through a new `IsQueueItemHeldForReview` check that holds any item with `status=proposed` or a non-empty `requires` slice. This is the precondition Proposal 2 generators need: external watchlist items can land in the queue without auto-execution risk. `status=ready` is the canonical released value; an empty status remains backward-compatible. Unknown statuses fail safe (held).
- `goal_weight` remains deferred. It has no Proposal 2 selector consumer yet and would invite under-specified ranking semantics. Will revisit when ranking pressure exists.

**Decision Ask #3 — deferred (path now unblocked).** External web/competitor/dependency findings still need a concrete generator. With `status` / `requires` first-class and selector-enforced, the next slice can land an external watchlist generator that emits `status=proposed`, `requires=["human-review"]` candidates without any change to the selector contract.

**Fitness signals shipping with Proposal 1** (visible on `IngestResult` and `ReduceResult`):
`generator_candidate_count`, `generator_duplicate_count`, `generator_duplicate_rate`, `generator_sidecar_count`, `generator_soft_fail_count`, `generator_candidates_routed`, `generator_candidates_skipped`, `generator_sidecars_aggregated`, `generator_sidecars_soft_failed`. Auto-revert thresholds from the Risks section (duplicate rate ≥ 0.75 for two consecutive nights, regression-halt rise) remain advisory until two nights of baseline data accumulate.

## Problem

AgentOps already has a working night loop:

1. `/dream` compounds repo-local knowledge and emits durable work into `.agents/rpi/next-work.jsonl`.
2. `/evolve` consumes that queue serially, one RPI cycle at a time.
3. Fitness gates and rollback semantics protect the write side.

The question for this pass is not "can AgentOps use more parallelism?" It already can on the implementation side when the operator opts into isolated worktrees. The sharper question is where parallelism has the best risk-adjusted payoff.

Working hypothesis: the highest-leverage parallelism is on the read side, where independent finding-generators discover candidate work and feed the same Dream queue, while `/evolve` remains serial and fitness-gated on execution.

Recommendation: prototype read-side generator fanout with a single serialized queue writer. Do not parallelize `/evolve` by default. Treat external/web/dependency sources as proposed, human-review findings until queue policy explicitly knows how to rank and release them.

## Current Behavior

### Scope and mutability

`PROGRAM.md` allows changes under `docs/**` and `.agents/**` runtime state when the active command owns it, while source code, hooks, scripts, schemas, and unrelated worktrees remain outside this pass's requested mutable scope (`PROGRAM.md:10-39`). The same file requires validation evidence and worktree disposition before session close (`PROGRAM.md:48-55`, `PROGRAM.md:82-90`). This RFC and the candidate queue rows stay within the allowed surfaces.

### Dream is bounded, serial, and knowledge-only

The Dream skill defines Dream v2 as a bounded `INGEST -> REDUCE -> MEASURE` loop with atomic checkpointed iterations, rollback on regression/metadata failure, and a hard knowledge-only constraint (`skills/dream/SKILL.md:46-57`). The Dream run contract repeats the same anti-goals: no source-code mutation, no RPI/code-mutating flow, no git operations, no symlinks, and no swarm/gc fanout inside iterations in the first slice (`docs/contracts/dream-run-contract.md:241-248`).

The implementation follows that contract. `RunIngest` is documented as read-only and serial, with no goroutine fanout in the first slice (`cli/internal/overnight/ingest.go:60-83`). The outer loop runs `ingest`, checkpoint creation, `reduce`, `measure`, fitness halt, and commit in sequence (`cli/internal/overnight/loop.go:141-183`) and stops on timeout, max iterations, or hard cap (`cli/internal/overnight/loop.go:284-310`).

`REDUCE` is the only Dream stage that mutates `.agents/` (`cli/internal/overnight/reduce.go:20-25`). Its contract is also serial: promote, dedup, prune, close-loop, route findings, refresh inject cache, and verify metadata (`cli/internal/overnight/reduce.go:98-127`).

### Evolve is intentionally serial by default

The Evolve skill describes the operator cadence as: post-mortem, analyze repo state, select or create the next highest-value item, run `/rpi`, harvest follow-ups, and repeat until a cap, breaker, or real dormancy (`skills/evolve/SKILL.md:37-41`). Its first work source is `.agents/rpi/next-work.jsonl`, and the selector picks the highest-value unconsumed item (`skills/evolve/SKILL.md:43-58`, `skills/evolve/SKILL.md:191-194`).

The existing write-side parallel option is explicit and isolated. `ao rpi parallel` runs multiple RPI epics concurrently, each in its own git worktree, then merges successful branches in order and runs a gate (`cli/cmd/ao/rpi_parallel.go:59-167`). It dispatches epics with goroutines, enforces a per-epic timeout, reports failures, and stops merging on conflicts (`cli/cmd/ao/rpi_parallel.go:284-454`). The older parallel execution reference makes the safety model explicit: heuristic independence is not a guarantee; the regression gate is the real safety net; each worker needs isolated artifacts and a worktree because concurrent RPI cycles would collide on `.agents/rpi/` and git locks (`skills/evolve/references/parallel-execution.md:47-86`).

### Queue shape and dedup constraints

`next-work.jsonl` is newline-delimited JSON. It is append-on-write and rewrite-on-lifecycle; producers append entries, consumers rewrite lines to claim, release, fail, or consume items; readers must tolerate unknown fields (`docs/contracts/next-work.schema.md:9-17`). Entry fields include `source_epic`, `timestamp`, `items`, `consumed`, aggregate `claim_status`, claimant fields, and consumed fields (`docs/contracts/next-work.schema.md:20-37`). Item fields require `title`, `type`, `severity`, `source`, and `description`; lifecycle fields are optional, and unknown metadata is allowed (`docs/contracts/next-work.schema.md:41-67`). Valid source enums are only `council-finding`, `retro-learning`, `retro-pattern`, `evolve-generator`, `feature-suggestion`, and `backlog-processing` (`docs/contracts/next-work.schema.md:90-121`).

Lifecycle rules currently say writers create entries in `available` state, consumers claim before execution, consumers finalize only after successful cycles, failures release claims, and items should never be consumed at pick-time (`docs/contracts/next-work.schema.md:123-150`). This matters for this RFC because the user's requested `status: proposed` and `requires: human-review` fields are not first-class schema fields yet. They are legal as unknown metadata, but today's selector safety should be enforced by a lifecycle hold, not by assuming future readers understand those advisory fields.

The findings registry already has a stronger dedup contract. It defines `dedup_key = <category>|<pattern-slug>|<primary-applicable-when>`, normalization rules, rank rules, and merge-by-`dedup_key` write semantics (`docs/contracts/finding-registry.md:71-128`). It updates the registry through temp file plus atomic rename and optional lock path (`docs/contracts/finding-registry.md:137-148`). That registry contract is a better model for parallel generator result merging than direct concurrent appends to `next-work.jsonl`.

### Existing finding-generators in Dream

| Source class | Current generators | Count | Last touched | Notes |
|---|---:|---:|---|---|
| Code-internal repo scan | `mine.Run` over `git`, `agents`, and `code`; counts code hotspots, orphaned research, co-change files, recurring fixes, error events, and gate verdicts | 1 | 2026-04-12, `eeea2b9f` | Runs in INGEST with `DryRun=true` (`cli/internal/overnight/ingest.go:253-312`) |
| Code-internal finding router | `.agents/findings/*.md` -> `next-work.jsonl`, deduped by finding ID | 1 | 2026-04-19, `fc42b0aa` | Single `O_APPEND` line plus fsync; NFS append is not atomic (`cli/internal/overnight/findings_router.go:54-68`, `cli/internal/overnight/findings_router.go:83-155`) |
| Runtime health | Dream fallback packets from explicit goal, retrieval coverage below threshold, metrics escape velocity false, and escalatable degradation | 4 | 2026-04-14, `a01235a3` | Packet IDs hash source parts with `dream-` prefix (`cli/cmd/ao/overnight_packets.go:258-343`, `cli/cmd/ao/overnight_packets.go:923-938`) |
| Runtime corroboration | Long-haul packet corroboration and council as optional confidence-improvement lanes | 2 | 2026-04-19, `2f6105bc` | Prior post-mortem says cheapest probe before council was the reusable pattern (`.agents/council/2026-04-14-post-mortem-dream-longhaul.md:140-158`) |
| Local external-ish curator | Ollama/Gemma-backed worker queue and event records | 1 | 2026-04-24, `04964419` | Allowed jobs are knowledge jobs and bounded events; no recursive runner budget-free calls (`skills/dream/SKILL.md:58-74`) |
| Web/competitor/dependency scan inside Dream | None found | 0 | N/A | Evolve has deps/perf/test/refactor generators, but Dream has no web/competitor/deps finding-generator today (`skills/evolve/SKILL.md:52-57`) |

Total current Dream finding-generator surfaces: 9 if runtime fallback/corroboration lanes are counted separately, or 3 if counting only direct candidate-work producers (`mine.Run`, findings router, morning packet builder).

### Prior internal conclusions

Internal notes consistently argue that write-side parallelism needs ownership boundaries. `docs/origin-story.md` says file collisions are the top swarm failure mode and reports about a 40% failure rate without ownership boundaries, near zero with pre-flight overlap checks and wave execution (`docs/origin-story.md:78-94`). A local learning records parallel sessions in one worktree repeatedly deleting untracked files during Tier 1 forge work; the fix was to use a separate git worktree and commit promptly (`.agents/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md:12-48`). The Swarm skill repeats that evidence and warns to abort overlapping multi-worker waves when worktree isolation does not engage (`skills/swarm/SKILL.md:571-589`).

At the same time, the repo already treats read/research fanout as useful when outputs are bounded. The shared orchestration profile includes both single-agent research and "Research phase (3 parallel agents)" variants (`docs/profiles/shared/orchestration.yaml:35-43`). The architecture page's Brownian Ratchet says parallel agents are useful when their output is filtered by a council and locked by a ratchet (`docs/ARCHITECTURE.md:64-87`). That supports read-side fanout only if the result has a strict merge/filter step before execution.

### Unknowns

These are not resolved by this pass and should not be guessed:

- The actual nightly latency and yield distribution of each Dream substage. We have static code and post-mortem evidence, not per-generator timing histograms.
- How often current Dream runs are bottlenecked by finding starvation versus execution backlog. The queue has unresolved rows, but the question is marginal yield, not raw queue length.
- Whether the user wants external web/competitor/dependency findings to become automatically executable or remain human-review-only.
- Whether `status`, `requires`, `goal_weight`, and `dedup_key` should become first-class next-work schema fields or stay advisory metadata.
- Whether the local filesystem and user workflows always satisfy the current O_APPEND local POSIX assumption for `next-work.jsonl`.

## Prior Art

### Aider repo-map

Aider parallelizes context discovery in the weak sense: it builds a compact repository map from Tree-sitter symbol extraction and graph ranking, then supplies that map to the LLM with each change request. It serializes code editing in the chat loop, with the user adding files to edit and the model asking for more files when needed. Conflict handling is mostly outside the repo-map layer: the map reduces context misses, but it does not coordinate multiple concurrent writers. This maps well to AgentOps read-side generators: use cheap static analysis to widen context before a single writer acts. Sources: https://aider.chat/docs/repomap.html and https://aider.chat/2023/10/22/repomap.html.

### Cognition Devin

Devin's public 2024 launch described a long-running single agent with planning, shell/editor/browser tools, progress reporting, user feedback, and autonomous setup/test/fix loops. Cognition's 2025 "Don't Build Multi-Agents" argued that multi-agent writers are fragile because context is hard to share and actions carry implicit decisions. Cognition's April 2026 update is the most relevant prior art for this RFC: it says useful multi-agent setups keep writes single-threaded while other agents contribute intelligence, and it explicitly notes that most multi-agent setups remain read-only subagents such as web/code search. Devin's March 2026 "Manage Devins" feature adds manager/child parallelism with isolated VMs, coordinator-scoped work, conflict resolution by the main session, and compiled results. Sources: https://cognition.ai/blog/introducing-devin, https://cognition.ai/blog/dont-build-multi-agents, https://cognition.ai/blog/multi-agents-working, and https://cognition.ai/blog/devin-can-now-manage-devins.

### OpenHands

OpenHands serializes an issue or PR attempt into a GitHub Action/Cloud session that comments, works, opens a PR if resolved, and summarizes the result. Its GitHub Action exposes iteration and target branch settings, and its docs describe feedback by relabeling or mentioning the agent on the issue/PR. Parallelism appears at the workload level: multiple issues/actions can run independently, but each issue resolution is one PR-producing session with branch and review boundaries. Conflicts are handled through PR review, target branches, and follow-up comments rather than shared live writes. Sources: https://docs.openhands.dev/openhands/usage/run-openhands/github-action, https://docs.openhands.dev/openhands/usage/cloud/github-installation, and https://openhands.dev/blog/open-source-coding-agents-in-your-github-fixing-your-issues.

### Sweep

Sweep's older GitHub App positioning was an issue-to-PR junior developer for bugs and small features; the current product has shifted toward a JetBrains coding assistant with project indexing, code search, diff review, web fetch tools, and an IDE agent. The reliable lesson is not "parallelize writers"; it is source-specific intake plus a reviewable PR/diff boundary. For AgentOps, Sweep is closer to a generator/source lane: index/search a codebase or issue stream, turn a bounded input into a candidate change, and hand it to review rather than letting multiple agents write a shared file. Sources: https://github.com/apps/sweep-ai, https://github.com/sweepai/sweep, and https://sweep.dev/.

### SWE-agent

SWE-agent is built around an Agent-Computer Interface that helps one agent browse, edit, navigate, and run tests in a repository. Its batch mode parallelizes across independent issue instances using `run-batch`, `--num_workers`, per-instance cost limits, instance files in `.jsonl`/`.json`/`.yaml`, and output predictions that can be converted to JSONL. It also has startup randomization to reduce resource pressure when many Docker-backed workers launch. This is strong support for AgentOps using parallelism across independent read or evaluation units, not inside one shared queue writer. Sources: https://swe-agent.com/latest/background/, https://arxiv.org/abs/2405.15793, and https://swe-agent.com/latest/usage/batch_mode/.

### RepoAgent

RepoAgent is documentation-oriented rather than PR-oriented. It automatically detects git changes, analyzes code structure with ASTs, identifies invocation relationships, maintains a hierarchy record, writes markdown docs, and explicitly advertises multi-threaded concurrent operations for documentation generation. It serializes integration through a generated hierarchy file, markdown output directories, and a pre-commit workflow that modifies staged docs. The relevant pattern is sharded analysis with a structured merge artifact, not direct concurrent mutation of one shared queue. Sources: https://github.com/OpenBMB/RepoAgent and https://arxiv.org/abs/2402.16667.

### Conflict evidence

A recent AgenticFlict study reports a deterministic merge-simulation dataset of 107K+ processed AI-agent PRs with 29K+ conflict-bearing PRs, a 27.67% conflict rate. This is not AgentOps-specific, but it reinforces the repo's internal conclusion: uncoordinated code-writing parallelism is high-risk, while read-side and branch-isolated parallelism are safer. Source: https://arxiv.org/abs/2604.03551.

## Design Space

### Axis A: Where parallelism happens

Options:

- Generation: run independent finding-generators concurrently, merge their findings, then write the queue once.
- Evaluation: run independent reviewers/corroborators concurrently over the same candidate packets, then synthesize confidence.
- Execution: run multiple RPI cycles concurrently.

Current loop already handles:

- Serial generation in Dream INGEST/REDUCE.
- Optional evaluation through Dream Council, currently runner loop is serial (`cli/cmd/ao/overnight_council.go:212-244`).
- Optional execution parallelism through `ao rpi parallel` with worktrees.

Open:

- Parallel generation with a single queue writer.
- Parallel evaluation that preserves one writer.
- Automated criteria for when parallel execution is worth the coordination cost.

### Axis B: What is shared

Options:

- Shared queue only: generators emit candidate records; only an aggregator writes `next-work.jsonl`.
- Shared repo read-only: generators read the same checkout but cannot mutate tracked source or `.agents/rpi/next-work.jsonl`.
- Shared branch: multiple writers operate in the same worktree or branch.
- Isolated branches/worktrees/VMs: writers work independently and merge later.

Current loop already handles:

- Shared repo read-only during Dream INGEST.
- Shared queue append by a single router/morning-packet writer.
- Isolated worktrees for explicit RPI parallelism.

Open:

- Staged generator outputs under per-run directories.
- Queue aggregator lock/merge semantics for multiple generator outputs.

### Axis C: Conflict resolution

Options:

- Dedup by source identity: finding ID or packet hash.
- Dedup by normalized semantic key: registry-style `dedup_key`.
- Priority/weight reconciliation: merge duplicates and keep max severity/goal weight, strongest evidence, and newest source timestamp.
- Fitness arbitration: let `/evolve` execute one selected item and keep it only if gates pass.
- Human gate: mark proposed items as human-review until released.

Current loop already handles:

- Finding router dedups by finding ID.
- Morning packet IDs are hash-derived.
- Evolve fitness arbitrates execution.
- Finding registry has normalized `dedup_key` rules.

Open:

- A next-work-level first-class `dedup_key`.
- A first-class `status/requires` hold that selectors understand.
- Weight reconciliation across multiple generator outputs.

### Axis D: Durability semantics

Options:

- At-least-once append with dedup: producers may duplicate; readers/aggregator suppress duplicates.
- Single-writer append after staged fanout: generators write sidecar files, aggregator appends one merged row.
- Lock plus temp/rename rewrite: stronger for lifecycle or registry state, heavier for queue producers.
- Exactly-once transactional queue: requires a different storage contract.

Current loop already handles:

- At-least-once append with local POSIX `O_APPEND` and fsync.
- Rewrite-on-lifecycle by consumers.
- Registry temp/rename semantics.

Open:

- Whether generator outputs should use `O_APPEND`, per-generator files, or registry-style temp/rename.
- Whether the project wants exactly-once semantics. This pass finds no evidence that exactly-once is worth introducing before a generator prototype proves yield.

## Proposals

### Proposal 0: Stay serial

Keep Dream and Evolve as they are. Add no generator fanout and no queue schema changes.

- Generators in parallel: none.
- Discovery: existing Dream INGEST/REDUCE and morning packet fallbacks.
- Queue write: existing single router/morning writer.
- Dedup key: current finding ID and packet hash.
- Budget/timeout: current Dream loop budget and council runner timeout.
- Stall failure mode: existing soft-fail/degraded stage behavior.
- Fitness signal: existing unresolved findings, retrieval coverage, escape velocity, and Dream yield.
- Smallest viable prototype: no implementation; only this RFC.

Where baseline wins: if unresolved queue backlog, not finding starvation, is the actual bottleneck; if advisory `status/requires` fields are not accepted into the queue contract; if most external findings are noisy; or if Dream run time is already dominated by REDUCE/MEASURE rather than read-side generators.

### Proposal 1: Read-only generator fanout with a single writer

Run independent read-only Dream generators concurrently, but forbid direct queue writes from workers. Each generator writes a per-run sidecar record. A deterministic aggregator reads all sidecars, reconciles duplicates, and appends exactly one `next-work.jsonl` entry.

- Generators in parallel: start with one existing generator adapter, `mine.Run` over `git`, `agents`, and `code`; later add retrieval-health and metrics-health adapters after timing/yield data exists.
- Discovery: the Dream run builds a generator manifest from configured adapters and repo-local state. The first prototype can hard-code one adapter and manifest entry to keep scope small.
- Queue write without races: generators write sidecars under a run-specific directory such as `.agents/overnight/<run-id>/generator-results/<generator>.json`; the aggregator is the only `next-work.jsonl` writer.
- Dedup key: `finding-generator|<generator>|<normalized-title>|<primary-target>` for next-work candidates; if the input comes from `.agents/findings`, preserve finding ID and registry `dedup_key`.
- Budget/timeout: 60 seconds for the prototype generator, bounded by the Dream run context; later set per-generator defaults in config.
- Stall failure mode: timeout marks the generator `soft-fail`, records degraded evidence, and aggregator continues with completed sidecars.
- Fitness signal: `dream_generator_candidate_count > 0`, `generator_duplicate_rate < 0.5`, no increase in Dream regression halts, and morning packet confidence/yield improves over a rolling baseline.
- Smallest viable prototype: one `mine.Run` adapter that emits sidecar JSON only, one fitness metric recording generator candidate count and duplicate rate, one `PROGRAM.md` line permitting `.agents/overnight/*/generator-results/*.json` as Dream-owned runtime state.

Why it may lose to baseline: if `mine.Run` is not latency-significant, fanout adds moving pieces without reducing wall-clock time. If candidates duplicate existing queue items at high rates, the aggregator mostly creates bookkeeping noise.

### Proposal 2: External watchlist lane, human-review only

Add a constrained external generator lane for sources that are useful but too noisy to auto-execute: competitor repo watchlists, dependency advisories, upstream release notes, or security/dependency drift. These run in parallel with internal read-only generators but can only emit `status=proposed`, `requires=human-review` candidates until the queue schema and selector explicitly support release semantics.

- Generators in parallel: one external watchlist generator in the prototype; later split web, competitor repos, and dependency advisories into separate adapters.
- Discovery: generator reads a checked-in or operator-local allowlist of repos/packages/URLs plus last-seen cursors; no broad web crawl in the first prototype.
- Queue write without races: external generator writes sidecar output; the same aggregator appends one human-held queue row after dedup.
- Dedup key: `external-watchlist|<source-url-or-package>|<normalized-title-or-advisory-id>`.
- Budget/timeout: 2 minutes per source, 5 minutes total lane budget, capped network fetch count; stale/unreachable sources degrade without failing Dream.
- Stall failure mode: source timeout becomes a degraded generator result and does not block internal findings.
- Fitness signal: human acceptance rate of proposed external findings, duplicate rate against existing queue, and zero auto-picked human-review rows.
- Smallest viable prototype: one dependency or repo-watch source, one fitness goal for accepted-proposed ratio, one `PROGRAM.md` change documenting external generator sidecars as Dream-owned runtime only.

Why it may lose to baseline: external signals can produce urgency theater. If the first month of proposed items has low acceptance, the lane should stay off or be deleted.

### Proposal 3: Parallel evaluator/corroborator lane before queue promotion

This is not finding-generation; it is included because prior art suggests evaluator parallelism is often safer than writer parallelism. Run independent read-only evaluators over candidate packets to improve confidence, then let one synthesizer decide whether to promote.

- Generators in parallel: none; evaluators include retrieval corroboration, static evidence check, and optional council runners.
- Discovery: use packets already produced by Dream and existing queue rows.
- Queue write without races: no evaluator writes `next-work.jsonl`; synthesizer updates packet confidence or emits one queue row.
- Dedup key: existing packet ID from `dreamPacketID`.
- Budget/timeout: 30 seconds for deterministic corroborators, existing council runner timeout for model-backed evaluation (`cli/cmd/ao/overnight_council.go:246-310`).
- Stall failure mode: evaluator soft-fails, confidence remains lower, packet can still be emitted with degraded evidence.
- Fitness signal: higher morning packet acceptance rate and fewer stale-audit consumptions for Dream-created packets.
- Smallest viable prototype: parallelize deterministic packet corroborators only, add one fitness goal for confidence lift per millisecond spent, and document that this is evaluation parallelism, not generator parallelism.

Why it may lose to baseline: it does not increase candidate diversity. If the bottleneck is missing candidate work rather than confidence, it only makes existing work prettier.

## Risks

### Proposal 1 risks and reversal

Failure modes:

- Sidecar schema drift creates aggregator skips.
- Duplicate candidates inflate queue noise.
- Generator timeout handling hides real bugs as soft-fail degradation.
- A future implementer bypasses the aggregator and writes `next-work.jsonl` from workers.

Blast radius: Dream run artifacts and next-work candidate quality; no source code writes if the generator contract holds.

Rollback path: disable the generator manifest, delete staged sidecar output from the active run directory, and return to existing serial Dream INGEST/REDUCE.

Auto-revert weight: if `generator_duplicate_rate >= 0.75` for two consecutive nights or Dream regression/degraded count rises above baseline by more than one hard failure, set the generator fitness goal weight to `-5` and disable the lane.

Morning digest surfacing: include generator count, duplicates suppressed, timed-out generators, and new rows emitted. A failed prototype should be visible as "0 accepted candidates, N duplicates/timeouts" rather than as silent queue churn.

### Proposal 2 risks and reversal

Failure modes:

- Network or API flakiness dominates Dream runtime.
- External findings duplicate existing beads/queue items.
- External reports over-prioritize competitor features or dependency churn without repo-local proof.
- Human-review hold is misunderstood as executable readiness.

Blast radius: proposed queue rows and operator attention. The queue selector must not auto-pick these rows.

Rollback path: disable the external source allowlist and leave existing proposed rows claimed by `human-review` until a human consumes or releases them.

Auto-revert weight: if human acceptance is below 20% after 10 proposed items, or any human-held item is auto-picked, set the source fitness goal weight to `-8` and disable external generation.

Morning digest surfacing: show proposed external findings in a separate "requires human review" section with source URL/package, dedup key, and why it was not auto-executable.

### Proposal 3 risks and reversal

Failure modes:

- Evaluator latency grows without increasing actionable work.
- Multiple evaluators disagree and require a more complex synthesizer.
- Council runner timeouts return as chronic degraded output.

Blast radius: packet confidence and morning digest clarity, not source code.

Rollback path: run corroborators serially or turn off optional model-backed evaluators; existing Dream packets still emit at lower confidence.

Auto-revert weight: if confidence lift per minute is lower than baseline or council/evaluator timeouts occur two nights in a row, set evaluator-fanout goal weight to `-3` and return to serial corroboration.

Morning digest surfacing: include before/after confidence, evaluator timeout count, and whether any packet changed rank because of evaluation.

## Decision Asks

1. Should the next prototype be Proposal 1, with a single existing `mine.Run` generator converted to sidecar output and a single writer aggregator?
2. Should `status`, `requires`, `goal_weight`, and `dedup_key` become first-class next-work schema fields before external watchlist findings are allowed?
3. Should external web/competitor/dependency findings ever become auto-executable, or should they always enter as human-review proposals?

Single decision most needed: approve or reject Proposal 1 as the next prototype. If approved, keep `/evolve` serial and make the first implementation prove yield with one read-only generator before adding any external sources.
