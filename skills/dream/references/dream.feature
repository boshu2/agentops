# Executable spec for the /dream skill — overnight compounding (BC1 Corpus).
# /dream runs bounded overnight compounding sessions: it resolves an operator lane, sets it
# up, and runs the bedtime loop (harvest → forge → close-loop → defrag) on the operator's
# cadence, writing dream reports. Hexagon: supporting; consumes: the day's .agents knowledge
# + operator config; produces: .agents/overnight/*/summary.{json,md}. (soc-qk4b)

Feature: Dream runs bounded overnight knowledge compounding
  As the overnight lane of the knowledge flywheel
  I want a bounded harvest-to-defrag loop on the operator's cadence
  So that knowledge compounds between sessions without unbounded runs

  Background:
    Given a day's worth of .agents knowledge and operator dream config

  Scenario: The operator lane is resolved before running
    When /dream runs
    Then it resolves and sets up the configured operator lane

  Scenario: The bedtime loop runs the bounded compounding stages
    When the bedtime run lane executes
    Then it runs harvest, forge, close-loop, and defrag within its bounds

  Scenario: Dream writes report artifacts
    When a dream session completes
    Then it writes a summary under .agents/overnight/
