# Executable spec for the /rpi skill — one turn's lifecycle executor (BC3 Loop).
# /rpi runs Research → Plan → Implement as strict, non-compressing phases that
# preserve the lifecycle objective end to end: phases never skip, validation is
# never bypassed, and context density survives every phase handoff. Hexagon:
# supporting; consumes: crank, discovery, domain, ratchet, validation;
# produces: .agents/rpi/*.md. (soc-qk4b.2)

Feature: RPI runs one turn's lifecycle without skipping moves
  As the loop's lifecycle orchestrator
  I want Research, Plan, and Implement run as strict ordered phases
  So that the objective is preserved across the whole turn, with validation enforced

  Background:
    Given a goal, bead, or execution packet entering the rpi lifecycle

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Phases run in order and never compress
    When /rpi executes
    Then it runs Research, then Plan, then Implement in order
    And no phase is skipped or merged into another (strict delegation is on by default)

  @covered-by:tests/e2e/rpi-phased-domain.sh
  Scenario: Validation cannot be skipped
    When /rpi reaches the end of Implement
    Then validation runs before the turn is considered done
    And the lifecycle objective is preserved, not silently dropped at a phase boundary

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
