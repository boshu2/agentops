# Executable spec for the /codex-team skill — Codex multi-agent coordination (BC3 Loop).
# /codex-team coordinates a swarm of Codex agents over a set of tasks: it defines the tasks,
# analyzes file targets to keep write-scopes disjoint, spawns agents, and waits for completion
# before reaping results. Hexagon: supporting; consumes: a task set + file-ownership map;
# produces: .agents/swarm/results/*.json. (soc-qk4b)

Feature: Codex-team runs a conflict-free Codex agent swarm
  As an orchestrator parallelizing work across Codex agents
  I want tasks dispatched with disjoint file ownership and joined on completion
  So that parallel work compounds without write collisions

  Background:
    Given a set of tasks to distribute across Codex agents

  Scenario: File targets are analyzed before any agent is spawned
    When /codex-team prepares a wave
    Then it analyzes each task's file targets to keep write-scopes disjoint

  Scenario: Agents are spawned only for a conflict-free wave
    When file ownership is disjoint
    Then it spawns one Codex agent per task in parallel

  Scenario: The wave is joined before results are reaped
    When agents are running
    Then it waits for all agents to complete and collects results into .agents/swarm/results/
