# Bounded Gas City operations

The package-owned surface is `../scripts/safe-gc.sh`. It supports read-only
qualification, one approved prepare, and one approved source-intent message to
the Mayor. It does not expose sling, session creation/repair, affinity apply,
supervisor changes, import changes, doctor fixes, unregister, or cleanup.

## Upgrade and repair boundary

Upgrading a city can change imports, supervisors, registrations, sessions, and
operator trust. Those operations remain upstream operator work and are not
attested by this package. Before any separately authorized upgrade, record the
selected `gc version --json`, city path, rig path, imports lock, and supervisor
binary. Run changes from outside the city and preserve the city until its Beads
state is backed up. Do not treat this reference as approval to mutate them.

## Observation envelope

After dispatch, inspect native run/session/bead/mail state at most 12 times on a
30-second cadence (six minutes total). Stop immediately when the run becomes
`completed`, `failed`, or `canceled`, when Mayor mail requests operator input,
or when the six-minute envelope expires. The envelope never renews itself.

Use visibility in this order:

1. supervisor run census and exact run detail;
2. the source and workflow bead graph;
3. the owning session roster and bounded pane snapshot;
4. Doctor, order history, and storage/events.

Run/bead state outranks prose for workflow progress. Pane evidence outranks a
roster for an interactive wedge. Health machinery proves metabolism, not
semantic acceptance.

On the first explained stall, send at most one further bounded message to the
Mayor naming the bead/run and evidence, then stop. Do not re-sling, manually
create a pack-owned session, edit workflow beads, close beads, patch a live
worktree, or repeatedly wake a worker. The Mayor and reconciler own those
transitions. A second nudge for the same subject is outside this package.

## Completion

Factory completion is runtime evidence only. Report the source bead, exact
run/workflow identifiers, terminal native state when reached, artifacts, and
all unobserved surfaces. A fresh AgentOps validation remains separate.
