# AgentOps — Technical Architecture Report

> Produced by the `codebase-report` skill (Standard/Deep mode), 2026-06-11.
> Onboarding/handoff grade. All paths relative to repo root `~/dev/agentops`.
> Source-of-truth precedence (per `CLAUDE.md`): executable code (`cli/**`, `scripts/**`, generated `cli/docs/COMMANDS.md`) > contracts (`skills/**/SKILL.md`, `schemas/**`) > narrative docs (`docs/**`).

---

## Executive Summary

**AgentOps** is a software-factory control plane for repo-native agent work: a portable **skills corpus** (166 skills) plus a **Go CLI (`ao`)** that compiles, compounds, and gates agent context across sessions, models, and harnesses. The product is the in-session operating loop (rpi → evolve → crank/swarm) and the `.agents/` knowledge corpus it compounds — explicitly **hookless** (ADR-0002) and **daemon-free** (ADR-0009); CI and the local pre-push gate are the enforcement surfaces.

**Key statistics:**

- ~382K lines of Go in `cli/` (~147K non-test, ~235K test) — heavily test-weighted by design
- **85 top-level `ao` commands** (generated reference: `cli/docs/COMMANDS.md`, 4,500+ lines)
- **166 skills** (`skills/*/SKILL.md`), 65 `.feature` acceptance files, schema-gated frontmatter
- **253 shell scripts** in `scripts/` (gates, generators, validators)
- **45 JSON schemas** in `schemas/` (versioned contracts: beads, handoffs, evals, provenance…)
- **7,691 Go test functions**, 124 bats files (1,029 `@test` cases), 117 shell test scripts
- Language: Go 1.26 (`cli/go.mod:3`), bash, Python (docs/lint tooling)
- Merge model: **push-to-main** since ag-qidx (2026-06-07); `scripts/pre-push-gate.sh` (2,210 lines) is the pre-merge wall, `validate.yml` (~103KB, 161 named steps) is the post-push backstop

---

## What It Is (one paragraph)

AgentOps 3.0 (`docs/3.0.md`) names the loop a software factory runs at every scale — turn declared intent into validated work, recycle the exhaust as context for the next turn — and runs it **in session**. Skills are the portable agent runtime; `rpi` is the inner loop (one research-plan-implement-validate cycle); `evolve` is the outer loop (N rpi cycles toward a goal); `crank`/`swarm` fan waves across worktrees. The `.agents/` corpus (66 top-level subdirs) is where context accumulates — "the moat." Out-of-session orchestration is deliberately delegated to an external substrate (NTM + MCP Agent Mail), not owned here.

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `cli/cmd/ao/main.go:10` (`func main` → `Execute()`) | Binary entry; `version` set via goreleaser ldflags (`main.go:8`) |
| Root command | `cli/cmd/ao/root.go:28` (`rootCmd`), `root.go:83` (`Execute`) | Cobra root; global flags, command groups, typed exit-code mapping |
| Per-command files | `cli/cmd/ao/*.go` (271 non-test files, ~625 entries incl. tests) | One file (+`_test.go`) per command surface, e.g. `gate.go`, `beads.go`, `forge`/`mine` batch files |
| Agent contract | `ao capabilities` / `ao robot-docs` | Machine-readable CLI contract for agents (`root.go:46-48` advertises it) |
| Skills runtime | `skills/*/SKILL.md` | Invoked by harnesses (Claude/Codex/Gemini) directly; frontmatter is the contract |
| Installer | `scripts/install.sh` | End-user install (`curl … install.sh`); also `homebrew-tap/` |
| Local gate | `scripts/pre-push-gate.sh:1` | Pre-push wall (15+ checks); Go orchestrator twin: `ao gate check` (`cli/cmd/ao/gate_check.go`) behind `AGENTOPS_GATE_GO=1` |
| CI | `.github/workflows/validate.yml` | Omnibus post-push validation (T0/T1/T2 tiers, all required) |
| Loop driver (legacy) | `cli/cmd/ao/rpi_*.go` (e.g. `rpi_loop_supervisor.go`, `rpi_phased_tmux.go`) | Load-bearing legacy lane — live, tested, but frozen: no new surface area (per `CLAUDE.md` "Legacy RPI lane") |
| Standalone tool | `bin/ralph` | Shell loop runner outside the Go CLI |

### Root command mechanics (`cli/cmd/ao/root.go`)

- Global flags: `--dry-run`, `--verbose`, `-o/--output {table,json,yaml}`, `--json`, `--config` (`root.go:143-147`)
- `PersistentPreRunE` (`root.go:52-79`): syncs `--config` → `AGENTOPS_CONFIG` env, sanitizes git worktree env (`internal/adapters/worktreeconfig`), builds the `App` struct and injects it into command context
- `Execute()` (`root.go:83-129`) maps typed errors to exit codes: `AgentsLintError`, `doctorExitError` (exit 1 = findings, not failure), `gateExitError` (exit code IS the verdict), `beadsExitError`, `tickExitError`
- Command groups for help: start / core / workflow / config / comms / knowledge (`root.go:133-140`)
- `--json` on a parent emits machine-readable subcommand listing instead of help (`root.go:158-164`)

### Top-level `ao` commands (85)

`agent agents anti-patterns autodev badge beads canon capabilities chaos-test ci citation claim close codex compile completion config constraint contradict corpus council-gate cron curate dedup defrag demo doctor eval extract feedback-loop findings flywheel forge gate goals guard-status handoff harness harvest help init inject install-guards knowledge lookup loop maturity mcp memory metrics mind mine next-work notebook operator orchestrate patterns pool provenance quick-start ratchet ready reconcile redact refinery registry retrieval-bench robot-docs rpi scenario scope search seed session sessions skills status tick trace turn validate verdict-gate version vibe-check wiki`

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `App` | `cli/cmd/ao/app.go:16` | Shared app state replacing globals (Terraform Meta + kubectl Options hybrid): flag values + DI seams (`ExecCommand`, `LookPath`, `RandReader`, `Stdout/Stderr`) for testing. Retrieved via `AppFromContext` (`app.go:50`) |
| `Config` | `cli/internal/config/config.go:20` | Full runtime config: `Output`, `BaseDir` (default `.agents/ao`), plus nested `Forge`, `Search`, `Paths`, `RPI`, `Flywheel`, `Models`, `Dream`, `Compile` configs |
| `Candidate` | `cli/internal/types/types.go:107` | A mined learning candidate flowing through the knowledge flywheel; scored via `RubricScores`/`Scoring` (`types.go:222,240`), human-reviewed via `HumanReview` (`types.go:275`) |
| `PoolEntry` | `cli/internal/types/types.go:293` | Candidate's residence in the promotion pool; tier semantics (bronze 0.50–0.69 requires human gate — `cli/cmd/ao/gate.go:27-30`; silver bulk-approve — `internal/pool/pool.go:838`) |
| `TranscriptMessage` / `ToolCall` | `cli/internal/types/types.go:13,37` | Parsed session-transcript units — raw input to forge/mine |
| `Learning` | `cli/internal/search/types.go:6`, `cli/internal/compile/compile.go:125` | The promoted knowledge artifact, searched (`ao search`) and compiled into injectable context (`ao inject` / `ao compile`) |
| `Bead` | `cli/internal/evolve/ladder/ladder.go:33` | Work item (from the external `bd` tracker) inside evolve's work-selection ladder |
| `MemRLPolicyContract` / `MemRLPolicyDecision` | `cli/internal/types/memrl_policy.go:181,202` | Memory-RL policy contract — feedback-driven promote/demote/rollback for corpus entries |
| `CitationEvent` | `cli/internal/types/types.go:617` | Citation tracking — evidence that corpus entries actually get used (feeds decay/promotion) |

The deeper domain split lives in `cli/internal/` (≈70 packages), notable clusters: `corpus`, `compile`, `forge`, `mine`, `harvest`, `pool`, `knowledge`, `search` (BC1 Corpus); `rpi`, `evolve`, `orchestration`, `turnstate` (BC3 Loop); `gates`, `quality`, `liveness`, `scenario*`, `eval*` (validation); `provenance*`, `verdictledger`, `drwitness` (evidence/provenance); `adapters`, `ports`, `bridge` (hexagonal shell).

---

## Architecture Model (5 Bounded Contexts + hexagonal seams)

Declared in `skills/domain/references/*.md` and generated to `docs/contracts/context-map.md` (from SKILL.md frontmatter; CI gate `validate-context-map-drift` keeps it honest).

- **BC1 Corpus** — context in/out: `ao inject`, `ao compile`, `ao maturity`; durable intent sources (GOALS.md, autodev, ADRs) (`skills/domain/references/context-compiler.md:30`, `autodev.md:22`)
- **BC2–BC5** — ascending from corpus through Loop (BC3: rpi/evolve work-selection — `evolve.md:22`, `rpi.md:32`) to Runtime. The load-bearing seam: orchestration (when/where/who-supervises) is **substrate-owned**, the loop and its context (what the agent does, how context compounds) is AgentOps domain (`factory.md:26`)
- Every skill declares `hexagonal_role`, `consumes`, `produces`, `context_rel` in frontmatter (e.g. `skills/forge/SKILL.md:7-11`); these are the generated context-map's input — **never hand-edit the inventory maps**
- Doctrine altitudes: north star `docs/3.0.md` → spine `docs/architecture/operating-loop.md` (the 7-move loop: BDD intent → bead → vertical slices → TDD per slice → conflict-free wave → integrated completion → evidence/learning ratchet) → executor `/rpi`

---

## Data Flow

### 1. The knowledge flywheel (the core product loop)

```
Session transcripts / repo exhaust
     │
     ▼
ao forge / ao mine / ao harvest          (cli/cmd/ao/batch_forge.go, mine)
     │  parse → TranscriptMessage (internal/types/types.go:13)
     ▼
Candidate (types.go:107) ── scored via RubricScores/Scoring
     │
     ▼
Pool (internal/pool/pool.go) ── tiers:
     │   bronze (0.50–0.69) → human gate: ao gate approve/reject (cmd/ao/gate.go)
     │   silver → bulk-approve after age threshold (pool.go:838)
     ▼
Promoted Learning (.agents/learnings, search/types.go:6)
     │
     ▼
ao search / ao lookup / ao inject / ao compile ── decay-ranked context
     │
     ▼
Next session's prompt context  ──► loop compounds
```

Feedback edges: `CitationEvent` (usage evidence) + MemRL policy (`types/memrl_policy.go`) drive promote/demote/rollback; `ao flywheel` reports loop health.

### 2. The change-delivery flow (SDLC control plane)

```
Intent (BDD .feature / bead ## Scenarios)
  → bd claim (external bd tracker; ao beads shells out — cmd/ao/beads.go:63)
  → bd worktree create (worktree-mandatory; shared checkout is contended)
  → implement + TDD
  → scripts/pre-push-gate.sh --fast  (local wall; Go twin: ao gate check)
  → push to main (no PRs since ag-qidx; rebase-on-reject)
  → validate.yml on main (post-push backstop)
  → provenance ledger append (docs/provenance/ledger.jsonl, schema agentops-sdlc-provenance.v1)
```

### 3. Command dispatch (any `ao` invocation)

User/agent invokes `ao <cmd>` → `main.go` → `rootCmd.ExecuteC()` → `PersistentPreRunE` builds `App` (DI seams) into context → subcommand reads `AppFromContext` + `internal/config.Config` (flag > env > project > home > default) → domain package in `cli/internal/<pkg>` does the work → output via `-o table|json|yaml`; errors map to typed exit codes in `root.go:83-129`.

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| `bd` (beads CLI, external binary) | Issue tracking; `ao beads` shells out via `exec.Command("bd", …)` (`cli/cmd/ao/beads.go:63`, `LookPath` check `:70`) | Yes (workflow) |
| `git` | Worktrees, provenance, all SDLC mechanics; env sanitized in `internal/adapters/worktreeconfig` | Yes |
| `codex` CLI | Headless agent runtime for `ao compile`/loops (`CompileConfig.PreferredRuntime`, `config.go:56-60`); LAW 0 forbids `claude -p` | Yes (for headless lanes) |
| `tmux` | RPI phased/legacy lane pane control (`AGENTOPS_RPI_TMUX_COMMAND`, `config.go:477`) | Lane-only |
| Ollama / local LLM | Dream curator + compile alternatives (`AGENTOPS_DREAM_CURATOR_OLLAMA_URL`, `config.go:489`) | No |
| spf13/cobra + pflag | CLI framework (`cli/go.mod:11-12`) | Yes |
| gopkg.in/yaml.v3, BurntSushi/toml | Config + frontmatter parsing | Yes |
| pgregory.net/rapid, goleak, go-cmp | Property tests, leak detection, diffing — test-only | Test infra |
| Dolt (via bd) | bd's backing store on the operator fleet; **not** linked by ao itself | No (indirect) |
| NTM + MCP Agent Mail | Live multi-agent substrate — deliberately external (ADR-0009); AgentOps delegates out-of-session orchestration | External by design |

Notably lean: 8 direct Go deps total — the CLI is stdlib-heavy and shells out to its ecosystem rather than linking it.

---

## Configuration

Precedence (`cli/internal/config/config.go:1-7`), highest first:

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flags | `--output json`, `--config /path` (synced to env at `root.go:194-200`) |
| 2 | Env vars `AGENTOPS_*` | `AGENTOPS_OUTPUT`, `AGENTOPS_BASE_DIR`, `AGENTOPS_RPI_RUNTIME`, `AGENTOPS_MODEL_TIER`, `AGENTOPS_DREAM_*`, `AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD` (~30 vars wired at `config.go:460-490`) |
| 3 | Project config | `.agentops/config.yaml` in cwd |
| 4 | Home config | `~/.agentops/config.yaml` |
| 5 | Defaults | Hardcoded (e.g. `BaseDir` = `.agents/ao`) |

Config sections: `Forge`, `Search`, `Paths` (artifact locations are configurable, not hardcoded), `RPI` (worktree/runtime/command overrides), `Flywheel` (auto-promote threshold), `Models` (tier), `Dream` (overnight runs + local curator engine), `Compile` (headless runtime preference: codex-cli vs Ollama). Override file path: `AGENTOPS_CONFIG` (`config.go:400`).

Behavioral toggles outside config.yaml: `AGENTOPS_GATE_GO=1` (Go gate orchestrator instead of shell), `AGENTOPS_HOOKS_DISABLED=1` (operator-fleet hook bypass; AgentOps itself ships no hooks).

---

## Module Structure

```
agentops/
├── cli/                  Go CLI (module github.com/boshu2/agentops/cli, Go 1.26)
│   ├── cmd/ao/           271 command files + tests (1 file per command surface)
│   ├── internal/         ~70 domain packages (corpus, compile, forge, pool, rpi,
│   │                     evolve, gates, provenance, adapters, ports, …)
│   └── docs/COMMANDS.md  GENERATED command reference (scripts/generate-cli-reference.sh)
├── skills/               166 skills — SOURCE OF TRUTH (installed copies symlink here)
│   └── <name>/SKILL.md   + references/, scripts/, subagents/, *.feature acceptance
├── skills-codex*/        Codex-runtime skill variants + overrides (manually maintained parity)
├── scripts/              253 shell scripts: pre-push-gate.sh, regen-all.sh, validators
├── schemas/              45 versioned JSON schemas (beads, handoff, eval, provenance, …)
├── lib/                  Shared shell helpers (ao-paths.sh, bats-common.bash) + lib/schemas
├── tests/                bats (124 files), integration, e2e, canaries, install, windows
├── docs/                 MkDocs site: 3.0.md (north star), architecture/, adr/, contracts/
│   └── provenance/ledger.jsonl   Append-only SDLC provenance (ledger wins over bd metadata)
├── .agents/              The knowledge corpus (66 subdirs: learnings, plans, council, …)
├── .github/workflows/    validate.yml (omnibus), nightly.yml, release.yml, install-e2e.yml
├── plugins/ .claude-plugin/ .codex-plugin/ .agy-plugin/   Harness plugin manifests
├── evals/ evidence/ manifests/ registry.json              Eval suites, run evidence, catalogs
└── bin/ralph             Standalone shell loop runner
```

---

## Test Infrastructure

| Type | Location | Count | Runner |
|------|----------|-------|--------|
| Go unit/integration | `cli/**/*_test.go` | 7,691 `Test*` funcs (~235K LOC) | `cd cli && go test ./...` (race + coverage in CI; cmd/ao coverage floor enforced) |
| bats (shell) | `tests/**/*.bats` | 124 files / 1,029 `@test` | `bats tests/scripts/<file>.bats` |
| Shell suites | `tests/**/*.sh` (117) incl. `run-all.sh`, `smoke-test.sh`, `release-smoke-test.sh` | — | per-suite |
| Contract canaries | `tests/canaries/` | — | CI "official AgentOps contract canaries" |
| Acceptance (Gherkin) | `skills/*/references/*.feature` (65) + bead `## Scenarios` | — | `check-scenario-test-linkage.sh` gate |
| E2E / install | `tests/e2e/`, `tests/install/`, `Dockerfile.e2e`, `install-e2e.yml` | — | CI |
| Quarantine | `tests/_quarantine/` | must stay EMPTY (CI gate, directive D3) | — |

**Test doctrine** (`.claude/rules/go.md`): L2 integration first, L1 always; no coverage-padding; exact-value assertions; fixture fidelity (round-trip real persisted shapes); test-count non-regression ratchet (`scripts/check-test-count-regression.sh`) and test-isolation ratchet in CI.

**CI tiers:** T0 (≤30s) / T1 (≤5min) / T2 (≤15min) — *all required*, none advisory; I0 = informational artifacts only. `validate.yml` is ~103KB with 161 named steps spanning: Go build/vet/race/coverage, skill frontmatter schema validation (v1+v2), skill-body CLI-ref validation against the live binary, six-surface derived-artifact drift sweep (`regen-all.sh --check`), context-map drift, Codex parity, secrets scan, ShellCheck, markdownlint, security toolchain, flywheel-proof + goals gates, provenance orphan gates, holdout-leak gate.

---

## Error Handling & Output

- Cobra-standard `error` returns; **typed exit-code errors** are the idiom: the exit code IS the verdict for gates/doctor/beads verify (`root.go:90-125`) — exit 1 = findings (silent), exit 2 = internal error (stderr)
- Go conventions: wrap with `fmt.Errorf("doing X: %w", err)`, `errors.Is`; complexity budget warn 15 / fail 25 (`.claude/rules/go.md`)
- Every read-side command supports `--json`; `ao capabilities` is the machine contract; flag typos get suggestion hints (`root.go:153`)

---

## Notes & Gotchas (load-bearing)

1. **Generated artifacts are everywhere — never hand-edit:** `cli/docs/COMMANDS.md`, `docs/contracts/context-map.md`, `registry.json` (190KB), badge/manifest surfaces. `scripts/regen-all.sh` regenerates; CI diffs them.
2. **Skill SOT is `skills/` here**, but on the operator fleet `~/.claude/skills` etc. *symlink into this checkout* — edits in place are live across three runtimes; never `cp` into the symlink (`CLAUDE.md`).
3. **Push-to-main supersedes the PR model AND the local-gate-retirement ADR** (reversed): `pre-push-gate.sh` is load-bearing again. Go-native gate (`ao gate check`) exists but is opt-in (~12/79 check parity at last report) — the 2,210-line shell gate is still the default. Don't confuse `ao gate` (human review of pool candidates) with `ao gate check` (push gate orchestrator) — same command tree, different domains.
4. **Legacy RPI lane** (`rpi_loop_supervisor.go`, `rpi_phased_tmux.go`, `rpi_parallel.go`, `rpi_c2_events.go`): live, tested, referenced by 13+ files — extend tests when touched, but **no new features and no deletion** without caller migration (soc-1gbpz). The Gas City bridge was fully removed; `runtime=gc` is not a valid mode.
5. **Hookless is doctrine** (ADR-0002): nothing auto-injects at session start. `ao session bootstrap` + `ao inject` are the explicit replacements. A "hooks-runtime drift gate" in CI keeps live-facing docs hookless.
6. **The shared checkout is contended** (hot repo, hundreds of commits/week): all edits go through `bd worktree create`; foreign uncommitted files are quarantined, not adopted. In-flight worktrees (`wt-ag-*`) at root are normal.
7. **Provenance ledger wins:** `docs/provenance/ledger.jsonl` (append-only) beats `bd` metadata on disagreement.
8. **No bead, no change to main** — every change cites a bead; acceptance must be Gherkin (free-text acceptance is invalid).
9. **Codex parity is manually maintained** (`skills-codex/`, overrides) — pre-push checks 9–10 are deliberately skipped; CI has its own Codex drift gates.
10. Two `Learning` types exist (`internal/search/types.go:6` and `internal/compile/compile.go:125`) — package-local views of the same artifact, not one shared struct; check which one you're holding.

---

## Onboarding Pointers (read in this order)

1. `docs/newcomer-guide.md` → `docs/index.md` / `docs/documentation-index.md` → `README.md`
2. Doctrine: `docs/3.0.md` (north star) → `docs/architecture/operating-loop.md` (7-move spine) → `docs/cdlc.md`
3. Workflow law: root `CLAUDE.md` / `AGENTS.md` + tiered `AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md`
4. Build: `cd cli && make build && make test`; gate: `scripts/pre-push-gate.sh --fast`
5. Agent-facing contract: `ao capabilities`, `ao robot-docs`, `cli/docs/COMMANDS.md`

---

*Generated: 2026-06-11 by codebase-report swarm worker (read-only audit; no source files modified).*
