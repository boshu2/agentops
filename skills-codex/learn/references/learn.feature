Feature: Learn bookkeeps an immutable verdict
  As the fourth RPI umbrella
  I want bounded post-verdict bookkeeping
  So that observations can feed future work without changing proof or delivery

  Scenario: Structured observations produce a Learn receipt
    Given a schema-valid Validate verdict and its digest
    When Learn bookkeeps its structured observations
    Then it preserves the verdict reference and digest
    And it emits a schema-valid Learn receipt

  Scenario: Learn cannot mutate proof
    Given an immutable PASS, WARN, or FAIL verdict
    When Learn records an observation disposition
    Then the original verdict fields remain unchanged
    And Learn does not operate repository, tracker, delivery, or Premortem state

  Scenario: Causal analysis remains optional
    Given an explicit retrospective causal question
    When Learn finishes bookkeeping
    Then it may return a Postmortem request to the orchestrator
    And it does not run Postmortem inline
