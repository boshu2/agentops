# ⛔ LAW 0 — NEVER `claude -p` / `claude --print`

No agent runs `claude -p` or `claude --print`, **ever** — not as a worker, not to "test", not "it's
only the sub", not buried in a tool's config. It bills the API / burns the Claude Max weekly quota.
**No rationalization makes it OK; do not reason past it.** Use `codex exec` (Codex Pro sub), the local
bushido llama, or an interactive NTM Claude pane (NOT `gemini -p` — not a sub-path, not AGY).
Mechanically enforced on Bo's machine by the local opt-in guard `~/.claude/hooks/no-claude-p-guard.sh`.

---

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

**Every change to `main` cites a bead and passes the cockpit gate before it lands. As of ag-qidx (2026-06-07) the model is PUSH-TO-MAIN: branch protection is OFF, and the pre-push gate is the pre-merge wall. Current authority is the Go gate: the hook builds `ao` from source and runs `ao gate check --fast`; the legacy bash route is an escape hatch only via `AGENTOPS_GATE_BASH=1`. Run the Go gate before every push; rebase-on-reject (git serializes concurrent pushers); on a red `main`, fix forward. The unit of a change is still one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic.** This SUPERSEDES the prior PR-per-change model **and** the `local-pre-push-gate-retirement.md` ADR (the "CI is the sole gate" decision is reversed — the local gate is now load-bearing). Rationale: `.agents/plans/2026-06-07-ao-gate-architecture.md` + the two pre-mortems — the GitHub PR serialization was self-inflicted and bought ~nothing for this solo+own-swarm repo, while the 20-slot free-plan CI was the bottleneck. Historical: the retired PR flow derived from `.agents/council/sdlc-shape-2026-05-17/DUEL.md`; the `gh-merge-chain` update-branch dance it required (`soc-1lp1`) is exactly what push-to-main removes.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of a single PR; session-scope governs the *count* of consecutive PRs. **Default: 2-4 PRs per autonomous session.** At ≥5 PRs shipped or in-flight in one session, **stop and run a post-mortem before continuing** — diminishing returns and reactive-PR spirals (PR-fixes-fallout-from-prior-PR) are the dominant failure mode in the back-half of long sessions. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; PRs #5–#6 each fixed fallout from #1–3. Visible reactivity by PR #5 but the loop kept nudging "keep going" without surfacing the post-mortem signal. Mechanical enforcement is the mandatory `/evolve` post-mortem checkpoint (council-gated, cannot be bypassed; `skills/evolve/references/postmortem-checkpoint.md`), which reads the session-PR count from `scripts/session-pr-scope.sh`. The pre-creation Bash hook `hooks/session-pr-counter.sh` (PR #362) was **removed** in the 3.0 hookless teardown (#511); re-author it as an **opt-in** hook via the hooks-authoring skill for the always-on pre-creation signal — AgentOps ships none. (soc-waxr, ag-o5xp)

**Tracker = br (beads_rust) + bv, as of 2026-06-11.** Issue tracking is **br** — offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache; `br sync` never touches git itself). Triage with **bv** (`bv --robot-insights`, `--robot-plan`, `--robot-priority`). **bd/Dolt is RETIRED LEGACY (2026-06-11):** delivery was coupled to a remote single-host Dolt server on bushido — a SPOF with no offline lane; its circuit breaker was observed open during the 2026-06-11 recon (P1 finding, `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md`). Do not run `bd` here. **Interim layout:** br lives at `_beads/` (prefix `ag` kept) because legacy `.beads/` still holds the bd/Dolt config and `br init` there would clobber it — until `.beads/` is retired, invoke as `BEADS_DIR=$PWD/_beads br <cmd>`. **The ledger is PRIVATE:** `_beads/` is its own git repo (remote `boshu2/agentops-beads`), gitignored by this PUBLIC repo — bead bodies carry private fleet/client context; tracker sync = `git -C _beads push`, never `git add _beads`. Legacy `.beads/` is preserved byte-for-byte pending reconciliation (post-mortem nuance: the Dolt server was actually up — the observed outage was a stale client port config — but the single-host coupling stands as the retirement rationale); the migration record lives at `.agents/swarm/results/br-migration.json`.

### Phases

1. **Claim.** `br ready` → pick a bead → `br update <id> --claim`. **No bead, no PR.** If the work is genuinely new, `br create "Title" -t task -p 2 --body "..."` first (deps: `--deps blocks:<id>` or `br dep add <child> <parent>`).
2. **Scope.** Read the bead's acceptance: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one PR per coherent arc** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The PR is the *atomic-revert unit*. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `git worktree add wt-<bead-id> -b <type>/<bead-id>-<scenario-token>-<short-slug>` — worktree-mandatory; do not edit in the shared checkout (canonical-root rules: `AGENTS-RUNTIME.md`). Implement. Run `ao gate check --fast --scope head` before push (smart conditional gate that runs the per-tool checks — `cd cli && make test`, `bats tests/scripts/<file>.bats`, etc. — only for the surfaces you changed); CI runs the omnibus validation on push.
4. **Land.** Push to `main` (the cockpit gate runs in the pre-push hook; rebase-on-reject). `validate.yml` runs on `main` as a post-push backstop. The bead closes when its arc is on `main` (or explicitly cancelled in bead metadata).

### Branch + PR shape

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · `<scenario-token>` = full slug if it fits, else `<slug-prefix>-<hash8>` |
| PR title | `<type>(<scope>): <subject> (<bead-id> #<scenario-slug>)` — full slug here |
| Required PR body trailers | `Closes-scenario: <bead-id>#<slug>` · `Bounded-context: BC<N>-<name>` · `Evidence: <path>` |
| Land | Push to `main` after the cockpit gate passes · rebase-on-reject (git serializes concurrent pushers) · no force-push · no deletes |
| Gate | cockpit pre-push gate (blocking, in the hook) + `validate.yml` on `main` (post-push backstop). No PR review (PR flow retired — ag-qidx) |

### Multi-agent discipline (shared checkout)

The host `~/dev/agentops` is contended. **Agents do not edit it directly.** Use `git worktree add <name> -b <branch>` for every change. Cross-bead merge serialization: git itself (rebase-on-reject serializes concurrent pushers) plus Agent Mail coordination (`am` reservations / build slots) when multiple lanes are landing — `bd merge-slot` is retired with bd. Foreign uncommitted files = quarantined; identify owner, attach to a bead, move into a worktree.

### Provenance

Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). Tracker state (`br` issue fields, notes, comments) is a derived projection — ledger wins on disagreement. The ledger is append-only: concurrent writers append events, never rewrite (the old `--set-metadata`/dolt-advisory-lock machinery is retired with bd). `claude-code-review` verdicts are first-class ledger events.

### Doctrine altitudes

- **North star:** [`docs/3.0.md`](docs/3.0.md) — what AgentOps 3.0 is (hookless-first CDLC, the SDLC↔CDLC loop, the four-practice waist). The single source of truth; everything below is consistent with it.
- **Spine:** [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — 7-move agent doctrine. **Primary navigation.**
- **One turn's executor:** `/rpi` skill. NOT primary.
- **Architecture:** 5 Bounded Contexts (BC1 Corpus → BC5 Runtime). Where code lives.
- **Consumer metaphor:** "CDLC" — the compounding Knowledge Flywheel framing (`Research → Plan → Implement → Validate → Knowledge Flywheel feedback`).

### Source layer — three axis owners, generated or schema-gated; **NEVER hand-edited inventory maps**

- **DDD (vocabulary):** `skills/domain/references/` — BC names + ubiquitous language.
- **Hex (structure):** `skills/*/SKILL.md` frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`) → generated to `docs/contracts/context-map.md`. CI gate: `validate-context-map-drift`.
- **Gherkin (acceptance):** `skills/*/references/*.feature` + bead-embedded `## Scenarios`. CI gate: `check-scenario-test-linkage` (in the `skill-gates` job).

### CI tiers (no "advisory")

- **T0 (≤30s)** required gates · **T1 (≤5min)** verification · **T2 (≤15min)** quality — **all required**.
- **I0** informational; runs and reports artifact but does NOT appear as a PR check.

## Session Constraints

- **Multi-phase / multi-agent work:** runs on the live orchestration substrate — **NTM** (tmux agent swarms) + **MCP Agent Mail** (locks / messaging / inboxes) + the `continuity-loop` renewal spine, under `.agents/agent-constitution.md`. The in-session `ao rpi` loop is **retired as the live workflow** — do NOT route new work through it. (Its Go lane is load-bearing legacy, not dead code — see "Legacy RPI lane" below; the command still compiles but is not how work is driven.)
- **Before spawning workers:** Verify no file overlap across the wave. File collisions are the #1 swarm failure mode.
- **Before proposing new capability:** check it doesn't already exist — `.github/workflows/validate.yml`, `GOALS.md`, existing `skills/**/SKILL.md`, and the `ao` command surface (`cli/cmd/ao/`, generated `cli/docs/COMMANDS.md`).
- **Gas City (gc) — optional out-of-session SDK, NOT the live substrate.** The live substrate is NTM + Agent Mail (above). `gc` is an optional dependency for out-of-session orchestration only; `ao` does NOT wrap it. (The CLI gc-bridge was removed — see next line.)
- **Gas City (gc) bridge — REMOVED (soc-2rtm0, wave 2).** The CLI gc-bridge glue (`cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`) was severed and deleted. The phased engine keeps its non-gc backends (`auto`/`direct`/`stream`/`tmux`); `runtime=gc` is no longer a valid mode. The injectable exec/look typedefs (`execFn`/`lookFn`, formerly `gcExecFn`/`gcLookFn`) now live in `rpi_phased_context.go`. The last dangling gascity compat — `internal/gascity`, its only importer (the orphaned `agentworker` GasCity adapter), and `internal/bridge/gc.go` — was removed in ag-hfc (3.1 teardown S2); the live `bridge` codex/semver helpers (`CompareSemver`, `ParseSemverParts`, codex lifecycle) stay in `bridge/semver.go` + `bridge/codex.go`.
- **Legacy RPI lane — load-bearing, live (tested) code; no new surface area.** `rpi_loop_supervisor.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`, `rpi_parallel.go` are live and **have substantial test suites** — extend those tests when a caller-driven change legitimately touches the lane (the test-count-regression ratchet expects them maintained, not abandoned). Do NOT add **new features / new surface area** here, and do NOT delete the files: live code references their symbols (`RPIC2Event`/`appendRPIC2Event` across 13+ `rpi_phased*` files + `mine`; `rpiLoopSupervisorConfig`/`runRPISupervisedCycle` in `rpi_loop`/`agentopsd`/`rpi_cancel`; `shellQuote` in `handoff`/`overnight_setup`; tmux helpers in `rpi_nudge`/`rpi_phased_stream`). Deleting any breaks the build; full removal needs a caller-migration refactor (soc-1gbpz), not a flat delete. `rpi_workers.go` and `fire.go` were already removed. (Prior wording said "do not write tests for them," which contradicted the on-disk test suites + the ratchet — ag-etgr.)

### Execution Discipline

- **Verify before committing.** Go: `go test ./...` and `go vet ./...`. Python: run relevant tests. Never commit unverified code.
- **First-Edit Rule.** First Edit/Write/Bash must happen within your first 3 responses. Execute first, research second.
- **Intent Echo.** Before non-trivial tasks, state in ONE sentence what you understand. Wait for confirmation on multi-file changes.
- **Two-Correction Rule.** If corrected twice on the same task: STOP, re-read, state what you now understand differently, and confirm before trying again.
