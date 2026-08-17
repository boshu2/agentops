Feature: Postmortem tests retrospective causal claims
  As an engineer learning from a validated outcome
  I want causal hypotheses challenged against evidence and counterfactuals
  So that retrospective stories do not become unsupported doctrine

  Scenario: An explicit causal question receives bounded analysis
    Given an immutable Validate verdict
    And an explicit retrospective causal question
    When Postmortem reconstructs the evidence-backed timeline
    Then it distinguishes supported claims, rejected claims, and unknowns
    And it cites evidence and counterfactuals

  Scenario: Postmortem does not repeat validation
    Given the acceptance verdict is already immutable
    When Postmortem begins
    Then it does not re-run acceptance validation
    And it does not change proof, bookkeeping, planning, tracker, or delivery state

  Scenario: Oversized evidence stops before causal judging
    Given the evidence packet exceeds 20 sources or 256 KiB
    When Postmortem freezes its inputs
    Then it reports incomplete without dispatching a judge or writing a complete report

  Scenario: A hung optional judge cannot become causal evidence
    Given a declared judge exceeds its finite deadline
    When Postmortem cancels the attempt
    Then it reaps the process group and records timed_out in postmortem-run.v1
    And the judge contributes no supported or rejected causal claim

  Scenario: Cleanup or subject drift is explicit failure
    Given an optional judge changes disposable state or cannot be reaped
    When Postmortem verifies cleanup and the subject digest
    Then the run receipt is incomplete and the primary subject remains unchanged
