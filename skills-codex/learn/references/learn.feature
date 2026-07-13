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

  Scenario: Material evidence changes the remaining plan through the orchestrator
    Given work remains and the Validate observations invalidate a plan assumption
    When Learn classifies the impact as material_change
    Then it emits cited proposed changes to the orchestrator
    And Learn does not mutate the plan or invoke Premortem

  Scenario: No material delta does not fabricate learning
    Given work remains and the verdict does not change the plan
    When Learn classifies the impact as no_change
    Then the orchestrator may retry, continue, stop, or escalate
    And no plan mutation or Premortem is implied

  Scenario: Distinct objectives establish advisory recurrence
    Given the same finding class was observed in two distinct objectives
    And one objective retried the same finding three times
    When Learn reconciles the observations
    Then recurrence counts two distinct objectives rather than five review events
    And one advisory producer candidate cites both objectives

  Scenario: One catch does not create policy
    Given one evidence-backed finding in one objective
    When Learn reconciles the observations
    Then producer_candidates is empty
    And no rule or delivery blocker is created

  Scenario: Terminal work closes the tick
    Given no work remains after validation
    When Learn classifies the impact as terminal
    Then the orchestrator closes the tick
    And Premortem is not invoked
