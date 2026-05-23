# Executable spec for the /design skill — product-validation gate (domain role).
# /design checks that a proposed goal aligns with the product's strategic direction (PRODUCT.md)
# BEFORE discovery commits major work, emitting a council verdict. It runs inline (--quick) or
# council-gated (--preset=product), with --strict raising the bar. Hexagon: domain; consumes
# standards; produces result.json (council verdict schema); shared-kernel with standards. (soc-qk4b)

Feature: Design validates goal-to-product fit before discovery
  As the pre-discovery product gate
  I want a proposed goal judged against the product's strategic direction
  So that off-strategy work is caught before discovery spends effort on it

  Scenario: a goal is validated against PRODUCT.md
    Given a PRODUCT.md in the repo
    When /design runs on a proposed goal
    Then it emits a council verdict on whether the goal aligns with the product direction
    And this happens before discovery begins major work

  Scenario: quick mode skips the council
    When /design --quick runs
    Then it does an inline check with no council spawning
    And the default mode is council-gated with --preset=product

  Scenario: strict mode raises the threshold
    When /design --strict runs
    Then it requires a higher alignment threshold (average score >= 2.5) to pass
