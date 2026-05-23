# Executable spec for the /harvest skill — knowledge promotion (BC1 Corpus).
# /harvest sweeps the .agents knowledge surfaces, consolidates and de-duplicates
# promotable items, and writes consolidated learnings — runnable as a manual sweep
# or as part of the nightly /dream compounding loop. Hexagon: supporting; consumes:
# .agents knowledge surfaces; produces: consolidated .agents/learnings/*.md. (soc-qk4b)

Feature: Harvest promotes scattered knowledge into consolidated learnings
  As the knowledge flywheel
  I want repeated, reusable knowledge consolidated and de-duplicated
  So that the corpus compounds signal instead of accumulating fragments

  Background:
    Given .agents knowledge surfaces with candidate learnings

  Scenario: A sweep gathers promotable knowledge
    When /harvest runs
    Then it scans the .agents knowledge surfaces for promotable items

  Scenario: Promotable items are consolidated and de-duplicated
    When candidates are gathered
    Then near-duplicate items merge and only reusable knowledge is promoted

  Scenario: Output is written to the learnings surface
    When promotion completes
    Then consolidated learnings are written under .agents/learnings/

  Scenario: Harvest runs manually or inside the nightly dream loop
    Then /harvest can be invoked directly for a manual sweep
    And it also runs as a bounded step of the /dream compounding loop
