# Self-Test — Trigger Phrases for /vibing-with-ntm

Validate that the skill's description reliably triggers on realistic operator phrasings. For each phrase below, the skill should be chosen over sibling skills (`/ntm`, `/agent-mail`, `/beads-br`, `/beads-bv`, `/account-rotation`).

## Should Trigger (operator context, tending verbs, recovery signals)

- "tend the swarm on asupersync"
- "babysit my NTM session for an hour"
- "run an orchestrator tick on myproject"
- "one of my cc panes has been stuck for 3 ticks, what do I do"
- "pane 4 is rate-limited — how do I rotate without losing its bead"
- "send marching orders to every pane in the session"
- "my codex panes look wedged, multi-line prompts aren't submitting"
- "how do I know when to stop the swarm / detect convergence"
- "flip my swarm into review-only mode"
- "dispatch the ship-or-surface prompt"
- "ready queue is empty, should I ideate new beads or stop"
- "before bulk-assigning the swarm, check if RCH or quota is saturated"
- "my prompt landed in the wrong pane after a restart"
- "dcg check doesn't exist anymore, what should the orchestrator use"
- "close the backlog — beads are ballooning past 200"
- "agents keep filing review beads but never closing them"
- "how do I safely restart a pane without losing in-flight work"
- "the orchestrator loop for tending cc/cod/gmi panes"
- "an agent keeps writing prose but never commits"
- "I ran --robot-restart-pane but the agent never came back"
- "disk is climbing 3% per tick, what's the right intervention"
- "tmux send-keys lands as a zsh command-not-found"

## Should NOT Trigger (adjacent but wrong skill)

| Phrase | Correct skill |
| --- | --- |
| "what does `ntm --robot-snapshot` return" | `/ntm` |
| "register a new MCP Agent Mail identity" | `/agent-mail` |
| "br ready is returning empty but I know work exists" | `/beads-br` or `/fixing-beads-problems` |
| "compute betweenness centrality on my bead graph" | `/beads-bv` |
| "add a new Claude Max account to CAAM" | `/account-rotation` |
| "which Gemini model does the review swarm use" | `/code-review-gemini-swarm-with-ntm` |
| "provision a fresh Ubuntu box for agents" | `/provision-new-machine` |

## Validator

After any edit to the skill's `description:` or decision tree, run these phrases through the skill-selection flow and confirm:

1. Every "should trigger" phrase selects `/vibing-with-ntm` (or /vibing-with-ntm alongside a sibling — both is fine if the sibling also fires).
2. Every "should NOT trigger" phrase selects the correct sibling first, with /vibing-with-ntm not in the top match.
3. Description still satisfies skill-authoring rules: third-person, ≤200 chars preferred (≤250 acceptable given scope), front-loaded triggers, explicit "Use when" clause.

## Reference Cards To Link When Matching

When /vibing-with-ntm triggers, the response almost always should point at one of these first:

| Operator phrasing → | Card |
| --- | --- |
| "tend / babysit / orchestrator tick" | Decision Tree at top of SKILL.md |
| "stuck / wedged / unresponsive pane" | OC-003 ladder + Liveness Truth Stack |
| "rate-limited" | OC-001 ping-probe + OC-002 rotate-by-pool |
| "restart-pane but agent never came back" | OC-027 two-step relaunch |
| "looks busy but no commits landing" | OC-004 Ship-or-Surface + AP-32 / AP-43 |
| "swarm done? time to stop?" | OC-016 convergence + scripts/convergence-check.sh |
| "queue dry / no ready work" | OC-043 Queue-Dry Guard + Queue-Dry Operator Prompt |
| "bulk assignment causing slowdown" | OC-044 Pressure-Aware Assignment + AP-59 |
| "wrong pane received prompt" | OC-045 Pane Identity + AP-60 |
| "stale helper command" | OC-046 Tool Contracts + AP-61 |
| "review mode / audit session" | REVIEW-MODE.md |
