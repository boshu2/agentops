# Executable spec for the /security skill — repository security scans (driven-adapter).
# /security runs the available scanners over the repo, gates on high/critical findings, and
# retains artifacts for audit. Its report feeds vibe's verdict. Hexagon: driven-adapter;
# consumes repo-context; produces security-report.json; supplier-to vibe. (soc-qk4b)

Feature: Security scans the repository and gates on severity
  As the repository security scanner
  I want the available scanners run and high/critical findings to fail the scan
  So that severe vulnerabilities block the release path

  Scenario: scanners run over the repository
    When /security runs
    Then it runs the available scanners over the repo and writes security-report.json

  Scenario: high or critical findings fail the scan
    When a scanner reports a high or critical finding
    Then /security fails (it does not pass with severe findings outstanding)

  Scenario: a clean full pass gates the release path
    When the full scanner pass reports no high/critical findings
    Then the release workflow may continue

  Scenario: artifacts are retained for audit
    When a scan completes
    Then its artifacts are retained for audit and incident response
