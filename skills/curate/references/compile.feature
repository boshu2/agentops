# Executable spec for the /compile skill — knowledge-wiki rebuild (BC1 Corpus).
# /compile rebuilds the .agents knowledge wiki through Mine → Grow → Compile → Lint → Defrag,
# producing the interlinked wiki + a lint report. It is the corpus warmup /evolve --compile invokes before
# a cycle. Hexagon: supporting; consumes:[] (runs ao mine/defrag CLI ops, not skills);
# produces .agents/compiled/lint-report.md. (soc-qk4b)

Feature: Compile rebuilds the .agents knowledge wiki
  As the corpus-compile warmup
  I want the wiki mined, grown, linted, and defragged into a fresh compiled state
  So that a cycle starts against a current, contradiction-checked corpus

  Scenario: the Mine → Grow → Compile → Lint → Defrag pipeline runs
    When /compile runs
    Then it mines sources, grows learnings (validation/synthesis/gap detection),
      compiles raw artifacts into interlinked wiki articles (the core step),
      lints the compiled wiki, and defrags stale/duplicate artifacts
    And it writes .agents/compiled/lint-report.md

  Scenario: lint surfaces problems but does not auto-fix
    When the lint step finds contradictions, orphans, or gaps
    Then they are reported in the lint report
    And /compile does not silently rewrite authored content to "fix" them

  Scenario: large corpora are processed incrementally
    Given a 2000+ file corpus
    Then /compile batches changed files per prompt rather than loading the whole corpus at once

  Scenario: compile is the warmup /evolve consumes
    Then /evolve --compile runs this rebuild before its first cycle
