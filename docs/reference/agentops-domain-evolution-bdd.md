# AgentOps Domain Evolution BDD

This is the acceptance contract for the AgentOps 3.0 evolution program.
AgentOps is not a pile of skills or a hook system; it is an SDLC control plane
and context compiler for LLM agents. It turns software-engineering practice into
compact, executable, verifiable agent context. The loop only starts after the
audit, domain map, and hexagonal target architecture exist.

```gherkin
Feature: Domain-governed AgentOps 3.0 evolution
  AgentOps must line up skills, CLI, hooks, docs, tests, beads, and knowledge
  around small provable changes, evolving through observable behavior, bounded
  contexts, ports, local proof, and trust evidence instead of broad text rewrites.

  Background:
    Given the local AgentOps repository has fetched origin/main
    And PRODUCT.md, GOALS.md, PROGRAM.md, and the operating loop are treated as direction sources
    And bead "soc-y5vh" is the active Loop epic
    And JSM-derived observations are used only through the clean-room policy

  Scenario: Audit every skill before changing shipped behavior
    Given the checked-in skill catalog contains 77 skills
    When the evolution bootstrap audits the catalog
    Then every skill is assigned exactly one primary bounded context
    And each skill has a preliminary keep, update, refactor, merge-review, or cut-review disposition
    And low-confidence assignments are called out before implementation begins

  Scenario: Map AgentOps onto the hexagonal architecture
    Given AgentOps uses BDD, DDD, Hexagonal Architecture, TDD, XP, CI/SRE, ADRs, and provenance as one system
    When the bootstrap builds the architecture map
    Then the core domains are Corpus, Validation, Loop, Factory, and Runtime
    And each domain lists its primary ports, driving adapters, driven adapters, and proof gates
    And Loop work from "soc-y5vh" depends on typed loop ports rather than shell-only state reads
    And no domain treats skills or hooks as the product by themselves

  Scenario: Bootstrap local Codex orchestration before productizing
    Given local Codex skills can be tested without changing shipped AgentOps skills
    When the bootstrap skill is installed under "/Users/bo/.codex/skills"
    Then it explains how to run the audit, BDD, domain map, architecture map, and evolution plan
    And it points to the AgentOps skill factory for per-skill upgrades
    And it does not mutate JSM-installed skills or copy JSM skill content

  Scenario: Run evolution in safe vertical slices
    Given the BDD contract and domain map pass validation
    When the operator starts an evolution loop
    Then each cycle selects one bead-backed or generated slice
    And the slice carries a Given/When/Then acceptance row
    And the first failing proof is named before implementation
    And the result is kept only when validation passes and evidence is recorded

  Scenario: Orchestrate unattended work through the ao CLI
    Given the worktree is clean, synced, and branch-attached
    And the installed ao binary exposes the same required commands as the source-built CLI
    When the operator starts an unattended evolution cycle
    Then "ao evolve" runs with lease, cleanup, and bounded max-cycle settings
    And landing policy starts as "off" until a reviewed cycle proves safe
    And commit or sync-push landing is used only after explicit authorization

  Scenario: Handle divergent main without corrupting local work
    Given local main may contain uncommitted user or agent work
    And origin/main may contain newer product direction
    When the bootstrap needs the latest direction
    Then it reads current direction from origin/main without resetting the dirty tree
    And it reports divergence before any merge, rebase, or branch-changing action
```

## Acceptance Checks

- `bash scripts/check-agentops-domain-evolution-plan.sh`
- `bash skills/heal-skill/scripts/heal.sh --strict`
- `bash scripts/validate-skill-frontmatter.sh --strict`
- `bash tests/docs/validate-doc-release.sh`
