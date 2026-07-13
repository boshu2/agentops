Feature: Learn records the fourth lifecycle receipt
  As an RPI orchestrator
  I want post-verdict observations isolated in Learn
  So that validation proof stays immutable while future work receives evidence

  Scenario: A completed validation verdict produces a Learn receipt
    Given an immutable Validate verdict with evidence
    When Learn captures bounded observations
    Then it writes a schema-valid learn receipt
    And RPI records Learn after Discovery, Crank, and Validate

  Scenario: Missing validation proof blocks learning
    Given no readable Validate verdict artifact
    When Learn is invoked
    Then it returns BLOCKED
    And it does not invent observations or change delivery state
