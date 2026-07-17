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

## Live evidence

The isolated city `/Users/bo/dev/gc-agentops-factory-v1b` admitted and delivered
this canary on 2026-07-17:

| Stage | Durable identity and result |
|---|---|
| Mayor planning | planning bead `gafv-are`; Codex context `gafv-wisp-mi7` |
| Fresh plan review | review bead `al-yon`; Claude context `gafv-wisp-yjj`; PASS |
| Program | program bead `al-5n6`; graph digest `cedc2ace35c0aab14c20898401bbfc22088dd18c99e147c987973ec033309715` |
| Experiment | experiment bead `al-139`; Claude Worker context `gafv-wisp-8pcm`; frozen candidate `4205fc6194a9c2ee93b2e59473311021b1d955ad` |
| Candidate judgment | fresh Codex Validator context `gafv-wisp-3pw7`; PASS admission certificate attached to `al-139` |
| Refinery | Refinery bead `al-h8n`; Codex Refiner; fenced integration head `5c4af4afbe8813b4b745e2e2e1c2a528d72ee60a` |
| Integration judgment | fresh Claude Validator context `gafv-wisp-9hzj`; PASS |
| Protected delivery | PR [#916](https://github.com/boshu2/agentops/pull/916); required checks passed; landed SHA `b80a752aad3843af66160b08a823aaed57e07169` |

The experiment, Refinery, and program beads are closed with the landed result.
The JSON graph, review, verdict, admission certificate, manifests, and delivery
record remain evidence referenced by bead metadata; none is a work queue or
lifecycle replacement.

## Qualification boundary

This proves the real single-bead happy path, provider interchange across author
and judge roles, exact-subject validation, fenced integration, and protected
landing. It does not prove multiple simultaneous writers, dependent waves,
rejection/rescope, stale fences, moved subjects, dead workers, provider outage,
or rebase recovery; those remain the proof-week fault matrix.
