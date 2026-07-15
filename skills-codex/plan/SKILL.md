---
name: plan
description: Shape intent into one behavior-first
---
# Plan

Turn the caller's intent into one bounded, testable behavior. Planning owns
research necessary to understand the behavior, acceptance shaping, and scope.
It does not schedule, claim, assign, implement, validate, or decide readiness.

## Workflow

1. Restate the intent and choose one active behavior.
2. Inspect only enough real context to make paths, interfaces, and evidence
   concrete. Existing research and specialist skills are advisory inputs.
3. Write at least one normal and one edge Given/When/Then scenario.
4. Name non-goals and the evidence required to judge every criterion.
5. Declare `write_scope.include` and `write_scope.exclude`, including generated
   companions. Scope describes permitted subject content; it grants no file
   ownership or delivery authority.
6. Name `first_acceptance_check` as either an executable command or an artifact
   path. Commands are data for the implementer, never executed by a packet
   parser.
7. Canonically serialize `plan-packet.v1` and compute its SHA-256 digest.

Optional decomposition may describe smaller behaviors, but it carries no owner,
ready, claim, priority, attempt, wave, queue, lease, admission, next-action,
close, release, or delivery fields.

## Required output

The packet conforms to [`schemas/plan-packet.v1.schema.json`](../../schemas/plan-packet.v1.schema.json)
and contains:

- intent and acceptance digests;
- one active behavior;
- normal and edge scenarios;
- non-goals and required evidence;
- inclusive and exclusive write scope;
- one first acceptance check;
- optional advisory decomposition.

Report the packet location and digest, then stop.
