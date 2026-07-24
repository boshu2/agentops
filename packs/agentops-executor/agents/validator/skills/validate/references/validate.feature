Feature: Validate writes one exact fresh verdict
  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_partial_observation_forces_not_proven
  Scenario: Incomplete observation stays unproven
    Given before and final manifests do not observe the repository root
    When Validate binds runtime scope facts
    Then verdict.v3 is NOT_PROVEN

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_mutation_outside_write_scope_is_observed_and_forces_fail
  Scenario: Proven scope violation fails
    Given complete repository observation
    When an actual path is outside frozen scope classes
    Then verdict.v3 is FAIL

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_duplicate_intent_final_subject_judgment_is_rejected
  Scenario: One intent and subject receive one linked judgment
    Given a stored verdict over exact intent and final subject digests
    When an unlinked judgment tries to store another verdict over that pair
    Then the duplicate judgment is rejected

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_candidate_proof_contract_cannot_activate_itself
  Scenario: Proof activation stays externally judged
    Given the candidate changes the active proof pointer
    When Validate tries to judge it under that candidate
    Then validation refuses self-activation

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_general_recorder_advances_epoch_one_to_two_content_addressed
  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_general_recorder_cas_refuses_races_and_keeps_active_pointer
  Scenario: A qualified next epoch advances by compare and swap
    Given a candidate at epoch N plus one passed under exact active epoch N
    When the external transition recorder verifies its frozen proof bindings
    Then it writes a content-addressed transition and replaces active last
