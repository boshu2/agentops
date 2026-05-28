# Executable spec for the /knowledge-activation skill — mature-knowledge activation (BC1 Corpus).
# /knowledge-activation promotes mature .agents knowledge into the operator layer:
# it consolidates evidence, distills durable beliefs and playbooks, compiles a goal-time
# briefing, and surfaces gaps. Hexagon: supporting; consumes: mature .agents knowledge;
# produces: .agents/beliefs.md, .agents/playbooks/*.md, .agents/briefings/*.md. (soc-qk4b)

Feature: Knowledge-activation promotes mature knowledge into operator surfaces
  As the knowledge flywheel's activation step
  I want mature, repeated knowledge distilled into beliefs, playbooks, and briefings
  So that durable insight shapes execution instead of decaying in the backlog

  Background:
    Given .agents knowledge that has matured past the promotion threshold

  Scenario: Evidence is consolidated before distillation
    When /knowledge-activation runs
    Then it consolidates the supporting evidence for candidate knowledge

  Scenario: Durable knowledge is distilled into operator surfaces
    When consolidation completes
    Then it distills beliefs and playbooks into the operator layer

  Scenario: A goal-time briefing is compiled
    When activation targets current goals
    Then it compiles a goal-time briefing under .agents/briefings/

  Scenario: Gaps are surfaced rather than silently skipped
    When activation finishes
    Then it reports knowledge gaps that still need evidence
