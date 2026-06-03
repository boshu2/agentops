# NTM background-agent loop post-mortem — 2026-06-03

## What we tested

The live `agentops-bg` NTM session was used as a supervised beta lane for real
AgentOps work:

- Claude worker `JadeBeacon` implemented `ag-skb2`.
- Codex worker `JadeElk` implemented `ag-fhbd`.
- Both workers used mcp-agent-mail for identity and coordination.
- Both workers worked from branch-specific worktrees.
- Both workers committed and pushed their branches.

## What worked

- NTM can keep Claude and Codex agents warm enough for real work.
- `mcp-agent-mail` can support identity, reservation, completion, and handoff.
- The `ao agent` surfaces are usable by humans and workers:
  - `roster`
  - `eligible`
  - `init-prompt`
  - `assign-prompt`
  - `ntm-spawn`
  - `ntm-status`
  - `ntm-stop`
- Background workers completed one code bead and one docs bead without editing
  the shared checkout or self-merging.
- The beta loop found real product bugs:
  - root `go run ./cli/cmd/ao ...` failed until `go.work` was added;
  - installed PATH `ao` was stale in two locations;
  - NTM's default Codex model was unsupported;
  - `ntm-spawn` needed clearer dry-run output;
  - roster bootstrap needed `--json`.

## What still hurts

- PR creation is not integrated: branches are pushed, but the dedicated PR tool
  cannot create PRs from nested AgentOps worktrees.
- Tracker health is still a real dependency risk:
  - `bd ready` / `bd list` can fail on duplicate issue/wisp ids;
  - `bd dolt push` diverges with no common ancestor.
- `ao status` does not yet show the operator what the background agents are
  doing; the operator still has to combine raw NTM and mcp-agent-mail state.
- Assignment is still semi-manual: `ao agent assign-prompt` renders a good
  message, but does not yet send it or reserve files itself.
- Live cleanup is manual: `ntm-stop` renders/executes a safe stop command, but
  there is not yet a closeout policy for stale failed panes.

## Follow-up beads filed

| Bead | Purpose |
|---|---|
| `ag-v4gu` | Integrate NTM background-agent state into `ao status`. |
| `ag-vhjb` | Send assignments through mcp-agent-mail instead of only rendering prompts. |
| `ag-9zls` | Fix nested-repo PR creation path for pushed background-agent branches. |
| `ag-w65c` | Repair duplicate issue/wisp and Dolt divergence blocking reliable background-agent queue/closeout. |

## Next loop recommendation

Drive `ag-v4gu` first. Operator visibility is the highest-leverage integration:
once `ao status` shows active NTM sessions, mailboxes, current assignments,
branches, reservations, and stuck panes, the rest of the background-agent system
becomes observable enough to run larger waves safely.

Then drive `ag-vhjb`: a visible dashboard plus a first-class assignment command
turns the current working primitives into a product workflow.
