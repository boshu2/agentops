# Executable spec for the /crank skill — wave execution (BC3 Loop, Move 5).
# /crank consumes a slice-validation plan and executes one ready wave under a
# wave-validity hard gate. It uses one direct implementer by default, runs deterministic
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

  Scenario: Wave evidence exits Crank before any re-plan
    When a wave reaches its acceptance verdict
    Then Crank hands canonical wave evidence to RPI
    And Crank does not invoke Discovery, Validate, Learn, Premortem, or a silent retry

  Scenario: Intermediate waves avoid semantic round trips
    Given the accepted tranche has fewer than three waves and remains under 90 minutes
    And the bound plan inputs and risk are unchanged
    When targeted wave acceptance passes
    Then RPI may pull the next sequential Crank wave
    And Validate and Learn wait until the tranche freezes

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

  Scenario: Crank returns evidence without controlling the next wave
    Given RPI selected one accepted wave of the current leaf
    When /crank completes or blocks that wave
    Then it returns targeted evidence and exact checkpoint identity
    And only the orchestrator records NOTE, REPAIR, REPLAN, HOLD, or ANDON
