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
| Active waist | triggered context → operating loop → `ao land <bead>` |
| Issue tracker | **br** (beads_rust) in `_beads/` — `BEADS_DIR="$(ao beads dir)" br <cmd>` until legacy `.beads/` retires |
| Skills SSOT | `skills/<slug>/SKILL.md` — never edit `~/.claude/skills/` |
| Deterministic proof | `ao gate check --fast --scope head` (Go registry in `cli/internal/gates/`); necessary evidence, not landing authority |
| Terminal landing | `ao land <bead>` binds the independent verdict, deterministic proof, and repository-selected delivery |
| CI | Backstop only — not routine landing authority for every `main` push |
| Hooks | AgentOps 3.0 ships **zero** hooks; context is pulled explicitly |
| `.agents/` | Runtime knowledge — **gitignored**; durable public truth goes to `docs/`, `GOALS.md`, or provenance ledger |
| Worktrees | **Mandatory** for every tracked edit intended to land: claim/create the bead, then use its linked worktree |
| RPI CLI | `ao rpi` was **removed in 3.0** (commit f61c5f0e7) — the RPI engine is gone; in-session navigation is the operating loop, out-of-session is NTM + Agent Mail |
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
| **Local context** | agents lose relevant evidence | triggered contracts, skills, execution packets |
| **Validation gates** | plausible ≠ correct | `/council`, `/validate`, `/premortem`, `ao gate` |
| **Learning ratchet** | lessons don't change future behavior | `/learn`, `/pattern-mining`, `/operationalize`, promotion ratchet |

**Honest fitness posture:** the apparatus to measure corpus delta exists; live-agent uplift is **not yet proven**. See [AgentOps effectiveness evidence](../evals/agentops-effectiveness-evidence.md).

---

## Current inventory

Use generated `registry.json`, `skills/catalog.json`, and
`cli/docs/COMMANDS.md` for current counts and command surfaces. This narrative
intentionally does not copy volatile inventory totals.

---

## Six bounded contexts

Product and code route through six DDD bounded contexts. Full routing: [Component Map](component-map.md). Contract: `docs/contracts/bounded-contexts.yaml`.

| BC | Name | Center of gravity |
|----|------|-------------------|
| **BC1** | Corpus | `.agents/`, `/pattern-mining`, `/operationalize` |
| **BC2** | Validation | `ao gate check`, `/validate`, `/council`, `/premortem` |
| **BC3** | Loop | operating loop, `/learn`, optional `/postmortem`, `/evolve`, `br`, goals |
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
| `skills-codex/` | Checked-in Codex runtime twins (61); maintained with refresh scripts |
| `skills-codex-overrides/` | Durable Codex tailoring when runtime must diverge |
| `cli/` | Go control plane — `cmd/ao/`, `internal/`, gates, corpus, RPI legacy |
| `scripts/` | Validation, regen, release (373 shell scripts) |
| `tests/` | Bats gate tests, integration, e2e, docs validation |
| `schemas/` | JSON schemas for config, provenance, packets |
| `docs/` | Narrative architecture, ADRs, contracts, MkDocs site |
| `.agents/` | **Runtime knowledge** (gitignored) — learnings, council, RPI queue |
| `_beads/` | **Private br ledger** (nested git repo) — never stage it from the public repo |
| `.beads/` | Pre-br bd config for this repo's tracking — preserved, not authoritative (bd/dolt itself is the gascity substrate store) |
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
Triggered context + operating loop   # BDD → br bead → slice → TDD → Validate → Learn
        ↓
ao land <bead>                      # fresh pawl verdict + deterministic gate + atomic landing
        ↓
validate.yml (optional)              # CI backstop on tags, PRs, manual dispatch
```

### Primary CLI commands (active)

| Command | Role |
|---------|------|
| `ao session bootstrap` | Explicit orientation output when the task calls for it; never an automatic startup ritual |
| `ao gate check --fast` | Deterministic pre-land evidence; not a substitute for the pawl verdict |
| `ao gate check --full` | CI-parity local proof |
| `ao goals measure` | Fitness against `GOALS.md` |
| `ao doctor` | Self-healing cockpit |
| `ao capabilities` / `ao robot-docs` | Machine-readable CLI contract |

Full surface: generated [`cli/docs/COMMANDS.md`](../../cli/docs/COMMANDS.md).

### Legacy but load-bearing (do not delete casually)

| Surface | Status |
|---------|--------|
| `ao codex *` | `legacy`-tagged archive; absent from the default spine |
| `ao inject`, `ao lookup`, `ao corpus *` | Tagged archives; `ao session bootstrap` is an explicit orientation command, never an automatic startup ritual |
| `ao mcp serve`, `ao agent` | `legacy`-tagged optional substrate surfaces; absent from the default spine |
| `ao rpi phased/loop/serve/stream` | **Removed in 3.0** (f61c5f0e7) — engine gone; use the operating loop in-session, NTM + Agent Mail out-of-session |
| `scripts/pre-push-gate.sh` | Bash escape hatch — `AGENTOPS_GATE_BASH=1` only |
| Gas City (`runtime=gc`) | **Removed** — bridge deleted. gc itself lives beside as a blessed coexisting substrate (owned fork; drive via `skills/using-gc` + `packs/agentops-membrane`) |
| In-repo daemon/scheduler | **Removed** — ADR-0009 |

**Navigation rule:** [Operating Loop](operating-loop.md) is primary navigation for *how work flows*. `/rpi` is one turn's executor skill — not the primary substrate. Explicitly requested multi-agent orchestration uses the tracked [NTM](../../skills/ntm/SKILL.md) and [Agent Mail](../../skills/agent-mail/SKILL.md) adapter contracts; [dependencies](../dependencies.md) owns the optional out-of-session substrate boundary.

---

## RPI terminology

These names overlap in docs and code but are **not contradictory**. Use this table when routing:

| Term | Status | Meaning |
|------|--------|---------|
| Operating loop | Primary navigation | Seven-move doctrine in [operating-loop.md](operating-loop.md) — how work flows in-session |
| `/rpi` skill | Live inner loop | One-turn executor skill; not primary navigation |
| `ao rpi` CLI | Removed (3.0) | Engine **removed** in f61c5f0e7 — was a load-bearing legacy lane; superseded by the operating loop (in-session) and NTM + Agent Mail (out-of-session) |
| Substrate dispatch | Out-of-session | `/agent-native` drives portable roles through NTM and uses Agent Mail only for contested writes; GC remains an explicit coexisting adapter (the `ao rpi` loop once wrapped here was **removed** in 3.0) |

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

The Go gate (`cli/internal/gates/`) is the deterministic evidence engine used by
the landing path. It establishes mechanical facts; it does not replace the
exact-candidate independent verdict or become landing authority by itself.

```text
Check { ID, Tiers (Fast|Full), Match[] globs, Blocking, Backing | Run }
  → Registry (declared checks in `checks/seed.go`)
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

## Post-verdict learning and optional compounding

```text
Validate verdict → /learn receipt → orchestrator decision
                 → optional /postmortem for a causal question
                 → /pattern-mining → /operationalize when recurrence earns promotion
                 → optional archive lookup surfaces when deliberately selected
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
| **Triggered operator detail** | `docs/agent-workflow-reference.md`, `docs/CI-CD.md`, `docs/contracts/codex-skill-api.md`, `docs/contracts/repo-execution-profile.md` |
| **Contributor onboarding** | [`newcomer-guide.md`](../newcomer-guide.md), this page |
| **Doctrine** | [`3.0.md`](../3.0.md), [`operating-loop.md`](operating-loop.md), `GOALS.md`, `PRODUCT.md` |
| **Deep workflow sidecar** | [`agent-workflow-reference.md`](../agent-workflow-reference.md) |
| **Full catalog** | [`documentation-index.md`](../documentation-index.md) |

---

## Work lifecycle (contributor path)

1. **Claim** — `BEADS_DIR="$(ao beads dir)" br ready --json` → `BEADS_DIR="$(ao beads dir)" br update <id> --claim --json`
2. **Scope** — read bead acceptance (`.feature` or embedded `## Scenarios`)
3. **Implement in worktree** — `git worktree add wt-<bead-id> -b <type>/<bead-id>-<slug>`
4. **Verify** — `ao gate check --fast --scope head` (+ targeted tests for touched surfaces)
5. **Land** — `ao land <bead>` obtains the exact-candidate pawl verdict, runs
   the deterministic gate, rebases, and performs the atomic repository-selected
   delivery. REFUTED or NO-VERDICT stops the transition.

Branch shape, provenance trailers, and session scope rules: [`agent-workflow-reference.md`](../agent-workflow-reference.md).

---

## Known footguns (read before you edit)

These recur in audits and findings registries:

| Footgun | Correct behavior |
|---------|------------------|
| Editing `~/.claude/skills/` | Edit `skills/` in **this repo** only |
| Using `bd` / Dolt for this repo's tracking | Use **`br`** with `BEADS_DIR="$(ao beads dir)"` — bd/dolt is the gascity substrate store, a different layer, not this repo's tracker |
| Editing canonical root under swarm load | Use a **worktree** per bead |
| Staging the private ledger from the parent repo | Never — private nested repo; sync with `git -C "$(ao beads dir)" push` |
| Hand-editing `registry.json` or context-map | Run `make regen-all` from source edits |
| Assuming a green local gate authorizes landing | `ao land <bead>` requires both the exact-candidate verdict and deterministic proof; CI remains a backstop |
| Treating `/rpi` as the live orchestration substrate | NTM + Agent Mail for out-of-session; operating loop for in-session navigation |
| Running `claude -p` / `claude --print` | **Forbidden** — burns quota; use Codex exec or interactive panes |
| Trusting narrative over executable behavior | Check `cli/`, generated docs, and gates first |

`ports-and-adapters.md` is the canonical hexagonal seam map. Treat conflicting
historical wording elsewhere as stale and reconcile it against that owner.
`cli/AGENTS.md` is a pointer stub to root `AGENTS.md` (Wave A).

---

## Strengths and open debt

**Strengths**
- Clear 3.0 product identity (validation-centered, hookless, in-session)
- Declarative gate registry with Fast/Full tiers and workflow parity tests
- Skill factory with disposition ledger and multi-runtime Codex parity
- Hexagonal CLI with strong agent ergonomics (`capabilities`, `robot-docs`, `--json`)
- Honest eval posture — refuses to market ahead of measured uplift

**Open debt**
- Gate migration debt is owned by the live Go registry and CI contract; use
  `cli/internal/gates/checks/seed.go` and `docs/contracts/ci-jobs.yaml` instead
  of copying a point-in-time script count here.
- Doc reconciliation: Wave C landed; `docs/3.0-readiness.md` checklist items still open
- Disposition debt is owned by `docs/contracts/skill-dispositions.yaml`; do not
  copy its volatile count into narrative docs. Historical triage context lives
  at `docs/audits/2026-06-16-skill-disposition-triage.md`.
- Worktree hygiene is measured live with `git worktree list` and
  `scripts/check-worktree-disposition.sh`; the dated audit at
  `docs/audits/2026-06-16-worktree-disposition-audit.md` is historical evidence,
  not a current count. Never use `--apply` without human acknowledgment.

---

## Recommended reading order

### Zero context (first session)

1. [`newcomer-guide.md`](../newcomer-guide.md)
2. **This page**
3. [`3.0.md`](../3.0.md)
4. [`operating-loop.md`](operating-loop.md)
5. [`component-map.md`](component-map.md)
6. [`agent-workflow-reference.md`](../agent-workflow-reference.md) + [`repo-execution-profile.md`](../contracts/repo-execution-profile.md)

### By task

| Task | Read |
|------|------|
| Pick a skill | [`SKILLS.md`](../SKILLS.md), [`skills-decision-tree.md`](../skills-decision-tree.md) |
| CLI behavior | [`cli/docs/COMMANDS.md`](../../cli/docs/COMMANDS.md) |
| CI / gates | [`CI-CD.md`](../CI-CD.md), [`agent-workflow-reference.md`](../agent-workflow-reference.md) |
| Codex parity | [`codex-skill-api.md`](../contracts/codex-skill-api.md) |
| Fitness / honesty | [`GOALS.md`](../../GOALS.md), [`evals/agentops-effectiveness-evidence.md`](../evals/agentops-effectiveness-evidence.md) |
| `.agents/` writes | [`contracts/agents-write-surfaces.md`](../contracts/agents-write-surfaces.md) |

---

## Related pages

- [Operating Loop](operating-loop.md) — seven-move doctrine (primary navigation)
- [Canonical Loop Model](canonical-loop-model.md) — one loop body, two drivers
- [Component Map](component-map.md) — where new work goes
- [Ports and Adapters](ports-and-adapters.md) — canonical hexagonal seams and current tracker/legacy-adapter boundary
- [Knowledge Flywheel](../knowledge-flywheel.md) — compounding mechanics
- [Dependencies](../dependencies.md) — required vs optional tools
