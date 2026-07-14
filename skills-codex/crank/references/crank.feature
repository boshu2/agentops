# Executable spec for the /crank skill — wave execution (BC3 Loop, Move 5).
# /crank consumes a slice-validation plan and executes one ready wave under a
# wave-validity hard gate. It dispatches /swarm + /implement, runs deterministic
# FIRE acceptance, and returns evidence before any between-wave decision.
# Hexagon: domain; consumes: beads, implement, swarm; produces: wave evidence.

Feature: Crank executes one conflict-free epic wave
  As the loop's wave executor
  I want each epic driven wave by wave under an explicit validity gate
  So that parallelism is owned, not chaotic, and the epic provably reaches DONE

  Background:
    Given an epic (or plan) with ready issues and a slice-validation plan

  Scenario: Wave-validity hard gate precedes parallel dispatch
    When /crank assembles a wave
    Then it dispatches in parallel only if every row holds: distinct write scopes,
      no shared migration/contract/CLI surface, declared integration order,
      owner per slice, and a discard path per slice
    And any failed row forces those slices to run sequentially, not in parallel

  Scenario: One FIRE wave returns before another wave starts
    When a wave runs
    Then /crank executes Find → Ignite → Reap → Vibe → Escalate
    And it returns DONE, PARTIAL, or BLOCKED evidence before another wave starts

  @covered-by:scripts/validate-workflow-contract.sh
  Scenario: Wave evidence exits Crank before any re-plan
    When a wave reaches its acceptance verdict
    Then Crank hands the wave evidence to Validate
    And Crank does not invoke Discovery, Learn, Premortem, or a silent retry

  Scenario: Orchestrator's own diff-read flags an out-of-boundary slice at acceptance
    Given a slice whose evidence JSON claims PASS and a green <promise>DONE</promise>
    But whose wave diff touches files outside the slice's declared write-scope boundary
    When the orchestrator reads the actual diff at Wave Acceptance (Step 3.5),
      distinct from the delegated sub-judges
    Then the slice is flagged and the wave verdict is FAIL, not silently counted
    And the out-of-scope file list is surfaced to the operator

  Scenario: A completion marker is mandatory
    When /crank stops
    Then it emits exactly one of <promise>DONE</promise>, <promise>BLOCKED</promise>,
      or <promise>PARTIAL</promise>
    And it never claims completion without one

  Scenario: The persistent governor bounds wave dispatch
    Given a stable RPI run with declared run-wide ceilings
    When /crank requests another wave admission
    Then it dispatches only after the governor durably returns authorized true
    And a refused admission returns BLOCKED evidence without resetting counters
