# Gas City reliability development contract

AgentOps develops against one admitted Gas City and Beads pair. Experiments do
not become provenance, and a failure near a dependency does not make that
dependency the owner.

## Ownership

- AgentOps owns role/model policy, worker worktree boundaries, Git command
  policy, semantic validation authority, parallel deconfliction, and delivery.
- Gas City owns only isolated SDK/runtime defects that reproduce on current
  upstream. Generic fixes use focused branches and upstream pull requests.
- Beads source remains untouched unless official `bd` reproduces an isolated
  defect. The issue tracker and the Beads product are different concerns.
- Environment/configuration owns ambiguous binary, city, store, supervisor,
  telemetry, and working-directory observations until exact identity is proven.

The machine-readable observations and dispositions live in
`deploy/gc/known-errors.json`. Every row records its release relevance; fixed,
rejected, and historical observations remain evidence but cannot silently
become release blockers or fork patches.

## Contribution-first fork

`boshu2/gascity` is a true GitHub fork whose `main` mirrors an exact observed
`gastownhall/gascity/main`. Before a clean-slate realignment, push an archive
tag for the prior fork tip and verify it resolves remotely. Use
`--force-with-lease`, never a bare force push.

Generic fixes live only on one focused contribution branch per defect. The
branch carries a minimal reproduction, focused tests, and an upstream PR. A
temporarily admitted GC binary may stack exact open PR commits when a verified
process-safety bug blocks use, but the stack is recorded in provenance and is
never merged into fork `main`.

Record each fork cleanup in `deploy/gc/fork-baseline.json` and run
`validate-fork-manifest`. The observed fork and upstream `main` SHAs must be
equal, every retained branch must reference open upstream work, and deleted
branches must reference merged upstream pull requests. The manifest is dated
evidence and must be regenerated after either remote changes.

## Clean-room workflow

1. Run `deploy/gc/reliability.py inventory` into a durable evidence directory.
   Pass every relevant top-level namespace with `--scan-root`, every retained
   or required-but-missing city with an exact `--path`, both canonical source
   repositories with `--git-repo`, and any proposed release pair with two
   `--selected-binary` arguments. The inventory records missing paths, declared
   Dolt state, live GC/Dolt/compiler processes, tmux sockets and sessions,
   repository worktrees/branches/remotes, ambient binaries, and the exact
   selected toolchain identity without starting a city or store.
   Inventory hashes observed binaries but never executes them; even a nominal
   `version` command can mutate registered runtime state in an old or unknown
   binary. Version/provenance comes from the selected lock and receipt in the
   next gate, not from probing ambient executables.
2. Run `validate-inventory` over the written artifact. The content digest,
   path/process identity uniqueness, ownership classifications, Dolt/tmux
   sections, and selected-toolchain ambiguity check must pass before using the
   artifact for cleanup admission.
3. Create a cleanup plan containing exact absolute targets and expected process
   commands. Never use a glob or name-only deletion rule.
4. Export dirty or unique material outside the target before admitting it.
5. Run `validate-cleanup`, then `apply-cleanup` without `--execute`.
6. Review the returned plan digest. Execution requires that exact digest in
   `--confirm`; process command drift and dirty worktrees fail closed.
7. Execute once. A second execution must be an idempotent no-op.
8. Re-inventory and explain every retained noncanonical process, path, binary,
   Dolt declaration, tmux server, and selected-toolchain result. An ambiguous
   selected pair is a stop, even when an ambient alias happens to work.

Cleanup moves ordinary experiment directories to a recoverable archive rather
than recursively deleting them. Gas City Git worktrees use `git worktree
remove` only when clean. Ambiguous paths are retained for operator disposition.

## Worker Git audit

Before dispatch, snapshot the repository with `git-snapshot`. After the worker
stops, snapshot it again and run `git-audit`. A changed stash, ref, reflog,
worktree set, HEAD/branch, or unexpected worktree content makes the candidate
invalid. The semantic content validator cannot waive this process-policy
failure.

## Proof ladder

1. Registry schema, cleanup admission, Git audit, bootstrap fixtures, and pack
   authority tests.
2. Exact GC/bd provenance and required safety-patch containment.
3. Bootstrap without start, repeated once for idempotence.
4. One role-routing microcanary.
5. Only after that passes, one mixed Codex/interactive-Claude isolated-worktree
   canary with a clean Git audit.
6. One fresh verdict over the exact final subject and external-state manifest.

The second non-PASS, any ambiguous destructive target, identity drift,
unexpected store/process, missing safety patch, forbidden Git mutation, or
cross-worktree write stops the wave. The unstable factory never repairs itself.
