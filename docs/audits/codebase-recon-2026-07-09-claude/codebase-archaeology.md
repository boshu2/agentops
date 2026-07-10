# AgentOps — Codebase Archaeology (Technical Architecture Summary)

> Produced by the `codebase-archaeology` skill, 2026-07-09. Method: documentation-first
> (AGENTS.md → codebase-overview.md → README), then entry points → key types → data flow →
> config → integration → tests, with doc claims verified against executable surfaces.
> Strictly read-only pass; `cd cli && go build ./...` verified green at time of writing.

---

## Executive Summary

**AgentOps** is a *verification membrane for coding agents*: a skills corpus + a Go CLI (`ao`)
+ a repo-local evidence trail that together enforce one invariant — **no verdict = not done**.
An agent's change only counts as done after something that did not write it (a cross-family
model review or a deterministic gate) checks it and the verdict is bound into a hash-chained
provenance ledger. It is hookless and in-session by design (3.0 identity): no daemon, no
scheduler, no hosted control plane; out-of-session orchestration is delegated to an external
substrate (NTM + Agent Mail + `ao agent`, with Gas City as a blessed coexisting substrate).

Architecturally it is three cooperating planes:

1. **Skills plane** (`skills/` — the SSOT, mirrored to `skills-codex/` for Codex parity):
   markdown contracts that instruct the agent (the stochastic side).
2. **Deterministic control plane** (`cli/` — cobra CLI, ~74 documented top-level commands,
   ~80 internal packages): gates, ledgers, corpus, doctor — "CLI for deterministic, skills
   are instructions."
3. **Evidence plane**: `docs/provenance/ledger.jsonl` (tracked, hash-chained, 425 records),
   `_beads/` (private br tracker, nested repo), `.agents/` (gitignored runtime corpus).

**Key Statistics**

| Dimension | Measured (this pass) |
|---|---|
| Go source files | 1,445 (`cli/`), cmd package alone ~460K LOC incl. tests |
| Internal packages | ~80 under `cli/internal/` |
| Documented top-level `ao` commands | 74 (`cli/docs/COMMANDS.md`, generated) |
| Gate checks in Go registry | **101** (`cli/internal/gates/checks/seed.go`) |
| Skills with `SKILL.md` on disk | **59** (`skills/`), 58 Codex twins |
| Shell scripts | 345 (`scripts/*.sh`) |
| Bats test files | 257 (`tests/**`) |
| Claude workflows | 4 (`.claude/workflows/*.js`) |
| Provenance ledger records | 425 (`docs/provenance/ledger.jsonl`) |
| Language / toolchain | Go (cobra CLI), bash, markdown contracts, JSON Schema |

*Note: the repo is a shallow clone locally (history starts 2026-07-08), so long-range
git-churn archaeology is not possible from this checkout.*

---

## Entry Points

| Entry | Location | Notes |
|---|---|---|
| `ao` CLI | `cli/cmd/ao/main.go:11` → `Execute()` in `cli/cmd/ao/root.go:81` | cobra root; version ldflags-injected (fallback `3.2.0-rc`) |
| Root command wiring | `cli/cmd/ao/root.go:28` | `PersistentPreRunE` builds an `App` (dry-run/verbose/output/workdir) into context; repairs worktree git config on every invocation |
| Pre-push gate (live) | `scripts/hooks/pre-push.local` installed into `$(git-common-dir)/hooks` by `scripts/install-pre-push-gate.sh` | THE release authority: fresh `ao` build → full race suite → serialized gate/provenance/pawl checks. Only audited bypass: `AGENTOPS_GATE_DISABLED=1` |
| Tracked `.githooks/` | `.githooks/pre-push` | **Historical bd shim, not the live gate** — the file itself warns that `core.hooksPath` hijack can silence the real gate; installer detects and refuses |
| Helper binaries | `cli/cmd/skill-frontmatter-json/`, `cli/cmd/witness-crosscheck/` | Small side tools |
| Skills (agent-side) | `skills/<slug>/SKILL.md` | Invoked by the agent runtime (Claude/Codex/OpenCode/AGY), not by `ao` |
| Make targets | `Makefile` → `local-ci`, `regen-all`, `regen-check`, `build` | `regen-all` is the finalizer after any skill/command inventory edit |

A distinctive pattern lives at the root: **the exit code IS the verdict**. `Execute()`
(`root.go:81-183`) unwraps ~12 typed exit errors (`pawlReviewExitError`,
`verifyPrePushExitError`, `landExitError`, `gateExitError`, `governorExitError`, …), each
mapping a domain verdict to a process exit code (e.g. pawl: 0 CONFIRMED · 3 REFUTED ·
4 advisory · 2 usage; plan-pawl: 3 REDO · 4 BLOCKED). Machine callers never parse prose.

---

## Key Types (the ones everything revolves around)

| Type | Location | Purpose |
|---|---|---|
| `gates.Check` | `cli/internal/gates/gates.go:57` | One gate check: `ID`, `Tiers` (Fast\|Full), `Match` globs (changed-file routing), `Blocking`, `Backing` script **or** native `Run`, `RepairHint`. 101 seeded in `checks/seed.go` |
| SDLC provenance event | `cli/internal/drrebuild/drrebuild.go:78` (schema `schemas/agentops-sdlc-provenance.v1.schema.json`) | Hash-chained ledger record: `from_id → to_id` with `relation` (e.g. `wasGeneratedBy`), `prev_hash`/`payload_hash`/`hash` where `hash = sha256(payload_hash + "\n" + prev_hash)` — tamper-evident append-only chain in `docs/provenance/ledger.jsonl` |
| `verdictledger.Record` | `cli/internal/verdictledger/verdictledger.go:69` | Tagged union (iteration vs cooldown) binding scenario verdicts to GOALS.md directive IDs — the goals-fitness feedback loop |
| `provenance.Record` / `Graph` | `cli/internal/provenance/provenance.go:204` | Artifact↔source lineage for corpus outputs (sessions, indexes) — distinct from the SDLC chain above |
| Skill frontmatter | `skills/*/SKILL.md` (e.g. `skills/validate/SKILL.md`) | Declared contract: `hexagonal_role`, `consumes`/`produces`, `practices`, tier metadata, `output_contract` pointing at a JSON schema — skills are typed ports, not loose prose |
| `App` | `cli/cmd/ao/app.go` | Per-invocation context (flags, workdir) injected via cobra context |

Contracts are schema-first throughout: `schemas/` holds ~30 versioned JSON Schemas
(execution packets, eval runs, bead, claim registry, codex task packets, verdicts).

---

## Data Flow (the membrane, end to end)

```text
Agent makes a change (any runtime)
        │
        ▼
ao verify <change-id>                     # front door; THIN alias forwarding verbatim to…
  = ao pawl review ──► scripts/pawl-review.sh
        │                 cross-family refuter (codex — never the author's model; LAW 0:
        │                 no `claude -p`); --strict = two-family cold quorum (currently
        │                 honest-UNAVAILABLE: no second strict-eligible cold family)
        ▼
CONFIRMED ──► commit-bound verdict written to docs/provenance/ledger.jsonl
        │       (hash-chained; drrebuild verifies chain integrity)
        ▼
git push main ──► $(git-common-dir)/hooks/pre-push (scripts/hooks/pre-push.local)
        │            builds fresh ao → full randomized race suite →
        │            ao gate check (Go registry: 101 checks, Fast tier routes by
        │            changed-file globs; shell-backed via scripts/check-*.sh or native Go)
        │            → pawl verdict check (push to main REQUIRES the bound verdict)
        ▼
main (no PR wall; CI validate.yml is tag/PR/manual backstop, not routine authority)
        │
        ▼
Evidence/learning capture ──► .agents/ (gitignored) ──► promotion ratchet ──► ao lookup
```

Gate internals: `Registry → Orchestrator (serial; changed-files routing in Fast mode) →
scriptrunner | native_inline → Report (PASS/WARN/FAIL/SKIP)` — all under
`cli/internal/gates/`. Triple orchestration is acknowledged migration debt: Go registry
(primary) + `.github/workflows/validate.yml` (backstop) + `scripts/pre-push-gate.sh`
(legacy bash, `AGENTOPS_GATE_BASH=1` escape hatch only).

Tracking flow: work is a **bead** in the private `br` ledger (`_beads/`, nested repo,
resolved via `BEADS_DIR="$(ao beads dir)"`); `ao done` closes work with its verdict
attached; `bd`/Dolt is explicitly *not* this repo's tracker (it is the Gas City substrate
store — a different layer).

---

## Six Bounded Contexts (routing map)

| BC | Name | Center of gravity |
|---|---|---|
| BC1 | Corpus | `.agents/`, `ao inject`/`ao corpus`, compile/harvest — **experimental tier** (demoted under its own help group; ADR-0004/0011: compounding unproven) |
| BC2 | Validation | `ao gate`, `ao verify`/pawl, `/validate`, `/council` — **the proven product** |
| BC3 | Loop | operating loop, `br` beads, goals, autodev |
| BC4 | Factory | skill-builder, registries, dispositions, standards |
| BC5 | Runtime | CLI, installers, plugin manifests (`.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/`) |
| BC6 | Orchestration | NTM / Agent Mail / swarm — substrate boundary, dispatches whole skills |

The help-surface grouping in `root.go:init()` mirrors this honesty posture: corpus/flywheel
commands are grouped under "Experimental" so proven spine commands lead `ao --help`.

---

## Configuration

| Source | Detail |
|---|---|
| Config file | `~/.agentops/config.yaml` (global `--config` flag; `AGENTOPS_CONFIG` env override) — `cli/internal/config/config.go` (~37K, with env-key fallbacks per setting) |
| Env vars | `AGENTOPS_*` family: `AGENTOPS_GATE_DISABLED=1` (audited gate bypass), `AGENTOPS_GATE_BASH=1` (legacy gate), `AGENTOPS_HOOKS_DISABLED=1`, `AGENTOPS_COUNCIL_MODEL_TIER`, `PAWL_*` (reviewer chain, `PAWL_STRICT`, `PAWL_NO_SERVICE=1`) |
| Per-repo verify config | `.aoverify.yaml` (e.g. `strict: true`) via `cli/internal/verifycfg` |
| Global flags | `--dry-run`, `--json` / `-o json|table|yaml`, `--verbose` — `--json` on any read-side command; parent commands with `--json` emit machine-readable subcommand listings |

---

## External Dependencies / Integration Points

From `docs/dependencies.md` (classification is explicit and graceful-degradation-minded):

- **REQUIRED:** an agent runtime (`claude`/`codex`/`opencode`) + `git`. Everything else optional.
- **Verification lane:** `codex exec` as the cross-family cold refuter (LAW 0 forbids
  `claude -p`); `agy` (Gemini) A7-benched for routine/fallback only.
- **Tracking:** `br` + `bv` (beads_rust, git-JSONL). `bd`/Dolt = Gas City substrate store only.
- **Orchestration (opt-in):** NTM (tmux swarm), `mcp_agent_mail`, `ao mcp serve` (JSON-RPC),
  `ao agent` managed agents.
- **Utilities:** `gh`, `jq`, `rg`, `tmux`, `cm`/`cass` (memory), Go toolchain to build from source.
- **Distribution:** Homebrew tap, install scripts per runtime (Claude plugin marketplace,
  Codex/OpenCode/AGY installers, Windows PowerShell), goreleaser (`.goreleaser.yml`).

No hosted services, no telemetry: all state is repo-local files (JSONL ledgers, markdown).

---

## Test Infrastructure

- **Go:** ~700+ `_test.go` files colocated in `cli/`; push gate runs the **full race suite
  with `-shuffle=on`**; strict conventions in `.claude/rules/go.md` (L2-first, no
  coverage-padding, `t.Cleanup` restoration of shared cobra globals — a documented recurring
  flake class).
- **Bats:** 257 files under `tests/` (gate scripts, install, e2e, land-queue, windows,
  canaries, spec-consistency…). `tests/run-all.sh` orchestrates; `Dockerfile.e2e` for
  containered e2e.
- **Gate parity tests:** `cli/internal/gates/checks/parity_test.go` + `seed_test.go` keep the
  Go registry honest against workflow coverage (`workflow_coverage.go` maps checks ↔ CI jobs).
- **Evals:** `evals/` + `ao eval` subtree (scenario A/B, moat, outcomes ingest) — the
  instrument for the *unproven* corpus-uplift hypothesis; the repo refuses to market ahead
  of this ruler (`docs/evals/agentops-effectiveness-evidence.md`).

Tests encode intent unusually well here: e.g. guard-test fixtures must round-trip the real
persisted shape (fixture-fidelity rule), and `tests/scripts/pawl-review-emit-catch.bats`
pins the newest membrane evidence-extraction behavior.

---

## Doc-vs-Code Mismatches Found (source-of-truth precedence: code wins)

Per the repo's own precedence rule these are reportable drift, not blockers:

1. **Gate check count:** `docs/architecture/codebase-overview.md` says "~77 checks";
   `cli/internal/gates/checks/seed.go` seeds **101**. The overview's Scale table is stale.
2. **Skill count:** overview claims "Active skills 73 (excl. fixtures)"; disk has **59**
   `skills/*/SKILL.md` (58 Codex twins). Consistent with the 2026-07-06 66-skill disposition
   audit (retire/merge wave) not yet reflected in the overview.
3. **Bats count:** overview says ~139 bats files; disk has **257** under `tests/`.
4. **Shell scripts:** "~280 shell validation scripts" vs 345 `scripts/*.sh` on disk.
5. Overview itself flags the remainder honestly (e.g. `ARCHITECTURE.md` /
   `ports-and-adapters.md` carry pre-3.0 wording; `ao rpi` removed in f61c5f0e7).

None of these change the architecture story; all are Scale-table/router staleness in one
narrative doc, and the drift direction (more checks, fewer skills) matches the stated
doctrine: grow the deterministic gate, cull the skill surface.

---

## Footguns (verified against sources, condensed)

| Footgun | Correct behavior |
|---|---|
| Editing `~/.claude/skills/` | Edit `skills/` in this repo (symlinked factory checkout) |
| `bd` for this repo's tracking | `BEADS_DIR="$(ao beads dir)" br …` |
| `git add _beads` | Never — private nested repo; `git -C "$(ao beads dir)" push` |
| Hand-editing `registry.json`/generated maps | `make regen-all` from sources |
| Trusting `.githooks/pre-push` as the gate | Live gate is `git-common-dir/hooks` via installer; check `core.hooksPath` for hijack |
| Assuming CI blocks pushes | Local cockpit gate is routine release authority; CI is backstop |
| `claude -p` anywhere (incl. as pawl refuter) | Forbidden (LAW 0) — codex exec / AGY / local models |
| Editing canonical root under swarm load | Worktree per bead |

---

## Reading Order for the Next Agent

1. `AGENTS.md` (root contract; CLAUDE.md symlinks to it) → tiered `AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md`
2. `docs/architecture/codebase-overview.md` (map; note Scale-table staleness above)
3. `docs/architecture/operating-loop.md` (primary navigation — the seven moves)
4. `cli/cmd/ao/root.go` + `cli/internal/gates/` (the deterministic spine in code)
5. `cli/cmd/ao/verify.go` + `cli/cmd/ao/pawl.go` + `scripts/pawl-review.sh` (the membrane)
6. `cli/internal/drrebuild/drrebuild.go` + `schemas/agentops-sdlc-provenance.v1.schema.json` (the evidence chain)
