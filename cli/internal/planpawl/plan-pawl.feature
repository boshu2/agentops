# Plan-Pawl Duel — deterministic quorum/round/breaker decision
#
# The plan-pawl (docs/contracts/pawls.md) is the multi-model pawl applied to a
# discovery PLAN artifact. The duel runs ≥2 model-family judge panes over the
# plan; this feature is the DETERMINISTIC core that turns their verdicts into one
# of three decisions — PASS / REDO / BLOCKED — with the circuit-breaker governance
# inherited verbatim from pawls.md. Pane spawning + the re-judge loop live in the
# skill (dual-pane-atm); this decider is the windshield: deterministic, no model.

Feature: Plan-pawl duel decision

  Scenario: Quorum PASS — both families ran, neither FAIL
    Given a claude judge verdict of PASS
    And a gpt judge verdict of PASS
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is PASS

  Scenario: A WARN with accepted risk does not block PASS
    Given a claude judge verdict of PASS
    And a gpt judge verdict of WARN of class judgment
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is PASS
    And the judgment WARN is surfaced

  Scenario: Auto-redo on FAIL — one family FAILs, rounds remain
    Given a claude judge verdict of PASS
    And a gpt judge verdict of FAIL
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is REDO
    And the breaker is not tripped

  Scenario: A mechanical WARN triggers auto-apply + re-judge
    Given a claude judge verdict of PASS
    And a gpt judge verdict of WARN of class mechanical
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is REDO
    And the mechanical WARN is auto-applied

  Scenario: BLOCKED on round > max (max-attempts breaker)
    Given a claude judge verdict of PASS
    And a gpt judge verdict of FAIL
    And it is round 4 of max 3
    When I decide the duel
    Then the decision is BLOCKED
    And the breaker tripped is max-attempts

  Scenario: BLOCKED on an explicit judgment flag (hard breaker)
    Given a claude judge verdict of PASS
    And a gpt judge verdict that raises the judgment flag
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is BLOCKED
    And the breaker tripped is judgment-flag

  Scenario: BLOCKED on oscillation (same failure repeating)
    Given a claude judge verdict of PASS
    And a gpt judge verdict of FAIL
    And it is round 2 of max 3
    And oscillation has been detected
    When I decide the duel
    Then the decision is BLOCKED
    And the breaker tripped is oscillation

  Scenario: Quorum not met — only one family ran
    Given a claude judge verdict of PASS
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is REDO
    And the reason mentions quorum
