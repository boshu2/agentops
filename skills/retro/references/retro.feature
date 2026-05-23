# Executable spec for the /retro skill — lightest learning capture (BC1 Corpus, loop Move 7).
# /retro quick-captures a SINGLE observation into .agents/learnings/ — for an insight too small
# to warrant a full /post-mortem but too useful to leave in the handoff. The captured entry is a
# candidate; the promotion ratchet still decides whether it becomes a pattern/rule. Hexagon:
# domain; consumes: standards; produces: result.json. (soc-qk4b)

Feature: Retro is the lightest single-observation learning capture
  As the move-7 quick-capture surface
  I want a single insight written to the learnings store with minimal ceremony
  So that small-but-useful observations are not lost in the handoff

  Scenario: a single observation is captured to the learnings store
    Given an insight too small for a full /post-mortem but worth keeping
    When /retro runs
    Then the observation is written to .agents/learnings/ (result.json records the capture)

  Scenario: a captured entry is a candidate, not yet promoted
    When a retro entry is captured
    Then the promotion ratchet still applies — it is a candidate learning, not yet a pattern or rule
    And promotion to a pattern/rule happens later under the ratchet, not at capture time

  Scenario: retro is lighter than post-mortem and forge
    Then /retro captures one observation directly (no council, no transcript mining)
    And it is the lightest of the move-7 capture surfaces
