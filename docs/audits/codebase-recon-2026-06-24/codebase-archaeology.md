# AgentOps — Technical Architecture Summary

> ⚠️ **HISTORICAL SNAPSHOT — recon run 2026-06-24 against `abc018c42`; `main` has since advanced past `882e71c01`.** A point-in-time architectural overview, **not** a current-state reference. Some facts are already superseded — notably the **P1 atomic-write DRY finding was partially actioned** (`storage.AtomicWriteFile` is now canonical and quest/llmwiki/doctor/wiki delegate to it, age-3azc/uja6; inject, vendorimage/codexruntime, and `pool.atomicMove` still carry their own copies). The **`ao` command (89) and gate (98) counts are reproducible at the pinned `abc018c42`** (re-verified there 2026-06-25 — and still current on `main`; the draft's `~60`/`~95`/`87`/`~77` figures were simply wrong). `main` has since advanced, so narrative *findings* may be superseded (notably P1, partially actioned), but these architectural counts are stable; all other figures are as-of the 2026-06-24 snapshot.

> **Method:** codebase-archaeology (documentation-first → entry points → data flow → key types → integration → config → tests).
> **Date:** 2026-06-24 · **Repo:** `/Users/bo/dev/agentops` @ `abc018c42` (777 commits, primary author Boden Fuller).
> **Verification posture:** This report follows the repo's own source-of-truth precedence — executable + generated artifacts over narrative docs. `go build ./...` and a *sample* of the test suite were run live (both green) — NOT a full test run or security scan, so this is a build-green snapshot, not a release-readiness verdict.
> **Command/gate counts re-verified 2026-06-25 at the pinned `abc018c42`** (corrected from the original draft; still current on `main`). The snapshot describes `abc018c42`; `main` has since advanced, so narrative findings — not the stable counts — may be superseded.

---

## Executive Summary

**AgentOps** is the **operational layer for coding agents**: a set of invocable *skills* + a single Go CLI (`ao`) + a repo-local `.agents/` markdown corpus. It does **not** write code — it wraps the agent you already use (Claude Code, Codex, Cursor, OpenCode) and answers two questions before granting more autonomy: *is the code right?* (validation) and *is the proof durable?* (evidence trail).

The product's own thesis (AGENTS.md, README, ADR-0004/0011) is sharp and self-aware: **the proven product is the "validation membrane"** — the loop that catches an agent declaring "done" when it isn't, and writes a verdict (*no verdict = not done*). The knowledge flywheel / corpus moat that sits beside it is explicitly demoted to an **unproven hypothesis** in the docs and ADRs. This is an unusually honest codebase about its own fitness.

**Key statistics (measured this session):**
- **Go:** ~154k LOC non-test source + ~223k LOC test across the `cli/` tree (~4,000 `.go` files repo-wide incl. fixtures; ~1,300 "real" per the overview). Test LOC > source LOC — heavily tested.
- **Shell:** **717 tracked `.sh` files** repo-wide; the canonical `scripts/` validation/regen/release fleet is **303 top-level `scripts/*.sh`** (311 recursive). (An untracked-inclusive `find` sees ~1,444, mostly vendored/generated — not the authored surface; the earlier "~2,600" draft figure was an over-count.)
- **Skills:** 77 skill dirs under `skills/` (the SSOT) with checked-in Codex twins in `skills-codex/`.
- **CLI surface:** **89 top-level `ao` commands** (per `ao --help`, across all command groups, incl. cobra's auto-added `help`/`completion`; 87 excluding them). Backed by ~97 `rootCmd.AddCommand(...)` call sites in non-test source (some grouped/conditional, so the visible surface settles at 89).
- **Gate registry:** **98 `Check` registrations** across `cli/internal/gates/checks/` (7 files; `seed.go` holds **87 of the 98**, the remaining 11 in 6 sibling files). Confirmed by `ao gate check` run totals (27 run + 71 not-run = 98).
- **Schemas:** 62 JSON schemas in `schemas/`. **ADRs:** 12. **Bats tests:** 181 files.
- **Dependencies:** minimal — cobra/pflag, yaml.v3, BurntSushi/toml, santhosh-tekuri/jsonschema, go-cmp, goleak, pgregory.net/rapid (property testing). Go 1.26. No web framework, no DB driver, no heavy runtime.
- **Build status:** `go build ./...` → **OK**. Sampled `go test ./internal/gates/...` and `./internal/ports/...` → **green**. `ao version` → `3.1.0-rc`, darwin/arm64.

---

## What this repository is (and is not)

| It is | It is not |
|---|---|
| A control plane + skill library that adds bookkeeping, gates, and a corpus *on top of* an agent runtime | A code-writing agent or an LLM harness |
| Hookless and in-session by design (AgentOps 3.0; ADR-0002) — context is pulled explicitly, no auto-injection | A daemon/scheduler/hosted control plane — the in-repo daemon was **deleted** (ADR-0009) |
| Local-first: everything lives in your repo's `.agents/` and `docs/provenance/` | A telemetry/SaaS product with a cross-team dashboard |
| Apache-2.0, fork-friendly; the durable asset is the markdown corpus, not the tool | Locked-in |

Out-of-session "always-on" work is delegated to a **swappable substrate** (reference: NTM tmux swarm + MCP Agent Mail + `ao agent`), which dispatches a *whole* skill loop as one unit and never decomposes the loop internals.

---

## Entry Points

The `ao` CLI is the single executable seam. Everything routes through cobra.

| Entry | Location | Role |
|---|---|---|
| `main()` | `cli/cmd/ao/main.go:11` | Trivial — calls `Execute()`. `version` set via ldflags (`3.1.0-rc` pre-tag). |
| `Execute()` | `cli/cmd/ao/root.go:83` | Runs `rootCmd.ExecuteC()`; maps typed exit errors (`AgentsLintError`, `doctorExitError`, `gateExitError`, `planPawlExitError`) to **exit codes that ARE the verdict** (e.g. gate FAIL=1, plan-pawl REDO=3/BLOCKED=4). |
| `rootCmd` | `cli/cmd/ao/root.go:28` | The `ao` root command; `PersistentPreRunE` builds an `App` struct from resolved flags and injects it into context (DI seam — replaces former global mutable state). |
| `App` / `NewApp()` | `cli/cmd/ao/app.go` | Shared app state (DryRun/Verbose/Output/JSON/WorkDir) + injectable `ExecCommand`/`LookPath`/`RandReader`/`Stdout`/`Stderr` for testing. Terraform-Meta + kubectl-Options hybrid pattern. |

Command registration is distributed: ~97 `rootCmd.AddCommand(...)` call sites in non-test source register the command families (a few grouped/conditional, so the visible `ao --help` surface settles at 89 top-level commands); each `<name>.go` in `cli/cmd/ao/` (606 files incl. tests) owns a command tree. Standalone helper mains exist outside the `ao` surface: `cli/cmd/skill-frontmatter-json/main.go` and `cli/cmd/witness-crosscheck/main.go` (CI helpers, not subcommands).

**Top-level `ao` command families** (from generated `cli/docs/COMMANDS.md`): `demo init quick-start seed`, `badge canon capabilities ci citation claim constraint contradict curate dedup doctor flywheel gate harness loop maturity metrics operator pool reconcile redact robot-docs status version vibe-check`, `autodev codex cron eval feedback-loop goals handoff orchestrate ratchet session tick validate`, `completion config memory notebook worktree`, `beads compile corpus defrag findings forge harvest inject knowledge lookup membrane …`.

---

## Architecture: DDD + Hexagonal, six bounded contexts

The codebase is **genuinely hexagonal** (ADR-0001), not aspirationally so. Verified by tracing a port to its adapter:

- **Ports** (`cli/internal/ports/`, 72 files) are real Go interfaces — `IssueTracker`, `CorpusReader`/`CorpusWriter`, `GateRunner`, `ContextCompiler`, `EventBus`, `HypothesisLedger`, `FindingCompiler`, `Closeout`, `ConvergenceCheck`, `Workspace`, `LLM`, `Operator`, `Orchestration`, `SafetyPolicy`, etc. Each ships an `inmemory_*.go` test double *in the same package* (e.g. `inmemory_tracker.go`).
- **Adapters** (`cli/internal/adapters/`, 40 files / 17 dirs) are the real driven implementations — `tracker_bd` (shells out to the tracker binary), `corpus_fs`, `storage_fs`, `workspace_git`, `mcpsurface`/`mcptransport`, `sessionspawn`, `mto`, `vendorimage`, `worktreeconfig`, etc.
- **Example flow traced:** `ports.IssueTracker` (interface, `Mode()/Ready()/List()/Show()/Create…`) → real adapter `adapters/tracker_bd/tracker_bd.go` (shells to `bd`/`br`) + `ports.InMemoryTracker` (test double). The interface doc comment even records *why* it was widened (soc-ebgjk) from create-only to cover read paths. This is a live, maintained seam.

### Six bounded contexts (per `docs/architecture/codebase-overview.md` + `docs/contracts/bounded-contexts.yaml`)

| BC | Name | Center of gravity |
|----|------|-------------------|
| BC1 | Corpus | `.agents/`, `ao inject`, `/forge`, `/compile`, `/harvest` |
| BC2 | **Validation** (the proven product) | `ao gate check`, `/validate`, `/council`, `/vibe`, membrane/pawl |
| BC3 | Loop | operating loop, `/evolve`, `br` tracker, goals, autodev |
| BC4 | Factory | skill-builder, registries, standards, dispositions |
| BC5 | Runtime | CLI, installers, plugin manifests |
| BC6 | Orchestration | NTM, Agent Mail, swarm — **substrate boundary** |

Edges: BC3 Loop → BC1 (compounding context) + BC2 (proof before land); BC4 → registries; BC5 → CLI+plugins; BC6 dispatches whole skills.

---

## Key Types / Concepts (the handful everything revolves around)

| Concept | Where | Why it's load-bearing |
|---|---|---|
| **Gate `Check`** | `cli/internal/gates/checks/` (98 registrations across 7 files; `seed.go` holds 87) + `gates/{registry,orchestrator,routing,report}.go` | `Check{ID, Tiers(Fast\|Full), Match[]globs, Blocking, Backing\|Run}`. The release authority. Fast mode routes by changed files; Full mode is CI parity. |
| **Verdict / Escape** | `cli/internal/verdictledger/`, `cli/internal/yieldledger/`, `cli/cmd/ao/membrane.go` | A *verdict* proves a bead done. An *escape* = a verdict that CONFIRMED then a later attempt REFUTED — the label that makes the membrane harder to fool. `ao membrane derive-checks`/`recall` is the self-improvement loop (epic age-cwo / EM spine). |
| **App** | `cli/cmd/ao/app.go` | DI container injected via cobra context; replaces global mutable state. |
| **Issue (bead)** | `cli/internal/ports/tracker.go` | The unit of tracked work; `br` (beads_rust, JSONL+SQLite) is the live tracker, `bd`/Dolt retired. |
| **Skill** | `skills/<slug>/SKILL.md` (frontmatter + body) | Invocable contract carrying hexagonal edges (`hexagonal_role`, `consumes`, `produces`, `context_rel`), `practices:` lineage, tier metadata. SSOT — never edit `~/.claude/skills/`. |
| **Provenance ledger** | `docs/provenance/ledger.jsonl` | The one corpus artifact tracked in git (append-only); ledger wins over tracker. |

---

## Data Flow — the active waist (what actually runs)

```text
ao session bootstrap         # explicit orientation (replaces hook injection)
      ↓
ao inject / ao corpus inject # decay-ranked prior context (BC1)
      ↓
Operating loop (7 moves)     # BDD intent → br bead → vertical slice → TDD → wave → prove acceptance → ratchet
      ↓
ao gate check --fast --scope head   # local cockpit gate = routine release authority (BC2)
      ↓
git push → main              # rebase-on-reject; no routine PR wall
      ↓
validate.yml (optional CI)   # backstop on tags/PRs/manual dispatch, NOT routine authority
```

The **seven-move operating loop** (AGENTS.md, `docs/architecture/operating-loop.md`) is the *primary navigation*. `/rpi` is one turn's executor skill, and `ao rpi` (CLI) is **load-bearing legacy** — heavily tested, compiled, but not the live orchestration substrate. This distinction is documented in three places and is the #1 navigation trap.

For out-of-session work the *same* loop opts into NTM + Agent Mail, dispatching a whole **skill loop** (the operating loop, via `ao agent` / managed-agents) as one unit per ready bead — **not** `ao rpi`, which is load-bearing legacy (see Navigation Traps), consistent with ADR-0009.

---

## Integration Points

| Integration | Mechanism | Notes |
|---|---|---|
| **Coding-agent runtimes** | Skill install manifests: `.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/`; `scripts/install-*.sh`/`.ps1` | Claude Code, Codex CLI, OpenCode, Gemini/Antigravity, Cursor. Deliberately not Claude-only. |
| **Issue tracker** | `adapters/tracker_bd` shells out to the `br`/`bd` binary | `br` (beads_rust, JSONL in `_beads/`) is live; `bd`/Dolt retired legacy. |
| **MCP** | `adapters/mcpsurface` + `mcptransport`; `ao mcp serve` | Curated tool surface (JSON-RPC) for managed agents. |
| **Codex execution** | `ao codex *` lane; `cli/cmd/witness-crosscheck` + Dolt-projection crosscheck | Non-interactive cross-family validation (the membrane's second opinion). |
| **Filesystem** | `adapters/corpus_fs`, `storage_fs`, `workspace_git` | `.agents/` corpus, git worktrees. No network DB. |
| **Schema validation** | `santhosh-tekuri/jsonschema` against `schemas/*.json` (62) | Config, provenance, packets, verdicts, eval runs — strongly typed contracts. |

There is **no HTTP server, no SQL database, no external API client** in the core — the integration surface is *other CLIs* (the agent runtime, `br`, `codex`, MCP) and the local filesystem/git. This matches the stated "sovereignty floor."

---

## Configuration (12-factor)

Config is environment-variable-driven (`practices: [twelve-factor-app]` on the entry files). 100+ `AGENTOPS_*` vars across `cli/` and `scripts/`, e.g. `AGENTOPS_GATE_BASH=1` (legacy bash gate escape hatch), `AGENTOPS_HOOKS_DISABLED`, `AGENTOPS_ACTOR`, `AGENTOPS_ALLOW_DIRTY`, `AGENTOPS_AO_BIN`, `AGENTOPS_COMPILE_*` (model selection for corpus compile). Global CLI flags: `--dry-run --verbose --output --json --config --work-dir`. The `_beads` ledger dir resolves via `ao beads dir`.

---

## Test Infrastructure

Test-heavy by design (test LOC exceeds source LOC):
- **Go:** `*_test.go` co-located; property tests via `pgregory.net/rapid`; goroutine-leak detection via `goleak`. Strong test-isolation discipline is *itself enforced by gates* (`go.test-isolation`, `go.test-home-isolation`, `go.command-test-pair`, `go.test-count-regression`) — the repo treats flaky shared-global state as a first-class bug class (see `.claude/rules/go.md`).
- **Bats:** 181 files across `tests/` (e2e, integration, cli, codex, claude-code, docs, scenarios, install, windows) + `scripts/`.
- **Gate registry parity tests:** `go test ./internal/gates/checks` validates the 98-check registry (`seed.go` holds 87 of them).
- Tests are the executable spec: `.feature` Gherkin acceptance lives beside skills (`skills/<slug>/references/*.feature`).

---

## Notable Patterns & Conventions

1. **`practices: [slug]` provenance** on source files and skills (e.g. `hexagonal-architecture`, `twelve-factor-app`, `escape-corpus-self-improvement`) — a lightweight, greppable lineage tag tied to `PRACTICE-REGISTRY.md`.
2. **Exit code = verdict.** `Execute()` maps typed errors to meaningful exit codes (gate FAIL=1, plan-pawl REDO=3/BLOCKED=4) so the shell/CI reads the decision without parsing stdout.
3. **Drift-gated generation.** Three registries (skills, workflows, CLI) are *generated* from sources via `make regen-all`; `make regen-check` is the CI drift gate. `registry.json`, `cli/docs/COMMANDS.md`, context-map are **never hand-edited**.
4. **Self-documenting + self-aware.** `docs/architecture/codebase-overview.md` is an excellent, current map; ADRs explicitly mark unproven hypotheses (ADR-0004 corpus moat, ADR-0011 escape-corpus compounding) rather than over-claiming.
5. **In-memory adapter per port** — every driven port ships a test double in-package, enabling fast hermetic tests of orchestration logic.

---

## Strengths

- Clear, enforced product identity: validation-membrane-centered, hookless, in-session, local-first.
- Real hexagonal seams with paired in-memory doubles; minimal external dependency footprint.
- Declarative gate registry (Fast/Full tiers, changed-file routing) as the release authority, with parity tests.
- Exceptionally honest about fitness — refuses to market the flywheel/corpus ahead of measured uplift (ADRs + README "Honest limitations").
- Strong agent ergonomics: `ao capabilities`, `ao robot-docs`, `--json` everywhere.

## Open Debt (per the repo's own overview + footguns)

- **Gate triple-orchestration migration in progress** — Go registry (primary) vs `validate.yml` YAML (CI backstop) vs `scripts/pre-push-gate.sh` (legacy bash escape hatch). ~11 deferred scripts remain to fold into the Go registry.
- **Doc reconciliation drift** — some older narrative docs (`ARCHITECTURE.md`, `ports-and-adapters.md`) still mention hooks, `bd`, or PR-per-change; flagged as historical-until-reconciled. The source-of-truth precedence rule exists *because* of this drift.
- **Skill disposition debt** — ~34 skills marked update/refactor (triage checklists under `docs/audits/`).
- **Worktree hygiene** — merge-eligible worktrees pending audited cleanup.
- **The headline flywheel/escape-corpus claims are structurally data-starved** (ADR-0011): a competent membrane catches at review, so real "escapes" are rare (measured: 0 escapes across 130 production verdicts), making the self-improvement signal anti-correlated with membrane quality.

---

## Navigation Traps (read before editing — high-value gotchas)

| Trap | Reality |
|---|---|
| Treating `/rpi` or `ao rpi` as the live orchestration substrate | It's a one-turn executor / load-bearing legacy CLI. Live navigation = the seven-move **operating loop**; out-of-session = NTM + Agent Mail. |
| Editing `~/.claude/skills/` | Edit `skills/<slug>/SKILL.md` in **this repo** — those are symlinked into the runtime. |
| Running `bd` / Dolt | Use `BEADS_DIR="$(ao beads dir)" br …` — bd/Dolt retired. |
| Hand-editing `registry.json` / `cli/docs/COMMANDS.md` / context-map | Generated — edit sources, run `make regen-all`. |
| Assuming CI gates every push | The **local cockpit gate** (`ao gate check --fast`) is routine authority; CI is a backstop. |
| Trusting narrative docs over code | Source-of-truth precedence: executable+generated > contracts (SKILL.md, schemas) > narrative docs. |
| `git add _beads` | Never — `_beads/` is a private nested git repo; sync with `git -C "$(ao beads dir)" push`. |

---

## Recommended Reading Order (for the next person)

1. `AGENTS.md` (root contract) — seven-move loop, footguns, source-of-truth precedence.
2. `docs/architecture/codebase-overview.md` — the consolidated subsystem map (this report's spine).
3. `docs/3.0.md` — north-star doctrine; `docs/architecture/operating-loop.md` — how work flows (primary navigation).
4. `docs/architecture/component-map.md` — where new work goes; `docs/architecture/ports-and-adapters.md` (treat tracker wording as pre-`br`).
5. Task-specific: `cli/cmd/ao/` + generated `cli/docs/COMMANDS.md` (CLI); `cli/internal/gates/checks/seed.go` (gates); `skills/**/SKILL.md` (skills); `schemas/**` (contracts); ADRs `docs/adr/` (decisions, incl. the honest unproven-hypothesis ones).
