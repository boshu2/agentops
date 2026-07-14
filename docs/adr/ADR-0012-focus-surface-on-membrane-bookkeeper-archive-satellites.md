# ADR-0012: Directly Cut the Unproven Satellite Surface

- **Status:** Superseded by direct-cut execution (2026-07-14)
- **Original decision:** Archive optional command families behind build tags
- **Current decision:** Retain, merge, or delete every command through one exact
  owner; remove the profile mechanism after the last disposition

## Context

The earlier decision reduced the default command surface by putting corpus and
factory families behind `flywheel` and `legacy` build tags. That made the
headline smaller without reducing the maintained system. It left multiple
compiled products, compatibility tests, generated projections, restoration
instructions, and old owners alive. The result was more surface to reason about
and a standing invitation to preserve behavior that the product no longer owns.

AgentOps now has a narrower responsibility: Discovery shapes accepted behavior,
Crank implements a bounded tranche, Validate records one immutable verdict from
fresh context, and Learn records the smallest useful consequence. Deterministic
evidence supports that judgment. Delivery belongs to the consumer repository,
whether the worker is local or cloud-hosted.

## Decision

Delete legacy code directly in the same owning leaf that installs its replacement or removes its last consumer.

Each current command receives exactly one disposition:

- **keep** under one final root and explicit module owner;
- **merge** into a named final root while deleting the former root and owner;
- **replace** with the final behavior while deleting the old owner; or
- **delete** with its unique tests, fixtures, docs, and dependencies.

No compatibility alias, alternate runtime, dormant scaffold, restoration path,
or dual owner lands between those states. An exact authority-and-consumer
manifest must prove the old and new paths belong to the same leaf before that
leaf starts.

## Sequencing

This ADR changes product authority, not executable code. Later master-plan
leaves own the physical cut:

- K5, K7, and K9 remove model-driving review, repository delivery, and retired
  gate implementations while installing deterministic recorders where needed;
- each exact `CLI.<source>` leaf keeps, merges, replaces, or deletes one compiled
  command root;
- F4 removes the `flywheel`, `legacy`, and combined build profiles plus their
  dedicated compatibility tests;
- D2 regenerates command and skill projections only after executable ownership
  is final.

## Consequences

- There is one final compiled product rather than hidden optional products.
- The four lifecycle umbrellas remain agent-owned; the CLI is a deterministic
  transaction kernel and evidence recorder.
- Historical commits and changelog entries remain history, but no active source
  promises restoration.
- Rollback reverts a complete same-owner leaf. It never restores only an old
  command owner or only a compatibility route.
