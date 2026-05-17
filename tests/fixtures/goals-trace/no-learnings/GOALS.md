# Agent Goals

Fixture: a valid GOALS.md with scenarios but no docs/learnings/ directory.
Exercises graceful degradation when the learnings dir is absent.

## Directives

### 1. Fitness gate blocks on scenario satisfaction

**Directive ID:** d-fitness-gate-bdd
**Steer:** Make `ao goals measure` fail when linked scenarios are unsatisfied.
**Scenarios:** s-2026-05-17-001
