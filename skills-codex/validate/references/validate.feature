Feature: Validate writes one fresh verdict over exact content
  Scenario: Identity gaps stay unproven
    Given missing, colliding, or unattested author and validator identities
    When Validate judges the candidate
    Then the verdict is NOT_PROVEN

  Scenario: Scope failure is distinct from missing proof
    Given complete changed-path coverage
    When a proven path is outside Plan write scope
    Then the verdict is FAIL

  Scenario: Validation stops after persistence
    Given any PASS, FAIL, or NOT_PROVEN verdict
    When Validate atomically persists it
    Then Validate returns the artifact digest and path
    And performs no repair, retry, Git, closure, release, or delivery action
