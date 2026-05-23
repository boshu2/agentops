# Executable spec for the /grafana-platform-dashboard skill — dashboard validation (BC5 Runtime).
# /grafana-platform-dashboard builds and validates OpenShift Grafana dashboards against the
# platform contracts: it locks scope from those contracts, enforces the information architecture,
# builds queries only from the known library, validates before apply, then applies and verifies
# sync. Hexagon: driven-adapter; consumes: platform contracts + query library; produces:
# validated Grafana dashboard JSON. (soc-qk4b)

Feature: Grafana-platform-dashboard validates dashboards against platform contracts
  As a platform operator publishing observability dashboards
  I want dashboards built from contracts and validated before apply
  So that dashboards stay consistent and never apply broken queries

  Background:
    Given the platform contracts and the known query library

  Scenario: Scope and information architecture come from the contracts
    When the dashboard is built
    Then scope is locked from the platform contracts and the information architecture is enforced

  Scenario: Queries are built only from the known library
    When panels are assembled
    Then queries are drawn from the known query library rather than ad-hoc PromQL

  Scenario: Validation precedes apply, and sync is verified after
    When the dashboard is ready
    Then it is validated before apply, then applied and verified in sync
