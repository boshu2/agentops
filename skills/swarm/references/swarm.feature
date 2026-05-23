# Executable spec for the /swarm skill — wave-execution parallel-fork (BC3 Loop, Move 5).
# /swarm is the primitive /crank invokes to run a wave. It spawns fresh-context workers
# in parallel ONLY when the wave is conflict-free (the wave-validity check is all-green);
# otherwise it runs sequential. Parallelism is explicit ownership, not chaos. Hexagon:
# supporting; consumes: implement, vibe; produces: .agents/swarm/results/*.json;
# customer-of crank. (soc-qk4b)

Feature: Swarm executes a wave in parallel only when ownership is conflict-free
  As the loop's wave-execution fork
  I want parallel workers spawned only on a verified-disjoint wave
  So that parallelism is explicit ownership, never a colliding free-for-all

  Background:
    Given a wave of slices handed down by /crank

  Scenario: the wave-validity check gates parallel spawn
    When /swarm prepares to run the wave
    Then it spawns parallel workers only if every wave-validity row is green
      (disjoint write scopes, no shared migration/contract/CLI surface,
       integration order declared, one owner per slice, a discard path per slice)
    And it defaults to sequential when any row is not green

  Scenario: each worker runs in fresh, isolated context
    When workers are spawned
    Then each receives one pre-assigned atomic task and executes it in fresh context (Ralph pattern)
    And no two workers share a write scope

  Scenario: results are captured and the backend is cleaned up
    When the wave completes
    Then per-worker results are written under .agents/swarm/results/*.json
    And spawned backend resources are released

  Scenario: swarm is the wave primitive crank invokes
    Then /crank drives wave execution through /swarm rather than spawning agents directly
