# Executable spec for /doc --mode=readme — gold-standard README generation (BC4 Factory).
# /doc --mode=readme drafts or improves a README that converts skimmers into users and satisfies
# deep readers, enforcing 8 non-negotiable patterns (problem-first lead, trust block
# near install, collapse-don't-delete depth, adoption-ordered sections), then validates
# the result with a council before reporting. Hexagon: supporting (doc factory); consumes: project files
# + interview answers; produces: documentation (README.md), council-validated. (soc-qk4b)

Feature: README generation converts skimmers into users and survives a council
  As an author publishing a tool
  I want a README that leads with the problem, proves it works, and earns trust
  So that both skimmers and deep readers adopt instead of bouncing

  Background:
    Given a repository with manifest files and an optional existing README.md

  Scenario: Mode detection routes by flags and existing README
    When /doc --mode=readme runs
    Then "--validate" with an existing README skips to council validation only
    And "--rewrite" with an existing README reuses it as rewrite context
    And no README and no flags generates from scratch after an interview

  Scenario: The lead states the problem before the framework
    When the README is generated
    Then the opening line names the user's pain in one plain sentence
    And methodology or framework names do not appear before the problem statement

  Scenario: A trust block sits near the install command
    Given the author reports that the tool runs hooks, modifies config, or makes network calls
    When the README is generated
    Then a trust block stating what it touches, exfiltration posture, and how to uninstall
      appears near the install command, not buried in an FAQ

  Scenario: Depth is collapsed, never deleted
    When deep architecture, theory, or reference material is included
    Then it is placed inside <details> blocks with a blank line after <summary>
    And the skimmer path stays short while deep readers can expand

  Scenario: A council validates before the skill reports complete
    When generation or rewrite finishes
    Then /doc --mode=readme runs a council over the README
    And reports a PASS, WARN, or FAIL verdict rather than claiming done unvalidated

  Scenario: Anti-patterns are flagged on rewrite or validate
    When /doc --mode=readme reviews an existing README
    Then it flags flywheel-echo, framework-first, guru tone, buried trust info,
      install scatter, and theory-before-try with a concrete fix for each
