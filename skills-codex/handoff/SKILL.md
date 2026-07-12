---
name: handoff
description: 'Write compact continuation handoffs. Triggers: "handoff", "capture this session for continuation", "prepare a continuation prompt".'
---
# Handoff — Durable Codex Session Continuation

> **Loop position:** write-side adapter for `handoff → clear → rehydrate`.
> It captures the live lane as two checked Markdown artifacts before context is
> cleared or ownership changes.

**Execute this workflow. Do not only describe it.**

## Constraints

- Write both artifacts before clearing context, ending the thread, or transferring ownership, because a partial handoff makes the next agent rediscover state.
- Ground every accomplishment, blocker, issue state, and next action in durable evidence such as paths, commit SHAs, verdicts, and tracker ids; do not promote conversational memory into fact because it drifts.
- Preserve pawl disposition separately from helper outcome. The disposition is one of `CONFIRMED`, `REFUTED`, `HOLD`, `ESCALATE`, or `REBOUND`; the helper outcome is only `UNSTUCK`, `ESCALATE`, or `not-run`. Plain `REFUTED` continues auto-redo; only a breaker enters `HOLD` and one helper pass. `CONFIRMED` alone authorizes the door; helper `UNSTUCK` resumes work but must re-earn `CONFIRMED`. Helper `ESCALATE` reaches a human; refusal-lane work, explicit judgment, and exhausted time/cost/quota budgets skip the helper and go directly to a human.
- Keep the continuation prompt as a pointer to the handoff document, not a second source of truth, because duplicated narrative diverges.

## Purpose and boundaries

Use `$handoff` when a productive Codex thread is pausing, changing agents,
nearing compaction/reset, or explicitly needs a continuation packet. The
handoff must let a fresh thread resume without reconstructing the lane from
conversation.

Do not use it as a post-mortem: handoff records **current state**;
`$post-mortem` records reusable learning. When there is no durable activity,
report `EMPTY` with the reason and do not fabricate accomplishments.

## Inputs

Given `$handoff [topic]`:

- An explicit topic wins.
- Otherwise derive a 2–4 word lowercase hyphenated slug from the current issue,
  most recent commit, or ratchet state.
- If none is descriptive, use `session-$(date +%H%M)`.

## Execution workflow

### 1. Gather durable session evidence

Use `ao beads exec` for tracker reads; in AgentOps it resolves the canonical
`br` / beads_rust store. Do not substitute another tracker.

```bash
mkdir -p .agents/handoff
git status --short
git log --oneline --since="2 hours ago" 2>/dev/null
git diff --stat HEAD~5 2>/dev/null | head -20
ao beads exec list --status in_progress 2>/dev/null | head -5
ao beads exec list --status closed 2>/dev/null | head -5
find .agents/research .agents/plans -type f -name '*.md' -print 2>/dev/null | tail -5
```

If an explicit multi-writer workflow is active, record held reservations,
peer/comms topology, and the working-thread pointer. Otherwise state that none
is active; do not infer orchestration from detected concurrency.

### 2. Pin the pause point

Record all four fields, even when their value is `none`:

1. Last completed action and its evidence.
2. Exact next action, preferably a command or file to inspect.
3. Open blocker, pawl disposition, helper outcome, and their evidence. Never
   store `UNSTUCK` as a disposition or treat it as authorization.
4. Dirty files, claimed issues, reservations, and external state inherited by
   the next thread.

**Checkpoint:** confirm the tracker id/status and current `git rev-parse HEAD`
against live commands before writing them.

### 3. Write both artifacts

Fill the authoritative handoff and short continuation pointer in the **Artifact
Templates** section below. Cite paths and SHAs, distinguish observation from
inference, and keep open questions separate from blockers.

The next action must be executable without rereading the prior conversation.
List priority files in read order and explain why each matters.

### 4. Validate before reporting

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

**Checkpoint:** verify before thread close that the handoff names current HEAD,
capture time, repository, HEAD, tracker state, dirty-worktree state, validation
evidence, pawl disposition, helper outcome, and a concrete next action. Repair
missing fields and rerun the commands.

### 5. Optional learning

When the thread produced a major decision or at least three meaningful commits,
suggest `$post-mortem --quick`; do not run it in place of handoff. If `ms` is
installed, grade only skills genuinely consulted with
`ms outcome <skill> --success|--failure`.

## Examples

**User says:** `$handoff validation-membrane` after a plain refutation.

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
- **Downstream handoff:** the next Codex thread reads the handoff first, verifies recorded HEAD/tracker state, then executes the named first action; later tooling may consume the artifact.

## Quality Checklist

- Evidence quality: accomplishments and state cite durable paths, issue ids, SHAs, or verdict artifacts rather than memory-only claims.
- Resume quality: the next action is executable, priority files are ordered, and inherited dirty/external state is explicit.
- Pawl quality: disposition includes `CONFIRMED`; helper outcome is separate; `REFUTED` auto-redoes, `UNSTUCK` re-enters work, and neither `HOLD` nor helper `ESCALATE` is collapsed into a pass.
- Artifact quality: both filenames match the same topic/date; capture/repository/HEAD/tracker and validation evidence are present; the continuation prompt points to the authoritative handoff.

## Codex Execution Profile

1. Capture the current objective, completed work, unresolved blockers, and the next command or file to inspect.
2. Prefer durable paths, issue ids, and validation evidence over conversational summaries.
3. Finish handoff-driven session closeout by running `ao codex ensure-stop --auto-extract`; the CLI already skips duplicate closeout for the same Codex thread.

Run closeout only after both Markdown artifacts pass validation. The command is
idempotent for the same thread; it does not make an incomplete handoff valid.

## Guardrails

1. Do not leave the next session guessing what to do first.
2. Do not start an orchestration substrate merely because one is installed.
3. Do not call the thread closed until artifact validation and Codex closeout
   have both completed or the closeout failure is recorded in the handoff.

## Report

Report both paths, the verified pause point, and the first action. End with:

```text
<promise>DONE</promise>
```

For an idle thread:

```text
<promise>EMPTY</promise>
Reason: No session activity found to hand off
```

## See Also

- `$post-mortem` — extract reusable learning after state is safe.
- `$bootstrap` — rehydrate a fresh thread before resuming.
