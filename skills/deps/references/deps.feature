# Executable spec for the /deps skill — dependency risk audit (driven-adapter).
# /deps scans the working directory's dependency manifests (go.mod, package.json, ...) and
# audits them for vulnerabilities, outdated versions, and license risk — across multiple
# coexisting ecosystems. Hexagon: driven-adapter; consumes repo-context; produces result.json.
# (soc-qk4b)

Feature: Deps audits dependency risks across ecosystems
  As the dependency-risk auditor
  I want the repo's manifests scanned for vulnerabilities, staleness, and license risk
  So that dependency risk is surfaced before it ships

  Scenario: audit scans the manifests and reports risk
    When /deps audit runs
    Then it scans the working directory for manifest files (go.mod, package.json, ...)
    And it reports vulnerabilities, outdated dependencies, and license issues to result.json

  Scenario: focused modes narrow the audit
    When /deps vuln runs
    Then it focuses on vulnerabilities and remediation
    And /deps license focuses on license compliance

  Scenario: multiple ecosystems are handled together
    Given a repo with manifests for more than one ecosystem
    When /deps runs
    Then it audits each ecosystem's manifest, not just the first one found
