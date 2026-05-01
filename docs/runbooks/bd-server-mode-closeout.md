# bd Server-Mode Tracker Closeout

This runbook distinguishes Git push, local bd durability, and remote bd sync.
It prevents agents from treating `bd dolt push` as mandatory when a workspace
uses a server-mode direct-write tracker with no configured Dolt remote.

## Tracker Modes

| Mode | Signal | Closeout rule |
|------|--------|---------------|
| JSONL-backed | `.beads/issues.jsonl` is tracked | Refresh the JSONL export after tracker writes, then commit and push Git. |
| Server-mode direct-write | `bd vc status` reports a shared Dolt server/database and `bd dolt remote list` is empty | Commit local tracker changes when needed; do not run or require remote push. |
| Remote-backed Dolt | `bd dolt remote list` returns a real backup/team remote | Commit tracker changes, then run `bd dolt push`. |

## Current AgentOps Fleet State

The active AgentOps development workspaces are wired to the shared `bushido`
server-mode database. Historical issue IDs still use the `soc` prefix. The
normal closeout path for these workspaces is local/server Dolt commit plus Git
push, not unconditional `bd dolt push`.

Runtime/city trackers with different prefixes stay isolated unless they get
their own served database:

- `/home/boful/cities/bushido` keeps `bu`.
- `/home/boful/ops/mt-olympus` keeps `mo`.

Do not join a differently prefixed tracker to `bushido` just to make session
closeout look uniform.

## Standard Closeout

Run this after any `bd update`, `bd close`, dependency edit, or issue creation:

```bash
bd vc status
bd dolt commit -m "tracker: <summary>"   # if tracker changes are pending
bd dolt remote list
bd dolt push                             # only when a real remote is configured
```

Then complete the repo closeout:

```bash
git pull --rebase
git push
git status
```

`git push` remains mandatory for repository changes. `bd dolt push` is
conditional on a real remote.

## No Remote Is Configured

If `bd dolt push` reports that no remote is configured:

1. Treat local tracker durability as handled by the local/server Dolt commit.
2. Record "bd Dolt push unavailable: no remote configured" in the session
   report.
3. Continue to the mandatory Git push.

Do not add a self-remote that points back to the same local server/database.
That can turn a clear "no remote" signal into a confusing no-common-ancestor
failure without improving backup or team sync.

## Mismatched Database Or Project State

If `bd show` or `bd ready` reads stale issues, missing issues, or an unexpected
database:

1. Run `bd vc status`.
2. Inspect the active database and project id from the workspace metadata or
   bd diagnostic output.
3. Verify the served database exists before mutating tracker state.
4. If recovery requires import, record the source and target database in the
   issue notes before closing the work.

This is a blocker for issue closure, not a reason to add a remote blindly.

## Agent Contract

- Agents must not fail a session solely because `bd dolt push` has no remote.
- Agents must not claim tracker sync to a remote when `bd dolt remote list` is
  empty.
- Agents must still push the Git branch and verify it tracks the remote branch.
- Agents should use the beads skill wording: "run `bd dolt push` only when a
  Dolt remote is configured."
