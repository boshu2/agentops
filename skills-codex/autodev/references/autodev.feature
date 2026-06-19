# Executable spec for the /autodev skill — bounded autonomous dev loop (supporting role).
# /autodev runs the full operating loop UNATTENDED within a declared contract (PROGRAM.md /
# AUTODEV.md): mutable/immutable scope, validation commands, escalation, stop conditions. It
# manages the contract — it does not replace /evolve or /rpi, which it delegates to. Loop
# discipline still applies under autonomy. Hexagon: supporting; consumes evolve + rpi. (soc-qk4b)

Feature: Autodev runs the operating loop unattended within a declared contract
  As the bounded autonomous-development manager
  I want the loop to run unattended only inside an explicit contract
  So that autonomy stays scoped, validated, and stoppable

  Scenario: a contract bounds the unattended loop
    Given a PROGRAM.md or AUTODEV.md declaring mutable/immutable scope, validation commands,
      escalation rules, and stop conditions
    When /autodev runs
    Then it executes the loop only within that contract

  Scenario: it manages the contract, it does not replace evolve or rpi
    When /autodev operates
    Then it creates/inspects/validates the contract and delegates execution to /evolve and /rpi
    And it does not reimplement the evolve or rpi loops

  Scenario: loop discipline holds under autonomy
    When /autodev runs a wave unattended
    Then no parallel wave runs without the wave-validity check
    And no slice closes without a passing test mapped to a Given/When/Then
    And captured learnings go through the promotion ratchet, not a landfill
