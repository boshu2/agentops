# Gas City Factory — Live Bead Canary

This document is a live factory canary. It is a small, real exercise of the
bead-native factory to confirm that the lifecycle and delivery invariants below
hold when work actually flows through Gas City. It is a canary, not a release,
and it does not prove rejection, rebase, outage, or recovery paths.

## Invariants

1. **The bead is the unit of work and the lifecycle truth.** In Gas City the
   bead is the single unit of work and the one source of lifecycle truth; every
   claim, state change, and closure is recorded on a bead.

2. **A pack is configuration, not work.** A pack configures reusable roles and commands, but it is not a work unit; it has no lifecycle of its own and never substitutes for a bead.

3. **All work is beads and dependencies.** Program, experiment, rescope or successor, and Refinery work are each represented by beads and dependencies between them — never by a separate side channel.

4. **Evidence files are immutable, not state.** Graph, verdict, admission, and delivery JSON files are immutable evidence referenced from bead metadata, and are not a parallel lifecycle state machine; the bead remains the authority.

5. **Delivery is protected.** Protected pull-request delivery is required, and
   agents do not push directly to main; every change reaches main only through a
   reviewed, protected pull request.

## Scope

This canary confirms that a single bounded unit of work moved through the
bead-native lifecycle and honored protected delivery. It is not a release and
is not proof of every failure mode.
