# Executable spec for the /curate skill — corpus mining + synthesis (BC1 Corpus).
# /curate mines transcripts, .agents knowledge, bd, and git for reusable signal, runs a
# selected mode under a lock, and emits synthesis, bd updates, or (rarely) skill diffs.
# Hexagon: supporting; consumes: transcripts + .agents + bd + git; produces:
# .agents/research/*.md synthesis, bd notes, occasional skill diffs. (soc-qk4b)

Feature: Curate mines the corpus and emits reviewed synthesis
  As the curation step of the knowledge flywheel
  I want transcripts and history mined into reviewed synthesis under a lock
  So that signal is consolidated without racing other writers

  Background:
    Given transcripts, .agents knowledge, bd, and git history

  Scenario: A mode and scope are resolved first
    When /curate runs
    Then it resolves the requested mode and scope before doing work

  Scenario: A lock is acquired when the mode writes shared state
    When the mode writes shared artifacts
    Then it acquires the curation lock before running the mode body

  Scenario: Mining produces synthesis and bd updates, not silent rewrites
    When the mode body runs
    Then it emits synthesis under .agents/research/ and bd notes, with skill diffs only when warranted
