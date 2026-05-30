# AgentOps Skills Repository

## What this is

AgentOps compiles and compounds the context that feeds your software factory. It automates agent bookkeeping — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes, with humans in or on the loop at whatever rigor level fits.

## Zero-Context Startup (Read First)

AgentOps 3.0 is hookless: nothing auto-injects orientation at session start. Run `ao session bootstrap` (the universal init prompt) to get the standard orientation report, then `ao inject` / `ao corpus inject --query "<topic>"` to pull decay-ranked prior context — this is the explicit replacement for the SessionStart context the runtime used to inject. Then, on your first message in a fresh session, read in this order:

1. `docs/newcomer-guide.md` for a practical repo orientation and learning path.
2. `docs/index.md` (MkDocs landing) and `docs/documentation-index.md` (full catalog) for navigation.
3. `README.md` for product-level framing.
4. Task-specific canonical surfaces:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, generated `cli/docs/COMMANDS.md`
   - Skills behavior: `skills/**/SKILL.md`
   - Gates: `.github/workflows/validate.yml` + `scripts/*.sh` (AgentOps 3.0 is hookless — CI is the authoritative gate)
   - Contracts/schemas: `schemas/**`, `lib/schemas/**`
5. `.agents/AGENTS.md` for knowledge store navigation (search on demand, don't pre-load).

## Source-of-Truth Precedence

When files disagree, trust in this order:

1. Executable implementation and generated outputs (`cli/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
2. Declared contracts/manifests (`skills/**/SKILL.md`, `schemas/**`)
3. Narrative docs (`docs/**`, `README.md`)

Always report mismatches; do not silently pick a lower-precedence doc over executable behavior.

## Project Structure

```
cli/          Go CLI (ao binary) — cmd/ao, internal packages
skills/       Skill definitions (source of truth)
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

## Deep reference (on-demand, not auto-loaded)

Building the CLI, the Key Scripts table, CI-validation detail + the "rules that break CI", testing rules, the release pipeline, and the `ao goals` command surface all live in **[`docs/agent-workflow-reference.md`](docs/agent-workflow-reference.md)**. Read it only when you're actually touching those surfaces. The AGENTS-side scope detail lives in the tiered split: `AGENTS-WORKFLOW.md`, `AGENTS-CI.md`, `AGENTS-CODEX.md`, `AGENTS-RUNTIME.md`.

## Workflow

**Every change to `main` is a PR. Every PR cites a bead. The unit of a PR is one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic. Small epics (≤5 child beads, same surface) ship as one PR with N commits. Large epics (15+ child beads) ship as N PRs sliced by scenario or wave.** Direct pushes to `main` are rejected by branch protection. Derivation: `.agents/council/sdlc-shape-2026-05-17/DUEL.md` (local, gitignored — duel between Claude Opus 4.7 and Codex gpt-5.5, 2026-05-17). 2026-05-19 evolution from "1 scenario per PR" after the 8-PR merge-arc burned out the `gh-merge-chain` dance — `soc-1lp1`.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of a single PR; session-scope governs the *count* of consecutive PRs. **Default: 2-4 PRs per autonomous session.** At ≥5 PRs shipped or in-flight in one session, **stop and run a post-mortem before continuing** — diminishing returns and reactive-PR spirals (PR-fixes-fallout-from-prior-PR) are the dominant failure mode in the back-half of long sessions. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; PRs #5–#6 each fixed fallout from #1–3. Visible reactivity by PR #5 but the loop kept nudging "keep going" without surfacing the post-mortem signal. Mechanical enforcement ships as the PreToolUse Bash hook at `hooks/session-pr-counter.sh` (soc-1aou, PR #362) — it fires at `count >= threshold-1` (default 5) and emits the post-mortem prompts to the agent via `additionalContext`, with optional hard-block via `AGENTOPS_SESSION_PR_BLOCK=1`. (soc-waxr)

### Phases

1. **Claim.** `bd ready` → pick a bead → `bd update <id> --claim`. **No bead, no PR.** If the work is genuinely new, `bd create` first.
2. **Scope.** Read the bead's acceptance: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one PR per coherent arc** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The PR is the *atomic-revert unit*. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `bd worktree create --branch <type>/<bead-id>-<scenario-token>-<short-slug>` — worktree-mandatory; do not edit in the shared checkout. Implement. Run per-tool sanity checks for the surfaces you touched (`cd cli && make test`, `bats tests/scripts/<file>.bats`, etc.); CI runs the omnibus validation on push.
4. **Close.** Open PR. CI validates the merge state. Squash-merge when green. The bead closes only when every scenario is merged (or explicitly cancelled in bead metadata).

### Branch + PR shape

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · `<scenario-token>` = full slug if it fits, else `<slug-prefix>-<hash8>` |
| PR title | `<type>(<scope>): <subject> (<bead-id> #<scenario-slug>)` — full slug here |
| Required PR body trailers | `Closes-scenario: <bead-id>#<slug>` · `Bounded-context: BC<N>-<name>` · `Evidence: <path>` |
| Merge | Squash only · linear history · branch up-to-date · no force-push · no deletes |
| Reviews | 0 humans + required `claude-code-review` check (automation gate) |

### Multi-agent discipline (shared checkout)

The host `~/dev/agentops` is contended. **Agents do not edit it directly.** Use `bd worktree create --branch <name>` for every change. Cross-bead merge serialization: `bd merge-slot`. Foreign uncommitted files = quarantined; identify owner, attach to a bead, move into a worktree.

### Provenance

Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). `bd update --metadata` is a derived projection — ledger wins on disagreement. Concurrent writes use `--set-metadata` / `--append-to` (never full-blob replacement) + dolt advisory locks. `claude-code-review` verdicts are first-class ledger events.

### Doctrine altitudes

- **North star:** [`docs/3.0.md`](docs/3.0.md) — what AgentOps 3.0 is (hookless-first CDLC, the SDLC↔CDLC loop, the four-practice waist). The single source of truth; everything below is consistent with it.
- **Spine:** [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — 7-move agent doctrine. **Primary navigation.**
- **One turn's executor:** `/rpi` skill. NOT primary.
- **Architecture:** 5 Bounded Contexts (BC1 Corpus → BC5 Runtime). Where code lives.
- **Consumer metaphor:** "CDLC" — the compounding Knowledge Flywheel framing (`Research → Plan → Implement → Validate → Knowledge Flywheel feedback`).

### Source layer — three axis owners, generated or schema-gated; **NEVER hand-edited inventory maps**

- **DDD (vocabulary):** `skills/domain/references/` — BC names + ubiquitous language.
- **Hex (structure):** `skills/*/SKILL.md` frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`) → generated to `docs/contracts/context-map.md`. CI gate: `validate-context-map-drift`.
- **Gherkin (acceptance):** `skills/*/references/*.feature` + bead-embedded `## Scenarios`. CI gate: `scenario-hash-stability`.

### CI tiers (no "advisory")

- **T0 (≤30s)** required gates · **T1 (≤5min)** verification · **T2 (≤15min)** quality — **all required**.
- **I0** informational; runs and reports artifact but does NOT appear as a PR check.

## Session Constraints

- **Multi-phase work:** Route through `ao rpi` (enforces timeouts and stall detection).
- **Before spawning workers:** Verify no file overlap across the wave. File collisions are the #1 swarm failure mode.
- **Before proposing new capability:** Check `ao rpi serve --help`, `.github/workflows/validate.yml`, and `GOALS.md` first.
- **Gas City (gc) — guided substrate, not an in-CLI mode.** `gc` is the optional out-of-session orchestration substrate that runs whole `ao rpi`/`ao evolve` loops (the reference City at `packs/agentops/`; mayor + refinery agents). It is a guided dependency, the way `bd` is — `ao` does NOT wrap it. Agent-facing workflow: the [`using-gc`](skills/using-gc/SKILL.md) skill. Full tool list: [docs/dependencies.md](docs/dependencies.md).
- **Gas City (gc) bridge — REMOVED (soc-2rtm0, wave 2).** The CLI gc-bridge glue (`cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`) was severed and deleted. The phased engine keeps its non-gc backends (`auto`/`direct`/`stream`/`tmux`); `runtime=gc` is no longer a valid mode. The injectable exec/look typedefs (`execFn`/`lookFn`, formerly `gcExecFn`/`gcLookFn`) now live in `rpi_phased_context.go`. The last dangling gascity compat — `internal/gascity`, its only importer (the orphaned `agentworker` GasCity adapter), and `internal/bridge/gc.go` — was removed in ag-hfc (3.1 teardown S2); the live `bridge` codex/semver helpers (`CompareSemver`, `ParseSemverParts`, codex lifecycle) stay in `bridge/semver.go` + `bridge/codex.go`.
- **Legacy RPI lane — load-bearing, not dead code.** Do not write new tests or features for `rpi_loop_supervisor.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`, `rpi_parallel.go`, but do NOT delete them: live code references their symbols (`RPIC2Event`/`appendRPIC2Event` across 13+ `rpi_phased*` files + `mine`; `rpiLoopSupervisorConfig`/`runRPISupervisedCycle` in `rpi_loop`/`agentopsd`/`rpi_cancel`; `shellQuote` in `handoff`/`overnight_setup`; tmux helpers in `rpi_nudge`/`rpi_phased_stream`). Deleting any breaks the build; removal needs a caller-migration refactor (soc-1gbpz), not a delete. `rpi_workers.go` and `fire.go` were already removed.

### Execution Discipline

- **Verify before committing.** Go: `go test ./...` and `go vet ./...`. Python: run relevant tests. Never commit unverified code.
- **First-Edit Rule.** First Edit/Write/Bash must happen within your first 3 responses. Execute first, research second.
- **Intent Echo.** Before non-trivial tasks, state in ONE sentence what you understand. Wait for confirmation on multi-file changes.
- **Two-Correction Rule.** If corrected twice on the same task: STOP, re-read, state what you now understand differently, and confirm before trying again.
