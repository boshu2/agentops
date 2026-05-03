# Workflow Guide

> Which workflow should I use? Find the right one in seconds.

## Golden Paths

Use these as the default user journeys before reaching for the full decision
tree.

| Situation | Start with | Continue with |
|-----------|------------|---------------|
| First repo setup | `ao quick-start` | `/quickstart`, then `/rpi "a small goal"` |
| Small, obvious change | [Quick Fix](quick-fix.md) | `/implement`, then `/vibe --quick recent` |
| New feature or broad change | [Complete Cycle](complete-cycle.md) | `/rpi "goal"` or `/plan` -> `/crank <epic-id>` |
| Existing tracked backlog | `bd ready` | `/implement <issue-id>` or `/crank <epic-id>` |
| Review-only checkpoint | `/council validate this PR` | `/vibe recent` before merge |
| Terminal-native operation | `ao factory start --goal "goal"` | `ao rpi phased "goal"` and `ao rpi status` |

## Decision Tree

| I want to... | Use this workflow | Complexity | Key Skills |
|--------------|-------------------|------------|------------|
| Fix a simple bug or make a quick change | [Quick Fix](quick-fix.md) | Low | /implement |
| Investigate and fix a hard bug | [Debug Cycle](debug-cycle.md) | Medium | /bug-hunt, /implement, /retro |
| Build a new feature end-to-end | [Complete Cycle](complete-cycle.md) | High | /research, /plan, /crank, /vibe |
| Deploy infrastructure changes | [Infrastructure Deployment](infrastructure-deployment.md) | High | /plan, /implement, /vibe |
| Work across multiple domains at once | [Multi-Domain](multi-domain.md) | High | /plan, /swarm |
| Validate research assumptions before planning | [Assumption Validation](assumption-validation.md) | Medium | /research |
| Understand a new codebase or extract patterns | [Knowledge Synthesis](knowledge-synthesis.md) | Medium | /research, /knowledge-activation |
| Run a retrospective after completing work | [Post-Work Retro](post-work-retro.md) | Low | /retro, /post-mortem |
| Improve system quality over time | [Continuous Improvement](continuous-improvement.md) | Medium | /retro, /vibe |
| Manage a session from start to finish | [Session Lifecycle](session-lifecycle.md) | Low | (natural language) |
| Coordinate N autonomous workers without a leader | [Meta-Observer Pattern](meta-observer-pattern.md) | High | /swarm |

## Workflows

### [Quick Fix](quick-fix.md)
**When to use:** Simple, low-risk changes to 1-2 files with an obvious solution.
**Complexity:** Low (10-30 min, single session)
**Key skills:** /implement

### [Debug Cycle](debug-cycle.md)
**When to use:** Production incidents, intermittent failures, performance issues, or integration bugs with unknown root cause.
**Complexity:** Medium-High (1-3 hours, 1-3 sessions)
**Key skills:** /bug-hunt, /implement, /retro

### [Complete Cycle](complete-cycle.md)
**When to use:** New features, complex multi-file changes, or architectural work that needs full Research-Plan-Implement-Validate-Learn phases.
**Complexity:** High (2-4 hours, 2-4 sessions)
**Key skills:** /research, /plan, /crank, /vibe, /retro

### [Infrastructure Deployment](infrastructure-deployment.md)
**When to use:** Infrastructure deployments that need validation gates, reality checks, and tracer bullets at every phase.
**Complexity:** High (multi-session)
**Key skills:** /plan, /implement, /vibe

### [Multi-Domain](multi-domain.md)
**When to use:** Work spanning multiple domains simultaneously (backend + frontend + infra, technical + personal tracking).
**Complexity:** High (multi-session)
**Key skills:** /plan, /swarm

### [Assumption Validation](assumption-validation.md)
**When to use:** Before planning, to verify that research findings match reality (APIs exist, images available, operators behave as expected).
**Complexity:** Medium
**Key skills:** /research

### [Knowledge Synthesis](knowledge-synthesis.md)
**When to use:** Onboarding to a new codebase, creating comprehensive docs, extracting patterns from multiple implementations.
**Complexity:** Medium (30-60 min, single session)
**Key skills:** /research, /knowledge-activation

### [Post-Work Retro](post-work-retro.md)
**When to use:** After completing significant work, to capture learnings and identify improvements.
**Complexity:** Low
**Key skills:** /retro, /post-mortem

### [Continuous Improvement](continuous-improvement.md)
**When to use:** Periodic reviews (weekly/monthly), after milestones, technical debt reduction, or pattern refinement.
**Complexity:** Medium (ongoing, periodic)
**Key skills:** /retro, /vibe

### [Session Lifecycle](session-lifecycle.md)
**When to use:** Managing any working session from start to finish -- starting work, resuming context, wrapping up.
**Complexity:** Low (use natural language)
**Key skills:** Just talk naturally; commands are optional.

### [Meta-Observer Pattern](meta-observer-pattern.md)
**When to use:** Coordinating N autonomous workers through shared memory (stigmergy) without central orchestration.
**Complexity:** High (advanced, multi-session)
**Key skills:** /swarm
**Deep dive:** [meta-observer/](meta-observer/) (pattern guide, examples, showcase)

## Flowchart

```
Start here
    |
    v
Is it a simple, obvious fix? ----YES----> Quick Fix
    |
    NO
    |
    v
Is something broken? ----YES----> Debug Cycle
    |
    NO
    |
    v
Am I deploying infrastructure? ----YES----> Infrastructure Deployment
    |
    NO
    |
    v
Does it span multiple domains? ----YES----> Multi-Domain
    |
    NO
    |
    v
Am I building something new? ----YES----> Complete Cycle
    |
    NO
    |
    v
Am I trying to understand code/patterns? ----YES----> Knowledge Synthesis
    |
    NO
    |
    v
Am I wrapping up or improving? ----YES----> Post-Work Retro / Continuous Improvement
```
