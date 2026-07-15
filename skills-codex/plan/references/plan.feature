Feature: Plan emits one behavior packet
  Scenario: Normal and edge acceptance are bounded
    Given a caller intent
    When Plan shapes the intent
    Then PlanPacket contains one active behavior and normal and edge scenarios
    And write scope includes generated companions and explicit exclusions

  Scenario: Advisory decomposition does not schedule work
    Given a PlanPacket has advisory decomposition
    Then it contains no owner, ready, claim, priority, attempt, wave, queue, lease, admission, next action, closure, release, or delivery state
