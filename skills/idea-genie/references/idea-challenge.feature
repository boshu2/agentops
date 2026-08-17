Feature: Independent challenge of consequential ideas

  @covered-by:tests/scripts/agentops-native-skills.bats::sealed
  Scenario: A one-way door receives sealed challenge evidence
    Given a contested decision is costly to reverse
    When distinct contexts propose before seeing one another and then cross-review
    Then dissent and refutation attempts remain in an idea-challenge packet
    And the packet is handed to Plan as advisory evidence
    And it carries no readiness verdict

  @covered-by:tests/scripts/agentops-native-skills.bats::reversible
  Scenario: A two-way door stays lightweight
    Given a choice is cheap to undo
    When its door class is evaluated
    Then it routes to one fresh challenge and then Plan
    And no persistent orchestration substrate is required

  Scenario: Missing authorization or an oversized packet blocks dispatch
    Given a near-identical challenge lacks caller authorization or exceeds 256 KiB
    When Idea Genie freezes its input
    Then it fails before starting a model context

  Scenario: A hung challenge remains an insufficient artifact
    Given a declared perspective exceeds its deadline
    When the dispatcher cancels and reaps it
    Then the attempt is recorded timed_out with bounded output and confirmed cleanup
    And fewer than two completed one-way perspectives cannot produce synthesis or a normal Plan route
