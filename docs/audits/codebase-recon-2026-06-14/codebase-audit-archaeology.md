# Codebase Archaeology — AgentOps v3.1.0 → HEAD release window

**Mode:** archaeology (review skill / REPORT-MODES.md)
**Date:** 2026-06-14
**Scope (constrained):** ONLY `git diff v3.1.0..HEAD` — not the whole repo.
**Range:** `v3.1.0` = `c98836977` (2026-06-10) → `HEAD` = `ab6039808`
**Scale:** 147 commits · 1602 files changed · +37,404 / −109,471 (large net deletion)
**Build state at HEAD:** `cd cli && go build ./...` → **exit 0** (clean)

> This is an archaeology report on a *release window*, not on a whole repo. The
> goal is a working mental model of **what changed in this window, why, and what
> risk it moved** — so a non-author can read it cold and know what shipped between
> 3.1.0 and now.

---

## Executive summary

This window is dominated by **subtraction, not addition.** The net −72K lines is
real: the headline event is a **skill-corpus prune** (105 → 65 → settling at 73
live skill dirs) that folded ~104 sibling skills into a small set of generalized
targets and deleted their Codex/Gemini twins (`skills-codex/`, `images/gemini/`).
Everything else rides on top of that contraction.

The additive work clusters into four coherent arcs:

1. **A real verification spine** — the SDLC provenance ledger
   (`docs/provenance/ledger.jsonl` + `ao provenance verify` + a tamper-evident CI
   gate) and the **context-quorum** rewrite (the cross-model gate flipped from a
   "family floor" to a "fresh-context floor").
2. **A convergence loop** — `ao converge`, a bounded judge-panel loop with a
   two-sided canary entry gate, plus the `converge` / `pre-land-refuters` skills.
3. **Contracts/DDD hardening** — workflows promoted to a first-class,
   drift-gated registry surface; a new **BC6 Orchestration** bounded context;
   additive artifact-classification schema; a schema-validated disposition ledger.
4. **A doctrine reframe** — `GOALS.md` / `PRODUCT.md` rewritten around the
   **"navigator for stochastic agentic work"** north star and the
   **AgentOps-standalone / Mount Olympus-optional-extension** separation of
   duties (one-way dependency MTO → AgentOps).

The dominant *archaeological texture* of the window is **regeneration churn from a
big merge**: the `ag-xwjlc` seams epic alone is 16 commits, ~11 of them
`chore(codex): rebase-N splice repair / re-register twins / regen` — the cost of
landing a 4-skill clean-room addition through a hot, twin-mirrored, generated-
registry repo. That churn is the clearest risk signal in the window (below).

---

## What changed — by magnitude (churn ≠ importance, but it orients)

| Surface | Files | Churn (ins+del) | What |
|---|---|---|---|
| `skills/` | 670 | 77,719 | The prune: ~104 skills deleted, 8 added, folds + regen |
| `skills-codex/` | 594 | 22,060 | Codex twins follow the prune (deleted/regenerated) |
| `docs/` | 112 | 17,128 | Contracts (dispositions, context-map), plans, doctrine, this audit lineage |
| `cli/` | 57 | 7,860 | converge, provenance verify, skills_retire, codex_schema, quorum |
| `registry.json` | 1 | 5,669 | Generated — full re-emit after prune + new fields |
| `.agy-plugin/` | 19 | 5,404 | Gemini plugin skills follow the prune |
| `scripts/` | 35 | 2,829 | New gates: workflow-governance, provenance, disposition-schema, skill-path resolve |
| `images/` | 18 | 2,803 | Worker-image skill manifests follow the prune; new codex task-packet/run-receipt |
| `tests/` | 35 | 1,425 | New bats for the gates above; skill-count SSOT test |
| `schemas/` | 2 | 506 | New `codex-task-packet` + `codex-run-receipt` schemas |

Commit type mix: **52 feat · 26 fix · 23 docs · 20 chore · 4 merge · 2 test.**
The high `chore` count is almost entirely post-rebase regeneration of generated
inventory maps — a tell that this window pushed several large changes through a
repo whose registries are machine-generated and twin-mirrored.

---

## The five arcs, with archaeology

### 1. The skill prune (the −72K story) — `ag-s43tg`, `ag-xwjlc`

- **Phase 2 prune** (`94db74318`, breaking): folded 39 sources into 21 targets,
  cut `reverse-engineer-rpi`, **105 → 65**. The disposition ledger was *not*
  truncated — retired skills flip to "terminal historical rows" (42 of them) so
  the cull is auditable, not amnesiac. All factory surfaces regenerated
  (registry, domain map, context map, Codex twins + hashes, overrides catalog,
  SKILL-TIERS, count diagrams). `using-agentops` retired as a BC5 carve-out.
- **Deleted (sample of ~104):** all the `codebase-*` siblings (archaeology,
  audit, briefing-report, pattern-extraction, report, risk-audit) folded into
  `review`; `bug-hunt`, `ubs`, `vibe`, `complexity`, `trace`, `ratchet`,
  `scenario`, `brainstorm`, `design`, the entire `rust-*` cluster, the `cc-*`
  cluster, `agy-*` cluster, `gh-*` cluster, `process-triage` (skill), etc.
- **Added (8):** `account-rotation`, `codex-approval`, `continuity-loop`,
  `converge`, `operationalize`, `pre-land-refuters`, `reality-check`,
  `toil-mining`.
- **Settling:** the seams epic `ag-xwjlc` re-added 4 clean-room skills and the
  count walked 65 → 67 → 70 → 71; HEAD has **73 live `skills/*/` dirs**.

**Why:** this realizes the memory-rule "agentops skills/ are the product — cull =
count" (166 → 114 → 105 → ~73, target 77). The product surface is the skill
corpus; fewer-but-more-general beats many-but-narrow.

**Archaeological note:** the prune was *not* a flat delete. `ao skills retire`
(`cli/cmd/ao/skills_retire.go`, new this window) was authored precisely because
removal ripples ~15 hand-maintained surfaces — the tooling was built to make the
cull mechanical and twin-consistent rather than a manual swarm.

### 2. The verification spine — `ag-8jf97`, `ag-slice1*`

- **Provenance ledger** (`479891017`): `docs/provenance/ledger.jsonl` was
  *declared* the append-only SOT in CLAUDE.md but **never actually existed** — a
  "doctrine-level lying instrument." This window poured the real slab: genesis
  event seeded, `ao provenance verify` added (verifies the *committed* chain in
  place — catches a tampered field / forged hash / reordered row and names the
  offending file line via `provenancegraph.Store.VerifyFile()`), and a blocking
  T1 CI gate (`scripts/validate-provenance-ledger.sh --gate`).
- **Context-quorum** (`7e64b9299`, `a860589f9`, `afb3535dc`): the cross-model
  acceptance floor was flipped from **family-floor** (≥2 model *families*) to a
  **fresh-context floor** (independence by fresh context, not just by vendor).
  New `ContextID` is first-class in the council gate; `quorum_canon_test.go`
  pins the canon. This is a genuine *semantics change* to the trust gate.

**Why:** both close the same gap — the repo had *claims* of verification
(provenance SOT, multi-family quorum) that weren't enforced or were enforcing the
wrong thing. The window made the claims true.

### 3. The convergence loop — `ag-slice2/3/4`, `ag-converge-hardening`

- `ao converge` (`16a67c1d4`): a **bounded** judge-panel convergence loop
  (bounded-by-design matters here — runaway loops are a named repo failure mode).
- **Two-sided canary entry gate** (`10b93c5d8`): proves the gate *bites* (rejects
  a known-bad) before a PASS is trusted — the "prove the gate works before
  trusting it" discipline, mirroring the provenance tamper tests.
- Thin `converge` skill memo over the CLI (`3e13ee4e6`) — keeps the determinism
  in Go, the skill as instructions (the repo's standing build rule).
- `afb3535dc` hardened context-quorum *from its own pre-land refuter findings* —
  i.e. the `pre-land-refuters` skill (new) caught real issues before landing.

### 4. Contracts / DDD hardening — `ag-jy8gj`, `ag-4akl8`, `ag-j3ge0`, `ag-2vz5v`

- **Workflows first-class** (`d4111ec9e`, #856): `.claude/workflows/*.js`
  promoted to a kind-discriminated registry surface with a **bidirectional
  `.js` ↔ ledger bijection** drift gate (forward: every `.js` has a ledger row
  with kind+BC+role; reverse: every `kind: workflow` row has a tracked `.js`,
  else STALE→FAIL). Claude-only — the gate never demands a Codex twin.
- **BC6 Orchestration** (`692f420ac`): a 6th bounded context; the 6
  orchestration skills (ntm/swarm/agent-mail/using-atm/vibing-with-ntm/
  continuity-loop) re-binned into it. Additive artifact-classification schema
  (kind/runtime_targets/parity_policy/capability_class/path/aliases/supersedes)
  backfilled onto all 71 active rows — *no rename*, quorum-ratified.
- **Schema-validated disposition ledger** (`validate-skill-disposition-schema.sh`
  + bats): the disposition YAML now has a rejecting schema gate.
- **Validators resolve skill paths via the dispositions ledger** (`85b7b97b1`,
  `ag-2vz5v`): removed hard-coded skill paths from validators (`resolve-skill-path`
  + 240-line bats) — a real decoupling so the next cull doesn't break validators.
- **errcheck** lint rule + fixes for unchecked `json.Marshal/Unmarshal`
  (`845b31446`, cross-family gated).

### 5. Doctrine reframe — `ag-6aqj3`, `ag-ol7ah`, `ag-qjccl`, head merge

- `GOALS.md` rewritten: from "an SDLC control plane for everything" (a *scope*)
  to **"AgentOps is the navigator for stochastic agentic work"** (a *destination*)
  — explicitly because the scope-framing let the backlog sprawl to 100+
  undifferentiated epics with nothing to prioritize against. Adds "The
  Destination" + Directive 16 (5 honest-status route milestones).
- **AgentOps standalone / MTO optional extension** (head merge `ab6039808`,
  Codex-authored, Claude cross-family reviewed): removes the "two factories,
  neither complete alone" co-dependence; AgentOps is a complete standalone
  product (own validation membrane, navigator, corpus, CDLC), MTO is the optional
  high-assurance extension, one-way dependency MTO → AgentOps.
- New write-surface contract `.agents/mto-handoff` (`ag-qjccl`); an AgentOps
  consumer reads the MTO recurrence file handoff → planning-rule, **off bd**
  (consistent with the bd retirement).

---

## Risk moved by this window

### Risk REMOVED
- **Doctrine-as-lie eliminated:** provenance ledger + tamper gate make a
  previously-fictional SOT real and enforced. High-value.
- **Cull is auditable:** retired skills become terminal historical rows, not
  deletions-without-trace; disposition schema gate prevents silent drift.
- **Validator/skill-path coupling cut:** future culls won't silently break
  validators (`ag-2vz5v`).
- **Surface area down ~72K lines:** less to maintain, fewer twin-mirror seams.
- **Quorum semantics corrected:** fresh-context independence is a stronger
  floor than vendor-family counting.

### Risk INTRODUCED / residual (P-rated)
- **P1 — Regeneration-churn fragility (the `ag-xwjlc` tell).** ~11 of 16 seams-
  epic commits are post-rebase splice repairs of generated marker JSONs / twin
  re-registration / full regens. Landing *any* skill change through this repo
  costs a long manual reconciliation of generated inventory maps + Codex twins.
  The new `ao skills retire` mitigates *removal* but not *addition/rebase*. This
  is the dominant maintainability cost in the window. **Next action:** measure
  whether twin generation + registry regen can be made a single idempotent
  command run in the pre-push gate (so rebase never leaves spliced markers).
- **P2 — Stray worktrees committed-adjacent in the repo root.** Six `wt-ag-*`
  worktrees sit in the working tree (`wt-ag-if7p`, `wt-ag-jy8gj`, `wt-ag-pj51`,
  `wt-ag-qidx`, `wt-ag-s43tg-gate-rehearsal`, `wt-ag-wi9w1`). Not in this diff,
  but they are the visible aftermath of this window's worktree-per-bead workflow
  — clutter + risk of an agent editing the wrong root. **Next action:** prune
  merged worktrees (`git worktree prune` + remove stale dirs).
- **P2 — Generated `registry.json` is a 5,669-line diff.** Large generated diffs
  are review-opaque; a hand-edit or a stale generator would be hard to catch in
  the noise. The new drift gates help, but the review surface is effectively
  "trust the generator." **Next action:** confirm `scripts/regen-all.sh --check`
  is a blocking gate (it should be) and that no generated file is ever hand-touched.
- **P3 — Lost reference content from the prune.** The prune body notes specific
  rescues ("vibe reference library + ship-loop anti-patterns + brainstorm
  ideation refs rescued into fold targets") — implying the *default* of a fold is
  reference loss unless explicitly rescued. Folds that didn't get a named rescue
  may have dropped reference material. **Next action:** spot-check 2-3 high-value
  folded skills (e.g. `codebase-risk-audit` → review) to confirm their method
  survived, not just their trigger.

### Residual (not inspected, by scope constraint)
- The behavior of the new `ao converge` loop under real multi-model load was not
  exercised here (read-only window audit; no runs).
- Codex/Gemini twin *correctness* post-prune was not byte-verified — only that
  the generators re-emitted them.
- The MTO-handoff consumer (`ag-qjccl`) was read as a contract, not run.

---

## Mental model for a newcomer (the window in one paragraph)

Between 3.1.0 and now, AgentOps **got smaller and more honest.** It cut ~104
narrow skills down to a generalized core (the product surface is the corpus, so
cull = count), built the tooling (`ao skills retire`) to make the cull
twin-consistent, and then made three things that were *claimed* but not *true*
actually enforced: the provenance ledger (now real + tamper-gated), the
cross-model quorum (now a fresh-context floor, not a vendor-family count), and
the workflow registry (now a bidirectional drift gate). It added a bounded
`ao converge` judge loop with a canary that proves the gate bites. Finally it
rewrote its own goal from "control plane for everything" to "**the navigator for
stochastic agentic work**," with Mount Olympus demoted from a co-equal factory to
an optional high-assurance extension. The cost was carried in regeneration churn:
landing changes through this twin-mirrored, generated-registry repo is expensive,
and the seams epic's eleven splice-repair commits are the receipt.

---

## Evidence index (key commits)

| Arc | Commit | Subject |
|---|---|---|
| Prune | `94db74318` | skill-corpus prune phase 2 (105→65) |
| Prune tooling | `skills_retire.go` (new) | `ao skills retire` |
| Provenance | `479891017` | SDLC provenance ledger + verify + tamper gate |
| Quorum | `7e64b9299` / `a860589f9` | family-floor → fresh-context floor |
| Converge | `16a67c1d4` / `10b93c5d8` | bounded judge loop + two-sided canary |
| Workflows | `d4111ec9e` (#856) | workflows first-class + bidirectional drift gate |
| BC6 | `692f420ac` | additive classification schema + BC6 Orchestration |
| Validator decouple | `85b7b97b1` | validators resolve skill paths via ledger |
| Doctrine | `b0b7ff1c2` / `ab6039808` | navigator goal + AgentOps-standalone/MTO-optional |
| Churn tell | `ag-xwjlc` ×16 | seams epic — ~11 post-rebase splice/regen commits |
