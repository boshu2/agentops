Feature: Validate emits immutable proof only
  As an independent validator
  I want one evidence-bound verdict with structured observations
  So that proof is separate from learning, retry, and delivery authority

  Scenario: A bounded artifact receives a verdict
    Given a pinned artifact and explicit acceptance commands
    When Validate remeasures the artifact in fresh context
    Then it emits PASS, WARN, or FAIL with findings and structured observations
    And every observation cites evidence

  Scenario: One bounded tranche receives one semantic review
    Given one to three low-risk waves passed targeted deterministic acceptance
    And their complete tranche is frozen at one exact candidate SHA
    When Validate verifies exact-input receipts and dispatches the fresh judge
    Then it reruns only missing, stale, suspicious, or invalidated facts
    And it emits one canonical result.json for the tranche
    And no intermediate wave received Validate or Learn

  Scenario: Factual readiness permits one fresh validator
    Given a request-bound factual receipt is READY
    And the candidate, acceptance, author, and judge identities still match
    When Validate prepares semantic review
    Then it dispatches VALIDATE_SINGLE_FRESH without a second work controller
    And the author and judge identities differ

  Scenario: Fail closed when proof is unavailable
    Given mandatory factual proof is FAIL, ERROR, UNKNOWN, missing, or malformed
    When Validate prepares semantic review
    Then it returns the factual evidence to the caller
    And it does not dispatch a validator or create local recovery state

  Scenario: Preserve the factual lane classifier
    Given diagnostic or release proof is FAIL
    And the S1 S8 aggregate factual receipt is READY
    When Validate prepares semantic review
    Then Validate does not reclassify that nonbinding lane
    And semantic judgment remains eligible

  Scenario: External runtime limits remain caller evidence
    Given the runtime cannot dispatch a judge because a hard external limit is spent
    When Validate returns control
    Then it reports the limit without creating cost or helper state
    And the caller owns the next disposition

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
