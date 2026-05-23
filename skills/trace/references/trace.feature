# Executable spec for the /trace skill — decision provenance tracing (BC1 Corpus).
# /trace reconstructs HOW a design decision evolved by searching across CASS sessions,
# handoffs, git history, and research artifacts, then merges the hits into one
# chronological timeline with a source citation per claim and writes a trace report.
# It routes file paths to /provenance and git refs to git-based tracing. Hexagon:
# supporting; consumes: session/handoff/git/research sources; produces: a trace report. (soc-qk4b)

Feature: Trace reconstructs how a decision evolved, with cited provenance
  As an agent recovering design rationale
  I want a cited, chronological trace across every source
  So that I understand when a concept appeared and why it was decided

  Background:
    Given a concept, file path, or git ref to trace

  Scenario: Target type selects the tracing strategy
    When /trace classifies its target
    Then a file path routes to /provenance for artifact lineage
    And a git ref uses git-based tracing
    And a keyword or concept uses design-decision tracing

  Scenario: Design-decision tracing searches every source in parallel
    When a concept is traced
    Then it searches CASS sessions, handoffs, git history, and research artifacts
    And it continues with the remaining sources when one is unavailable

  Scenario: Results merge into one chronological, cited timeline
    When the sources return
    Then events are merged oldest-first and same-day/same-session duplicates collapsed
    And every event carries a source citation

  Scenario: Each event records what changed, why, who, and the evidence
    When key decisions are extracted
    Then each lists the change, the reasoning if available, the author or session, and a source link

  Scenario: A trace report is written and gaps are noted, not hidden
    When /trace completes
    Then it writes a dated trace report under .agents/research/
    And it records which sources were empty rather than failing the trace
