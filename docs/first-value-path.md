# AgentOps First-Value Path

This is the path after a README skim or install. It proves the **product** —
the operating loop from intent to validated code — without requiring the full
factory, a substrate, or a council demo.

**Front door: skills.** The CLI bookkeeps and gates; it is not the product entry.

Canonical map: [Intent → Validated Code](architecture/intent-to-validated-code.md) ·
[Skills Matrix](skills-matrix.md).

## First value

One behavior through the loop:

1. Install AgentOps skills on your coding-agent runtime.
2. Shape a small intent as **Given / When / Then** (`/plan` or `/discovery` → `/plan`).
3. Implement against a **failing acceptance test** (`/implement`).
4. Prove that behavior with the membrane (`/validate`) — verdict cites the scenario.
5. Only then optionally track follow-ups (`/beads-br`) or run a full tick (`/rpi`).

Without a behavior contract, `/validate` has nothing rigorous to accept against.
That is intentional: **no runnable acceptance, no honest "done."**

## Target viewer

An engineer who already uses Claude Code, Codex CLI, Cursor, or OpenCode and
wants agent work to end in **validated code**, not a chat claim.

They do not need NTM, Gas City, evolve autonomy, or multi-judge council on day one.

## Time budget

| Step | Budget | Success signal |
|------|-------:|----------------|
| Install skills (+ optional `ao`) | 2–5 min | Agent lists `/plan`, `/implement`, `/validate` (or `/rpi`) |
| Shape one behavior | 3–8 min | One Gherkin scenario (happy + edge) written on a bead or plan |
| Implement | 5–15 min | Acceptance test went RED for the right reason, then green |
| Membrane | 3–10 min | `/validate` PASS/HOLD that **names the scenario / acceptance evidence** |
| Optional bookkeeping | 2 min | Follow-up bead or `/postmortem --quick` |

## Commands and expected outcomes

### 1. Install

Use the matching installer from the [README](../README.md). Then open your agent
and confirm skills resolve (exact UX varies by runtime):

```text
/plan
/implement
/validate
```

Optional CLI:

```bash
ao doctor
ao --version
```

### 2. Shape intent as BDD

In the agent:

```text
/plan "<one small capability>"
```

Or, if the idea is fuzzy:

```text
/discovery --ideate
```

then `/plan` on the resulting packet.

**Required output (minimum):**

- Feature / capability name
- One happy-path Given/When/Then
- At least one edge or failure path
- Non-goals
- What evidence will prove done (test name or command)

Template: [docs/templates/intent-issue.md](templates/intent-issue.md).
Discipline: [behavior-first-planning](../skills/behavior-first-planning/SKILL.md).

### 3. Implement one slice

```text
/implement <issue-id>    # or the slice just planned
```

**Success:** first acceptance test fails for **missing behavior** (not syntax),
then passes after the smallest change; refactor does not edit the test contract.

### 4. Validate against that behavior

```text
/validate
```

**Success:** verdict is PASS/WARN/FAIL (or CONFIRMED/HOLD) and explicitly maps
to the scenario / acceptance commands. If there is no scenario and no runnable
acceptance, treat HOLD as correct — do not celebrate a vibe review.

Optional upgrades after first value (not required):

```text
/council validate …
/premortem          # next time, before implement
```

### 5. Optional: one-tick wrapper next time

```text
/rpi "<small goal>"
```

Same loop (Discovery → Crank → Validate → Learn) as one orchestrated tick.
See [operating loop](architecture/operating-loop.md).

### 6. Optional out-of-session lane (later)

Only after a human has seen a membrane verdict on a real behavior. Substrate
(NTM / MCP / managed-agents / Gas City) dispatches whole `/rpi` ticks — it does
not replace the loop. Details: [docs/3.0.md](3.0.md).

## First artifacts to inspect

| Artifact | Why it matters |
|----------|----------------|
| Plan / bead with `## Scenarios` (Gherkin) | The contract "done" will be checked against |
| Failing-then-passing acceptance test | ATDD proof the behavior exists |
| `/validate` verdict (e.g. under `.agents/`) | Membrane bound to that contract |
| `_beads/issues.jsonl` (if tracked) | Intent left chat and became engineering work |

## Friction list

| Friction | Handling |
|----------|----------|
| Agent wants to "just code" | Stop at `/plan` until Gherkin exists — no bead without acceptance |
| `/validate` without scenarios | HOLD; write scenarios; re-run — do not lower the bar |
| Mixed `/council` needs two runtimes | Skip council on first value; single-runtime `/validate` is enough |
| Substrate docs distract | Ignore until after first membrane verdict on a behavior |
| Want CLI-only path | `ao verify` is a commit/pre-push ratchet, not the product front door |

## Related docs

- [Intent → Validated Code](architecture/intent-to-validated-code.md)
- [Skills Matrix](skills-matrix.md)
- [SKILLS.md](SKILLS.md) (router)
- [Getting Started](getting-started/index.md)
- [Operating Loop](architecture/operating-loop.md)
- [3.0 north star](3.0.md)
