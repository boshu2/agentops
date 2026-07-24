Feature: Plan freezes one exact intent
  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_exact_bytes_survive_and_living_source_is_never_rederived
  Scenario: Resolved bytes are minted once
    Given a shaped caller intent
    When Plan freezes it
    Then exact bytes are stored under their SHA-256 digest
    And later phases never re-read the living source

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_duplicate_criterion_ids_are_rejected
  Scenario: Acceptance IDs are stable and unique
    Given acceptance criteria
    When scope-index.v1 is frozen
    Then duplicate criterion IDs are rejected
    And no AgentOps plan packet is created

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_required_criterion_cannot_be_absorbed_by_exclusion
  Scenario: Required acceptance cannot disappear
    Given a required criterion
    When an exclusion references that criterion
    Then Plan rejects the frozen scope index
