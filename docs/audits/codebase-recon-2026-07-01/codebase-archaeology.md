# AgentOps — Codebase Archaeology (Technical Architecture Summary)

> Produced by the `codebase-archaeology` skill, 2026-07-01. Method: documentation-first
> (AGENTS.md → codebase-overview.md → README.md), then entry points → key types → data
> flow → config → integrations → tests. All claims below were verified against
> executable code, per the repo's own source-of-truth precedence (executable > contracts
> > narrative). Read-only pass; `go build ./...` verified green during this audit.

---

## Executive Summary

**AgentOps** is a *verification membrane for coding agents*: it catches an agent declaring
"done" on code that is still wrong. Every change must be independently verified — by a
different model family (`codex exec` refuter via the "pawl") or a deterministic gate —
and reaches *done* only with a proof artifact bound to the commit in a hash-chained
provenance ledger. **No verdict = not done.**

Structurally it is three things layered on one repo:

1. **Skills** (`skills/`, 73 active) — Markdown process contracts (SKILL.md) installed
   into Claude Code / Codex / Gemini / OpenCode runtimes via plugin manifests and
   symlinks. Hookless by design (ADR-0002): nothing auto-injects; context is pulled
   explicitly (`ao session bootstrap`, `ao lookup`).
2. **The `ao` Go CLI** (`cli/`, ~156K LOC source + ~226K LOC tests) — the control plane:
   gate orchestrator, pawl/verdict bookkeeping, corpus retrieval, bead-tracker glue,
   doctor, membrane catch/recall, provenance.
3. **Shell tooling** (`scripts/`, 317 scripts) — the gate check backings, the pawl
   review executable (`pawl-review.sh`), regen/release/validation tools.

It implements **DDD + hexagonal architecture** (ADR-0001) across six bounded contexts
(BC1 Corpus → BC6 Orchestration), with a genuine ports-and-adapters layer
(`cli/internal/ports/` — interfaces plus in-memory fakes for every port) and a
declarative, init()-registered gate registry (no central orchestrator monolith).

**Key statistics** (measured this pass):

| Dimension | Value |
|---|---|
| Go source (non-test) | 155,984 LOC |
| Go test code | 225,952 LOC (**1.45× source** — the product takes its own medicine) |
| `cli/cmd/ao/` files | 614 (`.go`), 79 top-level commands (generated `cli/docs/COMMANDS.md`) |
| `cli/internal/` packages | 79 |
| Shell scripts | 317 in `scripts/` |
| Skills | 73 in `skills/` + checked-in Codex twins in `skills-codex/` |
| Claude workflows | 4 (`.claude/workflows/*.js`: bdd-foundry, bead-crank, operating-loop, ship-beads) |
| Gate checks | ~77 in the Go registry (`cli/internal/gates/checks/`) |
| Provenance ledger | 213 hash-chained entries (`docs/provenance/ledger.jsonl`) |
| Go toolchain | go 1.26 (go1.26.3), cobra 1.10, minimal dep tree (9 direct deps) |
| Local git history | **shallow clone** — grafted at 2026-06-30, 50 commits visible |

---

## Entry Points

| Entry | Location | Notes |
|---|---|---|
| `ao` CLI | `cli/cmd/ao/main.go:10` → `Execute()` in `cli/cmd/ao/root.go:83` | cobra root; version `3.1.0-rc` set via goreleaser ldflags |
| Root command | `cli/cmd/ao/root.go:28` (`rootCmd`) | `PersistentPreRunE` builds the `App` DI struct, sanitizes git env, repairs shared-core worktree config, injects `App` into `cmd.Context()` |
| Exit-code protocol | `root.go:83–170` | typed exit errors where **the exit code IS the verdict**: pawl review (0 CONFIRMED / 3 REFUTED / 4 advisory), gate (1 FAIL / 2 internal), plan-pawl (3 REDO / 4 BLOCKED), governor (3 HARDEN), doctor, beads, corpus scan, wiki lint |
| Pre-push release gate | `.git/hooks` chained by `scripts/install-pre-push-gate.sh` → `scripts/hooks/pre-push.local` | **NOT** the tracked `.githooks/pre-push` (that is a historical bd shim; its own header says so) |
| Pawl (cross-family review) | `scripts/pawl-review.sh` (invoked by `ao pawl review`) | diff → adversarial `codex exec` refuter → parse VERDICT → write commit-bound verdict |
| Auxiliary binaries | `cli/cmd/skill-frontmatter-json/`, `cli/cmd/witness-crosscheck/` | small tools, not product surface |
| Runtime installs | `.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/`, `scripts/install-*.sh` | per-runtime plugin manifests + installers |

Help output is grouped (`start / core / workflow / config / comms / knowledge`); every
read-side command supports `--json`, and `ao capabilities` emits a machine-readable CLI
contract — the CLI is explicitly agent-ergonomic.

---

## Key Types (the ones everything revolves around)

| Type | Location | Purpose |
|---|---|---|
| `App` | `cli/cmd/ao/app.go` | DI container replacing globals (Terraform Meta + kubectl Options hybrid): flag values + injectable `ExecCommand`/`LookPath`/`RandReader`/`Stdout`/`Stderr`; retrieved via `AppFromContext` |
| `gates.Check` | `cli/internal/gates/gates.go` | one declarative gate: `ID`, `Tiers` bitmask, exactly one of `Backing` (script) or `Run` (native Go) — validated structurally on registration |
| `gates.Tier` | `cli/internal/gates/gates.go:24` | `Fast` (cockpit/pre-push, "must stay seconds") vs `Full` (CI/refinery) — filters over ONE registry so the lists can never drift |
| `gates.RunContext` | `gates.go:38` | read-only per-run inputs: `RepoRoot`, routed `ChangedFiles`, `Mode` |
| `ports.GateVerdict` | `cli/internal/ports/gate_runner.go:16` | the outcome unit of a gate run — the verdict is a value, not a log line |
| `Config` | `cli/internal/config/config.go` | full runtime config (Output, BaseDir, Forge, Search, Paths, RPI, Flywheel, Models, Dream…) |
| Provenance `Record`/`Graph` | `cli/internal/provenance/provenance.go` | load/trace the hash-chained ledger; `Trace(artifact)`, `FindBySession`, `FindBySource` |

The `ports/` package is the hexagonal seam: ~25 port interfaces (GateRunner, Tracker,
CorpusReader/Writer, ClaimEvidence, EventBus, FindingCompiler, Harness, LLM, Orchestration…)
each paired with an `inmemory_*.go` fake and its own test — this is why 226K LOC of tests
can run without external CLIs.

---

## Data Flow (the membrane, traced)

```
change lands on disk
  │
  ├─► ao gate check --fast --scope head           (deterministic lane)
  │     root.go → gates.Orchestrator
  │       → changedfiles.go computes routed change set (path globs in checks/seed.go)
  │       → registry filters by Tier + routing
  │       → each Check runs: Backing → scriptrunner → scripts/*.sh   (Phase A)
  │                          Run     → native Go CheckFunc           (Phase B)
  │       → report.go composes verdicts → exit code IS the verdict
  │
  ├─► ao pawl review <bead> --scope head          (cross-family lane)
  │     → scripts/pawl-review.sh
  │       → git diff → adversarial refuter prompt → codex exec (fresh context, read-only)
  │       → parse VERDICT
  │       → CONFIRMED: pawl-verdict.sh writes commit-bound verdict (.agents/pawl-verdicts/)
  │       → REFUTED:  defects saved to .agents/pawl-evidence/, exit 3
  │       → lineage-gated --converge for heuristic tails (advisory-only without lineage)
  │
  ├─► verdict bound into docs/provenance/ledger.jsonl
  │     append-only JSONL, each row: from_id(verdict)→to_id(commit sha),
  │     relation=wasDerivedFrom, prev_hash → payload_hash → hash  (tamper-evident chain)
  │
  └─► pre-push hook (installed, not tracked) re-runs the cockpit gate + full -race suite
        → push directly to main (PR flow retired; GitHub Actions = tag/PR/manual backstop only)
```

Supporting flows:

- **Context in**: `ao session bootstrap` → `ao lookup --query` (decay-ranked retrieval
  over the gitignored `.agents/` corpus). Corpus writes are contract-governed —
  `docs/contracts/agents-write-surfaces.md` catalogues every production write surface
  under `.agents/`, linted by `scripts/check-agents-write-surfaces.sh`.
- **Work tracking**: `br` (beads_rust) over `_beads/issues.jsonl` — a **private nested
  git repo**, gitignored here; resolved via `ao beads dir`. `bd`/Dolt is retired legacy
  (`.beads/` preserved, not authoritative).
- **Escapes → checks**: `ao membrane catch/recall/triage` records verdicts that later
  proved wrong and compiles them into future checks (the EM spine — mechanism proven
  e2e; corpus *compounding* explicitly demoted to unproven, ADR-0011).

---

## Configuration

Precedence (from `cli/internal/config/config.go` header):

1. CLI flags (`--config`, `--json`, `-o`, `--dry-run`, `-v`)
2. `AGENTOPS_*` environment variables
3. Project config: `.agentops/config.yaml` (cwd)
4. Home config: `~/.agentops/config.yaml`
5. Defaults (`BaseDir` default `.agents/ao`)

Notable env toggles found in scripts/gates: `AGENTOPS_GATE_BASH=1` (legacy bash gate),
`PAWL_NO_SERVICE=1` (cold pawl), `PAWL_UNTRUSTED_REPO=1` (stranger-repo hardening —
never execute the reviewed repo's own `cli/bin/ao`; a real RCE class closed under
age-a9iv), `AGENTOPS_LEGACY=1` (legacy build tag), `AO_BIN` (pin the trusted binary).

**Build-tag archive mechanism (ADR-0012)** — a de-facto configuration axis: satellite
command sets are archived behind `//go:build flywheel` and `//go:build legacy`
(17 tag-gated files in `cli/cmd/ao/`). The default build is the **spine** (corpus/flywheel +
RPI/factory commands omitted); `make build-flywheel` restores them; hidden
`ao buildtags` introspects which build you hold. *Footgun with a track record:* validators
that build spine-only miss archived-command references (hit twice — age-sydq, age-zei7).

---

## External Dependencies & Integration Points

| Integration | Mechanism | Notes |
|---|---|---|
| **codex exec** | subprocess from `pawl-review.sh` + `cli/cmd/ao/codex.go` (94.5K, largest file) | the cross-family refuter; LAW 0: never `claude -p` |
| **git** | shelled everywhere; worktree config repair in `PersistentPreRunE` | worktree-per-bead is mandatory under swarm load |
| **br / bv** | external Rust CLI over `_beads/` JSONL | tracker; not linked, just orchestrated |
| **NTM + MCP Agent Mail** (`ao mcp serve`) + `ao agent` | out-of-session substrate | deliberately external — AgentOps ships **no daemon** (ADR-0009) |
| Distribution | goreleaser + `homebrew-tap/`, install scripts per runtime | brew, npx skills, curl installers, Windows ps1 |
| Go deps | cobra/pflag, BurntSushi/toml, yaml.v3, santhosh-tekuri/jsonschema, go-cmp, goleak, rapid | notably lean; **no database, no network SDK** — persistence is files (JSONL/YAML/MD) |

There is no server, no DB, no HTTP router in the product path. All state is repo-local
files, which is a deliberate architectural stance (subscription-native, runs on your
hardware).

---

## Test Infrastructure

- **Go**: 225,952 LOC of tests co-located in packages; push runs the **full `-race
  -shuffle=on` suite** (a known flaky class in `cli/cmd/ao` due to shared cobra globals —
  documented in `.claude/rules/go.md` with the `t.Cleanup` discipline). L2-first test
  doctrine; fixture-fidelity rule (fixtures must round-trip the real persisted shape).
- **Bats**: ~139 files under `tests/` (gate scripts, skills, install, e2e, canaries,
  land-queue, windows); tiered master runner `tests/run-all.sh` (tier 1 fast → tier 3
  functional → `--all`), plus `smoke-test.sh` / `release-smoke-test.sh` and a
  `Dockerfile.e2e`.
- **Gate self-coverage**: `cli/internal/gates/checks/` carries parity, seed, coverage and
  constraint tests; the registry validates every check structurally at registration.
- **Verification of this audit**: `cd cli && go build ./...` → success (2026-07-01).

Tests are the intended-behavior oracle here more than docs are: e.g. the exit-code
protocol, the writer-isolation contract (`cobra_writer_isolation_test.go`), and codex
dispatch contracts (`codex_packet_contract_test.go`) are all specified test-first.

---

## Layered Mental Model (skill's layer template, instantiated)

```
ENTRY POINTS      ao CLI (cobra, 79 cmds) · pre-push hook · skills invoked by the agent runtime
      │
HANDLERS          cli/cmd/ao/*.go (614 files — thin cobra RunE handlers + typed exit errors)
      │
CORE DOMAIN       cli/internal/ (79 pkgs): gates (registry/orchestrator/routing/report),
                  provenance (hash chain), corpus, membrane/verdictledger, doctor, goals,
                  scenario/eval, rpi (load-bearing LEGACY — not live navigation)
      │
PORTS/ADAPTERS    cli/internal/ports (interfaces + inmemory fakes) · internal/adapters
      │
STORAGE/INTEGR.   repo files: .agents/ (gitignored runtime), docs/provenance/ledger.jsonl,
                  _beads/ (private nested repo), registry.json (generated), schemas/ (30+)
                  subprocesses: git, codex exec, br/bv, NTM/Agent Mail
```

---

## Archaeology Notes (what the dig itself surfaced)

1. **The repo self-describes unusually well, and the docs know their own limits.**
   Source-of-truth precedence is stated *in* AGENTS.md so injected/stale docs can't
   override executable truth. `docs/architecture/codebase-overview.md` matched measured
   reality on every figure I checked (skills 73 ✓, commands ~88 vs 79 generated —
   plausibly counting archived-tag commands, worth a regen check but not a defect).
2. **Local history is shallow** (`.git/shallow`, grafted 2026-06-30; only 50 commits
   visible locally). Deep git archaeology (blame-driven intent recovery) is not possible
   from this checkout; the repo compensates with an explicit paper trail: 13 ADRs,
   `CHANGELOG.md` (107K), `PRACTICE-REGISTRY.md`, and the provenance ledger.
3. **Honesty markers are structural, not cosmetic.** ADR-0004/ADR-0011 demote the
   product's own flywheel/escape-corpus hypotheses to "unproven"; GOALS/PRODUCT carry a
   "don't market ahead of the ruler" rule. Rare in any codebase.
4. **Two generations coexist by design**: the RPI/factory + flywheel command sets are
   archived (build tags), `bd`/Dolt config is preserved-but-dead in `.beads/`, and the
   tracked `.githooks/` is a historical shim superseded by the installed hook. When
   reading this codebase, always ask "spine or satellite?" before trusting a reference.
5. **The dominant risk shape is drift between generated and source surfaces** —
   `registry.json` (142K, generated), `cli/docs/COMMANDS.md`, `skills-codex/` twins
   (manually mirrored + hashed). The repo knows this: `make regen-check` and ~77 gate
   checks exist mostly to police exactly that.
6. **`cli/cmd/ao/codex.go` at 94.5K is the largest single file** and the highest-value
   deep-dive target for a follow-up (codex dispatch, packets, receipts); `beads*.go`
   (~150K across files) is the second center of gravity.

## Where to look first (reusable orientation for the next agent)

1. `AGENTS.md` (≡ `CLAUDE.md`) — the contract; then `docs/architecture/codebase-overview.md`.
2. `cli/cmd/ao/root.go` — the exit-code-is-verdict protocol tells you what the product *is*.
3. `cli/internal/gates/` — the declarative gate registry (read `gates.go` then `checks/seed.go`).
4. `scripts/pawl-review.sh` — the cross-family membrane, end to end, in one script header.
5. `docs/provenance/ledger.jsonl` — tail two lines; the hash chain makes "no verdict = not done" concrete.
