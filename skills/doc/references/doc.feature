# Documentation-only scenarios for the Doc skill (listed in scripts/.scenario-linkage-allow;
# no test executes them). Doc writes or updates the documentation the caller requested,
# checked against the repository's current source and existing docs. It selects only the
# mode the request needs; it does not classify the repository first.
# Hexagon: supporting; consumes repo-context; produces documentation. (soc-qk4b)

Feature: Doc writes and checks requested documentation against the repo
  As the documentation step
  I want requested docs written from the repo and existing docs checked against it
  So that documentation matches the project's current state

  Scenario: validation checks docs against the repo
    When /doc validates existing documentation
    Then it reports gaps or staleness measured against the current source

  Scenario: mixed-language coverage follows the selected entities
    Given the caller selects a Python public function and an exported Go function for a coverage check
    And the Python module contains quoted data unrelated to the function's documentation
    When /doc checks whether the selected APIs are documented
    Then it inspects each declaration and its associated docstring, comment or reference page
    And the unrelated quoted data contributes no documented entity
    And any ratio uses the same explicitly enumerated entity set for numerator and denominator
    And unknown or unexamined status is disclosed without a percentage

  Scenario: documenting one API creates only the requested document
    Given the caller requests an authentication API update in an existing reference page
    When /doc reads the implementation and callers and updates that page
    Then it verifies the documented inputs, outputs, errors and examples against the source
    And it returns the changed page, check results and unresolved gaps
    And it creates no coverage ledger, repository-wide audit or separate report
