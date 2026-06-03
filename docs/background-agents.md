# Background Agents Operator Guide

AgentOps background agents are long-lived Claude and Codex sessions supervised
by NTM and coordinated through mcp-agent-mail. AgentOps does not ship a daemon
or scheduler for this path. It supplies the `ao` CLI, skill contracts,
bootstrap/profile prompts, validation gates, and provenance expectations; NTM
owns process placement and tmux pane lifecycle.

Use this guide when you want a ready pool of agents to take well-scoped beads
out of session while a human remains on the loop for assignment, review, merge,
and cleanup.

## Architecture

| Surface | Responsibility |
|---|---|
| NTM | Starts, supervises, inspects, and stops Claude/Codex tmux sessions. |
| mcp-agent-mail | Carries assignments, check-ins, file reservations, and handoff messages. |
| MCP / `ao mcp serve` | Exposes AgentOps tools and local resources when a worker needs an MCP surface. |
| `ao agent` | Renders runtime profiles, prompts, eligibility filters, and NTM spawn/stop/status commands. |
| Skills | Define the execution contract for the worker's actual task. |
| `bd` + git | Track the bead, worktree, commit, evidence, and PR boundary. |

The worker session is still a normal Claude or Codex skill session. It must run
`ao session bootstrap --json`, read the repo instructions, reserve files before
editing, use one worktree per bead, and avoid deprecated `ao rpi` / `ao evolve`
wrappers.

## Operator Commands

Run these from the repository root after installing the current `ao` on PATH.

```bash
ao agent bundle --runtime codex-ntm --json
ao agent bundle --runtime claude-ntm --json
```

Prints a runtime-specific profile for one worker type. Use this when a substrate
or external launcher needs one profile rather than the whole roster. Default
skills are `session-bootstrap`, `standards`, `validation`, and `provenance`; the
bundle refuses to inline holdout/eval content.

```bash
ao agent roster --json
```

Prints the default background roster: one `claude-ntm` profile and one
`codex-ntm` profile. The roster includes the mailbox name, default skills, the
`ao session bootstrap --json && ao inject --query "$BEAD"` bootstrap command,
and coordination expectations. It does not start sessions.

```bash
ao agent init-prompt --runtime codex-ntm --mailbox agentops-codex-ntm-worker
```

Prints the initialization prompt for a newly started background pane. Send it to
the worker before any bead assignment. The prompt tells the worker to bootstrap,
confirm its mcp-agent-mail identity, wait for assignment, reserve files, and use
skills instead of deprecated loop wrappers.

```bash
ao agent assign-prompt \
  --bead ag-demo \
  --branch cursor/ag-demo-background-docs \
  --files docs/background-agents.md,docs/documentation-index.md \
  --skills doc,validation,provenance \
  --validation 'scripts/pre-push-gate.sh --fast'
```

Prints the assignment message a lead should send through mcp-agent-mail. File
paths are repo-root relative. If a validation command references `./cmd/ao`, run
that command from `cli/`, for example `cd cli && go test ./cmd/ao -run Agent`.

```bash
ao agent eligible --eligible-only
ao agent eligible --file /tmp/ready-fixture.json --eligible-only
```

Filters ready beads for background-agent execution. A candidate must explicitly
opt in with a `background-agent-safe`, `background_eligible`, or
`managed_eligible` label, or equivalent `background_eligible` metadata. The
filter excludes work marked as holdout, evaluator, PII, human, or
operator-gated. Use `--file` to test fixture JSON before trusting a queue.

```bash
ao agent ntm-spawn agentops-bg --dir "$PWD" --claude 1 --codex 1
```

Renders a dry-run spawn plan by default. It does not create panes unless
`--execute` is passed. With the default non-empty `--codex-model gpt-5.5`,
AgentOps asks NTM to spawn Claude panes and prints a separate manual Codex
`tmux split-window` command so the Codex model can be set explicitly:

```text
ntm --robot-spawn=agentops-bg --spawn-cc=1 --spawn-cod=0 --spawn-dir=... --dry-run
# Codex panes use manual tmux split-window so --codex-model can override NTM's default Codex model.
tmux split-window -t agentops-bg: -c ... codex ... -m 'gpt-5.5' ...
```

That `--spawn-cod=0` is intentional in the manual-Codex path. To use NTM's
default Codex spawn instead, set `--codex-model` to an empty value and verify
the NTM default model is supported in the current environment.

```bash
ao agent ntm-status agentops-bg --json
ao agent ntm-stop agentops-bg
ao agent ntm-stop agentops-bg --execute
```

Use status to inspect an NTM session. Stop is also dry-run by default and prints
the safe `ntm kill <session> --force` command. Use `--execute` only when the
operator intends to clean up a live session.

## Assignment Workflow

1. Pick only eligible, well-scoped beads. Background agents should get bounded
   implementation, documentation, or validation work with clear acceptance.
2. Generate the worker prompt with `ao agent assign-prompt`.
3. Send the assignment through mcp-agent-mail. Include bead id, branch/worktree,
   file manifest, skills, and validation command.
4. Require the worker to confirm the thread before editing.
5. Require exclusive mcp-agent-mail reservations for every path or glob it will
   edit.
6. Require a dedicated worktree, usually via `bd worktree create` or an explicit
   `git worktree add` command supplied by the lead.
7. Let the worker claim the bead only after it is in the assigned worktree.
8. Require focused implementation, validation evidence, provenance/evidence
   paths, commit, and push.
9. Keep merge authority with the operator or lead. Background workers should not
   self-merge.

## Safety Defaults

- `ao agent roster`, `init-prompt`, `assign-prompt`, `eligible`, and `ntm-status`
  are read-only render/filter/status commands.
- `ao agent ntm-spawn` and `ao agent ntm-stop` are dry-run by default. The
  operator must pass `--execute` for live process changes.
- Workers must not edit the shared checkout. They work in one bead-specific
  worktree.
- Workers must reserve files before editing. Treat reservation conflicts as a
  coordination stop, not as a warning to ignore.
- Workers must not route background-agent work through `ao rpi` or `ao evolve`.
  Those are compatibility surfaces, not the execution path for this substrate.
- Holdout/eval content is never inlined into a background-agent profile.

## PATH `ao` Deployment Caveat

Background panes inherit their shell PATH, and multiple `ao` binaries can exist
on one machine. During beta, one pane resolved `/home/boful/go/bin/ao` while
another resolved `/home/boful/.local/bin/ao`; stale binaries lacked the new
`ao agent` command.

Before relying on a warm pane, verify the binary the pane will run:

```bash
hash -r 2>/dev/null || true
type -a ao
ao session bootstrap --json | head -20
ao agent roster --json | head -40
```

If `ao agent` is missing, update every higher-priority PATH copy or adjust PATH
so the intended binary wins. Re-run `hash -r` in existing shells after replacing
the binary.

## Live Beta Evidence

The supervised beta for this flow is recorded in
[NTM Background-Agent Beta Report](evidence/2026-06-03-ntm-background-agent-beta.md).
That run exercised root `go run ./cli/cmd/ao ...`, installed PATH `ao`, roster
rendering, init and assignment prompts, eligibility filtering, NTM dry-run
spawn, and `AGENTOPS_BACKGROUND_E2E=1 tests/background-agents/e2e.sh`.

The critical beta issues were fixed and retested:

- root `go run ./cli/cmd/ao ...` works via root `go.work`;
- roster bootstrap uses `ao session bootstrap --json`;
- `ntm-spawn` explains manual Codex panes and uses supported `gpt-5.5` by
  default;
- PATH `ao` was updated in both observed install locations;
- live Claude and Codex workers reached READY through mcp-agent-mail identities
  `JadeBeacon` and `JadeElk`.

The remaining caveats are operational: keep PATH `ao` current in every pane,
clean up failed stale panes when a live session is no longer needed, and let CI
or the operator's local gate validate the branch before merge.
