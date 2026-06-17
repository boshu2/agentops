# Phase 1 acceptance — ao orchestrate instrument lane (windshield).

Feature: ao orchestrate multi-model windshield
  As an out-of-session operator
  I want deterministic preflight and verify instruments
  So that ATM spawn failures surface before and after human procedure

  Scenario: preflight blocks unhealthy tri-vendor setup
    Given orchestration-tools.yaml and tri-vendor profile exist
    When I run ao orchestrate preflight --profile tri-vendor --json
    Then the verdict status is PASS or WARN with coordination_degraded documented
    And missing atm binary yields FAIL

  Scenario: verify confirms pane map after spawn
    Given a tri-vendor session spawned per dual-pane-atm checklist
    When I run ao orchestrate verify --session agentops--smoke --profile tri-vendor --json
    Then panes include opus codex and agy slots via strong evidence tier
    And a ledger event orchestration.verify.v1 is appended
    And tmux-title-only evidence yields WARN not PASS

  Scenario: ledger append failure degrades verdict
    Given preflight checks pass but ledger append fails
    When I run ao orchestrate preflight --profile tri-vendor --json
    Then the verdict status is WARN with ledger_unwritten true
    And the verdict status is not PASS

  Scenario: out-of-session orchestration uses instrument lane first
    Given an agent needs multi-model work outside the in-session loop
    When they consult orchestration skills
    Then the first executable steps are ao orchestrate route and preflight
    And raw atm spawn remains the documented human procedure until earn-it wrappers exist

  Scenario: dual-pane-atm drift-gates against profiles contract
    Given spawn-checklist.md for dual-pane-atm after W3
    When an operator follows the checklist
    Then step 0 runs ao orchestrate preflight
    And post-spawn runs ao orchestrate verify
    And spawn steps match orchestration-profiles.yaml tri-vendor pane map
