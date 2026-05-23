# Executable spec for the /pr-research skill — upstream repo research (driven-adapter).
# /pr-research is the FIRST step before contributing to an external repo: it explores the
# upstream codebase's conventions, structure, and contribution process and writes a cited
# research artifact, before any planning or implementation. Hexagon: driven-adapter; consumes
# external-api; produces result.json (+ .agents/research/YYYY-MM-DD-upstream-*.md). (soc-qk4b)

Feature: PR-research explores an upstream repo before contributing
  As the first step of an open-source contribution
  I want the upstream repo's conventions and contribution process understood and recorded
  So that planning and implementation start from how the upstream actually works

  Scenario: the upstream repo is explored and recorded
    When /pr-research runs on an upstream repo
    Then it explores the repo's structure, conventions, and contribution process
    And it writes the findings to .agents/research/YYYY-MM-DD-upstream-*.md

  Scenario: it runs before planning or implementation
    When a contribution to an external repo begins
    Then /pr-research is the first step, before /pr-plan or /pr-implement

  Scenario: findings are cited
    When the research artifact is written
    Then claims about the upstream repo carry references to the upstream source
