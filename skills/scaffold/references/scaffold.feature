# Executable spec for bounded project/component/CI scaffolding.

Feature: Scaffold generates project, component, and CI structure
  As a developer starting new work
  I want consistent boilerplate generated from one command
  So that new projects, components, and pipelines start from a known-good shape

  Background:
    Given a scaffold request naming a target

  Scenario: A new project is scaffolded by language and name
    When "/scaffold <language> <name>" runs
    Then it creates the project files and directory structure for that language

  Scenario: A component is generated into an existing project
    When "/scaffold component <type> <name>" runs
    Then it generates the component of that type

  Scenario: A CI pipeline is scaffolded for a platform
    When "/scaffold ci <platform>" runs
    Then it sets up the CI pipeline for that platform

  Scenario: Existing paths are preserved
    Given the requested target contains an existing file
    When scaffolding runs without explicit overwrite authorization
    Then the existing file is not replaced

  Scenario: Verification failure cannot partially publish a scaffold
    Given generation and commands run in a disposable staging root
    When a check hangs, a forbidden target appears, or restoration verification fails
    Then the process group is reaped and the operation reports failure
    And the primary target retains its pre-run digest
