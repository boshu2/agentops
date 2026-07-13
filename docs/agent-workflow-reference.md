# Agent Workflow Reference

> Canonical on-demand mechanics for tracked repository changes. The doctrine is
> `docs/architecture/operating-loop.md`; this page owns tracker, worktree,
> landing, provenance, and closeout procedure.

## Change shape

A landed change is one coherent, reversible bead arc. Read-only work and a
one-response local or ignored artifact need no bead or worktree. Before a
tracked edit intended to land, claim or create its bead and use a bead-owned
linked worktree. Parallel writers require disjoint scopes; serialize collisions.

Acceptance is executable Given/When/Then behavior with an edge, non-goals,
rollback, and evidence for done. Slice vertically, demonstrate RED for the
intended reason, turn it green with the smallest change, then refactor under
green without changing acceptance.

An autonomous session normally ships 2–4 coherent arcs. At five shipped or
in-flight arcs, stop for a post-mortem and re-plan the remaining set; the
checkpoint may reorder, split, add, or drop later arcs rather than rubber-stamp
continuation.

## Tracker boundary

This repository uses `br` (beads_rust). The private nested ledger is resolved
from the canonical checkout; linked worktrees normally have no `_beads/`:

```bash
BEADS_DIR="$(ao beads dir)" br ready --json
BEADS_DIR="$(ao beads dir)" br show <id> --json
BEADS_DIR="$(ao beads dir)" br update <id> --claim --json
```

Writes fail closed on resolution:

```bash
BEADS_DIR="$(ao beads dir --require)" && export BEADS_DIR
br update <id> --status in_progress --json
br close <id> --reason "Completed" --json
```

`bd`/Dolt is the Gas City substrate store, not this repository's tracker. Never
stage `_beads/` in the public repository. Sync it through its private Git repo:

```bash
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A
git -C "$(ao beads dir)" commit -m "tracker: <summary>"
git -C "$(ao beads dir)" push
```

## Worktree lifecycle

The canonical root is the stable `main` anchor, not task scratch space. Every
foreign worktree ends as merged, preserved, exported, or deleted. Preserve
unfinished work under a documented `codex/preserve-*` ref. Repo-root `.agents/`
is gitignored runtime state; durable public evidence belongs in tracked docs,
provenance, or release artifacts.

Before landing and closeout run:

```bash
bash scripts/check-worktree-disposition.sh
```

## Proof and landing

Run focused checks, then the cockpit gate:

```bash
ao gate check --fast --scope head
```

For bead-backed work, `ao land <bead>` is the canonical landing transition. It
builds a fresh binary, obtains the commit-bound independent pawl verdict, and
performs the atomic landing path. REFUTED or NO-VERDICT stops the land. GitHub
Actions are a tag/PR/manual backstop, not routine direct-main authority.

Public provenance is append-only at `docs/provenance/ledger.jsonl`. Tracker
metadata is a projection; the provenance ledger wins on disagreement.

Branch names use `<type>/<bead>-<scenario>-<slug>`. Commit messages state the
behavioral change and carry the bead/provenance linkage expected by the landing
path. Helper/library extraction includes a shrink-only observational ratchet so
the removed duplication cannot silently regrow; promotion from observation to a
blocking gate requires separately demonstrated precision.

## Closeout

Before reporting completion:

1. Inspect the final diff and status.
2. Map each acceptance scenario to passing evidence.
3. Confirm non-goals and rollback.
4. Record the independent verdict against the exact reviewed artifact.
5. Align bead and provenance state.
6. Report outcome, checks, residual risk, unchecked scope, and required work.

If required work, proof, authority, tracker synchronization, or push remains,
the arc is not done. Never present “ready to push” as completed work.

## Triggered validation

```bash
cd cli && go build ./... && go vet ./... && go test ./...   # Go changes
bats tests/scripts/<focused-suite>.bats                     # shell gates
make regen-all && make regen-check                          # inventory sources
make docs-check                                              # documentation graph
```

Release preparation is explicit: use `scripts/ci-local-release.sh --quick` for
pre-tag iteration and the full command for actual release readiness.

Validation tiers are T0 (required fast gates), T1 (verification), T2 (quality),
and I0 (informational). T0–T2 are required when selected by the declared gate;
I0 reports evidence but does not authorize or block landing.
