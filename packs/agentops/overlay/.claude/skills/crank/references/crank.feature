# Executable spec for the /crank skill — wave execution (BC3 Loop, Move 5).
# /crank consumes a slice-validation plan and drives an epic to DONE wave by wave,
# dispatching /swarm + /implement per wave under a wave-validity hard gate, looping
# FIRE (Find→Ignite→Reap→Vibe→Escalate) until every issue is closed — then it must
# emit an explicit completion marker. Hexagon: domain; consumes: beads, implement,
# post-mortem, swarm, vibe; produces: wave-by-wave slice completion. (soc-qk4b.2)

Feature: Crank executes an epic through conflict-free waves to completion
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

  Scenario: FIRE loop repeats per wave until issues close
    When a wave runs
    Then /crank executes Find → Ignite → Reap → Vibe → Escalate
    And it loops to the next wave until all issues are closed or a blocker is hit

  Scenario: A completion marker is mandatory
    When /crank stops
    Then it emits exactly one of <promise>DONE</promise>, <promise>BLOCKED</promise>,
      or <promise>PARTIAL</promise>
    And it never claims completion without one

  Scenario: The global wave cap bounds the run
    Given cascading failures or circular dependencies
    Then /crank halts at MAX_EPIC_WAVES (50) rather than looping unbounded
    And reports BLOCKED with the reason
