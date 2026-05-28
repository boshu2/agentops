# Executable spec for the /handoff skill — compact session handoff (BC5 Runtime).
# /handoff captures the end-of-session state so the next session resumes without
# re-discovery: it derives a topic, gathers what was accomplished, pinpoints where
# work paused (including in-progress beads), lists the files to read first, and
# writes a structured handoff document plus a continuation prompt. Hexagon:
# supporting; consumes: git/bd/ratchet session signals; produces: a handoff doc. (soc-qk4b)

Feature: Handoff captures session state for seamless continuation
  As an agent ending a work session
  I want the pause point, accomplishments, and next-files recorded compactly
  So that the next session resumes without re-discovering context

  Background:
    Given a session with recent commits, beads activity, and artifacts

  Scenario: Topic is taken from the argument or derived from recent activity
    When /handoff runs
    Then a provided topic is used as the handoff identifier
    And with no topic it derives one from recent commits, the current bead, or ratchet state
    And it falls back to a timestamped slug when no descriptive source exists

  Scenario: Accomplishments are gathered from session evidence, not memory
    When the handoff is assembled
    Then it reviews recent commits, changed files, research/plans produced, and closed issues

  Scenario: The pause point and in-progress work are recorded
    When the handoff identifies where work stopped
    Then it states the last action, the intended next action, and any pending blockers
    And it lists beads still in progress so the next session can reclaim them

  Scenario: The next session is told which files to read first
    When the handoff is written
    Then it lists recently modified core files and the key research/plan artifacts

  Scenario: Output is a structured document plus a continuation prompt
    When /handoff completes
    Then it writes a dated handoff document under the handoff directory
    And it emits a continuation prompt the next session can act on directly
