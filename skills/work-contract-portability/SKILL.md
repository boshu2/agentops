---
name: work-contract-portability
description: 'Triggers: use when designing, reviewing, or handing off agent work that must be portable across agents through clear contracts, evidence, handoffs, durable context, and explicit role boundaries.'
skill_api_version: 1
user-invocable: false
practices:
- design-by-contract
- ddd-bounded-context
- wiki-knowledge-surface
hexagonal_role: domain
consumes:
- task-intent
- evidence
- handoff
produces:
- portability-guidance
- role-boundary-notes
context_rel: []
context:
  window: isolated
  intent:
    mode: questions
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: knowledge
  stability: experimental
  dependencies: []
output_contract: 'stdout: portability guidance that states contracts, evidence, handoff state, durable context, and role boundaries'
---

# Work Contract Portability

## Purpose

Make agent work transferable. Any qualified agent should be able to continue,
review, validate, or replace another agent's work without relying on private
memory, hidden assumptions, or personality-specific habits.

Portable work is not created by making agents identical. It is created by
shaping the task so continuation depends on explicit contracts, recorded
evidence, concise handoffs, durable context, and clear role boundaries.

## Core Rule

Prefer portable work products over personal continuity. A task is portable when
another agent can answer these questions from the artifacts alone:

- What outcome is required?
- What constraints are binding?
- What evidence proves the current state?
- What decisions have already been made, and why?
- What role is this agent playing now?
- What remains open, blocked, risky, or intentionally deferred?

## Operating Principles

### Clear Contracts

Define the work as a contract before optimizing execution. A useful contract
states the requested outcome, accepted inputs, forbidden moves, ownership
boundary, output shape, and validation criteria. If the contract is fuzzy, the
next agent inherits ambiguity instead of progress.

### Evidence

Evidence is the shared memory of the work. Prefer concrete artifacts: command
output, tests, diffs, citations, logs, screenshots, decision records, and
verdicts. Claims without evidence force the next agent to re-investigate the
same ground.

### Handoffs

A handoff should compress state without hiding uncertainty. Name what changed,
what was verified, what was not verified, what failed, and what assumption the
next agent must preserve or revisit. A good handoff is short enough to use and
specific enough to audit.

### Durable Context

Keep context portable by recording only the context needed to continue the work.
Do not depend on chat history as the source of truth. Promote durable facts into
repo files, issue records, generated reports, or explicit handoff notes. Remove
noise, but preserve decision lineage.

### Role Boundaries

State the role being played: researcher, planner, implementer, reviewer,
validator, release operator, or steward. Role boundaries prevent one agent from
silently changing the standard of proof. When a role changes, update the
contract and evidence expectations.

## Application Checklist

Use this checklist when shaping portable agent work:

1. Name the unit of work and its acceptance criteria.
2. List the artifacts another agent must inspect first.
3. Record decisions as decisions, not as incidental prose.
4. Attach evidence to every meaningful claim of completion.
5. Separate durable facts from temporary reasoning.
6. Identify the current role and the next expected role.
7. Close with a handoff that distinguishes done, pending, risky, and blocked.

## Failure Modes

- Private continuity: progress exists only in one agent's memory.
- Evidence debt: conclusions are present, but proof is missing.
- Boundary drift: an implementer silently becomes the approver.
- Context flooding: the next agent receives too much transcript and too little
  contract.
- Handoff theater: a summary sounds complete but omits verification gaps.

## Output

Return portability guidance that makes the work transferable: the contract,
required evidence, handoff state, durable context to preserve, role boundaries,
and any gaps that prevent another agent from safely continuing.
