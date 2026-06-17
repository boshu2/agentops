# Acceptance surface for dual-pane ATM collaboration (CEP slice 1).

Feature: Dual-pane ATM spawns Opus and Codex with reserved paths
  As an operator running collaborative ATM
  I want two panes with explicit work split and path reserves
  So that Opus and Codex can work in parallel without checkout collisions

  Scenario: Preflight requires using-atm and agent-mail readiness
    Given the dual-pane-atm skill is loaded
    When I follow references/spawn-checklist.md before first dispatch
    Then both lanes have reserves declared before any tracked-file edit

  Scenario: Work split follows the matrix for duel vs build patterns
    Given a bead scoped for dual-pane work
    When I choose a pattern using references/work-split-matrix.md
    Then Opus and Codex responsibilities are disjoint for that pattern
