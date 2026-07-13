# Executable spec for the /rpi skill — one turn's lifecycle executor (BC3 Loop).
# /rpi runs Discovery → Crank → Validate → Learn as strict, non-compressing
# umbrellas that preserve the lifecycle objective end to end: phases never
# skip, validation is never bypassed, and context density survives every phase
# handoff. Hexagon: supporting; consumes: crank, discovery, domain, learn, validate;
# produces: .agents/rpi/*.md. (soc-qk4b.2)

Feature: RPI runs one turn's lifecycle without skipping moves
  As the loop's lifecycle orchestrator
  I want Discovery, Crank, Validate, and Learn run as strict ordered umbrellas
  So that the objective is preserved across the whole turn, with validation enforced

  Background:
    Given a goal, bead, or execution packet entering the rpi lifecycle

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Phases run in order and never compress
    When /rpi executes
    Then it runs Discovery, then Crank, then Validate, then Learn in order
    And no phase is skipped or merged into another (strict delegation is on by default)

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Validation cannot be skipped
    When /rpi reaches the end of Crank
    Then Validate runs before Learn and before the turn is considered done
    And the lifecycle objective is preserved, not silently dropped at a phase boundary

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: A material verdict changes the remaining plan before Premortem
    Given work remains and Learn reports material_change
    When the orchestrator consumes the Learn receipt
    Then it changes the remaining plan through Discovery
    And it runs Premortem on that changed plan before the next Crank wave

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: No material delta permits an explicit no-op decision
    Given work remains and Learn reports no_change
    When the orchestrator consumes the Learn receipt
    Then it may retry, continue, stop, or escalate without fabricating a learning
    And it does not invoke Premortem

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
  Scenario: Autonomous run produces durable phase artifacts
    Given /rpi --auto
    Then the full lifecycle runs without per-phase human approval
    And it writes phase artifacts under .agents/rpi/*.md
