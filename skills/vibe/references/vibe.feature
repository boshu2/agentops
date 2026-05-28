# Executable spec for the /vibe skill — per-slice code-quality gate (BC4 Validation, loop Move 6).
# /vibe runs a multi-model council over a slice's changes and returns a PASS/WARN/FAIL
# verdict on complexity, architecture, security, and intent-fit — judging the code that
# reaches the behavior, NOT replacing the slice's first failing test. CRITICAL findings
# block. Hexagon: domain; consumes: standards; produces: result.json + verdict.json. (soc-qk4b)

Feature: Vibe judges a slice's code quality before it counts
  As the per-slice quality gate
  I want a council verdict on the code a slice produced
  So that a slice is only counted toward bead acceptance when its code is sound

  Background:
    Given a slice's code changes

  Scenario: the council returns a per-dimension verdict
    When /vibe runs
    Then it produces a PASS/WARN/FAIL verdict over complexity, architecture, security, and intent-fit
    And it writes verdict.json (the canonical council verdict contract)

  Scenario: CRITICAL findings block the slice
    When the council surfaces a CRITICAL finding
    Then the verdict is FAIL and the slice is not ready to count against the roll-up
    And it must be fixed and re-vibed before closing the bead

  Scenario: vibe judges code, not behavior
    Then /vibe complements the slice's first failing test (which proves behavior)
    And it does not substitute for that test

  Scenario: deep-audit mode widens the review
    Given /vibe --sweep recent
    Then per-file explorers feed the council for a deeper audit over the recent changes
