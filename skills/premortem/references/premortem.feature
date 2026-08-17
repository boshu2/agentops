Feature: Premortem optionally challenges one frozen plan
  Scenario: A fresh judge returns advisory findings
    Given a bead or caller intent with a runtime-derived digest and author context ID
    When a distinct fresh judge challenges its acceptance, scope, and evidence
    Then Premortem returns findings with checked and not-checked scope
    And an empty finding set grants no lifecycle permission

  Scenario: Premortem stops after the review
    Given any advisory finding set
    When the review is complete
    Then Premortem does not implement, validate, retry, schedule, claim, operate Git, release, or deliver
    And the caller owns whether to revise the plan or invoke RPI

  Scenario: A missing or oversized judge packet stops before dispatch
    Given the frozen packet exceeds its source or byte limit or lacks authorization
    When Premortem prepares the judge
    Then it returns explicit incomplete failure without starting a judge

  Scenario: A hung judge cannot become a finding set
    Given the declared judge exceeds its finite deadline
    When Premortem cancels the round
    Then it reaps the judge process group and reports judge_status timed_out
    And findings and checked are empty while not_checked names the missing review

  Scenario: A defeat attempt cannot mutate the primary subject
    Given an authorized defeat command may write files
    When Premortem executes it in disposable isolation
    Then restoration or cleanup failure is explicit failure
    And the primary-subject digest remains unchanged
