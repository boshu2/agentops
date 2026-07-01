# Driving Agents Reliably — the operator's field guide

> **Audience:** anyone pointing a coding agent (Claude Code, Codex, Cursor, Gemini, OpenCode) at real work and trying not to get burned.
> **Companion:** [behavioral-discipline.md](behavioral-discipline.md) is the *agent* side — what a good agent does. This page is the *operator* side — how you drive one so it behaves. Distilled from a large corpus of real, multi-hour agent-driving sessions; the recurring moves and corrections below are the ones that held up.

Coding agents are probabilistic. Give one a task and it will often do *a* version of it — not always the one you meant — and it will report the job done when it isn't. A better prompt doesn't fix that. Driving differently does. Three laws, a prompt pack, and a failure table.

## The three laws

**1. Set the destination, navigate the deviation.** Specify the *goal* and the done-condition, then correct the agent when it drifts — don't micro-script every step. A probabilistic worker won't reliably follow a scripted route, so the reliability has to come from the *environment* it runs in (the test that actually runs, the gate, the type checker), not from the agent's obedience.

**2. Nothing is done until something that didn't write it agrees — and it's delivered.** An agent's own "done" is the least reliable signal about its work. Treat done as: an *independent* check passed — a different model in fresh context, **or** a deterministic test — and the change is actually delivered (committed/merged/landed per the task's acceptance condition), not sitting unreviewed in a working tree. This is the AgentOps membrane: *no verdict = not done.*

**3. When a correction recurs, encode it as a check — not a reminder.** The second time you catch the same mistake, stop re-explaining it. Turn it into something mechanical: a gate, a test, a lint, a skill rule. A rule stated only in prose gets rationalized around; a rule that fails an exit code doesn't.

## The operator prompt pack (steal these)

Short, reusable prompts that do real work. They encode the laws above.

| When | Prompt | Why it works |
|---|---|---|
| Agent jumped straight to code | *"Do the research first. Load the relevant context, then act."* | Cold agents implement the wrong thing. Force context-loading before the first edit. |
| Handing off a big surface | *"Read all of X carefully. Don't write or propose anything yet — just study. When you're done, reply with exactly: STUDIED."* | Makes the agent finish reading before it can answer, and gives you a clean checkpoint. (The ack proves it followed the instruction, not that it understood — spot-check the first answer.) |
| You want a real review | *"Establish ground truth yourself; make no assumptions from this prompt."* | A reviewer that trusts your framing rubber-stamps it. Tell it to verify from the code. |
| Same-family build finished | *"Have a different model family check this — confirm it's correct, don't just take my model's word for it."* | Cross-family review catches what same-family self-review misses. |
| Keeping an autonomous run moving | *"Keep going toward the goal. Resolve forks yourself with a quorum of models; only escalate a genuine one-way door."* | Sets the agent to self-drive instead of stalling on every decision. |
| Closing a unit | *"Commit and push."* / *"Land it."* | A unit that isn't delivered isn't done (law 2). |
| After a long or expensive run | *"What did we actually get? What did we learn?"* | Forces an honest accounting of output vs effort, and captures the learning before it's lost. |
| Public/user-facing prose | *"Say it plainly and declaratively — no filler, no 'it's not X, it's Y'. Keep it short."* | Generation defaults to slop; name it explicitly. |

## The failure table — what to catch, and how

The recurring ways agents burn you, the operator move, and how AgentOps helps. The **How** column is honest about what is built-in, what is opt-in, and what stays a manual move.

| Agent failure | Your move | How AgentOps helps |
|---|---|---|
| **Fake-done** — "done" on work that's wrong or undelivered | Require an independent verdict + proof it landed | Built-in: the membrane / pre-land pawl (`no verdict = not done`) |
| **Going idle** — waits for a nudge instead of self-driving | "Keep going; don't wait for me" | Opt-in: autonomous loop + continuity substrate |
| **Spinning** — hours of motion, no progress | "You've been spinning — diagnose *why*, then fix it" | Manual move; supported by the post-mortem/`what did we learn` close |
| **Self-grading** — trusts its own or a peer's self-report | Route validation to a different context/family | Built-in: independent verification (cross-family *or* deterministic) |
| **Stale/poisoned context** — reaches for retired tools, re-solves solved | Call it out; start fresh | Supported: `ao session bootstrap`, fresh-context validation |
| **Lane collision** — edits shared scope without deconflicting | "Did you deconflict first, or just work?" | Opt-in: reservations / Agent Mail before hot-path writes |
| **Over-engineering** — builds the cathedral, ships nothing | "Ship the smallest real slice" | Discipline: vertical-slice + smallest-change standards |
| **Over-planning** — burns the session on plans, ships little | "Where do we actually stand?" | Discipline: behavior-first planning (no runnable acceptance test, no work) |
| **Decide-by-fiat** — skips the process on a real call | "Did you run it through discovery/pre-mortem?" | Skills: `/discovery` → `/pre-mortem` → `/council` for one-way doors |
| **Unbounded loops** — runaway workflows burning quota | Bound every loop up front | Built-in where used: circuit breakers (max-attempts / budget / oscillation) |
| **Slop** — generic AI prose passed off as human | "Plain and declarative" | Skill: `/de-slopify` on public surfaces |

## Starter commands

If you have the AgentOps CLI installed, these back the moves above:

```bash
ao session bootstrap                 # orient a fresh agent identically every time
ao lookup --query "<topic>"          # pull prior decisions/learnings before re-solving something
ao gate check --fast --scope head    # the deterministic release gate before you push
```

Tracking (beads) and the pre-land cross-family review are worth adding once a task outgrows a single session — see [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) and the [operating loop](architecture/operating-loop.md).

## The short version

Set the goal, not the route. Trust nothing the agent says about its own work — make something independent prove it, and prove it was delivered. And every time you catch the same mistake twice, turn the correction into a check. Steer, verify, ratchet.

---

*Provenance: distilled from a corpus of real agent-driving sessions and cross-checked against observed frequency, then reviewed cross-family before landing. See also [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) (what AgentOps is) and [behavioral-discipline.md](behavioral-discipline.md) (the agent-side companion).*
