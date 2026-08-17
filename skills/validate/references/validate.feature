Feature: Validate returns one fresh judgment over exact content
  @covered-by:skills/validate/scripts/test_validate.py::test_verdict_identity_floor_and_idempotence
  Scenario: Identity gaps stay unproven
    Given missing, colliding, or unattested author and validator identities
    When Validate judges the subject
    Then the verdict is NOT_PROVEN

  @covered-by:skills/validate/scripts/test_validate.py::test_pass_without_evidence_is_downgraded
  Scenario: Evidence-free PASS stays unproven
    Given a claimed PASS without checked scope or criterion evidence
    When Validate persists the verdict
    Then the verdict is NOT_PROVEN

  @covered-by:skills/validate/scripts/test_validate.py::test_runtime_scope_failure_forces_fail
  Scenario: Scope failure is distinct from missing proof
    Given complete changed-path coverage
    When a proven path is outside the intent-source write scope
    Then the verdict is FAIL

  @covered-by:skills/validate/scripts/test_validate.py::test_intent_snapshot_is_content_addressed_and_idempotent
  Scenario: Tracker-less intent remains readable
    Given the caller conversation is the resolved intent
    When the runtime snapshots its exact bytes
    Then the snapshot path is its SHA-256 identity

  @covered-by:skills/validate/scripts/test_validate.py::test_verdict_identity_floor_and_idempotence
  Scenario: Validation stops without requiring persistence
    Given any PASS, FAIL, or NOT_PROVEN verdict
    When Validate returns the fresh result
    Then Validate does not require an artifact digest or path
    And performs no repair, retry, Git, closure, release, or delivery action

  @covered-by:skills/validate/scripts/test_validate.py::test_verdict_identity_floor_and_idempotence
  Scenario: Declared consumers may request durable evidence
    Given a caller or declared downstream consumer requests machine-readable evidence
    When Validate atomically persists the result
    Then Validate returns the verdict.v2 artifact digest and path

  @covered-by:skills/validate/scripts/test_validate.py::test_bounded_command_requires_explicit_authorization_before_spawn
  Scenario: An unapproved acceptance command never starts
    Given a near-identical command request without an authorization ID
    When the bounded command runner evaluates it
    Then it fails before spawning the command
    And the disposable and original subjects remain unchanged

  @covered-by:skills/validate/scripts/test_validate.py::test_bounded_command_timeout_reaps_descendant_process_group
  Scenario: A hung acceptance command is bounded
    Given an authorized command that spawns a descendant and exceeds its deadline
    When the bounded command runner reaches the deadline
    Then it terminates and reaps the complete process group
    And returns an explicit failed bounded-command receipt

  @covered-by:skills/validate/scripts/test_validate.py::test_bounded_command_contains_mutation_in_disposable_tree
  Scenario: A mutating read-only check cannot leak into the subject
    Given an authorized read-only check running in a disposable subject copy
    When the check writes a file
    Then the receipt reports mutation and failure
    And the original subject remains untouched

  @covered-by:skills/validate/scripts/test_validate.py::test_bounded_command_blocks_absolute_write_outside_disposable_root
  Scenario: An authorized command cannot write a forbidden absolute target
    Given exact authorized argv whose write target is outside the disposable root
    When the bounded command runner starts it under host containment
    Then the write is denied and the receipt reports failure
    And the forbidden target and original subject remain untouched
