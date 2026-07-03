# AgentOps — Technical Architecture Report

> Produced by the `codebase-report` skill (Standard mode) · Generated 2026-07-01 · HEAD `e4ec22c46`
> All file:line references verified against the working tree at that commit. `cd cli && go build ./...` passes at HEAD.

---

## Executive Summary

**AgentOps** is a verification membrane for AI-agent software work: a Go CLI (`ao`), a corpus of 73 agent skills, and ~317 shell gate/validation scripts that together catch an agent declaring "done" when it isn't. Every change is independently verified (fresh-context, cross-family review via `codex exec` — the "pawl") and reaches *done* only with a proof artifact: a commit-bound verdict in a hash-chained provenance ledger. **No verdict = not done.** It ships as a hookless plugin + CLI for Claude Code, Codex CLI, and OpenCode; release authority is a local pre-push cockpit gate, not CI.

**Key statistics:**

| Metric | Value |
|---|---|
| Go source (non-test) | ~156,000 LOC across `cli/` (614 files in `cli/cmd/ao/` alone) |
| Go test code | ~226,000 LOC · 662 `_test.go` files · 7,225 `Test*` functions |
| Shell/Python tooling | ~61,000 LOC · 317 `.sh` scripts in `scripts/` |
| Skills | 73 in `skills/` (SSOT) + 65 manually-mirrored Codex twins in `skills-codex/` |
| CLI surface | 79 documented commands under 28 top-level groups (`cli/docs/COMMANDS.md`, generated) |
| Gate checks | ~91 seeded checks (`cli/internal/gates/checks/seed.go:105-178+`) |
| Contracts | 55+ JSON schemas in `schemas/` |
| Docs | 514 markdown files in `docs/`, incl. 13 ADRs |
| Provenance ledger | 213 hash-chained verdict entries (`docs/provenance/ledger.jsonl`) |
| Language / toolchain | Go 1.26 (toolchain 1.26.3), bash, minimal Python; module `github.com/boshu2/agentops/cli` |

Six bounded contexts organize the system (`docs/architecture/component-map.md:37-42`): **BC1 Corpus** (`.agents/`, retrieval), **BC2 Validation** (gates, verdicts), **BC3 Loop** (BDD intent, `br` beads), **BC4 Factory** (skill/workflow admission), **BC5 Runtime** (harness packages, `ao` distribution), **BC6 Orchestration** (substrate dispatch).

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `cli/cmd/ao/main.go:10` → `Execute()` in `cli/cmd/ao/root.go:83` | Cobra root; version set via goreleaser ldflags (fallback `3.1.0-rc`, `main.go:8`) |
| Root command | `cli/cmd/ao/root.go:28` | `PersistentPreRunE` sanitizes git env, repairs worktree config, injects an `App` context (`root.go:52-79`) |
| Pre-push cockpit gate | `scripts/hooks/pre-push.local` (installed by `scripts/install-pre-push-gate.sh` into the git-common-dir hooks) | THE release authority: fresh `ao` build, full `-race` suite on main pushes, gate + provenance + pawl checks; only bypass is audited `AGENTOPS_GATE_DISABLED=1` |
| Pawl review | `scripts/pawl-review.sh` (37 KB) | Runs the cross-family adversarial review: diff → `codex exec` refuter → parse VERDICT → write commit-bound verdict; exit 0 CONFIRMED / 3 REFUTED |
| Skills | `skills/<slug>/SKILL.md` (73 slugs) | Agent-invoked process moves; SSOT here, never `~/.claude/skills/` |
| Claude workflows | `.claude/workflows/{bdd-foundry,bead-crank,operating-loop,ship-beads}.js` | Deterministic multi-agent orchestration scripts (Claude-only, no Codex twin) |
| Plugin manifests | `.claude-plugin/plugin.json`, `plugins/marketplace.json`, `.codex-plugin/` | Marketplace install surface (`claude plugin install agentops@agentops-marketplace`) |
| Aux binaries | `cli/cmd/skill-frontmatter-json/main.go:14`, `cli/cmd/witness-crosscheck/main.go:29` | Gate-support tools |
| CI backstop | `.github/workflows/validate.yml` (109 KB), `nightly.yml`, `release.yml` | Tag/PR/manual backstop only — NOT routine authority (direct-push-to-main model) |
| Local CI | `Makefile` → `scripts/ci-local-release.sh` (default goal `local-ci`) | Release-grade local gate incl. build + release-binary validation |

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `gates.Check` | `cli/internal/gates/gates.go:57` | One release-gate check: ID (e.g. `go.vet`), `Tiers` (Fast/Full), path-glob `Match` for fast-mode selection, `Blocking` vs advisory, `Backing` script or native `Run` |
| `gates.Orchestrator` / `gates.Report` | `cli/internal/gates/orchestrator.go:48`, `report.go:15` | Selects affected checks by changed files (`changedfiles.go:35`), runs them, renders verdict report |
| `config.Config` | `cli/internal/config/config.go:20` | All runtime config (Output, BaseDir, Forge, Search, Paths, RPI, Flywheel, Models, Dream, Compile) |
| `provenance.Record` / `provenance.Graph` | `cli/internal/provenance/provenance.go:204,234` | Artifact→source lineage entries + queryable graph over the JSONL ledger |
| `verdictledger.Record` / `Ledger` | `cli/internal/verdictledger/verdictledger.go:69,88` | Pawl/gate verdict bookkeeping (iteration, cooldown inputs at `:95,:106`) |
| `types.Candidate`, `types.KnowledgeType`, `types.Tier` | `cli/internal/types/types.go:122-174` | Knowledge-corpus domain objects (BC1) flowing capture → curate → promote |

Supporting cast: ~80 packages under `cli/internal/` (corpus, goals, governor, lifecycle, orchestration, pawl-adjacent `planpawl`, `ratchet`, `scenario`, `yieldledger`, …) — the subsystem map is `docs/architecture/codebase-overview.md`.

---

## Data Flow

The product path is the seven-move operating loop (`docs/architecture/operating-loop.md`); mechanically it flows:

```
 Agent session start
        │
        ▼
 ao session bootstrap ──── universal init prompt (cli/cmd/ao/session_bootstrap.go:56)
        │
        ▼
 ao lookup --query … ───── decay-ranked prior context from .agents/ corpus (cli/cmd/ao/lookup.go)
        │
        ▼
 Operating loop ────────── BDD intent → br bead (BEADS_DIR="$(ao beads dir)") → vertical slices → TDD
        │
        ▼
 ao gate check --fast ──── gates.Orchestrator selects ~91 checks by changed paths,
   --scope head            runs backing scripts/ or native Go, blocking FAIL ⇒ exit 1
        │
        ▼
 git push (main) ────────► scripts/hooks/pre-push.local: fresh ao build + full -race suite
        │                  + serialized gate/provenance/pawl checks
        ▼
 pawl review ───────────── scripts/pawl-review.sh: diff → codex exec (cross-family, fresh
        │                  context, read-only) → VERDICT parse
        ▼
 CONFIRMED verdict ─────── commit-bound entry appended to docs/provenance/ledger.jsonl
                           (hash-chained: prev_hash → payload_hash → hash;
                           schema schemas/agentops-sdlc-provenance.v1.schema.json)
```

**Happy path:** an agent shapes intent as Given/When/Then, tracks it as a `br` bead, implements test-first, runs `ao gate check --fast --scope head`, and pushes; the pre-push hook re-verifies and the pawl writes the CONFIRMED verdict that makes the work *done*. A REFUTED verdict (exit 3) returns defects for the author to fix and re-run. A `#trivial` waiver of the pawl applies only when every changed file is under `docs/provenance/` (fail-closed, memory: age-u43w).

**Exit-code-as-verdict pattern:** `Execute()` (`cli/cmd/ao/root.go:86-165`) maps nine typed errors (`gateExitError`, `pawlReviewExitError`, `planPawlExitError`, `beadsExitError`, `doctorExitError`, `governorExitError`, `corpusScanExitError`, `tickExitError`, `wikiHealthExitError`) to process exit codes where **the code IS the verdict** (e.g. pawl: 0 CONFIRMED · 3 REFUTED · 4 converge-advisory · 2 usage).

---

## External Dependencies

The Go module is deliberately thin — no network clients, no DB drivers (`cli/go.mod`):

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| `spf13/cobra` + `pflag` v1.10/1.0 | CLI framework, command tree | Yes |
| `santhosh-tekuri/jsonschema/v6` | Validating the 55+ `schemas/*.json` contracts | Yes |
| `gopkg.in/yaml.v3`, `BurntSushi/toml` | Config + frontmatter parsing | Yes |
| `google/go-cmp`, `go.uber.org/goleak`, `pgregory.net/rapid` | Test-only: diffing, leak detection, property tests | Test-only |
| `golang.org/x/text` | Text normalization | No |

The **real** external dependencies are subprocess CLIs, resolved at runtime:

| Runtime tool | Purpose | Critical? |
|---|---|---|
| `git` | Everything: changed-file scoping, verdict binding, worktree repair (`root.go:57-66`) | Yes |
| `codex exec` | The cross-family pawl refuter (LAW 0: never `claude -p`) | Yes (for verdicts) |
| `br` / `bv` (beads_rust) | Issue tracker over `_beads/issues.jsonl` (private nested repo; `bd`/Dolt RETIRED) | Yes (for tracking) |
| `bats` | 206 shell test files / 1,680 cases | Yes (for gates) |
| NTM / tmux, Agent Mail (`ao mcp serve`) | Optional out-of-session substrate (ADR-0009: no in-repo daemon) | No (opt-in) |

---

## Configuration

Precedence documented and implemented in `cli/internal/config/config.go:1-8` (`Load` at `:359`):

| Source | Location/Example | Priority |
|--------|------------------|----------|
| CLI flags | `--config`, `--json`, `-o`, `--dry-run`, `-v` (`root.go:183-187`) | 1 (highest) |
| Env vars | `AGENTOPS_*` (`applyEnv`, `config.go:485-517`): `AGENTOPS_CONFIG`, `AGENTOPS_OUTPUT`, `AGENTOPS_BASE_DIR`, `AGENTOPS_RPI_RUNTIME_MODE`, `AGENTOPS_COUNCIL_MODEL_TIER`, … | 2 |
| Project config | `.agentops/config.yaml` in cwd (`projectConfigPath`, `config.go:423`) | 3 |
| Home config | `~/.agentops/config.yaml` (`homeConfigPath`, `config.go:410`) | 4 |
| Defaults | `Default()`, `config.go:298` (BaseDir `.agents/ao`, output `table`) | 5 (lowest) |

An explicit `--config`/`AGENTOPS_CONFIG` override IS the config file (`config.go:362`); the flag is synced to env in `root.go:234-240`. Gate/hook control envs live outside this struct: `AGENTOPS_GATE_DISABLED=1` (audited pre-push bypass), `AGENTOPS_GATE_BASH=1` (legacy bash gate), `AGENTOPS_COMMIT_SCOPE_OK=1`.

---

## Module Structure

```
agentops/
├── cli/                  Go CLI (ao) — the product's executable half
│   ├── cmd/ao/           614 files: one file per command + paired _test.go
│   ├── cmd/{skill-frontmatter-json,witness-crosscheck}/  gate-support binaries
│   ├── internal/         ~80 packages: gates/, config/, provenance/, corpus/,
│   │                     verdictledger/, goals/, orchestration/, lifecycle/, …
│   └── docs/COMMANDS.md  generated CLI reference (do not hand-edit)
├── skills/               73 skill dirs (SSOT) + SKILL-TIERS.md + catalog.json
├── skills-codex/         65 Codex twins — MANUALLY mirrored + regen-codex-hashes.sh
├── scripts/              339 entries (~317 .sh): gate backings, pawl*, release, regen
├── tests/                bats suites (206 files/1,680 cases): cli/, e2e/, integration/,
│                         skills/, canaries/, lint/, windows/ + run-all.sh, smoke-test.sh
├── schemas/              55+ JSON Schema contracts (pawl-verdict.v1, bead.v1, …)
├── docs/                 514 md files: architecture/, adr/ (13), contracts/, provenance/
│   └── provenance/ledger.jsonl   hash-chained verdict ledger (213 entries)
├── .claude/workflows/    4 Claude-only workflow scripts (kind: workflow)
├── .claude-plugin/ .codex-plugin/ plugins/   marketplace install manifests
├── evals/                membrane/memory/scenario eval harnesses
├── lib/                  shared shell/js helpers (ao-paths.sh, skills-core.js)
├── _beads/               PRIVATE br ledger (nested repo — never `git add _beads`)
├── .agents/              runtime knowledge corpus (gitignored; write surfaces
│                         contracted in docs/contracts/agents-write-surfaces.md)
└── registry.json         generated SKU catalog (142 KB) — `make regen-all` only
```

---

## Test Infrastructure

| Type | Location | Count |
|------|----------|-------|
| Go unit + integration | `cli/**/*_test.go` (paired with source; `go.command-test-pair` gate enforces a cmd test in the same commit) | 662 files / 7,225 `Test*` funcs |
| Bats (shell) | `tests/{cli,e2e,integration,skills,install,lint,windows,canaries,…}` | 206 files / 1,680 `@test` cases |
| Gate checks | `cli/internal/gates/checks/seed.go` — namespaces `always.*`, `go.*`, `skill.*`, `contract.*`, `corpus.*`, `ci.*`, `eval.*`, `governance.*`, `workflow.*` | ~91 |
| Eval harnesses | `evals/{membrane,memory,scenarios,workbench}` | membrane concordance, trap tasks |
| Smoke/e2e | `tests/smoke-test.sh`, `tests/run-all.sh`, `tests/Dockerfile.e2e`, `.github/workflows/install-e2e.yml` | — |

**Running:**
```bash
cd cli && go build ./... && go vet ./... && go test ./...   # Go (verified green at HEAD)
cd cli && make build && make test && make lint              # equivalent
ao gate check --fast --scope head                           # routine pre-push gate
make local-ci                                               # full release-grade local CI
bats tests/cli/                                             # shell suites
```

Test-shape doctrine (`.claude/rules/go.md`): L2-first/L1-always, no coverage-padding, fixture fidelity (round-trip real persisted shapes), and `t.Cleanup` restoration of shared cobra globals — the push gate runs the full `-race -shuffle=on` suite as late backstop.

---

## Error Handling

- Wrap-with-context convention: `fmt.Errorf("doing X: %w", err)`; `errors.Is/As` throughout.
- Nine typed exit-error structs unwrapped in `Execute()` (`cli/cmd/ao/root.go:86-165`); commands set `SilenceErrors` and print their own verdicts so the exit code is machine-consumable.
- Fail-closed is doctrine for gates: e.g. `corpus.scan` exit 1 on any leak marker *or unreadable file* (`root.go:143-149`); `#trivial` pawl waiver checks the diff, not the message.

## Logging

- No logging framework: stdout for verdicts/reports (agent-consumable, `--json` on read-side commands per `ao capabilities`), stderr for genuine failures only, `VerbosePrintf` behind `-v` (`root.go:228-232`).

---

## Notes & Gotchas

1. **The gate validates the worktree, not the commit** — a partial `git add` can land a broken commit that passed 29/29 checks; verify the committed file set with `git show <sha> --stat` (memory: age-tlj6).
2. **`.githooks/` is a historical bd shim, not the live gate** — `core.hooksPath` points at `.git/hooks`, where `scripts/install-pre-push-gate.sh` chains `scripts/hooks/pre-push.local` (stated in `.githooks/pre-push:7-12`). The tracked pre-commit shim also mis-derives `REPO_ROOT` as `.git`, falsely blocking `cli/*.go` commits.
3. **Build tags split the binary**: `flywheel`/`legacy` tags (`cli/cmd/ao/buildtags*.go`) archive commands out of the spine build; skill validators that build spine-only break on archived-command refs — build with `-tags "flywheel legacy"` when validating.
4. **Generated vs curated**: `registry.json`, `cli/docs/COMMANDS.md`, domain maps are generated (`make regen-all`; `make regen-check` is the drift gate). `skills-codex/` is NOT regenerated — manual mirror + `scripts/regen-codex-hashes.sh --only <name>`.
5. **Tracker footguns**: always `BEADS_DIR="$(ao beads dir)" br …` (worktrees lack `_beads/`); never `git add _beads`; `bd`/Dolt is retired legacy; the `br` SQLite cache can go stale under concurrent writes — the JSONL is truth.
6. **Precedence rule** (`AGENTS.md`): executable + generated (`cli/**`, `scripts/**`) > declared contracts (`skills/**/SKILL.md`, `schemas/**`) > narrative `docs/**`; some older docs still carry pre-3.0 wording (hooks, `bd`, PR-per-change) — historical unless reconciled.
7. **Honesty posture is structural**: ADR-0011 demotes escape-corpus compounding and ADR-0004 the corpus moat to *unproven*; the proven product is the verification loop itself (0 escapes across 130 production verdicts).

---

*Generated 2026-07-01 by the codebase-report swarm worker (read-only) at commit `e4ec22c46e94a3c8116697e57b28860efdf2b839`.*
