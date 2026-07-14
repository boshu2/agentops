# Build Tags During the Direct Cut

> Current executable truth and deletion ownership for the transitional command
> profiles introduced by [ADR-0012](../adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md).

## Current state

The current tree still compiles `flywheel`, `legacy`, and combined variants.
That fact is transitional, not a product contract. New code must not add a tag,
profile membership, restoration command, tagged fallback, or profile-specific
behavior.

Until F4 executes, the existing diagnostic commands remain useful for proving
the inventory that must disappear:

```bash
make verify-buildtags
AO_DUMP_REGISTERED_CMDS=1 go -C cli test -tags=flywheel,legacy \
  ./cmd/ao -run '^TestDumpRegisteredTopLevelCommands$' -count=1 -v
```

These commands describe the old tree; they do not authorize consumers to depend
on it.

## Cut ownership

The exact CLI disposition manifest assigns every currently compiled top-level
root to one leaf. That leaf retains behavior under a final owner or deletes it
with its unique tests, docs, fixtures, and dependencies. K5, K7, and K9 own the
four roots coupled to verdict, delivery, and retired gate behavior. Other roots
belong to their exact `CLI.<source>` leaves.

After those leaves finish, F4 deletes:

- the `flywheel`, `legacy`, and combined profile constants;
- build-tag-only command owners and tagged fallbacks;
- profile selection and default-root pruning;
- build-profile scripts, fixtures, and compatibility tests; and
- documentation that teaches restoration.

D2 then regenerates the command reference and other declared projections once
from the single final executable source.

## Admission rule

A command-cut leaf is not ready until its checked authority-and-consumer
manifest is complete and disjoint. The old owner and its replacement or last
consumer must share one writer, one candidate, one acceptance check, and one
rollback. No intermediate dual runtime is admitted.
