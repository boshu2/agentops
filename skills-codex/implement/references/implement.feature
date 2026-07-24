Feature: Implement freezes one exact candidate
  @covered-by:skills/rpi/tests/test_run_once.py::test_serialized_remote_boundary_preserves_single_mint_identity
  Scenario: Implement consumes the single-mint Plan identity
    Given Plan minted one exact intent snapshot
    When Implement receives the phase packet
    Then it verifies the same intent_ref, intent_digest, and byte_length
    And it never mints or re-derives the living source

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_repository_observation_includes_generated_companions_and_deletions
  Scenario: Actual effects are runtime-derived
    Given repository-wide before and final manifests
    When Implement derives effect-receipt.v1
    Then changed paths include generated companions and deletions

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_mutation_outside_write_scope_is_observed_and_forces_fail
  Scenario: Write scope does not hide observation
    Given a change outside frozen write scope
    When the final repository manifest is derived
    Then that path remains in actual changed paths

  @covered-by:skills/validate/scripts/test_kernel_v3.py::test_candidate_mutation_after_freeze_is_terminal
  Scenario: Candidate freeze is final
    Given Implement returned the final manifest
    When the candidate mutates
    Then the current invocation terminates without a repair revision
