# Anti-Patterns — Concrete, Named, With Fixes

<!-- TOC: Dispatch & Prompt (AP-1..6) | Observation & Monitoring (AP-7..12) | Pane Lifecycle (AP-13..18) | Coordination (AP-19..24) | Scale & Cadence (AP-25..28) | Dev-Loop (AP-29..33) | Meta/Orchestrator (AP-34..38) | Convergence Language Dictionary | How To Extend -->

## Contents

- Dispatch, prompt, observation, pane lifecycle, and coordination failures
- Scale/cadence, dev-loop, and orchestrator anti-patterns
- Convergence and handoff-failure language dictionaries
- Extension format for promoting repeated field failures into named entries

Every entry is a pattern observed in real swarm sessions. Organized by where in the orchestration flow they occur. Fix each by following the linked operator card or recipe.

---

## Dispatch & Prompt Anti-Patterns

### AP-1: `--all` Broadcast Without `--skip-first`

**Symptom.** User pane shows `zsh: command not found: <first-word-of-prompt>`.

**Root cause.** `ntm send --all` includes pane index 0 (or 1 depending on `pane-base-index`). Without `-s` / `--skip-first`, the prompt text hits the user's zsh shell.

**Fix.** Always pass `--skip-first`, or use `--robot-send` (excludes user pane automatically). See OC-013.

### AP-2: Dispatch Blocked By CASS Duplicate Check

**Symptom.** `ntm send` hangs at `Continue anyway? [y/N]` in an orchestrator loop. Operator never sees the prompt.

**Root cause.** `ntm send` has `--cass-check` on by default; it asks for confirmation if a similar prompt was used recently.

**Fix.**
- For orchestrator loops: always pass `--no-cass-check`.
- For structural safety: use `ntm --robot-send` (non-interactive by design).

### AP-3: Generic-Terse Nudges ("Next review.", "Keep going.")

**Symptom.** Ticks 1-3 produced real review beads; tick 4+ produces vague "looks fine" responses.

**Root cause.** Nudges shortened to lose all specificity. Agent has nothing concrete to act on.

**Fix.** OC-010: Specific verb + specific target + specific exit condition in every nudge. If you can't, stop nudging.

### AP-4: Templated Prompt With Stale Bead ID

**Symptom.** Agent spends a tick investigating "bd-3qoly" which closed 2 days ago.

**Root cause.** Reused a saved dispatch template without editing.

**Fix.** OC-017: Use palette edit-before-send, or substitute via `--var`.

### AP-5: "Reread AGENTS.md" In Every Nudge

**Symptom.** First ~30s of every tick is spent re-bootstrapping context; useful work starts late.

**Root cause.** Drag text left over from initial dispatch template.

**Fix.** Drop "Reread AGENTS.md" from steady-state nudges. Keep it only for initial dispatch and post-compaction resume.

### AP-6: Broadcast When Targeted Would Do

**Symptom.** Every pane receives the same prompt even though only 1-2 are relevant.

**Root cause.** Operator defaults to `--all` for convenience.

**Fix.** Dispatch by pane index or agent type (`--cc`, `--cod`, `--gmi`) when possible. Reserve `--all` for shutdown broadcasts or truly universal directives.

---

## Observation & Monitoring Anti-Patterns

### AP-7: Trusting Stale Rate-Limit Messages

**Symptom.** Operator treats pane as dead for 3 hours; wall actually lifted 2 hours ago.

**Root cause.** Pane buffer message ("resets 3pm") is stale the moment it renders; cc CLI doesn't re-check until user interacts.

**Fix.** OC-001: Ping-probe every 10 min. Use `--robot-health-oauth` for provider truth.

### AP-8: Bash-Grepping Pane Buffers Instead Of Robot Surfaces

**Symptom.** Operator's tick logic is `tmux capture-pane -p | grep "Working\|Idle"`. Flaky.

**Root cause.** Pane buffer is a display medium, not a state medium. Alt-screen, color codes, word wrap all break regex.

**Fix.** Use `ntm --robot-is-working`, `--robot-tail`, `--robot-agent-health`. Registry-backed, schema-stable.

### AP-9: `ntm activity` / `ntm health` With Stale Timestamps

**Symptom.** All timestamps read "56 years ago" or epoch.

**Root cause.** Legacy command path; does not refresh cached timestamp store.

**Fix.** Use `--robot-is-working`, `--robot-agent-health`, `--robot-diagnose` (live pane sampling + provider probe).

### AP-10: Missing `claimed` In Status Counts

**Symptom.** Orchestrator reports backlog = 20; actual backlog is 70.

**Root cause.** `br list --status=open,in_progress` skips the `claimed` state (beads a pane has locked but not started).

**Fix.** OC-005: Always `br list --status=open,claimed,in_progress`.

### AP-11: Single-Point State Observation

**Symptom.** Acted on `--robot-is-working` saying "idle", but pane was actually mid-compile.

**Root cause.** Only one observation; no cross-check.

**Fix.** OBSERVABILITY.md "Three-Observation Rule": need 3 independent signals (real-time + persistent + system) before state-changing action.

### AP-12: Convergence Language Accepted As Progress

**Symptom.** Every pane says "exemplary", "ready to ship", "no fixes needed"; orchestrator continues nudging for 2+ hours.

**Root cause.** Operator reading prose as work; git log says zero commits.

**Fix.** OC-016 Convergence Termination. If the three hard conditions hold, stop.

---

## Pane Lifecycle Anti-Patterns

### AP-13: Identical Tail ≥3 Ticks, Operator Keeps Nudging

**Symptom.** Tick 5 sees same 70 lines as tick 2, zero growth.

**Root cause.** Pane wedged on `/usage`, `/rate-limit-options`, paste buffer, or crashed. Nudges don't reach the CLI.

**Fix.** OC-003 Stuck-Pane Escalation Ladder. Don't skip rungs.

### AP-14: `ntm rotate` Timeout, Operator Gives Up

**Symptom.** `ntm rotate` runs for 5 min, returns timeout, operator escalates to user.

**Root cause.** Rotate uses CLI interactive prompts; wedged CLIs can't cooperate.

**Fix.** Skip `ntm rotate` when CLI is wedged; go straight to `ntm --robot-restart-pane` (uses `tmux respawn-pane -k` directly).

### AP-15: Single Ctrl-C Doesn't Exit CLI

**Symptom.** Operator sends `C-c`; CLI prints a new prompt (only canceled the current line).

**Root cause.** cc/cod/gmi CLIs require double C-c to exit.

**Fix.** OC-014: `tmux send-keys C-c; sleep 0.3; tmux send-keys C-c`.

### AP-16: Codex Buffer Corruption After Interrupt

**Symptom.** Next send into codex pane shows concatenation of old buffer + new prompt; prompt doesn't execute.

**Root cause.** Codex TUI retains partial buffer after C-c; new send prepends onto it.

**Fix.** **Always** `Escape Escape Escape C-u` before sending fresh prompts into any codex pane that had any prior interrupt. Not just "stuck" panes.

### AP-17: Saturated-Context Rehabilitation

**Symptom.** 4-day-old cc pane, circular planning, every nudge consumes more context.

**Root cause.** Operator trying to rescue a context-toast pane instead of replacing it.

**Fix.** OC-011: Replace, don't rehabilitate. Fresh pane on bead-scoped handoff > 4-day-old pane always.

### AP-18: Auto-Compact Eats Your Working Set

**Symptom.** Context hit 5%; auto-compact fired; next prompt returns vague "based on earlier context…".

**Root cause.** Compaction summarizer drops precise state details in favor of high-level narrative.

**Fix.** OC-009: Pre-empt at ≤85%. Write a bead-scoped handoff, then restart the pane.

---

## Coordination Anti-Patterns

### AP-19: Agent Mail Retry-Register Infinite Loop

**Symptom.** Pane retries `register_agent` for 4 hours; never produces code.

**Root cause.** Agent interprets mail server unavailability as a transient error to retry, not a signal to fall back.

**Fix.** OC-007: Mail-down → `br update --assignee` soft lock, proceed with work. Do NOT retry registration.

### AP-20: Too-Broad File Reservations

**Symptom.** One agent reserved `**/*.rs`; every other agent hits `FILE_RESERVATION_CONFLICT`.

**Root cause.** Agent used globstar pattern or leading-`/` in reservation.

**Fix.** OC-008: Detect and force-release; educate owner to use narrow patterns.

### AP-21: Bead Assignee Mail Ping-Pong

**Symptom.** "Please release bead br-xxx" thread; 4 hours of back-and-forth; no work landed.

**Root cause.** Operator used mail as an ownership transfer protocol; mail is for nuance, not control.

**Fix.** OC-015: Flip bead status + reassign. Don't wait for mail response.

### AP-22: Duplicate Work From Missing Domain Assignment

**Symptom.** Two agents claim overlapping beads; both edit the same files; merge hell.

**Root cause.** Spawn happened without explicit per-pane domain assignment.

**Fix.** OC-006: Assign crate/directory domains at spawn. Enable coordinator auto-assign.

### AP-23: Coordinator Digest False Negatives

**Symptom.** `ntm coordinator digest` says "no conflicts"; agents still report collisions.

**Root cause.** Known swallowed-error path in digest.go.

**Fix.** OC-019: Cross-check with `ntm coordinator conflicts` (separate code path). Trust conflicts.

### AP-24: Stale `build_slots` Leases

**Symptom.** Build operations silently blocked even with worktrees disabled.

**Root cause.** Switching between worktree/non-worktree mode leaves 1-hour leases active.

**Fix.** `ntm --robot-snapshot | jq '.build_slots.leases'` → force-release any stale lease or wait out TTL.

---

## Scale & Cadence Anti-Patterns

### AP-25: 7+ Project Orchestration By Single Operator

**Symptom.** Panes sit idle at prompt boxes for hours; orchestrator reports "tending 7 projects" but productivity is below 3-project baseline.

**Root cause.** Operator attention budget exceeded.

**Fix.** OC-020: Narrow to 3 projects × 6 panes. Use 2-tier orchestration for higher scale.

### AP-26: Sub-3-Minute Polling

**Symptom.** Ticks every 60s; orchestrator burns tokens on monitoring; real work interrupted.

**Root cause.** Default bias toward faster polling.

**Fix.** 4 min (just-dispatched), 10-17 min (steady), 30 min (deep-work) — see Steady-State Cadence.

### AP-27: Hand-Ticking For Hours

**Symptom.** Operator manually runs each tick; eventually misses ticks; drifts.

**Root cause.** No cron/schedule automation.

**Fix.** OC-022: Use `CronCreate`, `/loop` if available, or shell cron once orchestrating ≥30 min.

### AP-28: Over-Nudging A Converged Swarm

**Symptom.** 2+ hours of nudging after backlog was clear; every pane says "already complete"; zero commits.

**Root cause.** Orchestrator doesn't have a termination condition.

**Fix.** OC-016 Convergence Termination: three hard conditions → stop.

---

## Dev-Loop Anti-Patterns

### AP-29: rch File-Sync Assumed Universal

**Symptom.** Remote tests fail with "file not found" even though local edits compile.

**Root cause.** `rch exec` only syncs paths in `transfer.extra_sync_dirs`.

**Fix.** OC-018: Check config; add missing paths or fall back to local build.

### AP-30: Missing Handoff Notes At Shutdown

**Symptom.** Fresh pane picks up bead; spends 30 min re-discovering state; asks clarifying questions.

**Root cause.** Previous pane stopped without writing a handoff note to the bead.

**Fix.** OC-024: Handoff note is a shutdown requirement, not a nicety.

### AP-31: "Done" Without Push

**Symptom.** Agent reports "finished br-xxx"; local commit exists but `git push` never ran; work stranded.

**Root cause.** Definition of done was fuzzy.

**Fix.** OC-025: Done = commits + closed beads + push + verified working tree clean.

### AP-32: Prose Purgatory / Ship-or-Surface Violation

**Symptom.** Pane actively working; zero commits in 2+ hours; output is subsystem walkthroughs and self-reviews.

**Root cause.** Marching orders didn't constrain the output medium.

**Fix.** OC-004 Ship-or-Surface: commit within 60 min OR surface a concrete blocker. No prose.

### AP-33: Review-Bead Inflation Without Closure

**Symptom.** Open bead count grows from 50 to 300 across a multi-day swarm.

**Root cause.** Agents file new review beads every round; never close existing ones.

**Fix.** Alternate close/review prompts weighted by backlog depth (PLAYBOOK.md). Block new review-bead creation when backlog >100.

---

## Meta / Orchestrator Anti-Patterns

### AP-34: Guessing Instead Of Querying The Registry

**Symptom.** Operator invokes `--robot-foobar` which doesn't exist; silent failure or error.

**Root cause.** Memory/training-data recall instead of live capability query.

**Fix.** Always `ntm --robot-capabilities | jq` before using an unfamiliar flag. See ROBOT-MODE.md.

### AP-35: Deprecated Flag Usage

**Symptom.** Commands silently misbehave (e.g., `--assign-beads` ignored).

**Root cause.** Old docs or training data using pre-rename flags.

**Fix.** See ROBOT-MODE.md deprecated → canonical table. Verify via
`ntm --robot-capabilities` or `ntm --robot-docs=commands`.

### AP-36: Cursor Held Across Long Waits

**Symptom.** `ntm --robot-events --since-cursor=<old>` returns `CURSOR_EXPIRED` after sleeping for >1h.

**Root cause.** Cursor is GC'd after ~1 hour.

**Fix.** Resync with `--robot-snapshot` whenever cursor age > 45 min (proactive) or on first `CURSOR_EXPIRED` (reactive).

### AP-37: Acting On Stale Source Data

**Symptom.** Reassigned a bead that was already closed 10 min ago; wasted the tick.

**Root cause.** Used snapshot's bead list without checking `sources` / `degraded_sources` for the bead source.

**Fix.** OC-012: Source-health check before every state-changing action.

### AP-38: Not Using Native Stuck-Detect

**Symptom.** Operator manually tracks per-pane tail-hash; misses cases.

**Root cause.** Reinventing `--robot-health-restart-stuck`.

**Fix.** `ntm --robot-health-restart-stuck=<session> --stuck-threshold=10m --dry-run` in every tick.

---

## Convergence Language Dictionary (for AP-12)

When multiple panes produce these phrases across 2+ consecutive ticks AND git log is empty, the swarm has converged. Stop.

- "exemplary"
- "already complete"
- "no fixes needed"
- "ready to ship"
- "no changes required"
- "the implementation is solid"
- "code is clean"
- "nothing to add"
- "looks good to me"
- "LGTM"
- "tests are passing"
- "all conditions met"

None of these are a substitute for a commit SHA. If the pane can't name a SHA from this tick, it did no new work.

### Handoff-Failure Language (distinct from convergence — see AP-42 below)

These phrases mean the agent *finished the code* but then *parked* instead of validating/closing. They look like success but require a specific re-dispatch (OC-036), not just convergence-termination.

- "Ready for validation"
- "Ready for review"
- "MISSION ACCOMPLISHED"
- "Successfully identified and optimized"
- "Awaiting review"
- "Awaiting validation"
- "Handing off to …"
- "All done, please verify"

Treat each of these as a dispatch trigger, not an off-switch.

---

## Late-Additions From Real Sessions (AP-39+)

Promoted from post-session notes after being observed ≥3 times across independent swarms. Same format: symptom → root cause → fix.

### AP-39: `--robot-restart-pane` Mistook For Auto-Relaunch

**Symptom.** Operator runs `--robot-restart-pane --restart-prompt="…"`; pane appears "restarted" but the agent never boots; the restart-prompt text shows as a zsh error (e.g. `zsh: no matches found: …`).

**Root cause.** `--robot-restart-pane` uses `tmux respawn-pane -k`, which drops the pane to bare zsh. `--restart-prompt` is handed to zsh, not to the not-yet-launched agent CLI.

**Fix.** OC-027: Two-step relaunch. After restart-pane, explicitly `tmux send-keys "cc" Enter` (or cod/gmi), wait ~10s, then `ntm --robot-send` the marching orders.

### AP-40: Hardcoded `:0` Window Index

**Symptom.** `tmux send-keys -t <session>:0.N "…"` silently fails; the keypress never lands; operator keeps retrying into a bit bucket.

**Root cause.** `~/.tmux.conf` sets `base-index 1` / `pane-base-index 1`. There is no window `:0`.

**Fix.** OC-028: `WIN=$(tmux list-windows -t <session> -F '#{window_index}' | head -1)` at the top of every direct-tmux code path. Then address `:${WIN}.<pane>`.

### AP-41: `--robot-tail` As Ground Truth After An Action

**Symptom.** Operator just sent `"2" Enter` to dismiss a `/rate-limit-options` dialog. `--robot-tail` shows the dialog still on screen. Operator re-sends; nothing changes.

**Root cause.** `--robot-tail` can sample cached / stale buffer content for several ticks on transient states (dialog transitions, keypress echoes).

**Fix.** After any state-changing keypress, verify with `tmux capture-pane -t <session>:<win>.<pane> -p -S -20` instead of `--robot-tail`. `--robot-tail` is for bulk surveys; `capture-pane` is for single-pane ground truth.

### AP-42: Timer Labels Trusted As Activity

**Symptom.** Orchestrator sees "Cogitated for 35m" / "Worked for 1h 39m" and either (a) redirects a productive pane mid-work, or (b) leaves a silently-stuck pane alone thinking it's busy.

**Root cause.** The timer labels are display artifacts; they don't always advance in sync with actual activity (they can freeze during passive waits, or persist across productive ticks).

**Fix.** Cross-reference with `git log --since='15 minutes ago' --oneline | wc -l` and `pgrep -af 'cargo|rustc|go|bun'` before believing the timer. See OBSERVABILITY.md "Liveness Signals That Can Lie".

### AP-43: "Ready For Validation" Misread As Success

**Symptom.** Pane emits "MISSION ACCOMPLISHED", "Ready for validation via rch exec …", or "Successfully identified and optimized" — operator treats bead as done; hours later, nothing is closed or pushed.

**Root cause.** The agent finished code in-memory but parked itself waiting for external validation. Convergence language, not completion.

**Fix.** OC-036: Detect handoff-language (see dictionary above) as its own state; dispatch the self-close nudge ("validate your own work now: commit, push, close bead"). Done = OC-025.

### AP-44: Single-Enter For Multi-Line Codex Paste

**Symptom.** `ntm --robot-send --type=codex` returns `"success": true`; pane tail shows the prompt sitting in the input buffer; agent never starts thinking.

**Root cause.** Codex treats pastes as multi-line edits; one Enter inserts a newline, a second actually submits. Sometimes three are needed for long prompts.

**Fix.** OC-037: Bake a trailing multi-Enter loop (2-3 Enters, 2s apart) into every codex dispatch, then verify via `capture-pane` that a working/thinking indicator appeared.

### AP-45: Auto-"1" / Auto-Accept On Destructive Dialogs

**Symptom.** Pane proposes "Remove lock file?" / "Force-push?" / "Delete stale dir?" and the orchestrator loop's "accept default" path says yes; work or infrastructure disappears.

**Root cause.** Orchestrator assumed all dialogs are benign confirm-to-continue prompts.

**Fix.** OC-040: Regex-detect destructive keywords in the dialog text; default to "No" (option "3" or equivalent); re-prompt the agent to justify and propose a non-destructive alternative. Respect AGENTS.md RULE 1.

### AP-46: Parallel Heavy-Skill Broadcast To ALL Agents

**Symptom.** `ntm --robot-send --all --msg="apply /library-updater"` kicks off 7 parallel `cargo update` runs; lockfile thrashing; crates.io rate-limits; zero net commits.

**Root cause.** Heavy skills that touch shared state (workspace Cargo.lock, package-lock.json, global caches) don't parallelize safely.

**Fix.** Stagger or scope: dispatch `/library-updater` to one pane at a time, or scope it to specific sub-crates per pane. Same rule for `/security-audit-for-saas` across adjacent modules (see AP-48).

### AP-47: Reflexive `/clear` On Productive High-Context Pane

**Symptom.** Orchestrator sees context >400k, dispatches `/clear` reflexively; pane was mid-bead, loses the working set, reopens same bead an hour later from scratch.

**Root cause.** Threshold-only triggering without checking if the pane is actively producing.

**Fix.** Gate `/clear` on both "context high" AND "pane is between beads" signals. Look for the "What do you want to do next?" prompt OR a recent `br close` before dispatching `/clear`. High-context panes still landing commits should be left alone until they naturally hand off.

### AP-48: Cluster-Bury From Parallel Security Audits

**Symptom.** Two panes run `/security-audit-for-saas` on adjacent modules simultaneously; swarm backlog jumps by 15+ HIGH-severity beads in one tick; implementer pool can't keep up; everything else stalls.

**Root cause.** Audit-family skills produce clustered findings — a single systemic weakness often surfaces as 5-10 beads. Running them in parallel multiplies the burst.

**Fix.** Serialize audit-family skills across a swarm (one at a time), OR bound the burst: "file at most N beads per tick; queue the rest." See also PLAYBOOK.md close/review ratio for draining spikes.

### AP-49: Cross-Session Zombie Builds Hold Registry Locks

**Symptom.** Fresh swarm's builds stall on "waiting for file lock on registry"; no process in your own session obviously holds it; restarting your panes doesn't clear it.

**Root cause.** Parasitic `cargo` / `br` / `rsync` processes from closed terminals or dead worktrees kept running and hold cross-session file locks. Can outlive their session by hours or days.

**Fix.** OC-031: `pgrep -af 'cargo|br|rsync'`, check `/proc/<pid>/cwd`, kill any PID whose cwd is NOT in your live swarm's working dirs. Include D-state processes (`ps -eo pid,stat,etime,comm | awk '$2 ~ /D/'`) — they're uninterruptible but still block SQLite writes.

### AP-50: Absolute Disk-% Threshold Fires Too Late

**Symptom.** Disk hits 80%; fuzz corpus write fails mid-run; hours of fuzzing lost.

**Root cause.** Threshold-only alerting misses the runaway trajectory that preceded the cliff.

**Fix.** Track delta-per-tick and warn early. OBSERVABILITY.md "Disk trajectory beats absolute threshold": warn at ≥50% if delta >3pp/tick. Combine with OC-032 per-pane isolated `CARGO_TARGET_DIR` so targeted sweeps work.

### AP-51: `--no-cass-check` Flag Confusion

**Symptom.** `ntm --robot-send --no-cass-check …` fails with "unknown flag".

**Root cause.** `--no-cass-check` is valid on `ntm send` but NOT on `ntm --robot-send` — different parsers, different parser rules.

**Fix.** Remember the parser split:

| Use case | Correct form |
| --- | --- |
| Bypass CASS dupe check in orchestrator loop | `ntm send … --no-cass-check …` |
| Non-interactive dispatch (no CASS check by design) | `ntm --robot-send …` (no flag needed) |

`--robot-send` is already non-interactive, so the flag is redundant and rejected.

### AP-52: dcg Blocks Dispatch Because The Prompt TEXT Contains `rm -rf`

**Symptom.** `ntm --robot-send` fails because the prompt TEXT contains a destructive substring the operator is sending AS CONTENT to describe what an agent should do (e.g. "tell the agent to clean its incremental dir" containing literal `rm -rf`).

**Root cause.** dcg's PreToolUse filter matches on the full command string; it can't tell "executable text" from "message content."

**Fix.** Write dispatch strings in prose that avoids destructive literals: `"cargo clean -p <crate>"` / `"prune the incremental cache"` instead of `"rm -rf target/debug/incremental"`. Same rule for SQL DROP, git reset --hard, etc. in dispatch payloads.

### AP-53: `bv` / `br` JSON Schema Drift Across Projects

**Symptom.** A pipeline that works on project A breaks on project B with "null output / empty array"; running manually shows both projects have data.

**Root cause.** Older beads exports return `{issues: [...]}` at the top level; newer exports return `[...]` directly. Project A and B may be on different versions.

**Fix.** Write jq that handles both shapes:

```bash
br list --json | jq 'if type=="object" then .issues else . end | .[]'
bv --robot-triage | jq 'if has("recommendations") then .recommendations else .issues // . end'
```

### AP-54: Waiting For Background Terminal ≠ Stuck

**Symptom.** Pane shows "• Waiting for background terminal" for 60+ minutes; operator restarts it; kills in-flight `rustc` chaining that would have finished in another 10 min; loses 15-30 min of already-done compile work.

**Root cause.** Long rustc chains look identical to a hung shell from the outside.

**Fix.** Before restarting any "waiting for background terminal" pane, check `ps -fp $(tmux display -p -t <session>:<win>.<pane> '#{pane_pid}') -o pid,pcpu,etime,comm` AND `pgrep -af rustc` activity on the box. Rule: restart eligible only if >20 min wait AND no rustc PIDs have moved (verified by `ps ... --sort=+etime` snapshots across two observations).

### AP-55: Placeholder Suggestions Misread As Stuck

**Symptom.** Codex idle pane shows placeholder suggestions like `Summarize recent commits` / `Explain this codebase`; operator treats this as "waiting for prompt" and nudges, even when a real dispatch is already in flight on that pane.

**Root cause.** Those are codex's idle-state hints, not a waiting-for-input signal.

**Fix.** Authoritative signals: `• Working …` or `• Waiting for background terminal` prefix lines mean busy. Presence of placeholder suggestions alone means "idle, but not necessarily stuck or awaiting nudge — check dispatch history first."

### AP-56: Skill Rotation Without Session-Wide De-Dup

**Symptom.** Pane receives `/testing-metamorphic` in round 1, `/mock-code-finder` in round 2, `/testing-metamorphic` again in round 4; round-4 dispatch returns "Already covered, no bead filed" — wasted tick.

**Root cause.** Rotation logic de-duped per tick but not across the session's history.

**Fix.** OC-041: Maintain per-pane skill-history across the whole session; round-robin against the pool with recent-N exclusion. Reset only when the pool is exhausted.

### AP-57: zsh Silent Failure On Associative Arrays

**Symptom.** A dispatch loop with `declare -A NUDGES=(...)` completes without error, but no nudges actually land.

**Root cause.** The Bash tool runs zsh; zsh's `declare -A` behaves differently from bash's and can silently accept and ignore certain syntactic forms.

**Fix.** For orchestrator loops that use bash associative arrays, wrap the whole loop in `bash -c '…'` (or use plain parallel arrays / newline-delimited string parsing).

---

### AP-58: Queue-Dry Ideation Without The Guard

**Symptom.** `br ready` is empty, so the operator immediately runs an ideation/create flow and seeds duplicate or low-value beads.

**Root cause.** Treating "no ready work" as "invent work" instead of distinguishing dry queue, stale tracker state, blocked graph, and degraded coordination.

**Fix.** OC-043: run `br ready --json`, `bv --robot-triage`, then `ntm work queue-dry --format=json`. Create beads only after reviewing the non-mutating ideation guard.

### AP-59: Pressure-Blind Bulk Assignment

**Symptom.** Ten panes receive fresh work; five minutes later the machine is slower, RCH is queued, quota walls appear, and no commits land.

**Root cause.** Assignment fanout ignored resource pressure, build pressure, provider quota, and reservation contention.

**Fix.** OC-044: check `--robot-agent-health`, `--robot-rch-status`, `--robot-quota-status`, and lock state before broad dispatch. Reduce `--limit`, route only blocker-clearing work, or switch provider pools.

### AP-60: Remembered Pane Index As Target Truth

**Symptom.** A nudge lands in the wrong pane or the user shell after a restart/retile/base-index change.

**Root cause.** Operator remembered `session:0.N` from an earlier tick and reused it without verifying live tmux pane identity.

**Fix.** OC-045: query `tmux list-panes` and NTM robot identity immediately before raw `tmux send-keys`. Prefer NTM robot/serve paths that resolve by stable pane identity.

### AP-61: Integration Command Folklore

**Symptom.** Agents debug `dcg check`, stale robot flags, or an imagined NTM-driven RCH build path.

**Root cause.** Using memory of a helper's command shape instead of the current registry/tool contract.

**Fix.** OC-046: query `ntm --robot-capabilities`, use `ntm --robot-dcg-check --command=...`, and run heavy builds through `rch exec -- <cmd>` when repo rules require it.

---

## How To Extend This File

New anti-pattern format:

1. **Name** — short, searchable, unique (AP-NN)
2. **Symptom** — observable evidence (what you'd see in a log or snapshot)
3. **Root cause** — why it happens
4. **Fix** — pointer to operator card (OC-NN) or concrete recipe

If you observe a new pattern three times across different swarms, promote it from anecdote to entry here. If you observe it five+ times, also create an operator card (OC-NN) with a prompt module and a validator.
