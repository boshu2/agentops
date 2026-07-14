# Executable spec for the /rpi skill — one turn's lifecycle executor (BC3 Loop).
# /rpi preserves the Discovery → Crank → Validate → Learn authority boundaries
# while batching cheap waves into one bounded proof transaction. Validation is
# never bypassed, and context density survives each canonical handoff. Hexagon:
# supporting; consumes: crank, discovery, domain, learn, validate;
# produces: .agents/rpi/*.md. (soc-qk4b.2)

Feature: RPI runs one turn's lifecycle without skipping moves
  As the loop's lifecycle orchestrator
  I want Discovery, Crank, Validate, and Learn run as strict ordered umbrellas
  So that the objective is preserved across the whole turn, with validation enforced

  Background:
    Given a goal, bead, or execution packet entering the rpi lifecycle

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Typed responsibilities run in order without duplicate theater
    When /rpi executes
    Then Discovery shapes one bounded tranche and Crank runs one to three waves
    And one frozen tranche runs Validate, then Learn, in order
    And no typed responsibility or independent verdict is skipped
    But four handwritten phase summaries are not required

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Validation cannot be skipped
    When /rpi reaches the frozen tranche boundary
    Then one fresh Validate runs before Learn and before the turn is considered done
    And the lifecycle objective is preserved, not silently dropped at a phase boundary

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: Intermediate waves do not pay semantic proof cost
    Given a bounded tranche has another low-risk wave
    And acceptance, dependencies, write scope, and risk are unchanged
    When the prior wave passes targeted deterministic acceptance
    Then RPI may invoke Crank for the next sequential wave
    And it runs no per-wave Validate, Learn, delivery, or duplicate summary

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: Material plan change earns one new Premortem
    Given an intermediate wave changes acceptance, dependencies, write scope, or risk
    When the orchestrator classifies the evidence
    Then it returns REPLAN through Discovery
    And one fresh Premortem judges the changed plan before more work

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: Terminal work skips Premortem
    Given no work remains and Learn reports terminal
    When the orchestrator consumes the Learn receipt
    Then it closes the tick without re-planning
    And it does not invoke Premortem

  @covered-by:tests/integration/test-four-umbrella-packet.sh
  Scenario: Missing Learn is rejected
    Given a legacy packet with Discovery, Crank, and Validate receipts only
    When the completion receipt validator runs
    Then it rejects the packet and names the missing Learn receipt

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Context density survives phase handoffs
    When one phase hands off to the next
    Then intent, boundary, evidence, decision, constraint, and next-action carry forward
    And anything else is omitted or linked, not pasted wholesale

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Autonomous run produces canonical evidence without duplicate prose
    Given /rpi --auto
    Then the full lifecycle runs without per-phase human approval
    And the execution packet indexes canonical Crank evidence, Validate result.json, and learn-receipt.json
    And any legacy phase summary is a link-only compatibility projection

  @covered-by:skills/rpi/scripts/validate.sh
  Scenario: The bounded tranche stops before runaway execution
    Given an automatic goal has more aggregate demand
    When the tranche completes three waves or reaches 90 minutes
    Then RPI stops pulling new work and records exact resume state
    And the boundary is PARTIAL rather than HOLD, ANDON, or proof authorization

  @covered-by:tests/scripts/rpi-run-disposition.bats
  Scenario: Evidence selects one next move without a phase controller
    Given a wave, check, or review returned evidence for the current objective
    When the orchestrator records the next move
    Then the record is NOTE, REPAIR, REPLAN, HOLD, or ANDON
    And it binds the objective and evidence digests
    But it contains no counter, reservation, cost state, or helper state
