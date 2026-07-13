---
name: handoff
spine: true
description: 'Write compact session handoffs. Triggers: "handoff", "write compact session handoffs.", "handoff skill".'
practices:
- adr
- wiki-knowledge-surface
- code-complete
hexagonal_role: supporting
consumes: []
produces:
- .agents/handoff/*.md
context_rel: []
skill_api_version: 1
context:
  window: inherit
  intent:
    mode: none
  intel_scope: none
metadata:
  graph_root: true
  tier: session
  dependencies: []
output_contract: .agents/handoff/<date>-<topic>.md plus <date>-<topic>-prompt.md
---
# Handoff — Durable Session Continuation

> **Loop position:** write-side adapter for `handoff → clear → rehydrate`.
> It captures the live lane as two checked Markdown artifacts before context is
> cleared or ownership changes.

**Execute this workflow. Do not only describe it.**

## Constraints

- Write both artifacts before clearing context, ending the session, or transferring ownership, because a partial handoff makes the next agent rediscover state.
- Ground every accomplishment, blocker, issue state, and next action in durable evidence such as paths, commit SHAs, verdicts, and tracker ids; do not promote conversational memory into fact because it drifts.
- Preserve pawl disposition separately from helper outcome. The disposition is one of `CONFIRMED`, `REFUTED`, `HOLD`, `ESCALATE`, or `REBOUND`; the helper outcome is only `UNSTUCK`, `ESCALATE`, or `not-run`. Plain `REFUTED` continues auto-redo; only a breaker enters `HOLD` and one helper pass. `CONFIRMED` alone authorizes the door; helper `UNSTUCK` resumes work but must re-earn `CONFIRMED`. Helper `ESCALATE` reaches a human; refusal-lane work, explicit judgment, and exhausted time/cost/quota budgets skip the helper and go directly to a human.
- Keep the continuation prompt as a pointer to the handoff document, not a second source of truth, because duplicated narrative diverges.

## Purpose and boundaries

Use this skill when a productive session is pausing, changing agents, nearing a
context reset, or explicitly needs a continuation packet. The handoff must let a
fresh session resume without reconstructing the lane from chat.

Do not use it as a post-mortem: handoff records **current state**; `post-mortem`
records reusable learning. Do not call an idle session successful: when there is
no durable activity, report `EMPTY` with the reason and write no fabricated
accomplishments.

## Inputs

Given `/handoff [topic]`:

- An explicit topic wins.
- Otherwise derive a 2–4 word lowercase hyphenated slug from the current issue,
  most recent commit, or ratchet state.
- If none is descriptive, use `session-$(date +%H%M)`.

## Execution workflow

### 1. Gather durable session evidence

Create the artifact directory and inspect the live repository and tracker:

```bash
mkdir -p .agents/handoff
git status --short
git log --oneline --since="2 hours ago" 2>/dev/null
git diff --stat HEAD~5 2>/dev/null | head -20
ao beads exec list --status in_progress 2>/dev/null | head -5
ao beads exec list --status closed 2>/dev/null | head -5
find .agents/research .agents/plans -type f -name '*.md' -print 2>/dev/null | tail -5
```

If an explicit multi-writer workflow is active, also record held reservations,
peer/comms topology, and the working-thread pointer. Otherwise say that no such
topology is active; do not invent one.

### 2. Pin the pause point

Record all four fields, even when their value is `none`:

1. Last completed action and its evidence.
2. Exact next action, preferably a command or file to inspect.
3. Open blocker, pawl disposition, helper outcome, and their evidence. Never
   store `UNSTUCK` as a disposition or treat it as authorization.
4. Dirty files, claimed issues, reservations, and external state the next
   session inherits.

**Checkpoint:** confirm the tracker id/status and current `git rev-parse HEAD`
against live commands before writing them. A stale identifier is worse than an
omitted one.

### 3. Write the handoff document

Fill the authoritative handoff in the **Artifact Templates** section below.
Cite paths and SHAs for completed work, distinguish observed state from
inference, and keep open questions separate from blockers.

The next action must be executable without rereading chat. List priority files
in read order and state why each matters.

### 4. Write the continuation prompt

Fill the continuation-prompt template below. It must point to the handoff
document, restate the objective and verified pause point in no more than a
short paragraph, and name the first command or file to inspect.

Do not copy the full handoff into the prompt. The document is authoritative.

### 5. Validate before reporting

```bash
doc=.agents/handoff/YYYY-MM-DD-<topic>.md
prompt=.agents/handoff/YYYY-MM-DD-<topic>-prompt.md
test -s "$doc" && test -s "$prompt"
for heading in '## Objective' '## Verified state' '## Where we paused' '## Next action' '## Files to read' '## Validation evidence'; do
  rg -Fqx "$heading" "$doc"
done
for marker in 'Read first:' 'First action:'; do rg -Fq "$marker" "$prompt"; done
rg -q '^\*\*Captured:\*\* [0-9]{4}-[0-9]{2}-[0-9]{2}T[^ ]+$' "$doc"
rg -q '^\*\*Repository:\*\* .+$' "$doc"
rg -q '^\*\*HEAD:\*\* [0-9a-f]{40}$' "$doc"
rg -q '^\*\*Tracker:\*\* .+$' "$doc"
rg -q '^\*\*Pawl disposition:\*\* (CONFIRMED|REFUTED|HOLD|ESCALATE|REBOUND|none)$' "$doc"
rg -q '^\*\*Helper outcome:\*\* (UNSTUCK|ESCALATE|not-run)$' "$doc"
```

**Checkpoint:** verify before session close that the handoff names the current
capture time, repository, HEAD, tracker state, dirty-worktree state, validation
evidence, pawl disposition, helper outcome, and a concrete next action. Repair
any missing field and rerun the commands.

The behavior contract is
[references/handoff.feature](references/handoff.feature).

### 6. Optional learning and runtime closeout

When the session produced a major decision or at least three meaningful
commits, suggest `post-mortem --quick`; do not run it in place of the handoff.
If `ms` is installed, grade only skills genuinely consulted with
`ms outcome <skill> --success|--failure`.

Runtime-specific closeout belongs to the runtime projection. Do not claim a
session was stopped merely because the Markdown artifacts exist.
When explicitly requested for a managed session, `ao session handoff --no-kill`
may additionally capture structured orchestrator state; it does not replace the
Markdown pair.

## Examples

**User says:** `/handoff validation-membrane` after a plain refutation.

**Result:** both artifacts record disposition `REFUTED`, helper outcome
`not-run`, the failing evidence, and the next auto-redo command.

## Troubleshooting

| Problem | Recovery |
| --- | --- |
| The handoff says `HOLD` after one failed check | Restore `REFUTED` and continue auto-redo; reserve `HOLD` for the configured breaker. |

## Artifact Templates

Authoritative handoff:

```markdown
# Handoff: <Topic>
**Captured:** <ISO-8601 timestamp>
**Repository:** <path>
**HEAD:** <full 40-character SHA>
**Tracker:** <issue id and live status, or none>
**Pawl disposition:** <CONFIRMED | REFUTED | HOLD | ESCALATE | REBOUND | none>
**Helper outcome:** <UNSTUCK | ESCALATE | not-run>

## Objective
<Current objective and acceptance boundary.>

## Verified state
- <Completed action> — evidence: <path, command result, SHA, or verdict>
- Worktree / external state: <exact inherited state or none>

## Where we paused
**Last action:** <verified action>
**Blocker / questions:** <evidence-bound blocker, disposition, helper outcome, or none>

## Next action
<One command or file inspection and its expected result.>

## Files to read
1. `<priority path>` — <why first>

## Validation evidence
- `<command>` → <exit/result>
```

Continuation prompt:

```markdown
# Continuation: <Topic>
Read first: `.agents/handoff/YYYY-MM-DD-<topic>.md` (authoritative state).
Objective and pause point: <short evidence-bound summary>.
First action: `<command or file to inspect>`
Verify HEAD/tracker, then preserve pawl disposition and helper outcome separately.
```

## Output Specification

- **Path:** write both artifacts under `.agents/handoff/` in the current repository.
- **Filename:** use `YYYY-MM-DD-<topic>.md` for the authoritative handoff and `YYYY-MM-DD-<topic>-prompt.md` for its continuation pointer.
- **Format:** serialize both as UTF-8 Markdown; the handoff uses the exact required headings in the template and the prompt names its referenced handoff path.
- **Validation command:** run the `test -s` and `rg -q` checkpoint commands above; every command must exit zero before reporting `DONE`.
- **Downstream handoff:** the next session reads the handoff first, verifies the recorded HEAD/tracker state, then executes the named first action; post-mortem and corpus tooling may consume the artifact later.

## Quality Checklist

- Evidence quality: accomplishments and state cite durable paths, issue ids, SHAs, or verdict artifacts rather than memory-only claims.
- Resume quality: the next action is executable, priority files are ordered, and inherited dirty/external state is explicit.
- Pawl quality: disposition includes `CONFIRMED`; helper outcome is separate; `REFUTED` auto-redoes, `UNSTUCK` re-enters work, and neither `HOLD` nor helper `ESCALATE` is collapsed into a pass.
- Artifact quality: both filenames match the same topic/date; capture/repository/HEAD/tracker and validation evidence are present; the continuation prompt points to the authoritative handoff.

## Report

Report both artifact paths, the captured pause point, and the first continuation
action. End with exactly one marker:

```text
<promise>DONE</promise>
```

For an idle session with no context to capture:

```text
<promise>EMPTY</promise>
Reason: No session activity found to hand off
```

## See Also

- `skills/post-mortem/SKILL.md` — extract reusable learning after state is safe.
- `skills/bootstrap/SKILL.md` — rehydrate a fresh session before resuming.
