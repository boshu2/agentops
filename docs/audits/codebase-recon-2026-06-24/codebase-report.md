# AgentOps — Technical Architecture Report

> ⚠️ **HISTORICAL SNAPSHOT — recon run 2026-06-24 against `abc018c42`; `main` has since advanced past `882e71c01`.** A point-in-time architectural overview, **not** a current-state reference. Some facts are already superseded — notably the **P1 atomic-write DRY finding was partially actioned** (`storage.AtomicWriteFile` is now canonical and quest/llmwiki/doctor/wiki delegate to it, age-3azc/uja6; inject, vendorimage/codexruntime, and `pool.atomicMove` still carry their own copies). The **`ao` command (89) and gate (98) counts are reproducible at the pinned `abc018c42`** (re-verified there 2026-06-25 — and still current on `main`; the draft's `~60`/`~95`/`87`/`~77` figures were simply wrong). `main` has since advanced, so narrative *findings* may be superseded (notably P1, partially actioned), but these architectural counts are stable; all other figures are as-of the 2026-06-24 snapshot.

> Produced by the **codebase-report** skill (read-only recon) on 2026-06-24.
> Repo: `/Users/bo/dev/agentops` · branch `main` · remote `git@github.com:boshu2/agentops.git`
> Source-of-truth precedence (per repo doctrine): executable/generated (`cli/**`, `scripts/**`) > contracts (`skills/**/SKILL.md`, `schemas/**`) > narrative (`docs/**`). Where this report draws on `docs/`, it is reconciled against code.

---

## Executive Summary

**AgentOps** is the **operational layer for coding agents** — a Go CLI (`ao`) plus a corpus of markdown **skills** and a repo-local `.agents/` knowledge tree that wraps an existing coding agent (Claude Code, Codex, Cursor, OpenCode) with a **validation membrane** and a **durable evidence trail**. Its product thesis (stated in `AGENTS.md`): the loop produces *validated output with proof* — every change is independently verified and reaches "done" only with a recorded verdict in the ledger (**no verdict = not done**). It is **hookless and in-session** as of 3.0 — it ships no daemon, scheduler, or hosted control plane; out-of-session orchestration is delegated to an external substrate (NTM + MCP Agent Mail + `ao agent`).

**Key statistics:**
- **~154,000 lines** of non-test Go across `cli/`, plus **~223,000 lines** of Go test code (test code outweighs source — a deliberate verification-heavy posture).
- Languages by file count: **1,366 Go**, **717 shell** (`.sh`), **1,871 markdown** (skills + docs), **206 bats**, **192 `.feature` (Gherkin)**, **54 Python**.
- **Go 1.26** (toolchain go1.26.3), module `github.com/boshu2/agentops/cli`. Version string `3.1.0-rc` (`cmd/ao/main.go:9`).
- **89 top-level `ao` commands** (per `ao --help`; incl. cobra's `help`/`completion`, 87 excluding them), **75+ `internal/` packages**, **98 gate checks** in the Go registry (`seed.go` holds 87 of the 98).
- **77 skills** (`skills/**/SKILL.md`) with Codex twins in `skills-codex/`; **303 shell scripts** in `scripts/`; **181 bats files** + **192 Gherkin features** under `tests/`.
- Minimal external dependency surface: 9 direct Go deps, only **2 indirect** — a notably lean dependency tree for a CLI this size.
- **Build status:** `go build ./...` exits 0 (run 2026-06-24). This is **build-green only** — deep CVE/security scan, full `go vet`/lint, and the full `go test ./...` suite were **not** run, so release-readiness is **unassessed** (not a "verified green" release claim).

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `cli/cmd/ao/main.go:11` | `func main()` → `Execute()`; version injected via ldflags |
| Cobra root + command tree | `cli/cmd/ao/root.go` | Registers the command families on `rootCmd` via `AddCommand(...)` (~97 call sites; 89 visible top-level commands) |
| Release gate | `cli/cmd/ao/gate_check.go` (`gateCheckCmd`) | `ao gate check --fast --scope head` — the routine release authority (local cockpit / pre-push) |
| Gate orchestrator | `cli/internal/gates/orchestrator.go:63` (`Orchestrator.Run`) | Selects + runs the declarative gate registry; fast (changed-file routed) vs full (CI) mode |
| MCP server | `cli/cmd/ao/mcp_serve.go` (`ao mcp serve`) | Exposes the orchestration/Agent-Mail surface (BC6 substrate) |
| Aux binaries | `cli/cmd/skill-frontmatter-json/main.go`, `cli/cmd/witness-crosscheck/main.go` | Small single-purpose tools |
| Skills (advisory) | `skills/<slug>/SKILL.md` | Slash-command behaviors loaded by the harness, not auto-injected (hookless) |

The single user-facing binary is `ao`. Skills are the *other* entry surface — invoked as `/discovery`, `/crank`, `/council`, `/validate`, etc., they orchestrate the same `ao` CLI underneath.

---

## Key Types / Core Domain Objects

| Concept | Location | Purpose |
|---------|----------|---------|
| **Verdict ledger** | `cli/internal/verdictledger/` (`verdictledger.go`, `loader.go`, `writer.go`) | The proof spine. `Record` = a directive→verdict iteration; tracks `IterationsFor`, `FailureStreak`, `InCooldown`. "No verdict = not done" is implemented here. |
| **Gate `Check` / `Orchestrator` / `Registry`** | `cli/internal/gates/{gates,orchestrator,registry}.go` | The validation membrane in code: a registry of checks routed by changed files, run via native funcs or `ScriptRunner` (shells `check-*.sh`). |
| **Provenance / graph** | `cli/internal/provenance/`, `provenancegraph/`; storage `graph.jsonl` (`internal/storage/file.go:35`) | Durable evidence trail — verdict→commit edges, session graph (the `chore(provenance): record … verdict->commit edge` commits in the log). |
| **Config** | `cli/internal/config/config.go` (`Load`, line 359) | Layered runtime config: flags → `AGENTOPS_*` env → config file → defaults. |
| **Paths** | `cli/internal/paths/paths.go` (`Resolve`, `ResolveFromRepo`) | Resolves the `.agents/` runtime dir via `AO_HOME` / `CLAUDE_PLUGIN_DATA` / git repo root. |
| **Corpus / knowledge** | `cli/internal/corpus/`, `knowledge/`, `forge/`, `harvest/`, `mine/` | The knowledge flywheel (BC1): inject, forge learnings, mine sessions, promote. |
| **Beads (tracker)** | `cli/cmd/ao/beads*.go`, `internal/` adapters | `br` (beads_rust) JSONL-backed issue tracker; `ao beads dir` resolves the private `_beads/` ledger. |

---

## Data Flow

```
Coding agent in a session (Claude Code / Codex / Cursor)
        │
        ▼
ao session bootstrap  ─── universal init prompt (hookless; run explicitly)
        │
        ▼
ao inject "<topic>"   ─── decay-ranked prior context from .agents/ corpus (BC1)
        │
        ▼
Operating loop (7 moves)  ─── BDD intent → br bead → vertical slice → TDD →
        │                     wave (if scopes don't collide) → prove acceptance
        ▼
ao gate check --fast --scope head  ─── gate Orchestrator selects checks routed to
        │                              changed files; native Run funcs + ScriptRunner
        │                              (check-*.sh). Exit code = deterministic GATE
        │                              result (pass/fail) — the pre-check, NOT the
        │                              membrane verdict.
        ▼
Cross-family pawl (codex exec)  ─── adversarial refuter on the diff; a CONFIRMED
        │                           verdict bound to the commit IS the BC2 membrane verdict.
        ▼
Verdict written → verdictledger / provenance graph.jsonl  ─── "no verdict = not done"
        │
        ▼
git push → main  ─── pre-push hook is release authority, enforcing BOTH (gate green +
                     CONFIRMED cross-family pawl); rebase-on-reject; no PR wall
        │
        ▼
validate.yml (CI) ─── optional backstop on tags/PRs/manual dispatch, NOT routine authority
        │
        ▼
/forge, /post-mortem  ─── capture learnings, promote to corpus (flywheel ratchet)
```

**Happy path:** an agent boots cold → `ao session bootstrap` + `ao inject` load prior decisions → it runs the operating loop, slicing intent into TDD'd beads → `ao gate check --fast` runs the changed-file-routed gate subset (the same Go binary CI runs in `--full`). **Note:** the fast gate is the *deterministic cockpit pre-check* (changed scope only), **not** the membrane verdict itself — push-to-`main` separately requires a CONFIRMED **cross-family pawl** verdict (`check-pawl-pre-push.sh`; see "The Validation Membrane" below). On a green gate + CONFIRMED pawl, the verdict is bound to the commit and a verdict→commit provenance edge is appended → the agent pushes directly to `main` (the pre-push hook enforces both), and learnings are forged back into the corpus.

---

## External Dependencies

The Go control plane is deliberately lean (`cli/go.mod`):

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| `github.com/spf13/cobra` + `pflag` | CLI command tree / flag parsing | Yes |
| `github.com/BurntSushi/toml` | Config file parsing | Yes |
| `gopkg.in/yaml.v3` | Skill frontmatter / contracts / dispositions | Yes |
| `github.com/santhosh-tekuri/jsonschema/v6` | Validate JSONL/JSON against `schemas/**` | Yes |
| `github.com/google/go-cmp` | Test diffing | No (test) |
| `pgregory.net/rapid` | Property-based testing | No (test) |
| `go.uber.org/goleak` | Goroutine-leak detection in tests | No (test) |
| `golang.org/x/text` | Text normalization | No |

**No SQL DB, no network client, no heavy framework.** Persistence is **append-only JSONL on the local filesystem** (`internal/storage/file.go` — `sessions.jsonl`, `graph.jsonl`), not a database. External *services* are runtime-optional and shelled out, not linked: Codex (`codex exec`) / local bushido llama / an interactive NTM Claude pane for cross-family review (`scripts/pawl*.sh` — the live scripts invoke `codex exec`; per LAW 0 the review lane is NOT `gemini -p`/AGY), `br`/`bv` (Rust beads tracker), `tmux`/NTM and MCP Agent Mail for the orchestration substrate. The legacy `bd`/Dolt path is retired.

---

## Configuration

Layered precedence (`cli/internal/config/config.go`, `Load` at line 359):

| Source | Example | Priority |
|--------|---------|----------|
| CLI flags | `--config`, `--fast`, `--scope head` | 1 (highest) |
| Environment vars | `AGENTOPS_CONFIG`, `AGENTOPS_BASE_DIR`, `AGENTOPS_OUTPUT` … | 2 |
| Config file | resolved from `AGENTOPS_CONFIG` / repo default | 3 |
| Hardcoded defaults | in `config.go` | 4 (lowest) |

**Most-used env knobs** (by occurrence): `AGENTOPS_CONFIG` (the explicit config file — must BE the file, home-leak guarded per commit `c45adcf65`), `AGENTOPS_RPI_RUNTIME[_MODE]`, `AGENTOPS_OUTPUT`, `AGENTOPS_VERBOSE`, `AGENTOPS_BASE_DIR`, `AGENTOPS_NO_SC`, the `AGENTOPS_RPI_*_COMMAND` family (ao/bd/tmux/runtime command overrides), `AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD`, `AGENTOPS_MODEL_TIER`, `AGENTOPS_COUNCIL_MODEL_TIER`, `AGENTOPS_HOOKS_DISABLED`. Path resolution honors `AO_HOME` / `CLAUDE_PLUGIN_DATA` (`internal/paths/paths.go`).

Plugin install manifests live in `.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/`, `.opencode/` — one repo, multiple runtime adapters (BC5 Runtime).

---

## Module Structure (selected)

```
agentops/
├── cli/                      # Go control plane (~154k LOC src, ~223k LOC test)
│   ├── cmd/ao/               # 89 cobra commands per --help (beads, gate, council, forge, inject, …)
│   ├── cmd/{skill-frontmatter-json,witness-crosscheck}/  # aux binaries
│   └── internal/             # 75+ packages, e.g.:
│       ├── gates/            # validation membrane: registry + orchestrator + scriptrunner
│       ├── verdictledger/    # the proof spine (no verdict = not done)
│       ├── provenance/, provenancegraph/   # evidence trail (verdict→commit edges)
│       ├── corpus/, knowledge/, forge/, harvest/, mine/   # knowledge flywheel (BC1)
│       ├── config/, paths/, storage/        # config + JSONL persistence
│       ├── ports/, adapters/                # hexagonal seams (ADR-0001)
│       ├── rpi/, orchestration/, evolve/, autodev/   # loop + legacy RPI engine
│       └── eval/, evalsubstrate/, scenario*/, governor/   # effectiveness measurement
├── skills/                   # Skill SSOT — 77 SKILL.md + references + .feature acceptance
├── skills-codex/             # Codex runtime twins (manually mirrored)
├── scripts/                  # 303 shell tools (validation, regen, release, pawl membrane)
├── tests/                    # bats (181) + Gherkin (192) + integration/e2e/scenarios
├── schemas/                  # JSON schemas for config, provenance, packets, ledgers
├── docs/                     # narrative arch, ADRs (0001–0011), contracts, MkDocs
├── _beads/                   # PRIVATE br ledger (nested git repo — never `git add`)
├── .agents/                  # runtime knowledge (gitignored, local-only)
└── registry.json             # generated SKU catalog (never hand-edit; `make regen-all`)
```

### Six bounded contexts (DDD/hexagonal — ADR-0001)

| BC | Name | Center of gravity |
|----|------|-------------------|
| BC1 | Corpus | `.agents/`, `ao inject`, `/forge`, `/compile`, `/harvest` |
| BC2 | Validation | `ao gate check`, `/validate`, `/council`, `/vibe` — **the membrane** |
| BC3 | Loop | operating loop, `/evolve`, `br`, goals, autodev |
| BC4 | Factory | skill-builder, registries, standards, dispositions |
| BC5 | Runtime | CLI, installers, plugin manifests |
| BC6 | Orchestration | NTM, Agent Mail, swarm — **substrate boundary** |

---

## Test Infrastructure

| Type | Location | Count |
|------|----------|-------|
| Go unit/integration | `cli/**/*_test.go` (inline) | ~223k LOC (more than source) |
| Bats gate/integration | `tests/**/*.bats` | 181 files |
| Gherkin acceptance | `skills/**/*.feature`, `tests/**` | 192 features |
| Python tests | `tests/python/` | (pytest) |
| E2E / install | `tests/e2e/`, `tests/install/`, `tests/release-smoke-test.sh` | smoke + Dockerized e2e |
| Specialized suites | `tests/{scenarios,docs,spec-consistency,canaries,windows,team-runner}` | — |

**Running:**
```bash
cd cli && make build && make test && make lint   # Go
cd cli && go build ./... && go vet ./... && go test ./...
make regen-all && make regen-check               # after inventory edits (drift gate)
make local-ci                                     # full local CI release gate
bash tests/run-all.sh                             # bats suite
ao gate check --fast --scope head                 # the routine release gate
```

Notable test discipline (from `.claude/rules/go.md`): L2-integration-first, fixture-fidelity (round-trip the production writer/reader), and `t.Cleanup`-enforced isolation for shared cobra-global/process state — a known recurring flake class guarded by a late `-shuffle=on` race suite.

---

## The Validation Membrane (the product, in code)

The headline feature is the **cross-family review membrane** ("pawl"), implemented as shell over the `ao` CLI:
- `scripts/pawl.sh`, `pawl-review.sh`, `pawl-verdict.sh`, `pawl-land.sh`, `check-pawl-pre-push.sh` — run a diff through a *different model family* (the live scripts use `codex exec`; local bushido llama / an interactive NTM Claude pane are the other LAW 0-sanctioned lanes — never `claude -p`, and not `gemini -p`/AGY) as an adversarial refuter, parse the verdict, and bind it to the commit. A push to `main` requires a CONFIRMED cross-family verdict (`check-pawl-pre-push.sh`); `#trivial` waives only when every changed file is under `docs/provenance/`.
- The **self-improvement claim** (each *escape* — a verdict that said CONFIRMED but later proved wrong — compiles into a check that catches it next time) is implemented (the EM spine) but **demoted to unproven** in doctrine due to structural data-starvation (ADR-0011): a competent membrane catches at review, so escapes are structurally rare (measured 0 escapes across 130 real production verdicts).

---

## Error Handling, Logging, Persistence

- **Errors:** idiomatic Go — `if err != nil`, wrapped with `fmt.Errorf("...: %w", err)`, `errors.Is`. Gate exit code IS the verdict (0 = no blocking FAIL).
- **Persistence:** append-only **JSONL** on local disk (`internal/storage/file.go`), validated against `schemas/**`. No DB. File locking via `filelock_unix.go` / `filelock_windows.go`.
- **Output:** human + `--json` machine-readable mode on most surfaces (gate reports, council verdicts).

---

## Notes & Gotchas

- **Hookless by design (3.0, ADR-0002):** nothing auto-injects context. `ao session bootstrap` and `ao inject` must be run explicitly. Old docs mentioning hooks/`bd`/PR-per-change are historical (ADR/precedence note in `AGENTS.md`).
- **`bd`/Dolt is retired legacy.** The live tracker is `br` (beads_rust) over `_beads/issues.jsonl`; resolve the dir with `ao beads dir` before any `br` call (linked worktrees don't carry `_beads`). Never `git add _beads`.
- **`ao rpi` is load-bearing *legacy* code** — heavily tested but NOT the live orchestration path (NTM + Agent Mail is). `internal/orchestration` + `ao orchestrate` is the kept semantic layer (ADR-0009 deleted the in-repo daemon).
- **Generated artifacts must not be hand-edited:** `registry.json`, `cli/docs/COMMANDS.md`, skill domain maps. Edit sources, then `make regen-all`; `make regen-check` is the drift gate. **Codex twins (`skills-codex/`) are NOT regenerated** — manually mirror then `scripts/regen-codex-hashes.sh`.
- **`ao gate check --fast` only tests the *changed scope*** — it is the routine cockpit gate, not a full CI run (`--full` is). Stale-binary and worktree≠commit traps are documented footguns (the gate builds the working tree, so a partial `git add` can land a build-broken commit that passed the gate).
- **Honest fitness posture:** measured live-agent *uplift* is **not yet proven** (`docs/evals/agentops-effectiveness-evidence.md`); both the corpus moat (ADR-0004) and escape-corpus compounding (ADR-0011) are explicitly demoted to unproven hypotheses. The *proven* product is the verification itself.
- **LAW 0:** no agent runs `claude -p` / `claude --print` anywhere — mechanically guarded; cross-family review uses `codex exec` (what the live pawl scripts invoke) / local bushido llama / an interactive NTM Claude pane — **not** `gemini -p`/AGY.

---

*Generated 2026-06-24 by the codebase-report skill (read-only recon); command/gate counts re-verified 2026-06-25 at `abc018c42` (still current on `main`). `go build ./...` → exit 0 (build only — full test suite + security scan NOT run; release-readiness unassessed). All `file:line` references against `main` at commit `abc018c42`; `main` has since advanced.*
