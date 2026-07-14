Feature: Validate emits immutable proof only
  As an independent validator
  I want one evidence-bound verdict with structured observations
  So that proof is separate from learning, retry, and delivery authority

  Scenario: A bounded artifact receives a verdict
    Given a pinned artifact and explicit acceptance commands
    When Validate remeasures the artifact in fresh context
    Then it emits PASS, WARN, or FAIL with findings and structured observations
    And every observation cites evidence

  Scenario: Consume the persistent run budget before validator dispatch
    Given a request-bound factual receipt is READY
    And the persistent run exposes all four required meters
    When Validate requests one semantic-review admission for that same run ID
    Then the governor records the charge before validator dispatch
    And only typed AUTHORIZED evidence permits VALIDATE_SINGLE_FRESH

  Scenario: Fail closed when proof or a required meter is unavailable
    Given mandatory factual proof is FAIL, ERROR, UNKNOWN, missing, or malformed
    Or a required run meter is unavailable
    When Validate prepares semantic review
    Then it emits typed NONAUTHORIZING evidence
    And it does not dispatch a validator or create local recovery state

  Scenario: Preserve the factual lane classifier
    Given diagnostic or release proof is FAIL
    And the S1 S8 aggregate factual receipt is READY
    When Validate prepares semantic review
    Then the budget adapter does not reclassify that nonbinding lane
    And one governor admission remains eligible

  Scenario: A spent hard ceiling buys no helper
    Given a semantic-review charge would exceed a declared run ceiling
    When Validate requests admission
    Then it preserves the governor's typed hard-ceiling evidence
    And helper allowed is false
    And validator dispatch is forbidden

  Scenario: Validate stops after proof
    Given a schema-valid immutable verdict
    When Validate returns it to the caller
    Then it does not classify recurrence or promote learning
    And it does not retry, re-plan, close tracker work, or deliver the artifact

  Scenario: Self-validation cannot claim independence
    Given the author and validator identities are equal
    When the verdict would otherwise be PASS
    Then independence is waived and the verdict cannot satisfy independent proof

  Scenario: Factual proof does not impersonate semantic judgment
    Given a frozen factual-gate registry and a pinned candidate
    When deterministic pre-validation runs
    Then each gate proves one declared factual proof kind
    And missing backing is classified as registry integrity
    And prose quality and exact wording remain independent-review evidence
    And no advisory semantic observation becomes a blocking deterministic gate
