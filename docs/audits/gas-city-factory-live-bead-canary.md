# Gas City Factory — Live Bead Canary

This document records live factory canaries. They are small, real exercises of
the bead-native factory to confirm that the lifecycle, parallel-isolation, and
delivery invariants below hold when work actually flows through Gas City. They
are canaries, not releases, and they do not prove every failure path.

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

## Protected-delivery scope

This canary confirms that a single bounded unit of work moved through the
bead-native lifecycle and honored protected delivery. It is not a release and
is not proof of every failure mode.

## Protected-delivery evidence

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

## Mixed-provider parallel qualification

The isolated city `gc-agentops-deterministic-city-20260719-v16` ran the current
qualified toolchain pair on 2026-07-19. The Mayor admitted two dependency-free,
disjoint write scopes, so Gas City ran both experiments concurrently in
separate Git worktrees. Qualification delivery exercised the real Refiner and
integration Validator but intentionally performed no push, PR, merge, or
base-branch mutation.

| Stage | Durable identity and result |
|---|---|
| Toolchain | GC source `8dc1f0dfc8164b751d0c63bed051a468a44a3d51`; official Beads 1.1.0 source `8e4e59d39f3459a43cf21a3236a13eca4dd874f7` |
| Mayor planning | Claude/Opus 4.8 context `gadc2v-neh`; planning bead `gadc2v-37t`; graph digest `50528f1e520af47448c1930bf9f049e66479358e9f6cc18e9e1f438f15258c7b` |
| Fresh plan review | Codex/Sol context `gadc2v-3r0`; review bead `ag-911`; PASS; program bead `ag-nrt` |
| Codex experiment | bead `ag-t0b`; Terra Worker context `gadc2v-7g6`; candidate `8c7c036864372e7c1e62cf9838d4fddcf4562be7`; Claude/Opus Validator context `gadc2v-mgp`; PASS |
| Claude experiment | bead `ag-m59`; Opus 4.8 Worker context `gadc2v-thg`; candidate `a5dfa026094aea04dc7e0d4be53a04b603f6db8f`; Codex/Sol Validator context `gadc2v-261`; PASS |
| Refinery | bead `ag-zmk`; Claude/Opus Refiner context `gadc2v-qyk`; integration SHA `55f09b95cfd80ee127d93f0d0d8f9a1e67cfb653` |
| Integration judgment | fresh Codex/Sol context `gadc2v-j1y`; PASS; verdict digest `889313f59443bd538723540a79fe04be4ae029fe9a360ea3b3b52fe3a7887dc0` |
| Qualification receipt | delivery-record digest `a100d68d327a3fd3bf64a1daf6d83e9b09cc59715a1b0297dcf357da7d6f06fd`; program and Refinery beads closed `qualified` |

Every listed role completed on its original context. No replacement session,
manual nudge, or retry command was used. The two experiments ran in one parallel
wave, the GC-owned Refiner launched the opposite-family integration Validator,
and the original Refiner process waited for and consumed its verdict. An
independent Git tree check found exactly the two declared files with their exact
newline-terminated bytes. Managed teardown then stopped the private supervisor,
Dolt service, tmux socket, and city-scoped processes cleanly.

## Qualification boundary

Together these canaries prove the real single-bead protected-delivery path,
provider interchange across author and judge roles, two simultaneous isolated
writers with disjoint scopes, exact-subject validation, fenced integration,
qualification-only delivery, and protected landing. They do not prove dependent
multi-wave execution, rejection/rescope, stale fences, moved subjects, dead
workers, provider outage, or rebase recovery; those remain fault-matrix work.
