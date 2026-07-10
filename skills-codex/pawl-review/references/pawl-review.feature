Feature: portable pawl reviewer lane

  @covered-by:tests/scripts/agentops-native-skills.bats::nonce-bound
  Scenario: return independent semantic evidence
    Given an immutable nonce-bound request whose contract and diff digests match
    When a fresh read-only reviewer finishes
    Then its context differs from the author
    And its evidence is contained and nonempty
    And it returns a lane result without writing the panel verdict

  @covered-by:tests/scripts/agentops-native-skills.bats::transport
  Scenario: preserve transport uncertainty
    Given the requested reviewer cannot produce usable evidence
    When its route deadline ends
    Then the result has transport failure class
    And it has no semantic disposition
