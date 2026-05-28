# Executable spec for the /release skill — release validation + tag handoff (BC5 Runtime).
# /release takes a project from "code is ready" to "tagged, pushed by the operator,
# and verified green on the exact tagged SHA": it validates readiness (--check),
# curates release notes and reconciles CHANGELOG.md, then leaves the tag/push to the
# operator and verifies CI on the tagged SHA. Hexagon: supporting; consumes: repo +
# CI state; produces: CHANGELOG update, curated notes, exact-SHA CI verdict. (soc-qk4b)

Feature: Release validates readiness and verifies the tagged SHA
  As a maintainer shipping a version
  I want readiness validated and the tagged SHA verified green
  So that a release is provably built from exactly what was tagged

  Background:
    Given a repository whose code is ready to release

  Scenario: Check mode validates readiness without tagging
    When /release runs in --check mode
    Then it reports release readiness and does not create a tag

  Scenario: Release notes are curated and the changelog reconciled
    When release notes are produced
    Then a curated notes file is generated and CHANGELOG.md drift is reconciled

  Scenario: Tagging is the operator's action, not the skill's
    When the release is ready
    Then the skill prepares the release but the operator performs the tag and push

  Scenario: CI is verified on the exact tagged SHA
    When a tag is pushed
    Then /release confirms the CI verdict is green on that exact SHA before declaring done
