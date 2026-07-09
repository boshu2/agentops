# AgentOps — Technical Architecture Report (2026-07-02)

> **Skill:** `codebase-report` · **Run:** 2026-07-02 · Mode: Deep Dive
> Reference document derived from the parallel-agent mental model in `codebase-archaeology.md`. Persist-and-reference: this survives context compaction. When it disagrees with `cli/**` / generated docs, executable behavior wins.

---

## Executive summary

**AgentOps** is an in-session, hookless **verification membrane for coding agents**: it drives a stochastic agent from a goal to a *verified* done and refuses to call a change done until an independent verifier (a different model family, or a real test) proves it and the verdict is bound into a hash-chained provenance ledger — **no verdict = not done**. It ships as skills + the `ao` Go CLI + a repo-local `.agents/` corpus, running on Claude Code / Codex / Cursor / OpenCode. It ships **no daemon** (ADR-0009); out-of-session work is delegated to an external substrate (NTM + MCP Agent Mail + `ao agent`).

**Stats:** ~392K Go LOC (~1,300 files, Go 1.26, cobra) · ~112K shell LOC (~280 scripts) · 64 skills · ~646 command files / ~245 command structs · ~80 internal packages · ~90 gate checks · 682 Go test + 236 Bats files · 6 DDD bounded contexts · 1,178 commits in 30 days.

**Dependency profile:** deliberately minimal — no web framework, no ORM, no cloud SDK. The heaviest "dependency" is *other coding-agent runtimes and their subprocesses*.

---

## Entry points

| Entry | Location | Purpose |
|-------|----------|---------|
| `main()` | `cli/cmd/ao/main.go:10` | calls `Execute()`; `version` injected via ldflags |
| `rootCmd` | `cli/cmd/ao/root.go:28` | the one cobra root (`Use: "ao"`); 7 command groups, 5 global flags |
| `Execute()` | `cli/cmd/ao/root.go:83` | **typed-error → exit-code dispatch** — unwraps sentinel errors so exit code carries the verdict |
| per-command `init()` | ~245 files in `cli/cmd/ao/` | decentralized self-registration via `rootCmd.AddCommand` (343 calls) |
| `ao session bootstrap` | `cli/cmd/ao/session_bootstrap.go:136` | universal agent init prompt (replaces hook injection) |
| `ao gate check` | `cli/cmd/ao/gate_check.go:114` | validation gate = routine release authority |
| `ao pawl review` | `cli/cmd/ao/pawl.go:40` | cross-family refuter; exit code IS the verdict |
| Skills | `skills/<slug>/SKILL.md` | invocable capability contracts (the primary UX for agents) |
| Gate scripts | `scripts/check-*.sh` (~115) | shell-backed check implementations |
| CI | `.github/workflows/validate.yml` | backstop (~10 purpose-grouped jobs), not routine authority |

**No HTTP server, no long-running process in the product path.** The only server surface is `ao mcp serve` (JSON-RPC tool surface for hosted Claude loops), which is opt-in and Claude-only.

---

## Key domain types

| Type | Location | Purpose |
|------|----------|---------|
| `types.Candidate` | `cli/internal/types/types.go:174` | central knowledge artifact (learning/pattern/finding) — Tier, Utility (MemRL Q-value), Maturity, Confidence, supersession chain, expiry |
| `types.CitationEvent` | `cli/internal/types/types.go:684` | "artifact was used" reward signal — the flywheel's feedback edge; persisted by `ratchet` |
| `types.Scoring` / `RubricScores` | `cli/internal/types/types.go:307` / `:289` | multi-dimension scoring → assigns Bronze/Silver/Gold tier |
| `pool.PoolEntry` | `cli/internal/types/types.go:360` | candidate awaiting human review (age, urgency); unit `ao gate` (human) operates on |
| `gates.Check` | `cli/internal/gates/gates.go:57` | one declarative validation check: `ID, Tiers, Match[], Blocking, Backing XOR Run` |
| `provenancegraph.Edge` | `cli/internal/provenancegraph/edge.go:74` | hash-chained ledger edge: PROV-O relation, trust tier, verdict enrichment, `prev_hash/payload_hash/hash` |
| `turnstate` transition | `cli/internal/turnstate/turnstate.go` | append-only hash-chained lifecycle log; artifact `state = Fold(log)` |
| `config.Config` / `PathsConfig` | `cli/internal/config/config.go:20` / `:103` | resolved runtime config (output default, base dir, forge/search/paths/rpi/flywheel/models/dream sub-configs) |
| `NextWorkEntry` / `NextWorkItem` | `cli/internal/rpi/types.go:49` | `.agents/rpi/next-work.jsonl` claim/consume ledger (per-item consumed markers) |

---

## Component / bounded-context map

| BC | Name | Key packages | Command surface |
|----|------|--------------|-----------------|
| BC1 | Corpus/Knowledge | `search` (retrieval+decay), `pool`, `taxonomy`, `ratchet`, `wiki`, `llmwiki`, `harvest`, `mine`, `forge`, `provenance`, `provenancegraph`, `drwitness`, `drrebuild` | `ao inject`/`lookup`/`corpus`/`forge`/`pool`/`compile`/`provenance` |
| BC2 | Validation | `gates`, `verdictledger`, `vibecheck`, `quality`, `safety`, `refinery` | `ao gate check`/`vibecheck`/`doctor`/`metrics` |
| BC3 | Loop | `rpi`, `turnstate`, `evidencedturn`, `lifecycle`, `goals`, `goalsfitness`, `goalstrace`, `evolve`, `autodev`, `governor`, `posture` | `ao goals`/`ao done`/`ao governor` (loop CLIs legacy-gated) |
| BC4 | Factory | `skills`, `skillshealth`, `skillsresolve`, `canon`, `taxonomy` | `ao skills`/`ao capabilities`/`ao robot-docs` |
| BC5 | Runtime | `cmd/ao/`, `config`, `paths`, `adapters`, `ports` | the whole CLI + installers + plugin manifests |
| BC6 | Orchestration | `orchestration`, `agentworker`, `pool` (worker), `bridge`, `forge` (bundle) | `ao mcp serve`/`ao agent` (`ao orchestrate` legacy) |

Contract: [`docs/contracts/bounded-contexts.yaml`](../../contracts/bounded-contexts.yaml). Routing rule: [`component-map.md`](../../architecture/component-map.md).

---

## Data flow (end to end)

```
goal + acceptance            [Move 1: /discovery → BDD Given/When/Then]
      │
      ▼
br bead (_beads/ ledger)     [ao beads dir → git-common-dir resolves worktrees]
      │
      ▼
worktree slice + TDD         [Moves 3–4: failing test → green]
      │
      ▼
/validate → /pre-land-refuters
      │
      ▼
pawl.sh cross-family refuter → pawl-verdict.v1.json   ──not CONFIRMED──▶ HOLD (not done)
      │ CONFIRMED
      ▼
ao provenance emit-verdict → hash-chained Edge in docs/provenance/ledger.jsonl
      │ (auto-bind: chore(provenance): bind pawl CONFIRMED verdict … #trivial)
      ▼
ao gate check --fast --scope head    [local cockpit = release authority]
      │ PASS
      ▼
git push → main                       [branch protection off; validate.yml = backstop]
      │
      ▼
ao forge → pool (5-dim) → ratchet → .agents/ learnings → ao lookup (decay-ranked) next session
      │
      ▼
ao governor budget   [mines verdict/yield ledger for escapes; exit 3 = harden the line]
```

**Two axes** (from GOALS.md): the *work axis* (per artifact: evidence out → verdict back through the ledger) and the *recurrence axis* (per pattern: mine accumulated evidence → propose apparatus change → ratify through the same pawl).

---

## The verification membrane in code (the product)

Four independent, mechanical "no"s make "not done" an *absence*, not a slogan:

| Mechanism | Location | "No" when… |
|-----------|----------|-----------|
| Verdict ledger | `cli/internal/verdictledger/loader.go:22` | missing ledger record returns empty (literally "no verdict yet") |
| Constraint gate | `cli/internal/gates/checks/constraints.go` | fail-**closed**: malformed index / unreadable file / unknown detector all FAIL |
| Pawl verdict | `docs/contracts/pawls.md:171`, `internal/planpawl/decide.go` | timed-out/crashed/unreadable review = *no verdict*, never CONFIRM; quorum < 2 families → REDO |
| Provenance hash-chain | `cli/internal/provenancegraph/` + `provenance.chain` gate (`seed.go:314`) | broken hash chain fails the pre-push authority boundary |
| Evidenced-turn DoD | `cli/internal/evidencedturn/evidencedturn.go:88` | any of 7 predicates fails (incl. `author ≠ validator`) |

---

## External dependencies

### Go module deps (`cli/go.mod`)

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| `spf13/cobra` + `pflag` | CLI command tree, flags | Yes — the spine |
| `santhosh-tekuri/jsonschema/v6` | schema validation of packets/verdicts/config | Yes — contract enforcement |
| `BurntSushi/toml`, `gopkg.in/yaml.v3` | config + frontmatter/contract parsing | Yes |
| `google/go-cmp` | test diffs, golden comparisons | Test-only |
| `pgregory.net/rapid` | property-based testing | Test-only |
| `go.uber.org/goleak` | goroutine-leak detection in tests | Test-only |
| `golang.org/x/text` | text normalization | Minor |

**No** database driver, web framework, cloud SDK, or LLM client library in `go.mod`. This is intentional: AgentOps is a *control plane over subprocesses*, not a service.

### Runtime / external process dependencies

| Dependency | How used | Required? |
|------------|----------|-----------|
| `git` | worktrees, changed-file routing, provenance, beads-dir resolution | **Hard requirement** |
| An agent runtime | Claude Code / Codex / Cursor / OpenCode — runs the skills | **Hard requirement** |
| `br` (beads_rust) | issue tracker (shelled from `internal` via `BEADS_DIR`) | Recommended |
| `codex` CLI | cross-family refuter in the pawl (LAW 0: never same-model) | For pawl `multi`/`fresh` tier |
| `ntm` | out-of-session tmux swarm substrate (probed by capability) | Optional (BC6) |
| MCP Agent Mail | multi-agent file locks / messaging | Optional (BC6) |
| `bd` / Dolt | **retired legacy** — preserved, not authoritative | No |

Full matrix: [`docs/dependencies.md`](../../dependencies.md). Everything except `git` + a runtime degrades gracefully.

---

## Configuration

Precedence (later overrides earlier):

| Source | Priority | Example |
|--------|----------|---------|
| Compiled defaults | lowest | `DefaultInjectMaxTokens=1500`, gate tolerances, tier thresholds |
| Config file | ↑ | `AGENTOPS_CONFIG` path → TOML/YAML `config.Config` (`config.Load`) |
| Env vars (`AGENTOPS_*`, `AO_*`) | ↑ | see table below |
| CLI flags | highest | `--dry-run`, `--verbose/-v`, `--output/-o`, `--json`, `--config`, plus per-command flags |

**Notable env vars** (~40 total, `grep AGENTOPS_`): `AGENTOPS_CONFIG`, `AGENTOPS_BASE_DIR`, `AGENTOPS_REPO_ROOT`, `AGENTOPS_ACTOR[_EMAIL]`, `AGENTOPS_GATE_BASH=1` (legacy bash gate escape hatch), `AGENTOPS_ORCHESTRATION` (backend pin), `AGENTOPS_MODEL_TIER`/`COUNCIL_MODEL_TIER`, `AGENTOPS_HOLDOUT_EVALUATOR` (eval lockdown), the `AGENTOPS_DREAM_*` family (out-of-session curator), `AGENTOPS_LEGACY`, `AGENTOPS_HOOKS_DISABLED`. `NO_COLOR` and `AO_DOCTOR_LOG_LEVEL` also honored (published in `ao capabilities`).

**Machine-readable config contract:** `ao capabilities` (always-JSON) publishes the exit-code dictionary, env-var dictionary, and `robot_surfaces` map — generated by walking the live command tree so it can't drift.

---

## Schemas (contracts, `schemas/`)

~45 versioned JSON schemas enforce the typed boundaries. Load-bearing ones:

| Schema | Guards |
|--------|--------|
| `agentops-sdlc-provenance.v1` | ledger edge shape (hash-chained) |
| `pawl-verdict.v1` | cross-family verdict artifact |
| `verdict.v1` / `evidence-only-closure.v1` | validation output + closeout |
| `bead.v1`, `next-work-item.v1`, `next-work-batch.v1` | tracker + work queue |
| `learning.v1`, `finding`, `claim-registry.v1` | corpus artifacts |
| `domain-slice-manifest.v1` | ADR-0013 slice contract |
| `execution-packet`, `phase.v1`, `quest.v1` | loop packets |
| `orchestration-{backend,result,instrument}.v1` | BC6 substrate output-contract parity |
| `session-bootstrap.v1` | init-prompt status shape |

Schema validation runs both at write time (Go) and as gate checks.

---

## Test infrastructure

| Type | Location | Count / notes |
|------|----------|---------------|
| Go unit + integration | `cli/**/*_test.go` | 682 files; `rapid` property tests, `goleak` leak checks, table-driven |
| Bats (gate/integration/e2e) | `tests/**/*.bats` | 236 files |
| Gate registry parity | `cli/internal/gates/checks` | `go test ./internal/gates/checks -count=1` proves Fast/Full parity |
| Runtime smoke (multi-runtime) | `tests/skills/test-runtime-{claude-code,codex,cursor,opencode}-smoke.sh` | Tier S — must stay green in CI |
| Install smoke | `tests/install/test-install-smoke.sh` | gates all install scripts |
| E2E / scenarios | `tests/e2e/`, `tests/scenarios/`, `tests/integration/` | Dockerfile.e2e present |
| Quarantine | `tests/_quarantine/` | **kept empty by directive #3** — no hidden regression risk |
| Workflow-tool tests | `.claude/workflows/*.js` | 4 workflows, `exempt` parity policy |

**Test conventions** (`.claude/rules/go.md`): L2-integration-first, no coverage-padding, exact-value assertions, guard-test fixtures must round-trip the real persisted shape (ag-mjlg), shared-global state restored via `t.Cleanup` (recurring `-shuffle=on` flake class).

---

## Build / regen / release

| Command | Purpose |
|---------|---------|
| `cd cli && make build && make test && make lint` | verify Go before commit |
| `make regen-all` | regenerate registries/maps/codex-hashes from sources (dependency-ordered) |
| `make regen-check` | drift gate (13 no-write validators) — CI runs this |
| `ao gate check --fast --scope head` | routine pre-push release authority |
| `ao gate check --full --workflow-coverage` | CI-parity proof (shadow) |

Release model: **push-to-main with a local authoritative gate**. Branch protection is off; the local pre-push Go gate is the release authority; `validate.yml` is a tag/PR/manual backstop.

---

## Notes & gotchas

- **`internal/` names lie:** `corpus` = classifier (not retrieval), `knowledge` = render command, `refinery` = CI backstop. Retrieval lives in `internal/search`.
- **Two "gate"s:** `ao gate` (human promotion review) vs `ao gate check` (validation). Two "provenance"s: `internal/provenance` (tracker/projection) vs `internal/provenancegraph` (the authoritative hash-chained ledger).
- **Deleted, not deprecated:** the RPI engine, daemon, `ao rpi`/`ao evolve`/`ao loop`/`ao orchestrate` CLIs — many `//go:build legacy`. Grep for the command, not the package, to know what's live.
- **Codex bespoke twins are not auto-regenerated:** `make regen-all` refreshes only the hash record, not hand-authored prose in `skills-codex-overrides/`.
- **Never `git add _beads`:** it's a nested private repo; sync with `git -C "$(ao beads dir)" push`.
- **`ao beads dir` resolves via git-common-dir** so linked worktrees find the canonical `_beads`, not `$PWD/_beads`.
- **The hexagon is partial:** ports/adapters are retrofitted BC-by-BC; most commands still bind concrete packages.

---

## Related artifacts (this run)

- `codebase-archaeology.md` — the mental model / narrative
- `codebase-audit.md` — issues, risks, drift
- `codebase-pattern-extraction.md` — reusable patterns
- `SYNTHESIS.md` — cross-cutting conclusions
