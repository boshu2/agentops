# Decision Aids — Scoring, Proof Cards, Pathologies, Tiers, Red Flags

Operator classification and scoring tables moved out of the SKILL.md kernel. Use these when
the [Orchestrator Decision Tree](../SKILL.md#orchestrator-decision-tree) needs a finer-grained
classifier or a go/no-go score for an intervention.

## Intervention Score Matrix

When several actions are possible, score them before acting:

```
Score = (Evidence x Impact x Reversibility) / BlastRadius

Evidence       1-5: independent sources agree this state is real
Impact         1-5: likely to move code/beads/artifacts, not just produce prose
Reversibility  1-5: can undo/stop/retry without losing work
BlastRadius    1-5: one pane/message is low; all panes/session kill is high
```

Only take Score >= 2.0. If every action scores below 2.0, wait on the attention feed or gather better evidence.

| Candidate action | Evidence | Impact | Reversible | Blast | Decision |
|---|---:|---:|---:|---:|---|
| Terse prompt to one idle pane with no output and no commits | 4 | 3 | 5 | 1 | yes |
| Smart-restart one pane with identical tail across 3 ticks | 5 | 4 | 4 | 2 | yes |
| Broadcast "status?" to all panes | 1 | 1 | 3 | 4 | no |
| Kill a pane because it says "Cogitated 35m" | 1 | 3 | 1 | 4 | no |
| Stop swarm after convergence triple-check + queue-dry | 5 | 5 | 4 | 1 | yes |

## Operator Proof Card

Before any intervention stronger than a read-only check, fill this mentally or in the session log:

```markdown
## Intervention: <one-line action>
- Evidence: <robot attention/events>; <pane tail>; <git/br/mail/pipeline fact>
- Card matched: OC-___ / AP-___ / queue-dry / convergence / other
- Target: <session>; panes=<N or type>; user pane included? <yes/no>
- Expected state change: <commit/bead/artifact/tail/status>
- Reversibility: <wait/interrupt/smart-restart/checkpoint/restore/cancel>
- Verification command: <exact command>
- Escalation if unchanged: <next rung, not a jump to nuclear>
```

No proof card -> no nudge, restart, force-release, or shutdown.

## Swarm Pathology Triggers

| Pathology | Smell | First detector | First move |
|---|---|---|---|
| Prose treadmill | many "I'll investigate" lines, no commits/beads | `git log --since=1h`, `br list` | ship-or-surface prompt to that pane |
| False convergence | "LGTM"/"ready" everywhere, open work unchanged | convergence triple-check | stop if true, targeted artifact demand if false |
| Stale robot state | activity says old/stale but tail/git moving | attention + tmux capture | trust newer/direct source, resync cursor |
| Rate-limit mirage | pane text mentions old reset time | `--robot-health-oauth`, quota status | ping-probe before rotation |
| Coordination drag | mail/register retries consume ticks | stale/unavailable mail or reservation source | degraded fallback, record it, continue |
| Reservation deadlock | broad lock blocks ready work | `ntm locks list`, coordinator conflicts | narrow/renew/release with evidence |
| Review bead flood | agents file issues but close none | `br` plus BV status mix | switch to close-the-backlog mode |
| Context cliff | context >85%, repeated summaries | snapshot/context | handoff-then-restart |
| Queue-dry ambiguity | `br ready` empty but stale in-progress exists | `ntm work queue-dry` | decide stand-down vs ideate, do not invent silently |
| Operator overreach | repeated global broadcasts/restarts | tick log | pause, classify one pane at a time |

## Pattern Tiers

### Tier 1: Low-Risk Tending

| Pattern | Use when | Proof |
|---|---|---|
| Attention-feed wait | no urgent issue | cursor/event advances |
| Terse single-pane nudge | one pane genuinely idle | tail changes or blocker appears |
| Work graph refresh | ready/in-progress unclear | `br`/`bv`/queue-dry agree |
| Mail/lock digest | coordination seems stale | inbox/conflict list read |

### Tier 2: Directed Recovery

| Pattern | Use when | Guard |
|---|---|---|
| Account rotation | confirmed rate limit | provider/quota state, not pane text alone |
| Smart restart | pane not productively working | refuses active work |
| Interrupt + replace task | pane on wrong task | tail proves wrong task |
| Context handoff | context hot | handoff prompt and resume target ready |
| Reservation mediation | conflict blocks ready work | holder inactive or pattern too broad |

### Tier 3: Session Policy Changes

| Pattern | Use when | Guard |
|---|---|---|
| Review-only mode | code churn risky or user asked for audit | no implementer claims |
| Close-the-backlog mode | review beads exceed shipped fixes | one bead per pane, no new findings unless critical |
| Queue-dry stand-down | no ready/claimable work | `ntm work queue-dry` confirms |
| Pipeline handoff | repeated phase work | dry-run/status/cancel path known |
| Shutdown/convergence | triple-check passes | record final state; stop nudging |

## Red-Flag Phrases In Pane Tail (Scan → Match → Apply)

During a tick, skim each pane's last ~20 lines for these exact-or-near substrings. Match → apply the card. This is the fastest classifier when you don't have time to read every pane.

| If tail contains this phrase | State | Card to apply |
| --- | --- | --- |
| "resets 3pm", "You've hit your limit", "Upgrade to Max" | (maybe-stale) rate-limit | OC-001 Ping-Probe; then OC-002 Rotate if confirmed |
| "Stop and wait / Switch to extra usage" | `rate-limit-options` dialog | Autonomous Unstick → send `2` Enter |
| "[Pasted text]" alone | codex paste buffer | Autonomous Unstick → flush Enter |
| "Ready for validation", "MISSION ACCOMPLISHED", "Awaiting review", "Successfully identified and optimized" | **Handoff failure** (not success!) | OC-036 + Ship-Don't-Hand-Off prompt |
| "exemplary", "already complete", "no fixes needed", "ready to ship", "LGTM", "tests are passing" | Convergence language | OC-016 three-condition check; if all hold, STOP |
| "compile error", "build failed", "blocked by unrelated" | Build drift | PROMPTS.md → Build-Drift Repair |
| "bead DB was locked", "database is locked" | Bead DB contention | RECOVERY.md → `--no-db` bypass + OC-031 |
| "server unavailable" / "register_agent timed out" / "Resource temporarily unavailable" | Mail down/degraded | OC-007 fallback to bead-assignee lock |
| "FILE_RESERVATION_CONFLICT" | Over-broad reservation | OC-008 force-release |
| "Already covered, no bead filed" | Skill rotation dup | OC-041 session-scoped skill de-dup |
| "zsh: command not found" / "zsh: no matches found" | Prompt landed at bare zsh | OC-026 pid audit → OC-027 two-step relaunch |
| "Summarize recent commits", "Explain this codebase" (alone, no `• Working`) | Codex idle placeholder (NOT stuck) | Do not nudge; check dispatch history first |
| "waiting for file lock on registry" | Cargo registry contention | OC-031 cross-session zombie sweep |
| "Remove lock file?", "Force-push?", "Delete …?" (dialog) | Destructive-action dialog | OC-040 auto-decline (send "No") |
| `⏵⏵ bypass` bar at bottom of cc pane + long "Cogitated" | Actually executing | Do NOT nudge; cross-check with git log |

When in doubt, apply the Liveness Truth Stack (SKILL.md) before any state-changing action.

## Recovery Shortcuts

| State | First move | Escalation |
| --- | --- | --- |
| stale cursor | `ntm --robot-snapshot` | resume from new cursor |
| stale rate-limit text | ping probe + `--robot-health-oauth` | rotate account if confirmed |
| identical tail >=3 ticks | smart-restart dry run | hard-kill or respawn pane |
| paste buffer / dialog | `Escape Escape Escape C-u`, then targeted send | respawn if submit does not land |
| mail or reservation down | mark bead owner/blocker directly | backfill Agent Mail when fresh |
| prose without commits | ship-or-surface prompt | close-the-backlog or stand down |
| destructive dialog | decline by default | require explicit justification |

Full recipes live in [RECOVERY.md](RECOVERY.md), [OPERATOR-CARDS.md](OPERATOR-CARDS.md), and [ANTI-PATTERNS.md](ANTI-PATTERNS.md).

## Troubleshooting

| Problem | What to do |
|---|---|
| `spawn` cannot resolve the project | Use `ntm quick`, check `ntm config get projects_base`, or make the repo discoverable from that base |
| No clear next work item | Run `bv --robot-triage`, `bv --robot-next`, or `ntm work triage` |
| Coordination feels chaotic | Check Agent Mail inboxes, lock state, and `ntm coordinator digest/conflicts` |
| Agents appear idle | Use `ntm --robot-is-working=myproject` and `ntm --robot-agent-health=myproject` — they are the authoritative live signals |
| Pane stuck identical ≥3 ticks | `ntm --robot-health-restart-stuck` → `ntm --robot-smart-restart --hard-kill` → `ntm --robot-restart-pane` |
| Pane showing "resets Xpm" rate-limit | Probe with `tmux send-keys ping Enter` + `ntm --robot-tail`. If still limited: `ntm rotate myproject --all-limited` or `ntm --robot-switch-account=claude:<account>` |
| cc pane on `rate-limit-options` dialog | `tmux send-keys -t session:0.N "2" Enter` to pick "Switch to extra usage"; or rotate the account |
| codex pane on `[Pasted text]` limbo | `tmux send-keys -t session:0.N "" Enter` to flush the paste buffer |
| `ntm send` aborts with `Continue anyway?` | Pass `--no-cass-check`, or use `ntm --robot-send` (non-interactive) |
| Agent Mail server down/degraded/DB-busy | Proceed without it (see repo AGENTS.md); use `br update --assignee=...` as a soft coordination lock |
| `ntm activity` / `ntm health` show epoch / "56 years stale" | Use `--robot-is-working` / `--robot-agent-health` / `--robot-diagnose` instead |
| Cursor expired | Re-run `ntm --robot-snapshot` |
| Saturated-context cc (4+ days old, circular planning) | `ntm --robot-restart-pane --panes=N --restart-bead=br-xxx` on a fresh account |
| Beads look inconsistent | Use normal `br`/`bv` recovery commands for the repo; do not mutate `.beads` internals from habit |
| Duplicate-work collisions | Enable coordinator auto-assign; compute avoid-list from `br list --status=in_progress,claimed` at each dispatch |
| Swarm converges ("no fixes needed" × 2 rounds, zero new commits) | Stop. The backlog is exhausted; don't nudge further. |
