# Intent: Distill AgentOps to its core (the mind)

> RPI-shaped cleanup plan (intent → slices → beads → acceptance). 2026-06-06. Architecture context: `mt-olympus/docs/AGENTOPS-MTO-INTEGRATION.md` + `TWELVE-FACTOR-WHOLE-SYSTEM.md`. Do NOT abandon the repo (380 stars) — distill it.

## Feature
Re-shape AgentOps from what its README *claims* ("the SDLC control plane" — a title that belongs to **MTO**) into what it actually **is**: the worker's **mind** — the in-session loop + context compiler + the `.agents/` corpus — by relocating its orchestration sprawl to MTO and conforming its core to `agentops-core-sdk`, without breaking existing users.

## Why (the unlock)
The README mislabels AgentOps as the control plane, so the repo kept growing orchestration surface (264 command files: `autodev`, `batch_*`, `cron_self_adjust`, `pool`, `operator`, `orchestrate`…) to live up to a title that isn't its job. `docs/3.0.md` already says the truth ("the in-session loop and the context compiler that feeds it"). Fix the identity and every cleanup decision becomes obvious: **anything that isn't the mind relocates.**

## Bounded context
Repo-wide reorganization — executed as slices, one concern each (so each is one bead, per the operating-loop's "crosses contexts → multiple issues" rule).

## Acceptance examples (Gherkin)
```gherkin
Feature: AgentOps distilled to the mind

  Scenario: identity no longer claims the control-plane role
    Given the README and docs/3.0.md
    When a reader asks "what is AgentOps?"
    Then it answers "the in-session loop + context compiler — the worker's mind"
    And it does NOT call itself "the control plane" (that is MTO)

  Scenario: orchestration is relocated, not stranded
    Given a user who runs `ao autodev` / `ao cron` / `ao pool` / `ao operator` / `ao orchestrate`
    When they run it after the cleanup
    Then they get a deprecation pointer to MTO, never a silent break

  Scenario: the 380-star core is intact
    Given a user who came for in-session reliability + the corpus
    When they use `ao inject` / `ao compile` / the flywheel / the 81 skills
    Then everything works unchanged

  Scenario: the worker's verdict is a claim, not a binding verdict
    Given a worker runs `ao validate` / `ao gate` in-session
    Then it emits a claim + evidence — never a CriterionVerdict that closes a bead
    And only MTO/Themis (validator ≠ worker) writes the binding verdict

  Scenario: the core conforms to the contract
    Given the distilled core's evidence/verdict output
    Then it validates against agentops-core-sdk schemas (verdict / execution-packet / cycle-trace)

  Scenario: the mind tends and evolves itself (meta_skill)
    Given recent CASS sessions and the .agents/ corpus
    When meta_skill mines them
    Then it PROPOSES candidate skills + anti-patterns + prune suggestions (with evidence)
    And the AgentOps flywheel gate decides what gets PROMOTED into the runtime catalog
    And ms never writes a runtime Anthropic SKILL.md directly (it proposes; the gate promotes)
```

## Slices (vertical — each a bead, each with its own acceptance)
1. **Re-identify** *(non-breaking)* — rewrite README + `docs/3.0.md` lead: AgentOps = the worker's mind (in-session loop + context compiler + corpus); strike "control plane" (MTO's title). Acceptance: identity scenario.
2. **Relocate orchestration → MTO** *(deprecate-stub, don't hard-delete)* — `autodev`, `batch_*`, `cron*`, `pool*`, `operator`, `rpi serve` become stubs that point to MTO. **`ao orchestrate` is exempt** — it is the BC6 out-of-session instrument waist (`select`, `preflight`, `verify`, `tools`, `route`, `shape`); expand in-repo, do not stub to MTO. Acceptance: relocate scenario (orchestrate exempt).
3. **Resolve the ambiguous** *(split by scope)* — `eval` / `goals` / `defrag` / `notebook` / `beads`: this-session scope → keep; fleet/pool scope → relocate. Acceptance: each resolved + tested.
4. **Conform + lock the seam** — worker validation = claim (never binding verdict); core conforms to `agentops-core-sdk`. Acceptance: seam + conform scenarios.
5. **Integrate `meta_skill` (ms) as the skill-evolution engine** — *how the mind tends and evolves itself.* ms mines CASS → candidate skills + anti-patterns + cross-project patterns + bandit ranking + prune proposals. **It PROPOSES; the AgentOps flywheel gate (+ MTO assurance for fleet) PROMOTES** (same single-writer seam as worker=claim / MTO=verdict, one altitude down). ms is the evolution *engine*, NOT the catalog manager (it can't ingest Anthropic `SKILL.md` — 2026-05-31 verdict holds); the 81 Anthropic `SKILL.md` stay the runtime catalog. **Format fork:** (A) adapter Anthropic-`SKILL.md`↔ms [now], (B) adopt ms store, (C) contribute Anthropic-import to ms upstream [durable]. Acceptance: ms-evolution scenario below. (bead ag-452u; ref github.com/Dicklesworthstone/meta_skill)

## Sequence (protect the 380 stars)
Slice 1 (non-breaking re-identity) → 2 (stub-relocate) → 3 (resolve) → 4 (conform+seam). Distilling = giving users *more* of the mind they starred + shedding orchestration most never used. Keep repo + stars + identity throughout.

## Validation lanes
README/3.0.md grep (no "control plane" as AgentOps's self-label) · stub-pointer test per relocated command · core-command smoke (inject/compile/skills unchanged) · schema-conformance check vs agentops-core-sdk · seam test (worker output is a claim, not a verdict).

## Next
Tracked as beads (this doc = the intent). Execute slice 1 first.
