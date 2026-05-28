# Executable spec for the /validation skill — slice + bead acceptance roll-up (BC3 Loop, Move 6).
# /validation rolls every Given/When/Then of the intent into a verdict: it runs the
# vibe → lifecycle → post-mortem → retro → forge DAG by STRICT DELEGATION (separate
# Skill invocations, no compression) and produces verdict.json — activity logs never
# close beads. Hexagon: consumes forge, post-mortem, retro, shared, vibe;
# produces verdict.json. (soc-qk4b.2)

Feature: Validation rolls slice and bead acceptance into a verdict
  As the loop's acceptance gate
  I want every acceptance criterion mapped to a passing test and rolled into a verdict
  So that a bead closes on proof, not on an activity log

  Background:
    Given completed wave outputs for an intent issue with Given/When/Then acceptance

  Scenario: Every acceptance criterion maps to a passing test
    When /validation runs the slice-acceptance roll-up
    Then each Given/When/Then from the intent maps to a passing test
    And an unmapped or failing criterion blocks the verdict

  Scenario: Strict delegation across the validation DAG
    When /validation executes its DAG
    Then it delegates to /vibe, lifecycle skills, /post-mortem, /retro, and /forge
      as separate Skill invocations
    And it does not compress or skip those steps

  Scenario: The verdict is proof, not an activity log
    When validation completes
    Then it produces verdict.json capturing the per-criterion verdict
    And an activity log alone never closes a bead

  Scenario: Surface failures block under strict mode
    Given --strict-surfaces is set
    When any of the four closure surfaces fails
    Then the verdict is FAIL, not WARN
