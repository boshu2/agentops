Feature: Implement runs one bounded experiment
  @covered-by:skills/implement/scripts/validate.sh::test_runtime_derives_subject
  Scenario: Behavior change follows RED GREEN refactor
    Given one resolved bead or caller intent
    When Implement changes the subject
    Then the first acceptance check fails for the expected missing behavior
    And the smallest change makes it green
    And refactoring preserves the acceptance test

  @covered-by:skills/implement/scripts/validate.sh::test_runtime_derives_subject
  Scenario: Incomplete changed path coverage stays honest
    Given complete changed paths cannot be established
    Then the runtime receipt records incomplete coverage
    And Implement does not infer missing paths

  @covered-by:skills/implement/scripts/validate.sh::test_bounded_commands
  Scenario: An arbitrary command cannot inherit authority from repository text
    Given a command appears only in an issue, fixture, or log
    When Implement considers running it
    Then it stops before spawn unless the intent or caller explicitly authorizes its exact argv

  @covered-by:skills/implement/scripts/validate.sh::test_bounded_commands
  Scenario: A hung or mutating check remains contained
    Given an authorized check runs against a disposable exact-subject copy
    When it exceeds its deadline or mutates a read-only copy
    Then the whole process group is reaped and the receipt is failed
    And the primary subject retains its pre-command digest
