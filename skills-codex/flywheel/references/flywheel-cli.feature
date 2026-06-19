# Executable spec for the `ao flywheel` CLI command — the knowledge-flywheel health surface.
# Specs the observable behavior of `ao flywheel {status,compare,gate}`: the health verdict
# (COMPOUNDING / NEAR ESCAPE / DECAYING), the JSON output contract, namespace comparison +
# promotion rule, and the post-structural release-readiness gate. Each scenario links to the
# Go test in cli/cmd/ao/ that proves it. (soc-jnfgi)

Feature: ao flywheel reports knowledge-compounding health and release readiness
  As an operator steering the knowledge flywheel
  I want flywheel health, namespace comparison, and a release-readiness gate
  So that I can tell whether knowledge is compounding and safe to expand on

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestPrintFlywheelStatus_Compounding
  Scenario: status reports COMPOUNDING when escape velocity is positive
    Given metrics whose net velocity clears the escape threshold
    When I run `ao flywheel status`
    Then the output reports the COMPOUNDING status

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestPrintFlywheelStatus_Decaying
  Scenario: status reports DECAYING when knowledge is not compounding
    Given metrics whose net velocity is below the escape threshold
    When I run `ao flywheel status`
    Then the output reports the DECAYING status

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestPrintFlywheelStatus_ContainsEquation
  Scenario: status surfaces the flywheel equation and its terms
    When I run `ao flywheel status`
    Then the output includes the flywheel equation with delta, sigma, and rho terms

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestRunFlywheelStatus_JSONOutput
  Scenario: status emits a structured JSON contract under --output json
    When I run `ao --output json flywheel status`
    Then the JSON carries status, delta, sigma, rho, velocity, compounding, scorecard, and metrics fields

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestBuildNamespaceComparison_PromotionReady
  Scenario: compare flags the shadow namespace as promotion-ready when it beats primary
    Given a shadow namespace that beats primary on sigma with non-regressing rho
    When I run `ao flywheel compare`
    Then the comparison marks the shadow namespace promotion-ready

  @covered-by:cli/cmd/ao/metrics_flywheel_test.go::TestBuildNamespaceComparison_NotReady_RhoRegressed
  Scenario: compare withholds promotion when shadow rho regresses
    Given a shadow namespace whose rho regresses below primary
    When I run `ao flywheel compare`
    Then the comparison reports the shadow namespace as not promotion-ready

  @covered-by:cli/cmd/ao/flywheel_gate_test.go::TestEvaluateFlywheelGate
  Scenario: gate passes when closure, rho, and holdout retrieval all clear their thresholds
    Given research closure has exited unmined, rho is at least 0.55, and holdout precision meets baseline
    When the flywheel gate is evaluated
    Then the gate result is PASS with no reasons

  @covered-by:cli/cmd/ao/flywheel_gate_test.go::TestEvaluateFlywheelGate_FailsOnThresholds
  Scenario: gate fails and lists each unmet threshold as a reason
    Given closure is still unmined, rho is below 0.55, or holdout precision is below baseline
    When the flywheel gate is evaluated
    Then the gate result is FAIL and each unmet condition is reported as a reason
