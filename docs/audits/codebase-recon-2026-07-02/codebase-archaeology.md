# AgentOps — Codebase Archaeology (2026-07-02)

> **Skill:** `codebase-archaeology` · **Run:** 2026-07-02 refresh (prior: 2026-06-24, 2026-06-14, 2026-06-11)
> **Method:** documentation-first orientation + 5 parallel Explore agents, one per bounded-context cluster.
> **Purpose:** a reusable mental model of what AgentOps *is* and how the pieces connect. Sibling artifacts: `codebase-report.md` (architecture), `codebase-audit.md` (issues/risk), `codebase-pattern-extraction.md` (reusable patterns), `SYNTHESIS.md`.

---

## Executive summary

**AgentOps is a verification membrane for coding agents**: the loop produces validated output *with proof*. Coding agents declare "done" on code that is still wrong; AgentOps refuses to call a change done until something that did **not** write it checks it — a different model family, or a test that actually runs — and binds that verdict into a hash-chained provenance ledger. The product slogan is literal and mechanical: **no verdict = not done.**

It ships as **skills + the `ao` Go CLI + a repo-local `.agents/` corpus**, hookless and in-session (AgentOps 3.0, [ADR-0002](../../adr/ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md)). It ships **no daemon, scheduler, or hosted control plane** ([ADR-0009](../../adr/ADR-0009-daemon-deletion-in-session-only.md)); out-of-session always-on work is *delegated* to an external substrate (NTM tmux swarm + MCP Agent Mail + `ao agent`), never owned. It runs on the agent you already pay for (Claude Code, Codex, Cursor, OpenCode) via one shared skill set with Codex runtime twins.

**Honest posture (stated by the project itself):** *proven* = independent verification recording a durable, tamper-evident verdict. *Still measuring* = whether the accumulated corpus makes the next session measurably better (the "flywheel moat"), demoted to unproven under a structural data-starvation headwind ([ADR-0004](../../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](../../adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

### Key statistics (2026-07-02)

| Dimension | Value |
|-----------|-------|
| Go LOC / files | ~392K across ~1,300+ files (`cli/`) |
| Shell LOC | ~112K (~280 validation/regen/release scripts) |
| Python LOC | ~14K |
| Markdown docs | 1,790 files |
| Active skills | 64 (`skills/**/SKILL.md`) |
| CLI command files | ~646 (`cli/cmd/ao/`), ~245 self-registering command structs |
| Internal packages | ~80 (`cli/internal/`) |
| Gate checks (Go registry) | ~90 (~9 native Go, rest shell-backed) |
| Go test files / Bats files | 682 / 236 |
| Commits (last 30d / 90d) | 1,178 / 3,175 — **very active** |
| Most-churned file | `docs/provenance/ledger.jsonl` (90 of last 200 commits) |
| TODO/FIXME/XXX/HACK in `cli/` | 4 — remarkably clean |
| Go / cobra | Go 1.26 / cobra 1.10 |

The single most-churned file being the **provenance ledger** is itself the strongest archaeological signal: the "no verdict = not done" discipline is not aspirational — it is the repo's dominant write pattern.

---

## The one loop everything feeds

Every skill, gate, and CLI command feeds a single seven-move operating loop ([`docs/architecture/operating-loop.md`](../../architecture/operating-loop.md)). The *map* (moves + legal transitions + gates) is fixed; the *route* is re-planned on failure.

| Move | What | Primary skill(s) | Code anchor |
|------|------|------------------|-------------|
| 1. Shape intent as BDD | capability + Given/When/Then + non-goals + rollback + evidence | `discovery`, `product`, `plan` | — |
| 2. Track as a bead | linked-intent packet once work leaves the head | `beads-br` (`br`) | `cmd/ao/beads*.go`, `internal/rpi` next-work |
| 3. Slice vertically | one G/W/T per slice, one bounded context | `plan` | — |
| 4. TDD per slice | failing test first, then implementation | `implement` | — |
| 5. Wave only if write-scopes disjoint | parallelism = explicit non-overlapping ownership | `crank`, `swarm` | `internal/orchestration/shape.go` |
| 6. Close by proving acceptance | **the windshield** — deterministic ground truth | `validate`, `council`, `pre-land-refuters` | `internal/verdictledger`, `internal/evidencedturn`, `planpawl`, `scripts/pawl*.sh` |
| 7. Capture evidence + ratchet | promote what changes future behavior | `post-mortem` (primary) | `internal/lifecycle`, `pool`, `ratchet` |

Move 6 is load-bearing: the *unit of value is the proof, not the artifact*. Move 7 loops back into move 1 — re-plan on **evidence**, not just failure (the anti-waterfall rule).

---

## Six bounded contexts (DDD) — the code map

Contract: [`docs/contracts/bounded-contexts.yaml`](../../contracts/bounded-contexts.yaml). Routing: [`component-map.md`](../../architecture/component-map.md).

```
BC3 Loop ──▶ BC1 Corpus       (compounding context)
         ──▶ BC2 Validation   (proof before land)
BC4 Factory ──▶ skills / registries
BC5 Runtime ──▶ ao CLI + plugins
BC6 Orchestration ──▶ dispatches WHOLE skills (never decomposes RPI internals)
```

### BC5 Runtime — the `ao` CLI spine

A cobra CLI with a **thin, fixed spine** and a **massive decentralized leaf surface**.

- `cmd/ao/main.go:10` → `Execute()`. One true root `rootCmd` at `cmd/ao/root.go:28`.
- **Typed-error → exit-code dispatch** (`root.go:83`): `Execute()` `errors.As`-unwraps a family of sentinel errors (`gateExitError`, `doctorExitError`, `planPawlExitError`, `beadsExitError`, `governorExitError`, …) so **the process exit code carries the verdict**. This is the central contract by which gate-style commands talk to shells/CI/hooks.
- **No central registry:** each of ~245 command files declares `var xCmd` + `func init()` calling `rootCmd.AddCommand` (343 `AddCommand` calls). Grouped into 7 help buckets (`start`/`core`/`workflow`/`config`/`comms`/`knowledge`/`experimental`).
- Commands are thin adapters: resolve `config.Config` → call one `internal/` package → format by switching on `GetOutput()`.
- **Agent ergonomics is first-class and drift-proof.** `ao capabilities` (`capabilities.go`) and `ao robot-docs` (`robot_docs.go`) **generate their machine contract by reflecting over the live `rootCmd` tree**, so the published contract cannot drift from actual registration. `--json` works on both leaf commands and non-runnable parents (`group_json.go`). Exit-code and env-var dictionaries are published data.

### BC2 Validation — the gate system

> **Naming collision to flag:** `ao gate` (the cobra parent, `gate.go`) is the *human review gate* for bronze-tier promotion (approve/reject, backed by `pool`). The **validation gate** is `ao gate check` (`gate_check.go` + `internal/gates/`). Different concerns on the same parent.

- **Declarative registry.** `Check{ID, Tiers(Fast|Full bitmask), Match[]globs, Blocking, Backing XOR Run}` (`internal/gates/gates.go:57`). Fast/Full are **filters over one registry** — they cannot drift. Seed list ~90 checks (`checks/seed.go`), each self-registering via `init()` (anti-monolith: adding a check = one struct literal).
- **`ao gate check --fast --scope head`:** tier-filter → changed-file detection (`git show --name-only HEAD`) → per-check glob routing → serial execution → exit 1 iff any *blocking* check FAILs. A **full-run escape hatch** fires if the gate's own source (`cli/internal/gates/`, `go.mod`) changed, so a stale router can't silently skip its own guards.
- **Two implementation styles:** shell-backed (`Backing: scripts/check-*.sh`, the majority; exit-code map `0=PASS 2=WARN 75=SKIP else=FAIL`) and native Go (`go.build`, `go.vet`, `constraints.enforce`, `state.verify`, `learning.coherence`, claim-registry — 9 total, "Phase B" ports, in progress).
- **Triple orchestration (migration in progress):** Go registry (convergence target) · `.github/workflows/validate.yml` (CI, ~10 purpose-grouped jobs, runs `ao gate check --full` as a *shadow* with `--require-workflow-parity`) · `scripts/pre-push-gate.sh` (88KB bash monolith, legacy escape hatch — does **not** yet call `ao gate check`).
- **Other validation surfaces:** `vibecheck` (0–100 git-timeline "vibe" score), `verdictledger` (append-only per-iteration verdict store), `quality` (`ao doctor`/`ao metrics`), `safety` (9-item threat model T1–T9, runtime guards for autonomous agents).

### BC1 Corpus — knowledge & the flywheel

> **Directory naming traps (flag these):** `internal/corpus` is a maturity *classifier*, not retrieval. `internal/knowledge` is the `ao knowledge` render command, not the store. **The actual inject/lookup retrieval + decay-ranking engine is `internal/search`.** `internal/refinery` is the CI continuous-validation backstop, **not** knowledge refinement.

- **Three storage planes + a spine:** raw private corpus `.agents/` (gitignored by policy) · gold public wiki `.ao/wiki/` (tracked, sanitized) · durable authority `.ao/accepted` + `.ao/admissions/` · provenance spine `docs/provenance/ledger.jsonl` (tracked, hash-chained).
- **Retrieval:** `ao inject` (deprecated, token-budgeted) / `ao lookup` (by-id/query/bead, `--gold`, `--pointers`) / `ao corpus inject` (typed `CorpusReaderPort` path, behind `flywheel` build tag). Every hit records a `CitationEvent` via `ratchet.RecordCitation` — the feedback edge.
- **"Decay-ranked" is real math** (`internal/search/scoring.go`): freshness `exp(-ageWeeks × 0.17)`, composite MemRL two-phase `(z(freshness) + λ·z(utility)) × MaturityWeight`, confidence `exp(-weeks × 0.1)`. Cited learnings gain utility → rank higher next inject.
- **Flywheel:** Work → Forge (`ao forge`) → Pool (5-dim rubric: specificity .30 / actionability .25 / novelty .20 / context .15 / confidence .10) → Promote (Gold ≥0.85 auto / Silver 0.70 auto / **Bronze 0.50 human-gated** / Discard <0.50) → Learnings → Inject. Separate **maturity ratchet** (provisional→candidate→established, Brownian).
- **Provenance = two packages:** `internal/provenance` is the **tracker** (rebuildable lineage projection, `Audit` for stale citations). `internal/provenancegraph` is the **ledger** — hash-chained `ledger.jsonl`, PROV-O relations, trust tiers `authored > inferred > mined`, idempotent flock-serialized append. **Ledger wins over tracker** on disagreement (enforced by `drwitness.CrossCheck` re-deriving from Dolt and byte-comparing).

### BC3 Loop — turns, fitness, autonomy

**Defined by what was deleted.** The RPI *engine*, the daemon, and `ao rpi`/`ao evolve`/`ao loop`/`ao autodev`/`ao orchestrate` CLIs were removed (ADR-0009); what survives is utility code + deterministic deciders. The loop runs **in-session as skills**.

- `internal/rpi` — misleadingly named; now a bag of next-work queue (`next-work.jsonl` claim/consume ledger) + run-tracking utilities. No `rpiCmd`. `loop*.go` are `//go:build legacy`.
- **A "turn" is a fold, not a column** (epic ag-lmdx): `turnstate` is an append-only hash-chained `state_transition` log whose `Fold` enforces contiguous evidence-backed transitions (no state jumps). `evidencedturn` is the 7-predicate Definition-of-Done (chain intact, terminal state, scenarios covered, evidence resolves, provenance event, no orphan, **author ≠ validator**). Done iff all seven pass.
- **Fitness:** `ao goals measure` runs each GOALS.md directive's check command as a subprocess (pass / fail / skip=77) → Score %, plus `goalsfitness` scenario-satisfaction fractions (default threshold 0.8, zero-evidence = `unknown` not vacuous pass), anchored by `goalstrace` (ADR-0005 read-only graph: exact-ID edges `high` = closure proof, heuristic edges `low` = never counted).
- **Autonomy is bounded + governed, not LLM-decided:** `autodev` parses PROGRAM.md into a mutable/immutable-scope contract; the SPC `governor` makes two deterministic decisions over the yield ledger — error-budget burn-rate ship-vs-**harden** (**exit 3 = stop the line**) and noise-band adjust-vs-hold. An *escape* = a CONFIRMED a later verdict REFUTED (a membrane miss).
- `posture` types a project's ownership over 6 stack layers (AgentOps owns Role+Skill+Ledger+Distribution, imports Loop, Bead externally owned by `br`).

### BC4 Factory — the skill manufacturer

- **64 skills.** Each `skills/<slug>/` = `SKILL.md` (YAML frontmatter + body, 250-line ceiling) + `references/` (overflow, templates, `*.feature` acceptance) + optional `scripts/`. Frontmatter carries DDD/hex edges (`hexagonal_role`, `consumes`, `produces`, `context_rel`), `practices` lineage, `metadata.tier`, `output_contract`.
- **Three drift-gated registries** — edit sources, `make regen-all`, `make regen-check` is the CI gate: Skills (`skills/**/SKILL.md` → `registry.json`/`catalog.json`/maps), Workflows (`.claude/workflows/*.js` + dispositions → `registry.json` workflows[]), CLI (`cmd/ao/` → `COMMANDS.md`/`cli-surface.*`). `registry.json` carries **no timestamp** so regens are byte-identical.
- **Codex twins:** `skills-codex/` ships to Codex runtime (never `skills/` source). 41 `parity_only` (auto-refreshed from source), 19 `bespoke` (hand-authored in `skills-codex-overrides/`, **not** auto-regenerated — footgun: `make regen-all` refreshes only the *hash record*, not bespoke prose). Only `spine: true` sources get `source_hash` recomputed.
- **Dispositions ledger** (`docs/contracts/skill-dispositions.yaml`, 1148 lines): per-skill lifecycle verdict `keep (35) / update (24) / refactor (5)`; rows are flipped to `historical`, never deleted. `ao skills retire` is a deterministic 5-phase retire (validate → remove trees → flip ledger non-lossily → regen → ripple-scan).
- **Three read-only Go factory packages:** `skills` (catalog + consumers/producers graph), `skillshealth` (frontmatter + reference-link + codex-parity auditor), `skillsresolve` (MECE resolver: Jaccard mutual-exclusivity + coverage-gap detection).

### BC6 Orchestration — the substrate boundary

- **No daemon.** Out-of-session work is delegated: a substrate (NTM tmux swarm) runs `br ready`, dispatches a bead to a worker running the **whole `/rpi` skill**; `ao mcp serve` (Claude-only JSON-RPC tool surface) + `ao agent bundle` expose the surface across the seam.
- `internal/orchestration` detects runtimes **by capability, not `command -v`** (`ntm --robot-capabilities`), degrades **NTM → Claude-native → beads-floor** with output-contract parity. `shape.go` is the ATM/AM shape decider — model *proposes*, predicates *check* against live writer count + per-lane write-sets; `contention` fires only on overlapping write-sets ("partition before you lock").
- **The cardinal rule** — "dispatches whole skills, never decomposes RPI internals" — is enforced *architecturally*: there is deliberately **no `ao loop step` primitive**. A Workflow can only `agent()`-dispatch a black-box skill returning a validated schema, never drive the loop's insides. "Whoever owns the loop owns its invariants, and AgentOps owns the loop."
- `agentworker` is the durable-session contract (`lost`/`provider_unreachable` are non-success terminal states — absence is never promoted to success).
- **`ao orchestrate` surface is `//go:build legacy`;** only the library is live.

---

## The pawl — the cross-family acceptance membrane

"Pawl" (as in a ratchet's pawl: only lets the ratchet advance) is AgentOps's own acceptance gate, shipped **in-repo** (per GOALS.md — no separate product/daemon). Three layers:

1. **`internal/planpawl/decide.go`** — pure deterministic decider (no model calls) turning a round of family-judge verdicts into PASS / REDO / BLOCKED. **Fail-closed everywhere:** unrecognized disposition / off-roster family / missing warn-class all count as FAIL; quorum < 2 distinct families → REDO.
2. **`cmd/ao/pawl.go`** — `ao pawl review <bead>` dispatches the **codex refuter** (cross-family, never same-model self-review — LAW 0) and on CONFIRMED writes the commit-bound verdict the pre-push gate requires. **Exit code IS the verdict** (0 CONFIRMED / 3 REFUTED). Forge-proof RCE guard: live in-repo scripts run only when the `ao` binary physically lives inside the checkout; otherwise embedded scripts run with a sanitized PATH.
3. **`scripts/pawl.sh`** — a warm tmux cross-family membrane *service* you route to (not a hand-spun `codex exec` per bead). Tiers: `multi` (≥2 families = real cross-family gate) / `fresh` (1 family, refused by high-irreversibility doors like push-to-main). Fail-closed ALL-CONFIRM agreement; per-route nonce defeats stale-scrollback false positives.

**Verdict → binding → gate loop:** `/pre-land-refuters` → `scripts/pawl-verdict.sh write` → `ao provenance emit-verdict` builds a `verdict --wasDerivedFrom--> commit` ledger edge → auto-bind commits *only* `ledger.jsonl` (`chore(provenance): bind pawl CONFIRMED verdict for <bead> #trivial`). Consumed by: push gate (`check-pawl-pre-push.sh` refuses push without CONFIRMED for `bead@head`), merge gate (`reconcile-pr.sh`), close gate (`ao done` stamps `[verdict:<sha7>:CONFIRMED]`), publish gate (`wiki_publish`). The governor later mines these for escapes.

---

## Data flow (end to end)

```
goal + acceptance contract
   │  (Move 1: /discovery → BDD)
   ▼
br bead in _beads/ ledger ──(ao beads dir resolves via git-common-dir for worktrees)
   │  (Moves 3–4: slice + TDD in a worktree)
   ▼
implementation + failing→green test
   │  (Move 6: /validate → /pre-land-refuters)
   ▼
cross-family refuter (pawl.sh) ──▶ pawl-verdict.json  ──CONFIRMED?──▶ HOLD if not
   │
   ▼
ao provenance emit-verdict ──▶ hash-chained edge in docs/provenance/ledger.jsonl
   │  (auto-bind commit)
   ▼
ao gate check --fast --scope head ──(local cockpit = release authority)──▶ push to main
   │  (Move 7)
   ▼
ao forge / pool / ratchet ──▶ .agents/ learnings ──▶ ao lookup (decay-ranked) next session
   │
   ▼
ao governor budget ──(watches ratchet for escapes/oscillation; exit 3 = harden)
```

The loop closes **through the ledger**: a verdict is both the terminal proof of one turn and new sensor data for the recurrence axis (mining → apparatus improvement).

---

## Core types (the nouns everything revolves around)

| Type | Location | Purpose |
|------|----------|---------|
| `types.Candidate` | `cli/internal/types/types.go:174` | central knowledge artifact — extracted learning/pattern/finding with Tier, Utility (MemRL Q-value), Maturity, Confidence, supersession chain |
| `types.CitationEvent` | `cli/internal/types/types.go:684` | "this artifact was used" reward signal — the flywheel's feedback edge |
| `pool.PoolEntry` | `cli/internal/types/types.go:360` | candidate awaiting human review; the unit `ao gate` (human) operates on |
| `gates.Check` | `cli/internal/gates/gates.go:57` | one declarative validation check (ID/Tiers/Match/Blocking/Backing\|Run) |
| `provenancegraph.Edge` | `cli/internal/provenancegraph/edge.go:74` | hash-chained ledger edge (PROV-O relation, trust tier, verdict enrichment) |
| `turnstate` transition | `cli/internal/turnstate/turnstate.go` | append-only hash-chained lifecycle log; state = Fold(log) |
| `config.Config` | `cli/internal/config/config.go:20` | resolved runtime config (paths, forge, search, models, flywheel) |

---

## Hexagonal seams (partial retrofit)

`internal/ports` holds ~27 named interfaces, one per BC (`CorpusReaderPort`, `GateRunnerPort`, `LoopReaderPort`, `OperatorPort`, `HarnessPort`, …), each with in-package in-memory doubles (`inmemory_*.go`) and production adapters in `internal/adapters/<name>/` asserting `var _ ports.X = (*T)(nil)`. **Caveat:** the hexagon is *partial* — many high-traffic commands (`gate`→`pool`, `lookup`→`search`/`ratchet`) still bind directly to concrete packages. Ports are the newer DDD-scoped seams retrofitted BC-by-BC over older direct-dependency code.

---

## Source-of-truth precedence (when docs disagree)

1. **Executable + generated** — `cli/**`, `scripts/**`, `cli/docs/COMMANDS.md`, generated registries
2. **Contracts** — `skills/**/SKILL.md`, `schemas/**`, `docs/contracts/**`
3. **Narrative** — `docs/**`, `README.md`, `AGENTS*.md`

Some older narrative (`ARCHITECTURE.md`, `ports-and-adapters.md`) still mentions hooks, `bd`, or PR-per-change — historical unless reconciled.

---

## Where to look first (reading order)

1. [`docs/newcomer-guide.md`](../../newcomer-guide.md) → [`docs/architecture/codebase-overview.md`](../../architecture/codebase-overview.md) → [`docs/3.0.md`](../../3.0.md)
2. [`docs/architecture/operating-loop.md`](../../architecture/operating-loop.md) (primary navigation)
3. [`GOALS.md`](../../../GOALS.md) (the destination: autonomous goal → verified done) + [`PRODUCT.md`](../../../PRODUCT.md)
4. Task-specific: CLI → `cli/cmd/ao/` + generated `cli/docs/COMMANDS.md`; gates → `cli/internal/gates/checks/seed.go`; skills → `skills/**/SKILL.md`; verdicts → `cli/internal/{verdictledger,provenancegraph,evidencedturn}/`

---

## One-paragraph takeaway

AgentOps is a hookless, in-session **verification membrane** built as a cobra CLI (thin typed-exit-code spine, ~245 self-registering leaf commands) over six DDD bounded contexts. The product is the *proof*, not the artifact: a change is done only when an independent, cross-family reviewer (never the author, never the same model) writes a CONFIRMED verdict that is bound into a hash-chained provenance ledger — enforced mechanically by the pawl gate, the pre-push gate, and the 7-predicate evidenced-turn Definition-of-Done. Beneath the membrane a decay-ranked knowledge flywheel compounds context (measured, unproven as a moat). The loop was deliberately moved *into the session as skills* and the daemon deleted (ADR-0009); out-of-session work is delegated to a substrate that dispatches whole skills as black boxes and is architecturally forbidden from decomposing the loop's insides. The codebase is unusually disciplined — 4 TODOs in ~392K LOC, the provenance ledger as the most-churned file, and an honest self-assessment that refuses to market ahead of its own ruler.
