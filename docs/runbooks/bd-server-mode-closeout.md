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

Use a bounded wrapper for every bd/Dolt closeout command. On macOS this repo
expects GNU `gtimeout` from coreutils; on Linux, `timeout` is usually present.

```bash
bd_timeout() {
  local seconds="${1:-8}"
  shift
  if command -v gtimeout >/dev/null 2>&1; then
    gtimeout "${seconds}s" "$@"
    return $?
  fi
  if command -v timeout >/dev/null 2>&1; then
    timeout "${seconds}s" "$@"
    return $?
  fi
  "$@"
}
```

Run this after any `bd update`, `bd close`, dependency edit, or issue creation:

```bash
bd_timeout 8 bd vc status
bd_timeout 8 bd dolt commit -m "tracker: <summary>"   # if tracker changes are pending
bd_timeout 8 bd dolt remote list
bd_timeout 8 bd dolt push                             # only when a real remote is configured
```

Then complete the repo closeout:

```bash
git pull --rebase
git push
git status
```

`git push` remains mandatory for repository changes. `bd dolt push` is
conditional on a real remote.

## Command Timeout Outcomes

If a bd command times out before producing output, treat it as failed and retry
once after checking `bd vc status`. If a bd write command emits valid JSON and
then times out, treat the result as indeterminate-success:

1. Preserve the emitted stdout in the session notes.
2. Re-read the emitted issue ID with `bd_timeout 8 bd show <id> --json`.
3. If the readback matches the requested state, continue and record
   "bd command timed out after emitting JSON; readback confirmed".
4. If readback fails or shows stale state, retry once, then file a linked
   follow-up issue with the command, timeout, stdout, and stderr.

If `bd dolt remote list` prints "No remotes configured." and then times out,
record "bd Dolt remote list timed out after no-remote output", skip `bd dolt
push`, and continue to the mandatory Git push.

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

1. Run `bd_timeout 8 bd vc status`.
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
