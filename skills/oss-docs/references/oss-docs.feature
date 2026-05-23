# Executable spec for the /oss-docs skill — OSS documentation scaffold/audit (generic role).
# /oss-docs prepares a repo for open-source release: it AUDITS which standard docs exist/are
# missing (reading the repo), SCAFFOLDS the missing ones without clobbering, and tailors content
# to the project type. Hexagon: generic; consumes repo-context (audit reads the repo); produces
# documentation. (soc-qk4b)

Feature: OSS-docs audits and scaffolds open-source documentation
  As open-source release prep
  I want the standard docs audited and the missing ones scaffolded to project type
  So that a repo reaches OSS-release doc completeness without overwriting existing work

  Scenario: audit reports which standard docs exist or are missing
    When /oss-docs audit runs
    Then it reads the repo and reports which standard OSS docs exist and which are missing

  Scenario: scaffold creates only the missing standard files
    When /oss-docs scaffold runs
    Then it creates the missing standard files
    And it does not overwrite docs that already exist

  Scenario: generated content is tailored to the project type
    When /oss-docs generates a doc
    Then the content is tailored to the detected project type, not a generic stub
