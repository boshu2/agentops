# AgentOps — Technical Architecture Report

> Produced by the `codebase-report` skill (Standard mode) on 2026-07-09, at repo tip
> `2c2bfc3fb` ("feat(discovery): assemble recurring catch classes into the execution
> packet as known_risks (age-8rrz)"). All counts were measured against the working
> tree on that commit; where measured values disagree with narrative docs, both are
> reported (see "Doc-drift observations").

---

## Executive Summary

**AgentOps** is the **verification membrane for coding agents**: a skills corpus + a Go
CLI (`ao`) + a local release gate that together ensure agent work "reaches *done* only
with a proof artifact" — a verdict in a ledger; **no verdict = not done** (`CLAUDE.md`,
`cli/cmd/ao/root.go:32-33`). It is hookless and in-session by design (AgentOps 3.0):
no daemon, no scheduler, no hosted control plane; out-of-session orchestration is
delegated to an external substrate (NTM + Agent Mail; Gas City as a blessed coexisting
substrate via `packs/agentops-membrane/`).

**Key statistics (measured 2026-07-09):**

- Go: 1,445 `.go` files under `cli/` — ~165k LOC source + ~238k LOC test
- Go module `github.com/boshu2/agentops/cli`, Go 1.26 (toolchain go1.26.3) — `cli/go.mod:1-5`
- 60 skill directories in `skills/` (SSOT), with checked-in Codex twins in `skills-codex/`
- ~345 shell scripts in `scripts/` (gate backers, regen, release)
- ~101 gate checks registered in `cli/internal/gates/checks/seed.go`
- 7,558 Go `func Test*` functions; 257 bats files under `tests/`
- 4 Claude workflow scripts (`.claude/workflows/*.js`: bdd-foundry, bead-crank, operating-loop, ship-beads)
- 12 GitHub Actions workflows (`validate.yml` alone is 113 KB — CI backstop, not routine release authority)
- CLI version fallback `3.2.0-rc` set at build time via ldflags — `cli/cmd/ao/main.go:9`

**Product claim honesty (stated in-repo):** the verification mechanism is proven; the
escape-corpus "compounding" and knowledge-flywheel moats are explicitly demoted as
unproven (ADR-0004, ADR-0011; `docs/evals/agentops-effectiveness-evidence.md`).

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `cli/cmd/ao/main.go:11` | `func main() { Execute() }`; version via ldflags |
| Root command | `cli/cmd/ao/root.go:28` | cobra root `ao`; global flags; App injected into context in `PersistentPreRunE` (`root.go:50-77`) |
| Exit-code dispatch | `cli/cmd/ao/root.go:81-183` | `Execute()` maps typed errors to semantic exit codes — the exit code IS the verdict (gate, pawl review, plan-pawl, land, governor, corpus scan, etc.) |
| Command groups | `cli/cmd/ao/root.go:186-198` | Help surface: start/core/workflow/config/comms/knowledge + demoted `experimental` (corpus/flywheel, per ADR-0004/0011, age-h4y3) |
| Aux binaries | `cli/cmd/skill-frontmatter-json/main.go`, `cli/cmd/witness-crosscheck/main.go` | Skill frontmatter JSON extraction; witness Dolt↔JSONL crosscheck |
| Pre-push hook | `scripts/hooks/pre-push.local` installed into `$(git-common-dir)/hooks` → Go gate in `cli/internal/gates/` | Local cockpit = routine release authority; `.githooks/pre-push` is a historical bd-only shim, NOT the live gate; bash monolith `scripts/pre-push-gate.sh` only via `AGENTOPS_GATE_BASH=1` |
| Skills (agent-facing) | `skills/<slug>/SKILL.md` | Invocable contracts; the other half of the product surface |
| Claude workflows | `.claude/workflows/*.js` | Deterministic multi-agent orchestration scripts (kind: workflow) |
| MCP server | `ao mcp serve` (`cli/cmd/ao/`) | Managed-agents JSON-RPC over a curated tool surface |
| Membrane pack | `packs/agentops-membrane/` | Composes the verdict-bound close door onto the Gas City substrate |

There are ~300 non-test Go files in `cli/cmd/ao/` — one file per command family
(beads, gate, pawl, goals, corpus, doctor, land, governor, wiki, codex, session, …).
The generated full surface is `cli/docs/COMMANDS.md`.

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `Check` | `cli/internal/gates/gates.go:57` | One gate check: `ID`, `Tiers (Fast\|Full)`, `Match[]` globs (fast-mode changed-file routing), `Blocking`, and exactly one of `Backing` (shell script) or `Run` (native Go) |
| gate `Registry`/`Orchestrator`/`Report` | `cli/internal/gates/registry.go`, `orchestrator.go`, `report.go` | Seeded from `checks/seed.go` (~101 checks); serial orchestration; PASS/WARN/FAIL/SKIP report |
| `verdictledger.Record` | `cli/internal/verdictledger/verdictledger.go:69` | Tagged union (iteration vs cooldown) of one verdict-ledger entry: directive ID, scenario verdict/satisfaction, cooldown/re-steer events |
| `provenance.Record` / `Graph` | `cli/internal/provenance/provenance.go:204,234` | Artifact→source provenance links, queryable graph over the runtime store `.agents/ao/provenance/graph.jsonl` (`ao trace`); distinct from the COMMITTED hash-chained ledger `docs/provenance/ledger.jsonl`, which is owned by `cli/internal/provenancegraph` (append-only, tracked in git — ledger wins over tracker) |
| `skills.Catalog` / `CatalogEntry` | `cli/internal/skills/catalog.go:22,31` | The skill inventory as data: tier, disposition, DDD/hex edges (`ContextRel` at `catalog.go:45`) |
| `config.Config` | `cli/internal/config/config.go:20` | All runtime config; `BaseDir` default `.agents/ao` |
| `App` | `cli/cmd/ao/app.go` | Per-invocation context (DryRun/Verbose/Output/JSON/WorkDir) injected via `cmd.Context()` (`root.go:66-74`) |

Supporting packages worth knowing: `cli/internal/` has ~80 packages — notably
`corpus`/`corpusscan` (BC1 knowledge), `governor` (error-budget HARDEN decisions),
`planpawl` (plan-review ratchet), `evidencedturn`, `provenancegraph`, `goals`/
`goalsfitness` (fitness vs `GOALS.md`), `doctor` (self-healing cockpit), `orchestration`,
`adapters/` + `ports/` (hexagonal seams).

---

## Data Flow (the active waist)

```
Agent session start
      │
      ▼
ao session bootstrap ───────── universal orientation (hookless; run explicitly)
      │
      ▼
ao lookup / ao corpus inject ─ decay-ranked prior context from .agents/ (BC1)
      │
      ▼
Operating loop (7 moves) ───── BDD intent → br bead → vertical slice → TDD
      │                        (tracker: BEADS_DIR="$(ao beads dir)" br …; private
      │                         nested repo _beads/, never staged publicly)
      ▼
ao gate check --fast --scope head    (BC2 — local cockpit, routine release authority)
      │     Registry(~101 checks, seed.go) → Orchestrator (changed-file routing
      │     in fast mode) → Report PASS/WARN/FAIL/SKIP → blocking FAIL = exit 1
      ▼
pawl review (cross-family verdict; scripts/pawl-review.sh, 129 KB) ── CONFIRMED/REFUTED
      │     verdict bound into docs/provenance/ledger.jsonl (append-only, in git)
      ▼
git push → main (pre-push hook re-runs gate; rebase-on-reject; no routine PR wall)
      │
      ▼
CI backstop (.github/workflows/validate.yml — tags/PRs/manual only)
```

**Happy path:** an agent bootstraps, pulls context, works a bead through TDD, runs the
fast gate (always-on structural checks + checks whose `Match` globs hit changed files),
gets an independent cross-family verdict (pawl), lands directly on `main` with the
verdict recorded in the provenance ledger. Escapes (wrong CONFIRMED verdicts) compile
into new checks — the self-improvement spine (proven mechanism, unproven compounding).

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| `spf13/cobra` + `pflag` | CLI framework | Yes |
| `santhosh-tekuri/jsonschema/v6` | Validating `schemas/**` contracts | Yes |
| `gopkg.in/yaml.v3`, `BurntSushi/toml` | Config + contract parsing | Yes |
| `google/go-cmp`, `pgregory.net/rapid`, `go.uber.org/goleak` | Test-only (diffing, property tests, leak detection) | Test-only |
| `br` (beads_rust) + `bv` | Issue tracker over `_beads/issues.jsonl` (external binaries, not vendored) | Yes (dev workflow) |
| Codex CLI / `agy` / local llama | Cross-family verification workers (pawl lane) | Yes (membrane) |
| bats, shellcheck, golangci-lint | Shell/Go test + lint toolchain | Yes (gates) |
| NTM + MCP Agent Mail | Out-of-session substrate (explicitly *not* shipped in-repo, ADR-0009) | Optional |
| Gas City (owned fork) | Coexisting durable-agent substrate; membrane composes on top via `packs/agentops-membrane/` | Optional |
| MkDocs (`.venv-docs/`) | Docs site build | No |

Notably lean: the Go module has only ~9 direct deps and **no database, no network
service, no async runtime** — persistence is JSONL/YAML files in the repo and
`.agents/` (gitignored runtime corpus).

---

## Configuration

Precedence documented and implemented in `cli/internal/config/config.go:1-8`:

| Source | Example | Priority |
|--------|---------|----------|
| CLI flags | `--config`, `--json`, `-o`, `--dry-run`, `-v` (`root.go:201-205`) | 1 (highest) |
| Env vars | `AGENTOPS_*`; `AGENTOPS_CONFIG` override (`config.go:367`); `--config` flag is synced to env (`root.go:252-258`) | 2 |
| Project config | `.agentops/config.yaml` in cwd | 3 |
| Home config | `~/.agentops/config.yaml` (`config.go:415`) | 4 |
| Defaults | Hardcoded (e.g. `BaseDir` = `.agents/ao`) | 5 (lowest) |

Other config-like surfaces: `schemas/*.schema.json` (~20+ versioned contracts: beads,
eval runs, codex packets, admission ledgers), `docs/contracts/skill-dispositions.yaml`
(skill tier/disposition ledger), `registry.json` (generated SKU catalog — **never
hand-edit**; `make regen-all` / `make regen-check`), and escape hatches
`AGENTOPS_GATE_BASH=1` (legacy bash gate) and `AGENTOPS_HOOKS_DISABLED=1` (user-side
hook bypass, host-level).

---

## Module Structure

```
agentops/
├── cli/                 # Go control plane
│   ├── cmd/ao/          # ~300 command files + tests (one file per command family)
│   ├── cmd/{skill-frontmatter-json,witness-crosscheck}/
│   └── internal/        # ~80 packages: gates/, corpus/, provenance/, verdictledger/,
│                        #   governor/, goals*/, skills/, doctor/, ports/, adapters/ …
├── skills/              # Skill SSOT (60 dirs): SKILL.md + references/ + scripts/
├── skills-codex/        # Generated/maintained Codex runtime twins
├── skills-codex-overrides/  # Hand-kept bespoke twins (catalog.json)
├── scripts/             # ~345 shell tools (check-*.sh gate backers, pawl*.sh, regen)
├── tests/               # bats (257 files), e2e/, integration/, canaries/, python/
├── schemas/             # JSON Schema contracts (bead.v1, eval-run.v1, …)
├── docs/                # Architecture, ADRs, contracts, provenance/ledger.jsonl, MkDocs
├── packs/agentops-membrane/  # Gas City close-door composition
├── .claude/workflows/   # 4 Claude workflow scripts (js)
├── .claude-plugin/ .codex-plugin/ .agy-plugin/  # Runtime install manifests
├── .agents/             # Runtime knowledge corpus (gitignored)
├── _beads/              # Private br ledger (nested git repo — never git add)
└── registry.json        # Generated capability catalog
```

Six DDD bounded contexts route everything (contract: `docs/contracts/bounded-contexts.yaml`;
map: `docs/architecture/component-map.md`): **BC1 Corpus** (`.agents/`, `ao inject`),
**BC2 Validation** (gate, council, pawl), **BC3 Loop** (operating loop, br, goals),
**BC4 Factory** (skill-builder, registries), **BC5 Runtime** (CLI, installers),
**BC6 Orchestration** (substrate boundary — NTM/Agent Mail/gc, dispatches whole skills).

---

## Test Infrastructure

| Type | Location | Count | Notes |
|------|----------|-------|-------|
| Go unit + integration | `cli/**/*_test.go` | 7,558 `func Test*` (~238k LOC) | L2-first doctrine (`.claude/rules/go.md`); shared `rootCmd` demands `t.Cleanup` restore; `-shuffle=on` backstop |
| Bats (gate/shell) | `tests/**/*.bats` | 257 files | Gate parity, skills, CLI, docs consistency |
| E2E | `tests/e2e/*.sh` | ~11+ scripts + `Dockerfile.e2e` | Membrane, goals-trace, proof-run flows |
| Canaries / quarantine | `tests/canaries/`, `tests/_quarantine/` | — | `always.quarantine-empty` gate keeps quarantine drained |
| Python | `tests/python/`, `lib/` | 3 files | Minor surface (black/ruff/mypy per `.claude/rules/python.md`) |
| CI | `.github/workflows/` (12) | — | `validate.yml` (backstop), `nightly.yml`, `release.yml`, `install-e2e.yml`, `verdict-backstop.yml` |

**Running:** `cd cli && go build ./... && go vet ./... && go test ./...` (or `make build test lint`);
`ao gate check --fast --scope head` pre-push; `ao gate check --full` for CI parity;
`tests/run-all.sh` for the bats suite. Push to main runs the full race suite via the
pre-push hook.

---

## Error Handling & Logging

- Errors wrap with context (`fmt.Errorf("…: %w", err)`) per `.claude/rules/go.md`.
- The CLI's distinctive pattern: **typed exit-code errors as verdicts** — 10+ typed
  error structs unwrapped in `Execute()` (`cli/cmd/ao/root.go:83-178`) so that e.g.
  `ao pawl review` exits 0 CONFIRMED / 3 REFUTED, `ao plan-pawl decide` exits 3 REDO /
  4 BLOCKED, `ao governor budget` exits 3 HARDEN. Machine callers read codes, not prose.
- Unknown commands get migration hints (`printRemovedCommandHint`, `docs/MIGRATION.md`);
  unknown flags get typo suggestions (`root.go:211`).
- No logging framework: human output to stdout/stderr, `--json`/`-o json` for
  structured output on read-side commands, `-v` verbose (`VerbosePrintf`, `root.go:246`).

---

## Notes & Gotchas (footguns verified present in-repo)

1. **Skills SSOT is `skills/` in this repo** — `~/.claude/skills` symlinks into the
   live checkout; never edit there or `cp` into it.
2. **Tracker split:** this repo's tracking is `br` (`BEADS_DIR="$(ao beads dir)" br …`);
   `bd`/Dolt is the Gas City substrate store — a different layer. Never `br` from a
   worktree (forks the bead DB); never `git add _beads` (private nested repo).
3. **Generated artifacts:** `registry.json`, `cli/docs/COMMANDS.md`, context maps —
   regen via `make regen-all`, gate via `make regen-check`; hand-edits are gate failures.
4. **Local gate is the release authority; CI is a backstop.** Don't assume a PR wall —
   pushes land directly on `main` after the cockpit gate + pawl verdict.
5. **Gate validates the worktree, not the commit** — check `git show --stat` after
   multi-file commits (known finding in memory index).
6. **`ao rpi` is removed** (f61c5f0e7); the operating loop is in-session navigation,
   NTM + Agent Mail out-of-session. Some older docs still carry pre-3.0 wording —
   executable + generated surfaces win (`CLAUDE.md` source-of-truth precedence).
7. **LAW 0:** `claude -p` is forbidden repo-wide and gate-enforced
   (`always.door9-no-claude-p` in `checks/seed.go:241`).

### Doc-drift observations (narrative vs measured, per precedence rule)

`docs/architecture/codebase-overview.md` "Scale" table (lines 51-63) lags the tree:

| Claimed | Measured 2026-07-09 |
|---------|---------------------|
| 73 active skills | 60 skill dirs in `skills/` (post skills-audit retire wave, 2026-07-06) |
| ~77 gate checks | ~101 check entries in `cli/internal/gates/checks/seed.go` |
| ~139 bats files | 257 bats files under `tests/` |
| ~280 shell scripts | ~345 in `scripts/` |

Not a correctness bug — the overview self-declares "approximate" — but the deltas are
now large enough that a refresh (or a generated scale table) would keep the map honest.

---

*Generated: 2026-07-09 · By: Claude (codebase-report skill, Standard mode) · Repo tip: 2c2bfc3fb*
