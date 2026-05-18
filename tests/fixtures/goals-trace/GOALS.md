# Agent Goals

Strategic intent layer for the goalstrace walker fixture set.

## Directives

### 1. Fitness gate blocks on scenario satisfaction

**Directive ID:** d-fitness-gate-bdd
**Steer:** Make `ao goals measure` fail when linked scenarios are unsatisfied.
**Setpoint:** 100% of active scenarios judged before a gate passes.
**Scenarios:** s-2026-05-17-001, s-2026-05-17-002

The fitness gate must consume scenario-results artifacts and refuse to pass
when an active scenario has no verdict.

### 2. Trace chain is rendered end to end

**Directive ID:** d-trace-chain-render
**Steer:** Render directive to scenario to bead to verdict to learning.
**Setpoint:** Every directive has a navigable trace.

This directive deliberately declares no Scenarios attribute so the fixture
exercises the directive_no_scenarios warning path.
