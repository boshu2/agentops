# AgentOps — Comprehensive Technical Architecture Report

> **Date:** 2026-07-09
> **Mode:** Deep Dive (`codebase-report`)
> **Model:** GPT-5 / Codex runtime; exact deployment ID unavailable
> **Pinned base / inspected HEAD:** `fbba8af5ace635104775ef18f34fef362ba368ce`
> **Method:** independent code-first inspection; executable/generated surfaces first, contracts second, narrative last. Measured with `git ls-files`, `find`, `rg`, `jq`, `go list`, and the live default-build `ao capabilities` command. July 9 peer reports were excluded from the read set; the 2026-07-02 report was read only after the independent model was complete. No full repository gate or external-runtime execution was performed.

## Reading convention

- **Code fact** means directly supported by executable/generated source at the pinned SHA.
- **Interpretation** means an architectural conclusion drawn from multiple code facts.
- **Unknown** means it was not safely established by this bounded, read-only inspection.
- Source precedence is executable/generated > declared contracts > narrative, matching the repository rule at `AGENTS.md:64-71` and `docs/architecture/codebase-overview.md:110-115`.

## Executive summary

**Code fact.** AgentOps is a local, predominantly Go-and-shell control plane for independently validating coding-agent work. Its shipped unit is a hookless skills bundle plus the `ao` Cobra CLI; its durable proof substrate is an append-only, per-record hash-chained JSONL provenance ledger. The default binary is deliberately narrower than the source tree: ADR-0012 build tags omit archived `flywheel` and `legacy` command satellites, while `make build-flywheel` restores them (`cli/Makefile:11-20`, `cli/Makefile:38-50`).

**Interpretation.** The architecture is not a conventional service. It is a repo-local verification membrane around subprocesses and files: skills shape work, the CLI provides typed control surfaces, gates execute deterministic checks, pawl review binds an independent verdict, and landing records proof against Git history. Out-of-session scheduling and multi-agent supervision remain substrate concerns rather than an in-repo daemon.

### Measured scale

| Dimension | Fresh measurement | Measurement boundary / authority |
|---|---:|---|
| Tracked files | 5,163 | `git ls-files` |
| Go files / lines | 1,456 / 403,556 | tracked `*.go`, including tests and embedded/generated code |
| Non-test Go files / lines | 755 / 165,387 | tracked Go excluding `*_test.go` |
| Go test files / lines | 701 / 238,169 | tracked `*_test.go` |
| Go test/benchmark/fuzz functions | 7,585 | `rg '^func (Test|Benchmark|Fuzz)' cli -g '*_test.go'` |
| Go packages | 109 | `go list ./...` from `cli/` |
| `cli/cmd/ao` files | 669 total: 300 production, 369 tests | direct file count |
| Top-level internal packages | 82 | `cli/internal/*/` directories |
| Default-build top-level commands | 72 | live `ao capabilities`; agrees with `registry.json:3-11` |
| Generated command inventory | 257 command paths | `docs/cli-surface.json:1-16` and array length |
| Source skills / Codex twins | 59 / 58 | direct `SKILL.md` count; generated source count agrees with `registry.json:3-11` |
| Go gate registry | 112 checks | 101 `seed` literals at `cli/internal/gates/checks/seed.go:235-419` + 11 individual registrations |
| Shell files | 753 tracked; 363 under `scripts/` | tracked `*.sh`; canonical scripts subset |
| Bats files / tests | 256 / 2,212 under `tests/` | direct file and `@test` counts (283 Bats files repo-wide) |
| JSON schemas | 60 top-level + 4 fixtures | `schemas/` direct file count |
| Contract files | 72 | `docs/contracts/` direct file count, including 8 colocated JSON schemas |
| Eval JSON files / scenario specs | 70 / 15 | `evals/**/*.json`; `spec/scenarios/*.json` |
| GitHub workflows / Claude workflows | 12 / 4 | `.github/workflows`; `.claude/workflows` |
| Commits since the prior report date | 203 | after 2026-07-02 local midnight through pinned HEAD |

The generated registry summarizes 59 skills, 72 CLI commands, 4 workflows, and 145 catalog capabilities (`registry.json:2-11`). That is a different inventory from the 112-check runtime gate registry: the registry catalog's 14 “gates” are capability categories, not every `gates.Check` instance.

## System shape

```text
Agent runtime / operator
        │ invokes skills or ao
        ▼
Skills contracts ───────────────┐
        │                       │ context / intent
        ▼                       ▼
ao Cobra CLI ── ports ── domain packages ── adapters / subprocesses
        │                              │
        ├── gate registry ─────────────┤── scripts/check-*.sh, go vet, shellcheck
        │                              │
        ├── pawl review ───────────────┤── independent model/runtime evidence
        │                              │
        └── provenance graph ──────────┴── docs/provenance/ledger.jsonl
                                               │
                                               ▼
                                         guarded landing
```

The control plane is deliberately hybrid. Domain/port seams are typed Go, while many validators, installers, generation steps, and landing operations remain shell programs. There is no database or HTTP server in the core path; Git, JSON/YAML/Markdown, and local directories are the persistence layer.

## Entry points

| Entry | Location | Role |
|---|---|---|
| `ao` process main | `cli/cmd/ao/main.go:5-12` | Carries the build-injected version and delegates to `Execute()` |
| Cobra root | `cli/cmd/ao/root.go:27-78` | Defines `ao`, global flags, pre-run environment repair, and `App` injection |
| Process error/exit dispatcher | `cli/cmd/ao/root.go:80-183` | Maps typed errors to semantic process exit codes; verdict-bearing commands do not require stdout parsing |
| Decentralized command registration | `cli/cmd/ao/root.go:185-222` plus command-local `init()` functions | Builds the command tree; default live discovery is exposed by `ao capabilities` |
| Machine-readable CLI contract | `cli/cmd/ao/capabilities.go:18-40`, `cli/cmd/ao/capabilities.go:136-199` | Walks the live root command tree and emits stable JSON |
| Agent session front door | `cli/cmd/ao/root.go:35-43` | Documents `session bootstrap → lookup → operating loop → gate check` as the active waist |
| Validation entry | `cli/cmd/ao/gate_check.go:42-68`, `cli/cmd/ao/gate_check.go:114-170` | Selects and runs the declarative gate registry in Fast or Full mode |
| Trusted landing entry | `cli/cmd/ao/land.go:30-64`, `cli/cmd/ao/land.go:150-234` | Rebuilds/re-execs a trusted binary, obtains pawl review, runs atomic landing, then attempts post-land provenance |
| Provenance command family | `cli/cmd/ao/provenance_add.go:31-95` | Adds/lists/verifies/shows/emits ledger edges |
| MCP stdio server | `cli/cmd/ao/mcp_serve.go:11-49` | Opt-in JSON-RPC/MCP façade exposing six curated tools |
| Skill UX | `skills/<slug>/SKILL.md`; summary at `registry.json:3-11` | Primary agent-facing behavioral contracts; 59 source skills |
| Installer surfaces | `scripts/install.sh:1-27`, `scripts/install-codex.sh:1-16` | Runtime detection, hookless/default bundle installation, platform-specific packaging |
| Local build/gate/release | `Makefile:5-38`, `scripts/ci-local-release.sh:4-30` | Default make target is release-grade local CI; quick/fast/full modes |
| GitHub validation backstop | `.github/workflows/validate.yml:18-40` | Tags, manual dispatch, PRs, and merge queue; no routine branch push trigger |
| GitHub publisher | `.github/workflows/release.yml:1-28`, `.goreleaser.yml:4-20` | Tag/manual release, pre-publish evidence, cross-platform binaries |
| `skill-frontmatter-json` helper | `cli/cmd/skill-frontmatter-json/main.go:14-54` | Standalone YAML-frontmatter-to-JSON converter used by scripts/gates |
| `witness-crosscheck` helper | `cli/cmd/witness-crosscheck/main.go:1-19`, `cli/cmd/witness-crosscheck/main.go:29-66` | Standalone Dolt-projection versus committed-witness verifier; not an `ao` command |

## Command dispatch flow

```text
argv
  → main()
  → rootCmd.ExecuteC()
  → PersistentPreRunE
       --json normalizes output
       --config materializes AGENTOPS_CONFIG
       inherited Git env is sanitized
       shared worktree config is repaired
       App is injected into context
  → Cobra RunE handler
  → domain / port / adapter / subprocess
  → nil OR typed semantic error
  → Execute maps error to process exit code
```

1. `main()` is intentionally minimal (`cli/cmd/ao/main.go:11-12`).
2. The pre-run hook normalizes output, sanitizes Git process state, repairs worktree configuration, and injects resolved application state (`cli/cmd/ao/root.go:50-76`).
3. `App` centralizes global flag values and injectable process/IO seams (`cli/cmd/ao/app.go:12-42`).
4. Command handlers remain decentralized through package `init()` registration; `capabilities` walks the resulting live tree rather than maintaining a parallel list (`cli/cmd/ao/capabilities.go:136-168`).
5. `Execute()` recognizes command-specific typed errors—gate, pawl, land, doctor, pre-push, beads, governor—and directly propagates their verdict codes (`cli/cmd/ao/root.go:99-177`).

**Error model.** Normal negative outcomes are frequently values/exit codes, not exceptional crashes. A gate FAIL is exit 1, pawl REFUTED is exit 3, and plan-pawl BLOCKED is exit 4; the default-build typed exit contract is declared at `cli/cmd/ao/capabilities.go:73-99`.

## Key types and interfaces

| Type / interface | Location | Architectural purpose |
|---|---|---|
| `App` | `cli/cmd/ao/app.go:12-42` | Per-invocation state and dependency injection for command execution, path lookup, randomness, and IO |
| `config.Config` | `cli/internal/config/config.go:19-53` | Aggregate configuration for output, paths, RPI compatibility, models, dreams, search, forge, and compile |
| `packet.ExecutionPacket` | `cli/internal/domain/packet/aggregate.go:1-58` | Canonical rich work-packet aggregate; mirrors the schema and preserves legacy slim-packet compatibility |
| `gates.Check` | `cli/internal/gates/gates.go:53-86` | Declarative validation unit: identity, tier, routing globs, blocking semantics, and exactly one shell backing or native function |
| `ports.GateVerdict` / `GateRunnerPort` | `cli/internal/ports/gate_runner.go:11-35`, `cli/internal/ports/gate_runner.go:51-75` | BC2 terminal status/value contract and executable adapter seam |
| `gates.Report` | `cli/internal/gates/report.go:13-40` | Aggregates results and implements fail-closed exit semantics for blocking FAIL/UNKNOWN/evaluation errors |
| `provenancegraph.Edge` | `cli/internal/provenancegraph/edge.go:30-74`, `cli/internal/provenancegraph/edge.go:74-126` | Typed PROV-O relation plus trust, evidence, join keys, reviewer economics, and chain hashes |
| `provenancegraph.Store` | `cli/internal/provenancegraph/store.go:12-39`, `cli/internal/provenancegraph/store.go:97-159` | Append-only, idempotent, cross-process-locked ledger writer/reader |
| `ports.OrchestrationPort` | `cli/internal/ports/orchestration.go:28-69` | Backend selection seam and auditable degradation trace |
| `ports.CorpusReaderPort` | `cli/internal/ports/corpus_reader.go:5-49` | Narrow BC1 retrieval contract with ranking, limits, and optional decay semantics |

The execution packet is retained and schema-enforced even though `ao rpi` is absent from the default command path. Its decoder validates raw JSON before struct decoding, migrates the legacy slim shape, and revalidates the migration fail-closed (`cli/internal/domain/packet/invariants.go:29-87`).

## Module and bounded-context map

The canonical six-context contract is declared at `docs/contracts/bounded-contexts.yaml:21-125`; the table below maps that declaration onto current executable centers of gravity.

| BC | Responsibility | Current executable/module centers | Important ports |
|---|---|---|---|
| BC1 Corpus | Capture, retrieve, compile, cite, promote knowledge | `internal/search`, `corpus`, `forge`, `mine`, `pool`, `canon`, `provenancegraph`, `wiki`; `.agents/` adapters | `CorpusReaderPort`, `CorpusWriterPort`, `CitationPort`, `FindingCompilerPort` (`bounded-contexts.yaml:22-38`) |
| BC2 Validation | Judge plans, code, docs, dependencies, releases | `internal/gates`, `quality`, `safety`, `vibecheck`, `eval`; pawl scripts and verdict schemas | `GateRunnerPort`, `CIStatusPort`, evidence binders (`bounded-contexts.yaml:40-55`) |
| BC3 Loop | Select/execute work, measure fitness, converge | `internal/goals*`, `governor`, `lifecycle`, retained `rpi` compatibility code; `ao done`, beads commands | Loop reader/writer, hypothesis, convergence (`bounded-contexts.yaml:57-73`) |
| BC4 Factory | Build/govern skills and claims | `internal/skills*`, `claimpolicy`, `claimproof`, registries/generators, `skills/**` | catalog, scorer, admission, claim evidence (`bounded-contexts.yaml:75-89`) |
| BC5 Runtime | Adapt control plane to harness/shell/workspace | `cmd/ao`, `internal/adapters`, `paths`, `config`, installers, plugin manifests | Harness, operator, workspace, tracker, event bus (`bounded-contexts.yaml:91-108`) |
| BC6 Orchestration | Select/coordinate external multi-agent substrates | `internal/orchestration`, `agentworker`, MCP surface/transport, NTM/Agent Mail skills | orchestration and convergence seams (`bounded-contexts.yaml:110-125`) |

### Directory-level ownership

| Path | Role |
|---|---|
| `cli/cmd/ao/` | Cobra adapters and some still-inline application logic |
| `cli/internal/domain/` | Canonical domain aggregates; currently packet is the clearest explicit aggregate |
| `cli/internal/ports/` | 28 explicit production interfaces and their contract types |
| `cli/internal/adapters/` | 17 top-level adapter families for filesystem, tracker, Git workspace, CI, MCP, vendor images, sessions |
| `cli/internal/gates/` | Gate registry, routing, serial orchestration, reports, script runner |
| `cli/embedded/` | Generated/copied runtime payload embedded into the binary; refreshed by `make sync-hooks` (`cli/Makefile:25-36`) |
| `skills/` | Source skill contracts; `skills-codex/` is the checked-in Codex artifact and overrides hold deliberate divergence |
| `scripts/` | Validation, install, generation, release, landing, and compatibility programs |
| `schemas/`, `docs/contracts/` | Machine contracts and higher-level policy contracts |
| `tests/`, `evals/`, `spec/` | Bats/shell/Python integration, behavior/effectiveness evals, scenario specifications |
| `docs/provenance/ledger.jsonl` | Tracked audit authority; local runtime state under `.agents/` remains untracked |

**Interpretation.** Hexagonal adoption is meaningful but incomplete. Strong seams exist around gates, corpus, orchestration, storage, and workspace, yet the 300 production files in `cmd/ao` show that command adapters remain a major application-logic center.

## Gate verdict flow

```text
ao gate check [--fast|--full] [--scope ...]
  → resolve repo root + optional explicit range
  → Registry.All() (stable ID order)
  → Orchestrator.selectCheckPlans()
       tier filter
       fast: changed-file routing / invalidation
       full: every matching-tier check
  → serial runOne()
       native CheckFunc OR ScriptRunner via GateRunnerPort
  → CheckResult[] + SkippedCheck[]
  → Report
       human | JSON | GitHub annotations
       fail closed on blocking FAIL / UNKNOWN / evaluation error
  → exit 0 or 1
```

- The command explicitly distinguishes the registry-wide `gate check` surface from one-check `gate run` (`cli/cmd/ao/gate_check.go:17-24`).
- Fast is the default and accepts `head|staged|worktree|upstream|range:<base>..<head>` routing; Full ignores path routing (`cli/cmd/ao/gate_check.go:57-68`, `cli/internal/gates/orchestrator.go:129-159`).
- Checks register into a concurrency-safe registry that rejects duplicate IDs and malformed backing/run combinations (`cli/internal/gates/registry.go:9-34`, `cli/internal/gates/registry.go:65-90`).
- The orchestrator currently runs selected checks serially because scripts may share state (`cli/internal/gates/orchestrator.go:45-59`, `cli/internal/gates/orchestrator.go:62-105`).
- Report exit semantics are fail-closed for any blocking unknown, malformed status, execution error, or explicit FAIL (`cli/internal/gates/report.go:26-40`).
- JSON output includes selection reason, backing, artifact path, repair hint, bounded log tail, duration, skipped gates, and optional workflow coverage (`cli/internal/gates/report.go:74-169`).

The measured registry contains 112 checks. The large shell-backed seed is intentional and declarative: the 101 seed entries live in a flat slice and register as data (`cli/internal/gates/checks/seed.go:235-240`, `cli/internal/gates/checks/seed.go:416-419`), while native checks demonstrate the porting pattern (`cli/internal/gates/checks/native_inline.go:15-22`).

## Provenance and landing flow

### Proof data model

The authoritative record is `docs/provenance/ledger.jsonl`, not a database projection (`cli/internal/provenancegraph/edge.go:1-17`, `AGENTS-WORKFLOW.md:34-36`). Each edge is field-validated, sealed with:

```text
payload_hash = sha256(canonical payload JSON)
hash         = sha256(payload_hash + "\n" + prev_hash)
```

(`cli/internal/provenancegraph/edge.go:11-17`, `cli/internal/provenancegraph/edge.go:222-267`). Chain verification recomputes every link and hash (`cli/internal/provenancegraph/edge.go:282-307`). Store append holds a cross-process advisory lock across read → dedupe → seal → append, preventing concurrent forks (`cli/internal/provenancegraph/store.go:107-159`, `cli/internal/provenancegraph/store.go:162-199`).

### Land sequence

```text
ao land <bead>
  → resolve genuine AgentOps checkout
  → build cli/bin/ao from current source
  → re-exec under that binary
  → pin AO_BIN
  → best-effort warm pawl service
  → ao pawl review <bead> --scope head
       REFUTED / no verdict → non-zero, stop
       CONFIRMED → auto-bound verdict artifact/commit
  → scripts/pawl-land.sh
       fetch → rebase → restamp → push through pre-push gate
  → scripts/post-land-provenance-emit.sh (best effort in ao land)
       emit landed and verdict edges in disposable worktree
       commit #trivial provenance-only delta
       retry push races by re-emitting onto new chain tip
```

`ao land` fresh-builds and re-execs specifically so trust checks and child commands cannot accidentally use a stale installed binary (`cli/cmd/ao/land.go:73-103`, `cli/cmd/ao/land.go:150-195`). It refuses to call the land script until pawl review succeeds (`cli/cmd/ao/land.go:197-224`).

`emit-landed` derives bead→commit edges only from recognized commit-message forms, uses full commit OIDs, and may require ancestry on the trunk ref (`cli/cmd/ao/provenance_emit.go:86-107`, `cli/cmd/ao/provenance_emit.go:155-198`, `cli/cmd/ao/provenance_emit.go:237-255`). `emit-verdict` derives verdict→commit edges and reviewer/evidence/cost enrichment from the pawl artifact (`cli/cmd/ao/provenance_emit_verdict.go:124-156`, `cli/cmd/ao/provenance_emit_verdict.go:210-243`).

The post-land script isolates writes in a disposable detached worktree, re-emits rather than rebasing a hash chain, and retries push races (`scripts/post-land-provenance-emit.sh:9-25`, `scripts/post-land-provenance-emit.sh:139-160`, `scripts/post-land-provenance-emit.sh:188-211`).

## External dependencies and processes

### Go modules

The Go module is intentionally small (`cli/go.mod:1-22`):

| Dependency | Role | Runtime criticality |
|---|---|---|
| Cobra + pflag | CLI tree and flags | Core |
| `jsonschema/v6` | Embedded schema compilation/validation | Core contract enforcement |
| YAML v3 + BurntSushi TOML | Configuration, frontmatter, contracts | Core for affected commands |
| `x/text` | Text normalization | Supporting |
| `go-cmp`, `rapid`, `goleak` | deterministic comparisons, property testing, leak detection | Test-only |

There is no DB driver, web framework, ORM, cloud SDK, or agent-vendor SDK in `go.mod`.

### Runtime processes and systems

| Dependency | Use | Requirement |
|---|---|---|
| Agent runtime | Executes the skill contracts | One runtime required |
| Git | roots/worktrees, diffs, landing, provenance, private tracker coordination | Required |
| `ao` | typed local control plane | Strongly recommended / built from source here |
| `br` + `bv` | AgentOps repo tracking and graph triage | Required for tracked repo work |
| shell + core utilities | Hundreds of gates/install/release steps | Required for source/release workflows |
| Codex / other independent reviewer | Pawl/refuter proof | Required for selected review posture |
| NTM / Agent Mail / MCP-aware host | Out-of-session and multi-lane substrate | Optional |
| `gh` / GitHub Actions | CI query, external PRs, tag publishing | Optional for normal local landing; required for hosted publishing |
| `jq`, `rg`, shellcheck, markdownlint | JSON processing, search, validation | Workflow-dependent |
| gosec, gitleaks, trivy, semgrep, SBOM tools | Release/security evidence | Full release lane |

The canonical dependency matrix says only a runtime plus Git is genuinely required for baseline value and requires optional-tool degradation to be explicit (`docs/dependencies.md:3-6`, `docs/dependencies.md:9-30`, `docs/dependencies.md:32-37`).

## Configuration sources and precedence

The central loader's priority is:

```text
CLI flag overrides
    > AGENTOPS_* environment
    > .agentops/config.yaml in cwd
    > ~/.agentops/config.yaml
    > compiled defaults
```

This order is declared at `cli/internal/config/config.go:1-8` and implemented at `cli/internal/config/config.go:357-406`. Important details:

- Root flags are `--dry-run`, `--verbose`, `--output`, `--json`, and `--config` (`cli/cmd/ao/root.go:200-205`).
- `--config` is materialized as `AGENTOPS_CONFIG` during pre-run (`cli/cmd/ao/root.go:241-257`).
- An explicit config path is a replacement file over defaults, not an extra fifth layer: ambient home/project files are skipped, explicit read/parse failure is fatal, then env and flags still override (`cli/internal/config/config.go:362-383`).
- Without an explicit path, home loads first, project second, then environment and flags (`cli/internal/config/config.go:386-405`).
- Defaults include `.agents/ao`, automatic worktree/runtime modes, model tiers, local corpus paths, and compatibility command names (`cli/internal/config/config.go:291-354`).
- The codebase also uses command-specific `AO_*`/`AGENTOPS_*` variables outside the central aggregate. A fresh lexical measurement found 69 distinct direct Go string literals in those namespaces.

Configuration is therefore layered but not fully centralized: `config.Config` covers common options, while commands and scripts own many operational escape hatches.

## Schemas and contracts

There are 60 top-level JSON schemas under `schemas/` plus four fixtures. The most load-bearing contracts are:

| Contract | Boundary |
|---|---|
| `agentops-sdlc-provenance.v1` | Typed endpoints, PROV-O relation, trust, timestamp, hash chain; mirrored by `provenancegraph.Edge` (`edge.go:30-74`) |
| `pawl-verdict.v1` | Evidence-bound, commit-bound independent-review artifact consumed by `emit-verdict` |
| `execution-packet` | Closed rich packet shape, fail-closed default verdict, criteria, routing, validation lanes (`schemas/execution-packet.schema.json:1-12`, `schemas/execution-packet.schema.json:107-123`) |
| `skill-frontmatter.v2` | Skill metadata/hexagonal contract used by frontmatter and generation validators |
| `session-bootstrap.v1` | Machine-readable startup report shape |
| `orchestration-{backend,result,instrument}.v1` | Backend decision and output parity across substrates |
| `swarm-evidence` + strict worker result | Worker evidence compatibility and completion result |
| `goal-design-{intent,driver}.v1` | New pre-discovery checked goal-design packets |

Go embeds and compiles the execution-packet schema once, validates raw bytes before decoding, and does not let unknown fields disappear through struct unmarshalling (`cli/internal/domain/packet/invariants.go:15-27`, `cli/internal/domain/packet/invariants.go:38-53`, `cli/internal/domain/packet/invariants.go:90-110`). Other contracts are enforced by a mix of Go validators and shell/Python tooling. The boundary is consequently strong but heterogeneous.

Generated/declared governance includes six canonical bounded contexts (`docs/contracts/bounded-contexts.yaml:21-125`), 72 contract files, skill dispositions, claim registry, CI job manifest, execution profile, and generated context maps.

## Test, gate, build, regen, and release infrastructure

### Tests

| Layer | Location / mechanism | Fresh size |
|---|---|---:|
| Go unit/property/integration | `cli/**/*_test.go`; table tests, `rapid`, `goleak` | 701 files, 7,585 test/bench/fuzz funcs |
| Bats gate and behavior tests | primarily `tests/**/*.bats` | 256 files / 2,212 cases under `tests/` |
| Shell/Python tests | `tests/**/*.sh`, three Python test files | 122 shell files |
| Scenario contracts | `spec/scenarios`, skill `.feature` files | 15 JSON scenarios; 62 tracked `.feature` files repo-wide |
| Eval substrate | `evals/**` | 70 JSON files plus harness scripts |
| Workflow/runtime smoke | `tests/install`, `tests/e2e`, `tests/integration`, `tests/codex`, `tests/windows` | platform and installer behavior |

The standard Go target builds first and runs the suite shuffled (`cli/Makefile:58-65`). The release-grade lane builds, vets, runs race+coverage tests, and records coverage (`scripts/ci-local-release.sh:477-508`). The Go gate registry itself has routing, registry, report, workflow-coverage, and script-runner tests under `cli/internal/gates/`.

### Build

- `make` defaults to the release-grade local CI (`Makefile:5-15`).
- `make build` produces the spine/default binary; archived satellites require explicit build tags (`Makefile:17-24`, `cli/Makefile:11-20`).
- Every CLI build first copies embedded scripts/schemas/skill references into `cli/embedded` (`cli/Makefile:25-41`).
- `make inner` is the bounded compile+vet loop (`cli/Makefile:96-99`).

### Regeneration

`scripts/regen-all.sh` is dependency-ordered: skill counts → domain map → registry/SKU → context map → embedded assets → CLI reference → command surfaces → CLI inventory → Codex twins/hashes → skill catalog (`scripts/regen-all.sh:59-85`). Its no-write mode executes 13 drift families and aggregates failures (`scripts/regen-all.sh:86-105`). `generate-cli-reference.sh` builds a temporary default `ao`, recursively walks help, and scrubs machine-local paths (`scripts/generate-cli-reference.sh:22-50`, `scripts/generate-cli-reference.sh:178-245`).

### Gate and CI

- Routine authority: local `ao gate check --fast --scope head` and the repo pre-push path (`AGENTS-WORKFLOW.md:5-18`).
- Full parity: `ao gate check --full --workflow-coverage --require-workflow-parity` (`AGENTS-WORKFLOW.md:57-64`).
- Hosted validation: tags/manual/PR/merge-group only; a tag or merge-group forces all path-filter families (`.github/workflows/validate.yml:18-32`, `.github/workflows/validate.yml:42-73`).
- Local release rehearsal exposes full (~78 min), quick (<5 min), and fast modes with explicit scope differences (`scripts/ci-local-release.sh:8-30`).

### Release

The release publisher gates docs and pre-publish evidence, installs security/SBOM tooling, validates release readiness, then runs GoReleaser (`.github/workflows/release.yml:28-68`, `.github/workflows/release.yml:70-118`, `.github/workflows/release.yml:225-232`). GoReleaser produces Darwin/Linux/Windows amd64/arm64 archives and updates the Homebrew tap (`.goreleaser.yml:4-29`, `.goreleaser.yml:39-55`).

## Logging, performance, and security characteristics

- The CLI mainly writes human output or JSON to stdout and diagnostics to stderr. There is no central logging framework; command and script output is the trace.
- Gate reports retain per-check duration and bounded log tails, with the final 15 lines in JSON (`cli/internal/gates/report.go:92-104`, `cli/internal/gates/report.go:172-203`).
- Gate selection is smart, but execution is serial; full validation cost is dominated by subprocess startup and repeated tree scans.
- The MCP transport is newline-delimited JSON-RPC over stdio with a 4 MiB scanner bound (`cli/internal/adapters/mcptransport/transport.go:55-84`).
- The MCP façade fail-closes requests whose arguments contain holdout/eval markers (`cli/internal/adapters/mcpsurface/surface.go:32-71`).
- Provenance append is concurrency-protected and corrupt JSON lines are hard errors (`cli/internal/provenancegraph/store.go:26-68`, `cli/internal/provenancegraph/store.go:107-159`).
- Release security includes pattern checks, gosec, gitleaks, trivy, semgrep, and SBOM/readiness evidence (`.github/workflows/release.yml:55-108`).

## Notes, gotchas, and risks

These IDs are stable references for follow-up.

| ID | Classification | Finding | Evidence / consequence |
|---|---|---|---|
| **R-01** | Code/doc drift | `scripts/install.sh --dev` still states the local pre-push gate was retired and CI is sole authority. | `scripts/install.sh:75-87` contradicts the live Go-gate authority in `AGENTS-WORKFLOW.md:5-18` and `.github/workflows/validate.yml:18-26`. Behavior installs hooks/builds, but the embedded guidance is stale. |
| **R-02** | Measured doc drift | The first-read overview's scale block is materially stale. | It reports 73 skills, ~88 commands, ~77 gates, ~139 Bats, 172 capabilities (`docs/architecture/codebase-overview.md:51-62`); fresh values are 59, 72, 112, 256-under-tests, 145. Because narrative is lower precedence, this report uses measured/generated values. |
| **R-03** | Contract completeness | `ao capabilities` publishes only three environment variables while executable Go references at least 69 distinct direct `AO_*`/`AGENTOPS_*` literals. | Published map is `cli/cmd/ao/capabilities.go:101-106`; agents cannot discover much of the configuration surface from the advertised machine contract. |
| **R-04** | Archived-runtime residue | Central defaults still set RPI runtime command to `claude`, and the orchestration port auto-ladder prefers NTM then Claude while Codex is pin-only. | `cli/internal/config/config.go:313-320`; `cli/internal/ports/orchestration.go:6-25`, `:49-69`. These may be compatibility surfaces rather than current routing; wiring status is an unknown. On bo-mac, the host override explicitly requires Codex/local shell. |
| **R-05** | Landing semantic split | Canonical `ao land` treats post-land provenance failure as best-effort, while `scripts/land.sh` has a strict provenance mode and closes the bead with `ao done`. | `cli/cmd/ao/land.go:226-234` versus `scripts/land.sh:163-197`. Operators must know which wrapper they invoked; “landed” and “closed with strict provenance” are not identical outcomes. |
| **R-06** | Performance | Gate orchestration is serial despite a 112-check registry and a documented ~78-minute full release rehearsal. | `cli/internal/gates/orchestrator.go:45-47`; `scripts/ci-local-release.sh:15-26`. Correctness rationale is shared-state safety; latency pressure may encourage bypass if Fast routing regresses. |
| **R-07** | Generated-surface coupling | A skill/command change fans out across many generated and runtime-specific projections. | `scripts/regen-all.sh:59-105`. The one-command finalizer is a strength, but failures can arise far from the edited source and bespoke Codex artifacts remain special cases. |
| **R-08** | Naming / archaeology | “Gate” and “provenance” name multiple different systems; source presence does not imply default-build availability. | `ao gate` also owns human promotion review (`cli/cmd/ao/gate.go:24-35`); `gate check` owns validation. `internal/provenance` is a separate audit/projection package from authoritative `provenancegraph`. Build tags archive flywheel/legacy files (`cli/Makefile:11-20`). |
| **R-09** | Persistence boundary | `.agents/` runtime evidence is gitignored/private while the provenance ledger is tracked, and `_beads` is a separate private repository. | `AGENTS-RUNTIME.md:22-30`; `AGENTS-WORKFLOW.md:34-36`, `AGENTS-WORKFLOW.md:176-180`. Evidence promotion must cross these boundaries deliberately; never stage `_beads` in the public repo. |
| **R-10** | Binary freshness | MCP tool execution shells the string `ao`, not the currently running executable or an injected absolute binary. | `cli/internal/adapters/mcpsurface/surface.go:114-137`. An MCP server started from a fresh checkout can dispatch to a stale PATH binary, unlike `ao land`, which goes to unusual lengths to pin `AO_BIN`. |
| **R-11** | Partial hexagon | The repository has 28 explicit production port interfaces, but command/application code remains broad. | 300 production files live directly in `cli/cmd/ao`; concrete command globals and per-command `init()` remain common. Ports are strong seams, not yet the universal application boundary. |
| **R-12** | Schema heterogeneity | Core packet/provenance contracts are enforced in Go, but other schema checks depend on shell/Python tool availability and sometimes degrade or skip. | Compare embedded Go enforcement (`cli/internal/domain/packet/invariants.go:38-53`) with the optional validator posture described in `scripts/validate-next-work.sh:270-322`. Full CI supplies dependencies; local partial runs may not prove the same floor. |

## Strengths

1. **Fail-closed typed verdicts.** Gate status, report semantics, packet defaults, and process exit codes turn absence/unknown into non-success instead of optimistic continuation (`cli/internal/ports/gate_runner.go:22-35`, `cli/internal/gates/report.go:26-40`, `cli/internal/domain/packet/aggregate.go:90-123`).
2. **Tamper-evident, concurrent proof ledger.** Edge validation, deterministic hashing, idempotency, and process locks are cohesive and tested (`cli/internal/provenancegraph/edge.go:251-307`, `cli/internal/provenancegraph/store.go:107-199`).
3. **Live machine discovery.** `ao capabilities` derives command groups from the registered Cobra tree, while generated CLI references build and recurse through the actual binary (`cli/cmd/ao/capabilities.go:136-199`, `scripts/generate-cli-reference.sh:178-245`).
4. **Exceptional test investment.** Test Go LOC exceeds non-test Go LOC, backed by thousands of test functions plus Bats behavior/gate coverage.
5. **Explicit release authority.** Local gate, hosted backstop, and publisher roles are separated in triggers and documentation rather than conflated (`.github/workflows/validate.yml:18-40`, `.github/workflows/release.yml:1-28`).
6. **Honest architectural posture.** The first-read docs explicitly demote unproven corpus uplift and define executable precedence (`docs/architecture/codebase-overview.md:47`, `docs/architecture/codebase-overview.md:110-115`).
7. **Portable runtime boundary.** The core Go module is small; agent vendors and orchestration substrates are mostly subprocess/adapters rather than deep SDK coupling.

## Unknowns and limits of this report

- **Unknown:** how many of the 28 ports are reached by routine default-build commands versus retained compatibility code. Static references show seams, not production frequency.
- **Unknown:** real wall-clock distribution and flake rate of the 112-check registry. The full gate was intentionally not run.
- **Unknown:** current health/availability of Codex, NTM, Agent Mail, GitHub, Homebrew, security scanners, and any remote substrate. No external calls were made.
- **Unknown:** end-to-end success of `ao land`; it would mutate Git and remote state and was outside this read-only assignment.
- **Unknown:** exact model deployment ID for this report; only GPT-5 / Codex runtime is available.
- **Boundary:** LOC counts include checked-in generated/embedded code and tests. They are repository-maintenance scale, not a claim of hand-authored product LOC.
- **Boundary:** command counts distinguish 72 visible top-level default-build commands from 257 generated command paths. Neither includes every tagged satellite in the same way.

## Current versus 2026-07-02 report

This comparison was performed only after the independent inspection.

| Area | 2026-07-02 report | 2026-07-09 fresh state | Delta / interpretation |
|---|---:|---:|---|
| Pinned activity | — | 203 commits since July 2 | High-change week; inherited counts were unsafe |
| Go files / LOC | ~1,300 / ~392K | 1,456 / 403,556 | roughly +156 files / +11.6K lines, with exact methods now stated |
| Source skills | 64 | 59 | net -5 versus prior report; history includes a deliberate 66→58 retirement wave followed by `goal-design` |
| Gate checks | ~90 | 112 | about +22; current count is statically reconstructed from registration sites |
| Go test files | 682 | 701 | +19 |
| Bats under tests | 236 | 256 | +20 (283 repo-wide) |
| JSON schemas | ~45 | 60 top-level + 4 fixtures | substantial contract growth, including goal-design artifacts |
| Top-level commands | not normalized | 72 live default-build | now measured from `ao capabilities`, not source-file heuristics |
| Landing | described as pawl/gate/push flow | canonical `ao land` executable added | fresh-binary re-exec and atomic pawl-land handoff are now explicit (`cli/cmd/ao/land.go:30-64`) |
| Build shape | legacy noted | ADR-0012 explicit spine vs archived satellite builds | default artifact boundary is clearer (`cli/Makefile:11-20`) |
| Key type emphasis | candidate/flywheel-heavy | gate/report/packet/provenance/application seams | reflects the product's verified-done center and archived flywheel posture |

The prior report's architectural center—skills + CLI + independent verdict + hash-chained proof—still holds. The largest change is emphasis and enforcement: the default build and skill catalog have been narrowed, gate coverage expanded, `ao land` made the trusted executable path, and goal-design/schema/test surfaces grew. The current repository is more explicitly a verification membrane and less accurately described through the older candidate/pool/flywheel types.

## Bottom line

**Code fact:** AgentOps at `fbba8af5ace635104775ef18f34fef362ba368ce` is a large, test-heavy, file-and-subprocess-oriented verification control plane whose strongest invariants are typed gate outcomes, independent pawl verdicts, and a locked hash-chained provenance ledger.

**Interpretation:** Its architecture is strongest where it converts stochastic agent claims into deterministic, inspectable refusal or proof. Its main technical debt is not missing mechanism but surface reconciliation: archived/default command boundaries, duplicated shell/Go orchestration, incomplete machine-readable configuration, and fast-moving generated/narrative inventories.

**Operational conclusion:** use the default spine binary, discover it through `ao capabilities`, treat `ao gate check --fast --scope head` as routine authority, land through `ao land`, and independently verify the resulting verdict and provenance state before calling work done.
