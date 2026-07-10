# Plan-Pawl Duel — deterministic quorum/round/breaker decision
#
# The plan-pawl (docs/contracts/pawls.md) is the multi-model pawl applied to a
# discovery PLAN artifact. The duel runs ≥2 model-family judge panes over the
# plan; this feature is the DETERMINISTIC core that turns their verdicts into one
# of three decisions — PASS / REDO / BLOCKED — with the circuit-breaker governance
# inherited verbatim from pawls.md. Pane spawning + the re-judge loop live in the
# skill (dueling-idea-genies); this decider is the windshield: deterministic, no model.

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

  # Degradation-aware decision (age-gascity-port-slate-irye.2). A judge lane that
  # infrastructure-failed (provider timeout / rate-limit / no verdict) is a
  # retryable outage, NOT a refutation: it is excluded from the FAIL tally and from
  # quorum coverage. This fixes the age-5olx incident, where a warm panel whose
  # codex+agy panes timed out was recorded REFUTED at 1/3 coverage.

  Scenario: DEGRADED — transient lane loss drops coverage below quorum (age-5olx)
    Given a claude judge verdict of PASS
    And a codex judge verdict of <timeout>
    And an agy judge verdict of <timeout>
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is DEGRADED
    And the degraded families are gpt and gemini
    And the reason says degraded coverage is retryable — re-run the panel

  Scenario: PASS with degraded coverage — the floor still holds on the survivors
    Given a claude judge verdict of PASS
    And a gpt judge verdict of PASS
    And an agy judge verdict of <timeout>
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is PASS
    And the outcome is marked degraded
    And the degraded families are gemini

  Scenario: A hard infrastructure failure stays fail-closed (a refutation)
    Given a claude judge verdict of PASS
    And a gpt judge verdict with a hard failure class
    And it is round 1 of max 3
    When I decide the duel
    Then the decision is REDO
