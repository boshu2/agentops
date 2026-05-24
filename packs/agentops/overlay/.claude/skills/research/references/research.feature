# Executable spec for the /research skill — Move 1 of the operating loop (driving-adapter).
# /research investigates a topic prior-art-first, dispatches an explore agent that uses
# iterative retrieval, and writes a cited artifact to .agents/research/ — every claim
# carries a file:line reference. Interactive runs gate on human approval; --auto skips it.
# Hexagon: driving-adapter; consumes inject + repo-context; produces .agents/research/*.md
# + result.json. (soc-qk4b)

Feature: Research produces a cited investigation artifact, prior-art first
  As Move 1 of the operating loop
  I want a topic investigated against existing knowledge before fresh exploration
  So that findings are grounded, cited, and not redundant with what is already known

  Scenario: prior art is searched before fresh exploration
    When /research runs on a topic
    Then it first searches existing knowledge (ao inject/lookup + the .agents/ knowledge dirs)
    And applicable prior learnings are cited in the output, not just loaded passively

  Scenario: an explore agent investigates with iterative retrieval
    When the investigation runs
    Then an explore agent is dispatched (not merely described)
    And it uses iterative retrieval — score results, extract new terms from high-relevance
      hits, refine over up to 3 cycles

  Scenario: findings are written as a cited artifact
    When the investigation completes
    Then findings are written to .agents/research/YYYY-MM-DD-<slug>.md
    And every claim carries a file:line citation

  Scenario: interactive runs gate on approval, --auto does not
    When /research runs without --auto
    Then it requests human approval (Gate 1) before reporting completion
    And with --auto it proceeds without the approval gate
