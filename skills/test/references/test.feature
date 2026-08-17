# Executable spec for the /test skill — test generation + coverage (supporting role).
# /test loads the language's test standards, generates REAL tests for existing code, runs them to
# verify they pass (it does not stop at a plan), and fills coverage gaps — writing artifacts to
# .agents/scratch/tests/. Hexagon: supporting; consumes standards (test conventions) + repo-context (the
# code under test); produces test-evidence. (soc-qk4b)

Feature: Test generates real, passing tests and coverage
  As the test-generation step
  I want tests generated to the project's standards and verified by running them
  So that coverage improves with real passing tests, not a plan

  Scenario: standards and language are loaded before generating
    When /test runs
    Then it detects the language and loads the test standards (AI-native test shape) for it

  Scenario: generate produces real tests that are run and verified
    When /test generate runs on existing code
    Then it writes real tests, runs them, and verifies they pass
    And it does not output a plan and stop

  Scenario: coverage analyzes and fills gaps
    When /test coverage runs
    Then it analyzes coverage gaps and fills them, writing a coverage report to .agents/scratch/tests/

  Scenario: an unapproved test process never starts
    Given a command is missing caller or repository-test authorization
    When Test considers running the command
    Then Test stops before spawning it

  Scenario: a hung test process fails closed
    Given an authorized test command
    When an authorized command exceeds its deadline or output cap
    Then Test reaps the complete process group and records explicit failure

  Scenario: mutation-kill evidence is disposable
    Given a test needs a deliberate product mutation to prove its oracle
    When the mutation-kill experiment runs
    Then it runs only in a digest-matching disposable copy
    And cleanup or restoration failure leaves the primary tree unchanged and blocks handoff
