# Executable spec for the optional /push driving adapter.

Feature: Push follows repository-selected delivery
  As a repository operator
  I want delivery to use deterministic checks and my repository policy
  So that AgentOps proof does not become a Git control plane

  Scenario: repository policy selects the adapter
    Given a repository chooses direct push, a PR, or user-owned CI
    When /push prepares delivery
    Then it uses the repository-selected delivery path
    And it does not create an AgentOps Git queue

  Scenario: deterministic failure stops delivery
    Given the repository declares deterministic checks
    When any required check fails
    Then /push does not deliver the payload
    And it reports the failing command and exit status

  Scenario: lifecycle proof remains immutable
    Given Validate has emitted immutable proof
    When delivery consumes or cites that proof
    Then /push does not require another LLM verdict
    And it does not rewrite proof, close a tracker, or complete the lifecycle
