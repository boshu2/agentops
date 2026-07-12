# Skills Matrix

> Every AgentOps skill placed on the **operating loop** from intent to validated
> code. Skills are the product front door; this matrix is the map.
>
> **Read first:** [Intent → Validated Code](architecture/intent-to-validated-code.md).
> **Discipline:** [Operating Loop](architecture/operating-loop.md).
> **Router ("what do I run?"):** [SKILLS.md](SKILLS.md).
> **Tiers (editorial taxonomy):** [SKILL-TIERS.md](../skills/SKILL-TIERS.md).
>
> Inventory count comes from `registry.json` (generated from `skills/**/SKILL.md`).
> Do not hard-code totals in prose; when you add/retire a skill, update this
> matrix in the same change.

## How to read the matrix

| Column | Meaning |
|--------|---------|
| **P** | Primary — this move's default skill(s) |
| **S** | Supporting — usually run with or just before/after P |
| **O** | Optional / escalate — only when stakes or scale require it |
| **—** | Not this move's concern |

Membrane skills (**P** on move 6) still require a behavior contract from moves
1–4. Validating without Gherkin/ATDD is taste, not acceptance.

---

## Loop spine matrix (moves 1–7)

| Skill | 1 Shape | 2 Track | 3 Slice | 4 TDD | 5 Wave | 6 Membrane | 7 Ratchet | Notes |
|-------|:-------:|:-------:|:-------:|:-----:|:------:|:----------:|:---------:|-------|
| **discovery** | P | S | S | — | — | — | — | Dense execution packet; ideate→research→plan handoff |
| **product** | P | — | — | — | — | — | — | PRODUCT.md / positioning intent |
| **goal-design** | S | S | — | — | — | — | — | Checked goal-design packets before discovery/plan |
| **goals** | S | — | — | — | — | O | S | Setpoint / measure; not day-one build path |
| **idea-genie** | S | — | — | — | — | — | — | Opportunity portfolio → discovery |
| **dueling-idea-genies** | O | — | — | — | — | O | — | Contested one-way-door decisions |
| **research** | S | — | — | — | — | — | — | Codebase / topic findings into plan |
| **codebase-recon** | S | — | — | — | — | — | — | Entry-to-test reconstruction |
| **reverse-engineer** | O | — | — | — | — | — | — | Authorized reverse-engineering |
| **plan** | P | S | P | — | S | — | — | Behavior-sized issues + waves + acceptance |
| **behavior-first-planning** | S | S | P | S | — | — | — | Gherkin → EXECUTED-red → acceptance-gated DAG |
| **beads-br** | — | P | S | S | S | S | S | Tracker; acceptance rides on the bead |
| **beads-bv** | — | S | S | — | S | — | — | Graph triage / bottlenecks |
| **pre-mortem** | S | — | S | — | — | S | — | Stress-test plan before build (also feeds membrane) |
| **implement** | — | — | — | P | S | — | — | One bead: RED → green → refactor |
| **test** | — | — | S | S | — | S | — | Test/coverage plans alongside implement |
| **refactor** | — | — | — | S | — | S | — | Safe refactors under green / own slice |
| **scope** | — | — | S | S | S | — | — | Frozen path guard during risky work |
| **crank** | — | — | — | S | P | S | — | Epic waves through the loop |
| **swarm** | — | — | — | — | P | — | — | Parallel agents; needs disjoint scopes |
| **rpi** | S | S | S | S | S | S | S | **One full tick** of the loop (meta orchestrator) |
| **validate** | — | — | — | — | — | P | S | PASS/WARN/FAIL vs acceptance; no verdict = not done |
| **council** | O | — | — | — | — | P | S | Multi-judge consensus under high stakes |
| **pawl-review** | — | — | — | — | — | P | — | Fresh-context lane → pawl / land evidence |
| **converge** | — | — | — | S | — | S | — | Fix → re-judge until agreement or BLOCK |
| **reality-check** | — | — | — | — | S | S | — | Mid-epic drift: code vs plan |
| **security** | — | — | — | — | — | S | — | Vuln/secrets/release security gate |
| **post-mortem** | — | — | — | — | — | S | P | Evidence + learning ratchet |
| **pattern-mining** | — | — | — | — | — | — | S | Recurring shapes → durable patterns |
| **operationalize** | — | — | — | — | — | — | O | Experimental: corpus → operator surfaces |
| **handoff** | — | S | — | — | — | — | S | Session continuity packet |
| **status** | — | S | — | — | S | — | S | Where am I / recover |
| **bootstrap** | S | S | — | — | — | — | — | First-time repo / AgentOps setup |
| **push** | — | — | — | — | — | S | — | Validate, commit, push discipline |
| **release** | — | — | — | — | — | S | S | Changelog / tag / release validation |
| **pr-prep** | — | — | — | — | — | S | — | PR body/commits when using PR flow |
| **doc** | S | — | — | — | — | S | S | Docs packs; often paired with product changes |
| **domain** | S | S | S | S | — | S | S | Ubiquitous language (library-ish knowledge) |
| **evolve** | O | O | O | O | O | O | O | Experimental outer loop (N ticks) |

---

## Session, meta, substrate, and tooling (off the critical spine)

These skills matter; they are not the default intent→validated-code path.

| Skill | Role | Use when |
|-------|------|----------|
| **automation-shape-routing** | Meta front door | Choosing inline vs fanout vs substrate before spawning agents |
| **agent-native** | Substrate lifecycle | Persistent factory workers (substrate-neutral) |
| **ntm** | Substrate | tmux swarm panes / robot APIs |
| **agent-mail** | Coordination | ≥2 writers — locks, inboxes, reservations |
| **codex-exec** | Orchestration | Codex-shaped execution adapter |
| **using-gc** | Substrate (opt-in) | Gas City city-shaped work — never auto-routed |
| **gc-membrane** | Library | Membrane pack close-door reference for GC |
| **ms** | Discovery of skills | Search/load skills across corpora |
| **cass** | Session mining | Mine past sessions for prompts/decisions |
| **heal-skill** / **skill-builder** / **workflow-builder** | Meta | Author/repair skills and workflows |
| **converter** / **agy-native** | Cross-vendor | Format / Gemini·AGY runtime bridges |
| **shared** / **standards** | Library | Contracts and coding standards loaded JIT |
| **scaffold** | Tooling | Project/CI scaffolds |
| **cc-hooks** | Tooling | Claude Code hooks (opt-in; product is hookless) |
| **dcg** / **sbh** / **rch** / **account-rotation** | Tooling | Safety, browser, remote workers, account rotation |
| **toil-mining** | Meta | Find repeated toil → automation candidates |

---

## Composition patterns (full flow)

| Work size | Skill sequence | Acceptance spine |
|-----------|----------------|------------------|
| **One behavior** | `/plan` → `/implement` → `/validate` | One Gherkin scenario → one RED test → membrane cites it |
| **One tick (wrapped)** | `/rpi "goal"` | Same loop; orchestrator owns re-plan |
| **Multi-bead epic** | `/discovery` → `/plan` → `/pre-mortem` → `/crank` → `/validate` → `/post-mortem` | Each bead carries scenarios; epic closes on roll-up |
| **Parallel wave** | `/plan` (disjoint scopes) → `/swarm` or `/crank` → `/validate` | Wave invalid if write scopes collide |
| **High-stakes close** | `/validate` + `/council` + `/pawl-review` → land | Independent judges + commit-bound verdict |
| **Unattended / city** | Substrate dispatches `/rpi` per bead (`/ntm`, `/using-gc`, …) | Loop invariants stay in skills — substrate does not re-encode them |

---

## Membrane row (what "validated" means)

| Input the membrane needs | Who produces it | What `/validate` does |
|--------------------------|-----------------|------------------------|
| Given/When/Then (or linked `.feature`) | `/plan`, `/behavior-first-planning`, bead body | Maps each scenario to fresh evidence |
| Runnable acceptance test | `/implement` / `/test` (ATDD) | Confirms RED-was-real then green |
| Diff / artifact under review | git / PR | Judges implementation against the contract |
| Independent judge (when required) | `/council`, `/pawl-review` | No self-grade; context_id ≠ author |

If scenarios and acceptance commands are missing, the honest outcome is **HOLD**
("no behavior to validate against") — not a vibe PASS.

---

## CLI vs skills

| Job | Prefer | Not |
|-----|--------|-----|
| Run the loop | Skills (`/plan`, `/implement`, `/validate`, `/rpi`, …) | Treating `ao verify` as the product |
| Track beads | `/beads-br` + `br` via `ao beads dir` | Ad-hoc chat memory |
| Release gate | `ao gate check --fast --scope head` | Green CI alone as done |
| Commit ratchet / ledger | `ao verify`, `ao provenance`, `ao land` | Skipping the skill membrane on the slice |
| Retrieve prior context | `ao lookup`, `ao search` | Pasting whole histories |

---

## Maintenance

1. New skill → add a row here + disposition row + tier in frontmatter; run `make regen-all`.
2. Retired skill → move to historical disposition; delete or strike the matrix row in the same PR.
3. Keep the short [Skill → loop-move map](architecture/operating-loop.md#skill--loop-move-map) in sync with the **P** column of the spine matrix above.
4. Router trees in `SKILLS.md` / `SKILL-ROUTER.md` must not invent a different product order than [Intent → Validated Code](architecture/intent-to-validated-code.md).
