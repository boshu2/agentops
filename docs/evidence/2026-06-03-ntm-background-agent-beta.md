# NTM background-agent beta report — 2026-06-03

## Scope

Two live NTM background agents tested the AgentOps background-agent branch
`cursor/ag-n90tx-ntm-background-sessions-1944415608179314`:

- Claude worker `JadeBeacon` (mcp-agent-mail agent #169).
- Codex worker `JadeElk`.

They were instructed not to edit repo files, not to claim beads, and not to use
deprecated `ao rpi` / `ao evolve` wrappers. The test goal was to exercise the
new NTM/background-agent CLI surfaces like a beta user and report bugs.

## Commands exercised

- `go run ./cli/cmd/ao agent roster --json`
- `go run ./cli/cmd/ao agent init-prompt --runtime <runtime> --mailbox <name>`
- `go run ./cli/cmd/ao agent assign-prompt ...`
- `go run ./cli/cmd/ao agent ntm-spawn ...`
- `go run ./cli/cmd/ao agent eligible --file <fixture> --eligible-only`
- `AGENTOPS_BACKGROUND_E2E=1 tests/background-agents/e2e.sh`
- Installed PATH checks:
  - `ao session bootstrap --json`
  - `ao agent roster --json`

## Findings from beta pass

| Finding | Severity | Resolution |
|---|---:|---|
| Root `go run ./cli/cmd/ao ...` failed because the Go module lived under `cli/`. | High | Added root `go.work` using `./cli`; retest passed from worktree root. |
| Roster `bootstrap` field used `ao session bootstrap` while init prompt required `ao session bootstrap --json`. | Medium | Updated roster bootstrap to use `ao session bootstrap --json`. |
| `ntm-spawn --codex 1` rendered `--spawn-cod=0` plus a separate manual Codex pane, which looked like Codex was dropped. | Medium | Added explanatory dry-run comment and defaulted manual Codex pane to supported `gpt-5.5` so the output is intentional. |
| Installed PATH `ao` was stale; Claude had `/home/boful/go/bin/ao` with no `agent` command and Codex had `/home/boful/.local/bin/ao` first in PATH. | High | Installed the branch-built `ao` to both paths with timestamped backups; both workers verified `ao session bootstrap --json` and `ao agent roster --json` now work on PATH. |
| The live NTM default Codex model was unsupported (`gpt-5.2-codex`). | High | Added `ao agent ntm-spawn --codex-model`, defaulting to `gpt-5.5`; live manual Codex pane reached READY as `JadeElk`. |

## E2E evidence

- `tests/background-agents/e2e.sh` passed in skip mode.
- `AGENTOPS_BACKGROUND_E2E=1 tests/background-agents/e2e.sh` passed.
- Live NTM session `agentops-bg` was created.
- Claude background agent reached READY:
  - runtime: `claude-code (Opus 4.8)`
  - mailbox: `JadeBeacon`
- Codex background agent reached READY:
  - runtime: `codex-cli (gpt-5.5)`
  - mailbox: `JadeElk`
- mcp-agent-mail reservation smoke succeeded:
  - worker: `JadeBeacon`
  - path: `docs/provenance/README.md`
  - reservation id: `27`
  - result: granted, no conflicts, released, no repo modification

## Remaining caveats

- The original NTM-spawned Codex pane remains in `agentops-bg` and shows the
  unsupported `gpt-5.2-codex` failure. A working manual Codex `gpt-5.5` pane is
  present too. The session was intentionally left running for operator follow-up.
- `bd dolt push` still fails because local and remote Dolt histories diverged
  with no common ancestor. Git branches are pushed; tracker force-push was not
  attempted.
- `ManagePullRequest` cannot create PRs for this nested AgentOps worktree from
  the current workspace (`No branch name available for PR creation`).

## Verdict

Ready for a supervised beta of the NTM background-agent flow.

The critical beta issues found by the live workers were fixed and retested. The
remaining items are operational/deployment follow-ups: merge the branch, keep
PATH `ao` current, and decide how to handle the stale failed Codex pane in the
live `agentops-bg` session.
