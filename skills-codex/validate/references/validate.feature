Feature: Validate emits immutable proof only
  As an independent validator
  I want one evidence-bound verdict with structured observations
  So that proof is separate from learning, retry, and delivery authority

  Scenario: A bounded artifact receives a verdict
    Given a pinned artifact and explicit acceptance commands
    When Validate remeasures the artifact in fresh context
    Then it emits PASS, WARN, or FAIL with findings and structured observations
    And every observation cites evidence

  Scenario: Validate stops after proof
    Given a schema-valid immutable verdict
    When Validate returns it to the caller
    Then it does not classify recurrence or promote learning
    And it does not retry, re-plan, close tracker work, or deliver the artifact

  Scenario: Self-validation cannot claim independence
    Given the author and validator identities are equal
    When the verdict would otherwise be PASS
    Then independence is waived and the verdict cannot satisfy independent proof
