# Vibing With NTM Prompt Bank

<!-- TOC: Generic Start-of-Session | NTM-Repo Marching Orders | Code Review | Move-to-Next-Bead | Post-Compaction | Self-Review | Cross-Review | Random Exploration | Commit-Only | Graceful Shutdown | Ship-or-Surface | Close the Backlog | Terse Steady-State | Post-Compaction Resume | Domain Assignment | Orchestrator Diagnosis | Review-Only (P1/P2/P3/P4 + variants) | Mode-Switch | Autonomous Unstick -->

## Contents

- Start, resume, domain, and marching-order prompts
- Ship-or-surface, close-backlog, queue-dry, and build-drift prompts
- Review-only and mode-switch prompts
- Autonomous unstick operator prompts

Use these prompts as building blocks. Adjust them to the repo's actual `AGENTS.md`
instead of sending them blindly.

## Generic Start-of-Session Prompt

```text
Before doing anything else, read all of AGENTS.md and README.md and understand both. Then inspect the codebase enough to understand the project purpose, architecture, and the workflows that matter for this repo.

Register with MCP Agent Mail if the repo expects it, introduce yourself to the other agents, and check for any existing coordination threads. If the repo uses Beads and BV, use them to find ready work and pick the next task you can usefully advance now.

Do not get stuck in communication purgatory. Announce what you are taking on, reserve the relevant files or worktree scope, start doing real work, keep your bead status current, and reply promptly to important agent mail.

If the repo AGENTS.md has special rules for builds, tests, lints, or remote execution helpers such as rch, follow those rules exactly.
```

## NTM-Repo-Specific Marching Orders

This is closer to the marching order style used on the NTM repo itself:

```text
First read all of AGENTS.md and README.md carefully and understand both. Then investigate the codebase enough to understand the project architecture and the concrete workflow surfaces that matter for the current task.

Register with MCP Agent Mail, introduce yourself to the other agents, check your inbox, and respond promptly to any coordination messages. Use Beads and BV to decide what to work on next, mark your work clearly, and keep progress visible in both bead state and mail.

Do not drift into communication purgatory. Take one useful task, announce it, reserve the relevant scope, and start executing. When you are unsure what to do next, use BV or the repo's preferred triage surface and immediately start on the best ready task.
```

## Code Review Agent Prompt

```text
Explore the codebase deeply enough to understand the relevant execution flows, then do a methodical fresh-eyes review to find obvious bugs, regressions, race conditions, reliability issues, security problems, or sloppy assumptions. Fix the real problems you find and verify them according to the repo instructions.
```

## Move-to-Next-Bead Prompt

```text
Reread AGENTS.md so the repo rules are fresh. Use the available work graph tools to find the highest-impact ready task you can usefully advance now, claim it clearly, coordinate with the other agents, and start coding immediately instead of waiting for more instructions.
```

## Post-Compaction Prompt

```text
Reread AGENTS.md and re-establish the repo-specific rules before you do anything else. Then re-check your current bead, inbox, and the current work graph so you resume from real state instead of memory.
```

## Self-Review Prompt

```text
Read over all of the code you just wrote and the existing code you modified with fresh eyes. Look for obvious bugs, regressions, unsafe assumptions, confusing logic, missing tests, and sloppy edge cases. Fix anything you find before you move on.
```

## Cross-Review Prompt

```text
Turn your attention to code written by the other agents and review it critically for bugs, regressions, reliability problems, security issues, and poor assumptions. Diagnose root causes, then fix what actually needs fixing.
```

## Random Exploration Prompt

```text
Randomly explore unfamiliar parts of the codebase, trace the real execution flow, understand how those pieces fit into the larger workflow, and then do a fresh-eyes pass for obvious bugs and bad assumptions. Fix what you can justify.
```

## Commit-Only Prompt

```text
Based on your current understanding of the repo, group the existing changes into logical commits with detailed messages. Do not edit code. Exclude obviously ephemeral files. Follow the repo's branch and commit rules exactly.
```

## Graceful Shutdown Prompt

```text
Finish the smallest coherent piece of work you are currently in the middle of, checkpoint your status, update the bead and coordination thread, and then stop cleanly.
```

## Ship-or-Surface (Stop Prose, Commit Code)

Use when an agent has been writing prose / mental models / subsystem walkthroughs without shipping a commit for ≥1 hour.

```text
STOP. You have been writing prose, mental models, or subsystem walkthroughs. That is not productive work. New hard rule:

1. Pick ONE open/claimed bead you can complete in under 60 minutes (prefer fuzz targets, metamorphic tests, bug fixes — single-file, single-session scope).
2. Claim it: `br update <id> --status=in_progress`. Reserve the files you will edit (Agent Mail reservation or, if AM is down, the bead assignee serves as a soft lock).
3. Edit the actual code file(s). No prose. No mental models. No summaries until a commit lands.
4. Verify locally (compile, tests, lint as the repo requires).
5. `git add` → `git commit` with a real message → close the bead.
6. Repeat.

If you cannot ship a commit within 60 minutes, surface a specific blocker (not "need more context") and mark the bead blocked with the concrete reason. Write no other prose.
```

## Close the Backlog

Use when `open + claimed + in_progress` bead count exceeds 100, or whenever the swarm is filing more review beads than it closes.

```text
Backlog hygiene round. Do NOT file any new review beads until the total (open + claimed + in_progress) drops below 100.

1. Run `bv --robot-triage | jq '.recommendations[:8]'` and pick one you can close fully.
2. Close it: write the code, verify, commit, `br close <id>`.
3. Repeat until you close at least three beads this round, or one hour elapses.

If a bead is genuinely blocked, update it with the specific blocker and move on — don't spawn a new review bead for it.
```

## Queue-Dry Operator Prompt

Use when ready work appears exhausted. This is for the **operator**, not worker panes.

```text
Queue-dry check. Do not invent work yet.

1. Run `br ready --json` and `bv --robot-triage | jq '.quick_ref'`.
2. Run `ntm work queue-dry --format=json` and inspect `queue_dry`, `evidence`, `recommendations`, and `warnings`.
3. If `queue_dry=false`, assign the ready work and stop this flow.
4. If `queue_dry=true` and warnings are clean, either stand down or preview `ntm work queue-dry --ideate --format=markdown`.
5. Only create beads after reviewing the preview guard: `ntm work queue-dry --ideate --create-beads --yes --plan-version=<review-token>`.
6. After creation, run `br dep cycles --json`, `bv --robot-triage | jq '.quick_ref'`, and `br sync --flush-only`.

Report exactly which branch fired: ready-work, stand-down, preview-only, or beads-created.
```

## Terse Steady-State Nudge (specific-terse, not generic-terse)

Use for rapid ticks once agents are up and warm. Always include a specific verb and a specific target; never send bare "Next review." or "Keep going."

```text
Next bead: <specific bead id or 2-sentence problem>. Claim, code, verify, commit, close. Report back only with the commit SHA or a concrete blocker.
```

## Post-Compaction / Context-Reset Resume Prompt

Use after a cc pane auto-compacts or a fresh cc pane has just been spawned into an existing session.

```text
You just compacted or were freshly spawned into an active swarm. Re-read AGENTS.md and your pane's domain assignment (see below). Then:

1. `bv --robot-triage | jq '.quick_ref'` — see the work state.
2. `br list --status=in_progress,claimed --assignee=$(whoami) --json` — find any bead that was mid-flight under your identity.
3. If you have an in-flight bead, resume it. Otherwise claim the top-ranked ready bead inside your domain.
4. Do NOT re-introduce yourself in Agent Mail if you already did so earlier in this session; check your inbox tail first.

Your domain: <crate/directory scope>.
```

## Domain Assignment (session start)

Use at initial spawn for any swarm with ≥3 agents on a multi-crate / multi-domain workspace.

```text
You are pane <N> (<cc|cod|gmi>). Your crate/directory domain is <specific scope>. Do not edit outside this domain without reserving the files first AND announcing the cross-domain work in your next commit message.

Ready-work search is scoped to issues labeled or pathed inside your domain. If none are ready, surface that fact and stop — don't pick up work outside your domain silently.

Other panes' domains: <list>.
```

## Orchestrator Diagnosis Prompt (for the operator/babysitter loop, not a worker)

Use at the top of every orchestrator tick. This replaces "bash-grep pane buffers and hope."

```text
Tick <N>. Before nudging anything, capture real state in this order:

1. ntm --robot-is-working=<session> | jq '.panes'
2. ntm --robot-health-oauth=<session>
3. ntm --robot-health-restart-stuck=<session> --stuck-threshold=10m --dry-run
4. ntm coordinator status <session>
5. bv --robot-triage | jq '.quick_ref'
6. git -C <repo> log --since="1 hour ago" --oneline | wc -l
7. br list --status=in_progress,claimed --json | jq '.issues | length'

Decide which of these actions to take (pick at most two per tick, skip the rest):
- Restart stuck panes (smart-restart → restart-pane if wedged)
- Rotate rate-limited accounts (`ntm rotate --all-limited`)
- Dispatch close-backlog prompt (if total ready+claimed+in_progress > 100)
- Dispatch ship-or-surface prompt (if any pane has been producing prose with no commit for >1h)
- Dispatch terse next-bead nudge to an idle pane
- Do nothing (if convergence language x2 and git log shows no commits → stop)

Log your decision and one-line reason. Never broadcast the same generic nudge to all panes.
```

## Review-Only Mode Prompts

For the full mode definition (spawn, dispatch cadence, kill-relaunch cycle, mixed-swarm ratios, quality rubric), load REVIEW-MODE.md from the skill reference index. These are the canonical prompts. Agent-agnostic — works with cc, cod, gmi.

### P1 — Study The Project (once per round, to every reviewer pane)

```text
First read ALL of the AGENTS.md file and README.md file super carefully and understand ALL of both. Then use your code investigation agent mode to fully understand the code, technical architecture, and purpose of this project.

You are in REVIEW-ONLY MODE:
- Do NOT register with MCP Agent Mail.
- Do NOT claim beads from bv or br.
- DO explore deeply, find bugs, fix them, verify with the repo's tests/linters.
- DO read git log / git diff to understand recent activity by other agents.
```

### P2 — Explore And Fresh-Eyes Review (3× per round)

```text
I want you to sort of randomly explore the code files in this project, choosing code files to deeply investigate and understand and trace their functionality and execution flows through the related code files which they import or which they are imported by.

Once you understand the purpose of the code in the larger context of the workflows, I want you to do a super careful, methodical, and critical check with "fresh eyes" to find any obvious bugs, problems, errors, issues, silly mistakes, etc. — and then systematically, meticulously, and intelligently correct them.

Be sure to comply with ALL rules in AGENTS.md and ensure that any code you write or revise conforms to the best-practice guides referenced in AGENTS.md.

Tag each finding by severity: [CRITICAL], [HIGH], [MEDIUM], or [LOW].
Name specific files, line numbers, and root causes — not vague narration.
Verify every fix by running the repo's tests/linters.
```

### P3 — Cross-Review Other Agents' Code (3× per round, always after a P2)

```text
Reread AGENTS.md so it is still fresh in your mind.

Now turn your attention to reviewing the code written by your fellow agents and checking for any issues, bugs, errors, problems, inefficiencies, security problems, reliability issues, etc. Carefully diagnose their underlying root causes using first-principle analysis and then fix or revise them if necessary.

Use `git log --since="2 hours ago"` and `git diff HEAD~10..HEAD` to find recent changes. Don't restrict yourself to the latest commits — cast a wider net and go super deep.

Tag findings by severity. Name specific files and line numbers.
```

**Important:** Always include "Reread AGENTS.md so it is still fresh in your mind" as the first line of P3. Without it, reviewers drift from repo-specific rules during cross-review and can introduce conformance violations.

### P4 — Continuation Nudge (between iterations)

```text
Continue your code review. Explore more code files, trace execution flows, look for bugs or issues in parts of the codebase you haven't touched yet. Be thorough and methodical. Tag findings by severity.
```

Short — saves context. The pane already has codebase understanding.

### P-verify — Post-Fix Verification (after each P3)

```text
Now verify your fixes: run the project's test suite, linter, and formatter. Report whether everything passes. If anything fails, diagnose and fix before moving on. Confirm no regressions were introduced.
```

### P-summary — Request Structured Findings Report

```text
Write a REVIEW_SUMMARY.md file (or append to the existing one) documenting all the bugs you found, what you fixed, and what you verified. Include file paths, line numbers, root cause, and severity tag for each issue. Also include the commit SHAs for fixes you landed this round.
```

### P-redirect — Switch Project Mid-Session

```text
Stop what you're doing. First read ALL of the /dp/<NEW_PROJECT>/AGENTS.md and /dp/<NEW_PROJECT>/README.md carefully. Then review the code in <NEW_PROJECT> using the same REVIEW-ONLY MODE rules as before. P2/P3 prompts now apply to the new project.
```

### P-shutdown — Graceful End Of Review Session

```text
Finish the smallest coherent piece of work you are currently in the middle of, write (or update) REVIEW_SUMMARY.md with all findings from this session, and then stop cleanly.
```

### Sequence Templates

| Template | Chain | Use when |
| --- | --- | --- |
| Standard round | P1 → P2 → P3 → P4 → P2 → P3 → P4 → P2 → P3 | Default; works for most codebases |
| Deep dive | P1 → P2 → P2 → P3 → P4 → P2 → P2 → P3 → P-verify | Large codebases where one explore is insufficient |
| Quick audit | P1 → P2 → P3 | Small codebases (<5K lines); single pass |
| Post-refactor | P1 → P3 (with commit range) → P-verify | Focused audit on recent commits |

### Mode-Switch Prompt (hot-swap a pane between implement and review)

```text
MODE SWITCH: You are now a REVIEWER (was implementer). Stop claiming beads. Do not register Agent Mail. Begin with P1 (study), then P2 (explore), then P3 (cross-review). Sequence reference: /vibing-with-ntm REVIEW-MODE.md.
```

```text
MODE SWITCH: You are now an IMPLEMENTER (was reviewer). Register with Agent Mail. Pick the top ready bead from bv --robot-triage. Claim it, reserve files, ship a commit within 60 min or surface a blocker. Sequence reference: /vibing-with-ntm SKILL.md Next-Bead Prompt.
```

## Depth-Gate Prompt (Before Accepting "Clean")

Use on any reviewer / auditor pane that declares a domain "clean — rotate me" in under a few minutes. Forces evidence before the claim is accepted. See OC-034.

```text
Before declaring <domain> clean, produce the evidence:

1. Counts: run `grep -rEn 'unwrap\(\)|todo!|unimplemented!|panic!' <scope> | wc -l` and separately for each of unwrap/todo/unimplemented/panic. Quote the 3 hottest files by match count.

2. Read-through: open the top-3 hottest files end-to-end, then paste 3 specific function signatures with a one-sentence note each on what the function does and what its risk surface is.

3. Test run: execute the repo's verify command on <scope> (e.g. `cargo test --lib --package <name>` / `go test ./...` / `bun test <dir>`). Paste the last 20 lines of output.

Only after all three → I'll accept "clean" and rotate you to the next domain. Short-circuiting any step means rotation is denied.
```

## Session-Summary Prompt

Use at natural convergence, at handoff, or before shutdown. Produces a durable handoff artifact (OC-035).

```text
Produce a ONE-PARAGRAPH session summary (≤8 lines) including:
  - Bead IDs you closed this session (comma-separated list)
  - Commit SHAs you authored (short hashes, comma-separated)
  - One concrete thing still open and who/what should pick it up next
  - Any gotcha the next pane on this scope needs to know (broken build, flaky test, stale lock, etc.)

Then stop. Do not start new work.
```

## Ship-Don't-Hand-Off Prompt

Use when a pane's tail shows handoff-failure language ("Ready for validation", "MISSION ACCOMPLISHED", "Awaiting review") without a matching `br close` / `git push`. See OC-036 and AP-43.

```text
Don't hand off. Validate your own work NOW:

1. Run the repo's verify command (tests + lints + build) yourself.
2. If anything fails: fix it, re-run, repeat until clean.
3. `git add -A; git commit -m "<message>"; git push`.
4. `br close <id> --reason "Verified and shipped in <commit_sha>"`.

If you cannot close within 30 min, write a 3-line concrete blocker note on the bead (specific file, specific error, specific question) and mark it blocked. Do not park waiting for someone else to validate.
```

## Rotate-Into-New-Phase Prompt

Use when a pane declares its domain "fully blocked by dependencies" or runs out of ready work. Keeps the pane productive via phase rotation (OC-039).

```text
Your primary domain is blocked. Do NOT idle. Rotate into your next phase now — pick the next one you haven't done this session:

  A. Cross-review: `git log --since="2 hours ago" --oneline` → pick 3 commits outside your domain and review each deeply. File HIGH/CRITICAL review beads for real defects only.
  B. Fresh-eyes pass on your own domain's recent commits (HEAD~20..HEAD). Different lens from your implementer pass.
  C. Apply one of these cross-cutting skills to your domain scope: /testing-conformance-harnesses, /testing-fuzzing, /mock-code-finder, /reality-check-for-project. Pick one you haven't used recently.
  D. Write the next missing fuzz target or metamorphic test for your domain.

State which phase you chose and why. File at least one review bead or commit before declaring "phase done."
```

## Stale-In-Progress Self-Audit Prompt

Use periodically (once a session, or when in-progress counts drift high) to close beads whose work already shipped.

```text
For every bead currently assigned to you with status=in_progress, run:
  git log --all --grep='<bead_id>' --oneline

If a commit references that bead → `br close <bead_id> --reason 'Shipped in <commit_sha>'`.
If no commit references it → verify the work is genuinely in-flight; if not, `br update <bead_id> --status=open --reason 'Not actually shipped; releasing'`.

Report the close-count and reopen-count. Do this before claiming anything new.
```

## Build-Drift Repair Prompt

Use when another pane's tail contains build-fail language ("compile error", "build failed", "blocked by unrelated"). See AP-4 / broken-build-drift.

```text
Your primary work is paused. One of the other panes is blocked on a broken baseline. Route yourself to repair it:

1. Fetch the latest: `git pull --rebase`.
2. Run the repo's build command at HEAD. Capture the first error verbatim.
3. Fix it with a minimal patch (no speculative refactoring).
4. Verify the fix with the full verify command.
5. Commit with message starting "fix(build): <one-line summary>" and push.

When green, resume your previous work. If you can't fix it in 45 min, file a P0 bead with the exact error and escalate.
```

## Autonomous Unstick Playbook (for the orchestrator)

```text
A pane is stuck or rate-limited. Before escalating to the human:

1. Probe if rate-limited: `tmux send-keys -t <session>:0.N "ping" Enter`; wait 5s; tail pane. If it pongs, the limit already lifted — dispatch work.
2. If still rate-limited: `ntm rotate <session> --pane=N --account=<next healthy>` OR `ntm --robot-switch-account=<provider>:<account>`.
3. If a cc pane is on `/rate-limit-options`: `tmux send-keys -t <session>:0.N "2" Enter` to pick "Switch to extra usage".
4. If a codex pane is in `[Pasted text]` limbo: `tmux send-keys -t <session>:0.N "" Enter` to flush.
5. If tail is identical across 3+ ticks with zero output growth: `ntm --robot-smart-restart=<session> --panes=N --hard-kill --prompt="<dispatch>"`, then `ntm --robot-restart-pane --restart-prompt="..."` if smart-restart times out.
6. Before sending anything fresh into a codex pane: `tmux send-keys -t <session>:0.N Escape Escape Escape C-u` to clear leftover buffer garbage.

Only surface to the human if steps 1-6 all fail and you've tried on at least two account rotations.
```
