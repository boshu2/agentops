Feature: Research answers one bounded question
  @covered-by:skills/research/scripts/validate.sh
  Scenario: Load-bearing claims are cited
    Given a bounded question and required evidence
    When Research examines the smallest relevant sources
    Then observations and inferences are distinguished
    And every load-bearing claim cites authoritative evidence

  @covered-by:skills/research/scripts/validate.sh
  Scenario: Research stops at the evidence boundary
    Given a cited answer with checked and unchecked scope
    When Research reports the result
    Then it does not approve work, select a next action, retry, or mutate lifecycle state

  @covered-by:skills/research/scripts/validate.sh
  Scenario: Multiple caller-supplied reports are synthesized once
    Given several identified reports that address one bounded question
    When Research compares their load-bearing claims
    Then every claim preserves its report identity and evidence reference
    And agreement, contradiction, and unknown are reported separately
    And Research emits one synthesis without creating an umbrella or starting a new runtime

  @covered-by:skills/research/scripts/validate.sh
  Scenario: External research is authorized and bounded
    Given a current-fact question with declared public domains and limits
    When Research uses GET or HEAD within the allowlist
    Then request, byte, redirect, and deadline effects are recorded
    And missing authorization or an out-of-allowlist target stops before contact

  @covered-by:skills/research/scripts/validate.sh
  Scenario: Sensitive evidence cannot leak into output
    Given a source contains a credential, personal data, or restricted excerpt
    When Research prepares model input or a durable report
    Then it redacts the value while retaining a source identity
    And raw output requires separate approval for the exact excerpt, audience, path, and retention
