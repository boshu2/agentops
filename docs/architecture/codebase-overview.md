# Codebase Overview

> **Audience:** contributors, operators, and agents orienting in this repo.  
> **Scope:** what the repository *is*, how major pieces connect, and where to look first.  
> **When docs disagree:** executable code and generated artifacts win — see [Source-of-truth precedence](#source-of-truth-precedence).

For product doctrine read [AgentOps 3.0 — the north star](../3.0.md). For *how work flows* read [Operating Loop](operating-loop.md). This page is the **map of the territory**.

---

## For agents (quick facts)

| Fact | Value |
|------|-------|
| Product | In-session autonomous code validation for coding agents |
| Active waist | `ao session bootstrap` → `ao inject` → operating loop → `ao gate check --fast` → push to `main` |
| Issue tracker | **br** (beads_rust) in `_beads/` — `BEADS_DIR=$PWD/_beads br <cmd>` until legacy `.beads/` retires |
| Skills SSOT | `skills/<slug>/SKILL.md` — never edit `~/.claude/skills/` |
| Release gate | Local cockpit: `ao gate check --fast --scope head` (Go registry in `cli/internal/gates/`) |
| CI | Backstop only — not routine release authority for every `main` push |
| Hooks | AgentOps 3.0 ships **zero** hooks; context is pulled explicitly |
| `.agents/` | Runtime knowledge — **gitignored**; durable public truth goes to `docs/`, `GOALS.md`, or provenance ledger |
| Worktrees | **Mandatory** for bead work when canonical root is contended — do not edit shared checkout |
| RPI CLI | `ao rpi` is **load-bearing legacy code**, heavily tested — not the live orchestration substrate (NTM + Agent Mail is) |
| Regen after inventory edits | `make regen-all` then `make regen-check` |

---

## What this repository is

AgentOps is the **operational layer for coding agents**: skills + the `ao` Go CLI + a repo-local `.agents/` corpus. It answers two questions before you grant more autonomy:

1. **Is the code right?** (validation membrane — tests, gates, council, vibe)
2. **Is the proof durable?** (evidence trail — verdicts, learnings, provenance)

The 3.0 identity is **hookless and in-session**. AgentOps does not ship a daemon, scheduler, or hosted control plane. Out-of-session always-on work is delegated to an external substrate (reference: NTM + MCP Agent Mail + `ao agent`).

Four product layers (public framing):

| Layer | Problem | Key surfaces |
|-------|---------|--------------|
| **Bookkeeping** | work vanishes between sessions | `.agents/`, RPI packets, council verdicts |
| **Context compiler** | agents start cold | `ao inject`, skills, execution packets |
| **Validation gates** | plausible ≠ correct | `/council`, `/vibe`, `/pre-mortem`, `ao gate` |
| **Knowledge flywheel** | lessons don't compound | `/forge`, promotion ratchet, `ao lookup` |

**Honest fitness posture:** the apparatus to measure corpus delta exists; live-agent uplift is **not yet proven**. See [AgentOps effectiveness evidence](../evals/agentops-effectiveness-evidence.md).

---

## Scale (approximate)

| Dimension | Size |
|-----------|------|
| Go source files | ~1,300+ |
| Active skills | 72 (`skills/`, excl. fixtures) |
| CLI top-level commands | ~88 |
| Gate checks (Go registry) | ~77 |
| Shell validation scripts | ~280 |
| Bats test files | ~139 |
| Claude workflows | 4 (`.claude/workflows/*.js`) |
| Registry capabilities | 172 (skills + commands + gate jobs) |

---

## Six bounded contexts

Product and code route through six DDD bounded contexts. Full routing: [Component Map](component-map.md). Contract: `docs/contracts/bounded-contexts.yaml`.

| BC | Name | Center of gravity |
|----|------|-------------------|
| **BC1** | Corpus | `.agents/`, `ao inject`, `/forge`, `/compile`, `/harvest` |
| **BC2** | Validation | `ao gate check`, `/validate`, `/council`, `/vibe` |
| **BC3** | Loop | operating loop, `/evolve`, `br`, goals, autodev |
| **BC4** | Factory | skill-builder, registries, standards, dispositions |
| **BC5** | Runtime | CLI, installers, plugin manifests |
| **BC6** | Orchestration | NTM, Agent Mail, swarm — **substrate boundary** |

```text
BC3 Loop ──▶ BC1 Corpus (compounding context)
          ──▶ BC2 Validation (proof before land)
BC4 Factory ──▶ skills/registries
BC5 Runtime ──▶ ao CLI + plugins
BC6 Orchestration ──▶ dispatches whole skills (never decomposes RPI internals)
```

---

## Directory map

| Path | Owns |
|------|------|
| `skills/` | **Skill SSOT** — `SKILL.md`, references, Gherkin `.feature` acceptance |
| `skills-codex/` | Checked-in Codex runtime twins (72); maintained with refresh scripts |
| `skills-codex-overrides/` | Durable Codex tailoring when runtime must diverge |
| `cli/` | Go control plane — `cmd/ao/`, `internal/`, gates, corpus, RPI legacy |
| `scripts/` | Validation, regen, release (~280 shell tools) |
| `tests/` | Bats gate tests, integration, e2e, docs validation |
| `schemas/` | JSON schemas for config, provenance, packets |
| `docs/` | Narrative architecture, ADRs, contracts, MkDocs site |
| `.agents/` | **Runtime knowledge** (gitignored) — learnings, council, RPI queue |
| `_beads/` | **Private br ledger** (nested git repo) — never `git add _beads` |
| `.beads/` | Legacy bd/Dolt config — preserved, not authoritative |
| `registry.json` | Generated SKU catalog — **do not hand-edit** |
| `.claude/workflows/` | Claude-only workflow scripts (kind: `workflow`) |
| `.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/` | Runtime install manifests |

---

## Source-of-truth precedence

When narrative docs disagree with executable behavior:

1. **Executable + generated** — `cli/**`, `scripts/**`, `cli/docs/COMMANDS.md`, generated registries
2. **Contracts** — `skills/**/SKILL.md`, `schemas/**`, `docs/contracts/**`
3. **Narrative** — `docs/**`, `README.md`, `AGENTS*.md`

Report mismatches; do not silently follow stale docs.

---

## The active waist (what actually runs today)

```text
ao session bootstrap          # explicit orientation (replaces hook injection)
        ↓
ao inject / ao corpus inject  # decay-ranked prior context
        ↓
Operating loop                # BDD → br bead → slice → TDD → validate
        ↓
ao gate check --fast --scope head   # local cockpit gate (BC2)
        ↓
git push → main               # rebase-on-reject; no routine PR wall
        ↓
validate.yml (optional)       # CI backstop on tags, PRs, manual dispatch
```

### Primary CLI commands (active)

| Command | Role |
|---------|------|
| `ao session bootstrap` | Universal init prompt for any agent runtime |
| `ao inject` | Knowledge injection from `.agents/` corpus |
| `ao corpus inject` | Typed BC1 corpus reader path |
| `ao gate check --fast` | Pre-push release authority |
| `ao gate check --full` | CI-parity local proof |
| `ao codex *` | Non-interactive Codex execution lane |
| `ao mcp serve` | Managed Agents JSON-RPC (curated tool surface) |
| `ao goals measure` | Fitness against `GOALS.md` |
| `ao doctor` | Self-healing cockpit |
| `ao capabilities` / `ao robot-docs` | Machine-readable CLI contract |

Full surface: generated [`cli/docs/COMMANDS.md`](../../cli/docs/COMMANDS.md).

### Legacy but load-bearing (do not delete casually)

| Surface | Status |
|---------|--------|
| `ao rpi phased/loop/serve/stream` | Compiled, heavily tested; **not** live orchestration path |
| `scripts/pre-push-gate.sh` | Bash escape hatch — `AGENTOPS_GATE_BASH=1` only |
| Gas City (`runtime=gc`) | **Removed** — bridge deleted |
| In-repo daemon/scheduler | **Removed** — ADR-0009 |

**Navigation rule:** [Operating Loop](operating-loop.md) is primary navigation for *how work flows*. `/rpi` is one turn's executor skill — not the primary substrate. Live multi-agent orchestration runs on NTM + Agent Mail under `.agents/agent-constitution.md`.

---

## RPI terminology

These names overlap in docs and code but are **not contradictory**. Use this table when routing:

| Term | Status | Meaning |
|------|--------|---------|
| Operating loop | Primary navigation | Seven-move doctrine in [operating-loop.md](operating-loop.md) — how work flows in-session |
| `/rpi` skill | Live inner loop | One-turn executor skill; not primary navigation |
| `ao rpi` CLI | Legacy lane | Load-bearing compiled code and tests; not the live workflow driver |
| Substrate dispatch | Out-of-session | NTM/ATM runs a whole `ao rpi` loop as one dispatched unit |

---

## Skills ecosystem

### Structure per skill

```text
skills/<slug>/
  SKILL.md              # invocable contract (frontmatter + body)
  references/           # overflow, templates, *.feature acceptance
  scripts/              # validate.sh, helpers (optional)
```

Frontmatter carries DDD/hex edges (`hexagonal_role`, `consumes`, `produces`, `context_rel`), practice lineage (`practices: [slug]`), and tier metadata.

### Three drift-gated registries

Edit **sources**, then regenerate:

```bash
make regen-all      # write mode
make regen-check    # drift gate (CI uses this)
```

| Registry | Sources | Generated |
|----------|---------|-----------|
| Skills | `skills/**/SKILL.md` | `registry.json`, `docs/contracts/context-map.md`, domain map |
| Workflows | `.claude/workflows/*.js` + `skill-dispositions.yaml` | `registry.json` workflows[] |
| CLI | `cli/cmd/ao/` | `cli/docs/COMMANDS.md`, `docs/cli-surface.{json,md}` |

Curated routers (editable, gated for refs): [`docs/SKILLS.md`](../SKILLS.md), [`skills/SKILL-TIERS.md`](../../skills/SKILL-TIERS.md), [`docs/contracts/skill-dispositions.yaml`](../contracts/skill-dispositions.yaml).

Retirement: `ao skills retire <slug> [--into <target>]` — flips ledger to `historical`, regens, scans for stale refs.

---

## Gate architecture

The Go gate (`cli/internal/gates/`) is the **routine release authority** (ag-qidx push-to-main model).

```text
Check { ID, Tiers (Fast|Full), Match[] globs, Blocking, Backing | Run }
  → Registry (~77 checks in checks/seed.go)
  → Orchestrator (serial; changed-file routing in Fast mode)
  → Report (PASS / WARN / FAIL / SKIP)
```

**Fast mode** (`--fast --scope head`): runs always-on structural checks + checks whose `Match` globs hit changed files.

**Full mode** (`--full`): all registry checks — CI parity lane.

**Implementation styles:**
- Shell-backed → `scripts/check-*.sh` (majority)
- Native Go → `go.build`, `go.vet`, `learning.coherence`, etc.

**Triple orchestration (migration in progress):**
1. Go gate — primary, growing
2. YAML purpose jobs in `.github/workflows/validate.yml` — CI backstop
3. Bash monolith `scripts/pre-push-gate.sh` — legacy escape hatch only

Local commands:

```bash
ao gate check --fast --scope head                    # routine pre-push
ao gate check --full --workflow-coverage             # CI-parity proof
cd cli && go test ./internal/gates/checks -count=1   # registry parity tests
```

---

## Knowledge flywheel

```text
Work → /forge → .agents/learnings/pending/
     → pool score → promote (gold/silver/bronze)
     → inject (decay-ranked, cited) → next session
     → gates enforce promotion ratchet
```

| Surface | Path | Tracked in git? |
|---------|------|-----------------|
| Runtime corpus | `.agents/` | No (policy) |
| Findings ledger | `.agents/findings/registry.jsonl` | No |
| RPI work queue | `.agents/rpi/next-work.jsonl` | No |
| Provenance | `docs/provenance/ledger.jsonl` | **Yes** — append-only, ledger wins over tracker |
| Goals fitness | `GOALS.md` | Yes |

Write contract for `.agents/` surfaces: [`docs/contracts/agents-write-surfaces.md`](../contracts/agents-write-surfaces.md).

Doctrine import (not locally proven): [Effective Feedback Compute](../doctrine/effective-feedback-compute.md).

---

## Documentation altitude stack

Read the right layer for your question:

| Altitude | Files |
|----------|-------|
| **Router** | `AGENTS.md`, `CLAUDE.md` (≤250 lines; pointers only) |
| **Tiered operator detail** | `AGENTS-WORKFLOW.md`, `AGENTS-CI.md`, `AGENTS-CODEX.md`, `AGENTS-RUNTIME.md` |
| **Contributor onboarding** | [`newcomer-guide.md`](../newcomer-guide.md), this page |
| **Doctrine** | [`3.0.md`](../3.0.md), [`operating-loop.md`](operating-loop.md), `GOALS.md`, `PRODUCT.md` |
| **Deep workflow sidecar** | [`agent-workflow-reference.md`](../agent-workflow-reference.md) |
| **Full catalog** | [`documentation-index.md`](../documentation-index.md) |

---

## Work lifecycle (contributor path)

1. **Claim** — `BEADS_DIR=$PWD/_beads br ready` → `br update <id> --claim`
2. **Scope** — read bead acceptance (`.feature` or embedded `## Scenarios`)
3. **Implement in worktree** — `git worktree add wt-<bead-id> -b <type>/<bead-id>-<slug>`
4. **Verify** — `ao gate check --fast --scope head` (+ targeted tests for touched surfaces)
5. **Land** — push to `main`; cockpit pre-push hook runs gate; rebase-on-reject on conflict

Branch shape, provenance trailers, and session scope rules: [`AGENTS-WORKFLOW.md`](../../AGENTS-WORKFLOW.md).

---

## Known footguns (read before you edit)

These recur in audits and findings registries:

| Footgun | Correct behavior |
|---------|------------------|
| Editing `~/.claude/skills/` | Edit `skills/` in **this repo** only |
| Using `bd` / Dolt | Use **`br`** with `BEADS_DIR=$PWD/_beads` — bd is retired legacy |
| Editing canonical root under swarm load | Use a **worktree** per bead |
| `git add _beads` | Never — private nested repo; sync with `git -C _beads push` |
| Hand-editing `registry.json` or context-map | Run `make regen-all` from source edits |
| Assuming CI blocks every `main` push | **Local gate** is routine authority; CI is backstop |
| Treating `/rpi` as the live orchestration substrate | NTM + Agent Mail for out-of-session; operating loop for in-session navigation |
| Running `claude -p` / `claude --print` | **Forbidden** — burns quota; use Codex exec or interactive panes |
| Trusting narrative over executable behavior | Check `cli/`, generated docs, and gates first |

Some older docs (`ARCHITECTURE.md`, `ports-and-adapters.md`) still mention hooks, bd, or PR-per-change — treat as historical unless reconciled. `cli/AGENTS.md` is a pointer stub to root `AGENTS.md` (Wave A).

---

## Strengths and open debt

**Strengths**
- Clear 3.0 product identity (validation-centered, hookless, in-session)
- Declarative gate registry with Fast/Full tiers and workflow parity tests
- Skill factory with disposition ledger and multi-runtime Codex parity
- Hexagonal CLI with strong agent ergonomics (`capabilities`, `robot-docs`, `--json`)
- Honest eval posture — refuses to market ahead of measured uplift

**Open debt**
- Gate migration: collapse YAML/bash triple orchestration into Go registry (Wave E drained 2 deferred scripts; 11 remain)
- Doc reconciliation: Wave C landed; `docs/3.0-readiness.md` checklist items still open
- Disposition debt: ~34 skills marked update/refactor — triage checklist at `docs/audits/2026-06-16-skill-disposition-triage.md`
- Worktree hygiene: 27 merge-eligible worktrees in dry-run — audit at `docs/audits/2026-06-16-worktree-disposition-audit.md`; no `--apply` without human ACK

---

## Recommended reading order

### Zero context (first session)

1. [`newcomer-guide.md`](../newcomer-guide.md)
2. **This page**
3. [`3.0.md`](../3.0.md)
4. [`operating-loop.md`](operating-loop.md)
5. [`component-map.md`](component-map.md)
6. [`AGENTS-WORKFLOW.md`](../../AGENTS-WORKFLOW.md) + [`AGENTS-RUNTIME.md`](../../AGENTS-RUNTIME.md)

### By task

| Task | Read |
|------|------|
| Pick a skill | [`SKILLS.md`](../SKILLS.md), [`skills-decision-tree.md`](../skills-decision-tree.md) |
| CLI behavior | [`cli/docs/COMMANDS.md`](../../cli/docs/COMMANDS.md) |
| CI / gates | [`AGENTS-CI.md`](../../AGENTS-CI.md), [`agent-workflow-reference.md`](../agent-workflow-reference.md) |
| Codex parity | [`AGENTS-CODEX.md`](../../AGENTS-CODEX.md) |
| Fitness / honesty | [`GOALS.md`](../../GOALS.md), [`evals/agentops-effectiveness-evidence.md`](../evals/agentops-effectiveness-evidence.md) |
| `.agents/` writes | [`contracts/agents-write-surfaces.md`](../contracts/agents-write-surfaces.md) |

---

## Related pages

- [Operating Loop](operating-loop.md) — seven-move doctrine (primary navigation)
- [Canonical Loop Model](canonical-loop-model.md) — one loop body, two drivers
- [Component Map](component-map.md) — where new work goes
- [Ports and Adapters](ports-and-adapters.md) — hexagonal seams (some tracker wording predates br)
- [Knowledge Flywheel](../knowledge-flywheel.md) — compounding mechanics
- [Dependencies](../dependencies.md) — required vs optional tools
