# Operator Cards — Operationalized Field Expertise

## Contents

- OC-001..OC-016: rate limits, stuck panes, source health, convergence, and core coordination
- OC-017..OC-032: dispatch hygiene, cron, handoff, restarts, retries, and build/process contention
- OC-033..OC-047: phase rotation, handoff failures, queue-dry, pressure-aware assignment, pane identity, integration contracts, and the spawn-then-send boot race

Each card codifies one lesson from real multi-agent swarm operation across many projects. Structure follows the `operationalizing-expertise` methodology: **trigger → failure mode → recipe → prompt module → validator**.

Cite these cards by ID in swarm post-mortems. Future-you will recognize patterns faster.

---

## OC-001 — Ping Before You Rotate

**Trigger.** A pane shows "You've hit your limit · resets 3pm". Orchestrator about to rotate or restart.

**Failure mode (observed).** Agents trust the message, wait hours, miss the fact that the wall lifted long ago.

**Recipe.**

```bash
tmux send-keys -t <session>:0.<pane> "ping" Enter
sleep 5
ntm --robot-tail=<session> --lines=10 --panes=<pane>
# If it pongs, the wall already cleared — dispatch real work
# If still silent, consult --robot-health-oauth before rotating
```

**Prompt module (to send if pane was alive):**

```text
Welcome back. Identify as agent '<name>' for this swarm. Run bv --robot-triage | jq '.recommendations[:3]', claim one, reserve the files, code it. No prose; ship a commit or surface a blocker within 60 min.
```

**Validator.**

```bash
# Succeeded if:
#   - Pane tail advanced after ping
#   - OR --robot-is-working.is_rate_limited = false
#   - AND pane committed to git within 1 hour
```

---

## OC-002 — Rotate By Pool, Not By Pane

**Trigger.** Multiple panes hit rate limits, but not all providers are out.

**Failure mode.** Operator rotates every pane mechanically; burns working accounts, hits the same wall again.

**Recipe.**

```bash
# Check pool health per provider
ntm --robot-health-oauth=<session> | jq '.panes | group_by(.provider) | map({provider: .[0].provider, any_rate_limited: any(.rate_limited)})'

# Only rotate panes whose pool is depleted
ntm rotate <session> --all-limited
# Or surgical, if you know which provider is out:
ntm --robot-switch-account=claude:<healthy_account>
```

**Decision matrix.**

| Provider | Pool | Agents drawing from it |
| --- | --- | --- |
| Anthropic | Claude Max subscription | `cc` |
| OpenAI | ChatGPT Pro subscription | `cod` |
| Google | Gemini Ultra | `gmi` |

**Prompt module (to send after rotation):**

```text
Your pane was rotated to a fresh account (~full quota). Resume from the bead you were on; its assignee/reservations still hold. If your last output was mid-commit, `git status` to reconcile before continuing.
```

**Validator.** `ntm --robot-accounts-list --provider=<p>` shows ≥1 healthy account remaining; swarm productivity recovers within 2 ticks.

---

## OC-003 — Three Ticks, Then Escalate

**Trigger.** Pane tail looks identical across three consecutive orchestrator ticks. Output length unchanged.

**Failure mode.** Operator pastes nudge #4, #5, #6 into a dead buffer while believing work is happening.

**Recipe (strict escalation ladder — do not skip rungs):**

```
Rung 1 (tick N):    Wake ping (tmux send-keys "" Enter)
Rung 2 (tick N+1):  C-u + re-send marching orders via --robot-send
Rung 3 (tick N+2):  --robot-smart-restart
Rung 4 (tick N+3):  --robot-smart-restart --hard-kill
Rung 5 (tick N+4):  --robot-restart-pane (tmux respawn-pane -k)
Rung 6 (tick N+5):  ntm add + ntm kill (replace the pane)
```

**Rule:** Never stay on the same rung for two ticks. Either it worked → new activity is visible, or climb.

**Validator.**

```bash
ntm --robot-health-restart-stuck=<session> --stuck-threshold=10m --dry-run | jq '.stuck_panes | length'
# Should decrease monotonically as you climb the ladder
```

---

## OC-004 — Prose Is Not Progress

**Trigger.** Pane is actively generating tokens (`is_working=true`) but `git log --since="1 hour ago"` attributed to that pane shows 0 new commits.

**Failure mode.** The "swarm is productive" illusion — panes narrate mental models, exemplary self-reviews, subsystem walkthroughs. Zero code lands.

**Recipe.**

```bash
# Immediately dispatch the Ship-or-Surface prompt (see PROMPTS.md)
ntm --robot-send=<session> --panes=<pane> --msg="$(cat prompts/ship_or_surface.txt)"

# If another tick passes with no commit, move to stuck-pane escalation
```

**Prompt module.** Ship-or-Surface (see PROMPTS.md for full text):

```text
STOP writing prose. Pick ONE open bead under 60 min. Claim, reserve, code the diff, run the repo's verify command, commit, close. Do not file new review beads until backlog < 100. If you cannot commit within 60 min, write a 3-line blocker note and move on.
```

**Validator.** At least one new commit attributed to this pane within 90 min of the prompt landing.

---

## OC-005 — Track `claimed`, Not Just `open + in_progress`

**Trigger.** Orchestrator reports backlog of N items but the bead database shows N+50 because `claimed` was missed.

**Failure mode.** Operator decides the swarm has "no work" and winds down, while 50 claimed beads sit orphaned on saturated panes.

**Recipe.**

```bash
# Always check all three active statuses
br list --status=open,claimed,in_progress --json | jq '.issues | length'

# Separate them for triage
br list --status=claimed --json | jq '.issues[] | {id, assignee, updated_at}'
```

**Prompt module (to free a stale claim):**

```text
Bead <id> is claimed by <assignee> but has not progressed in >2h. Flip to open and reassign:
  br update <id> --status=open
  ntm --robot-bead-claim=<id> --bead-assignee=<new_agent>
Do not mail-ping the old assignee — the pane may be days from responding.
```

**Validator.** `br list --status=claimed | jq '[.issues[] | select(.updated_at < (now - 7200))] | length'` → 0.

---

## OC-006 — Explicit Domain, Zero Collisions

**Trigger.** Spawning ≥3 agents in a multi-crate/multi-directory workspace.

**Failure mode.** Two agents claim overlapping beads; edits collide; both re-work because commits cross-wire.

**Recipe (at spawn, before any work):**

```bash
ntm spawn <session> --cc=N --cod=M --gmi=P

# Assign explicit domains — do this IN the marching orders, not as a note
ntm --robot-send=<session> --panes=1 --msg="Pane 1 cc: OWN crates/foo. Do not edit outside your domain without reserving first."
ntm --robot-send=<session> --panes=2 --msg="Pane 2 cod: OWN crates/bar and tests/bar/. ..."
# ... one dispatch per pane

# Enable coordinator auto-assign to catch any drift
ntm coordinator enable auto-assign
ntm coordinator enable digest --interval=15m
```

**Prompt module (domain assignment):**

```text
You are pane <N> (<cc|cod|gmi>). Your crate/directory domain is <scope>. Do not edit outside this domain without reserving the files AND announcing cross-domain work in your commit message. Ready-work search is scoped to issues labeled or pathed inside your domain. If none ready, surface and stop — don't silently pick up work outside your domain.
```

**Validator.** `ntm coordinator conflicts <session>` returns 0 file-overlap conflicts per tick.

---

## OC-007 — Agent Mail Down Is Not Agent Mail Blocked

**Trigger.** `send_message` / `file_reservation_paths` return "server unavailable",
`Resource temporarily unavailable`, SQLite/database-busy errors, or repeated timeouts.

**Failure mode.** Agents retry `register_agent` in a loop for hours. Zero code lands.

**Recipe.**

```bash
# Confirm real unavailability (not just your pane)
ntm --robot-snapshot | jq '{sources: .sources.sources, degraded_sources}'

# Fall back to bead-assignee as soft coordination lock
br update <bead_id> --status=in_progress --assignee=<agent_name>

# Coordinate via bead descriptions (free-text; everyone can read)
br update <bead_id> --description-append="<progress note>"

# Do NOT retry registration/reservation in a loop
```

**Prompt module (to send when mail is down):**

```text
Agent Mail server is unavailable or degraded (mail/reservation source in `.sources.sources` is not fresh, or calls are returning DB-busy/timeouts). Do NOT retry registration or reservations in a loop. Coordinate via br update --assignee=<name> as soft lock, br description for progress notes. Proceed with work; mail will reconcile when service returns.
```

**Validator.** Commits continue to land (git log), even while mail is down. `ps -ef | grep mcp-agent-mail` is not required to be running for work to progress.

---

## OC-008 — Broad Reservations Are Sabotage

**Trigger.** `ntm --robot-snapshot | jq '.reservations[] | select(.paths[] | test("^\\*\\*|^/|^\\*$"))'` returns entries.

**Failure mode.** One agent reserves `**/*.rs` or `**`; the whole swarm blocks on every file.

**Recipe.**

```bash
# Detect
ntm locks list <session> --all-agents --json | jq '.reservations[] | select(.paths[] | test("^\\*\\*|^/|^\\*$"))'

# Force-release
ntm locks force-release <session> <lease_id> --note "too-broad pattern"

# Educate (one-shot prompt)
ntm --robot-send=<session> --panes=<owner_pane> --msg="Your reservation '<pattern>' was too broad and blocked others. Re-reserve specific paths (e.g. crates/foo/src/bar.rs). Patterns like **/*.ext or leading-/ are never acceptable."
```

**Validator.** No reservations match the too-broad pattern test.

---

## OC-009 — Pre-empt Compaction, Don't Survive It

**Trigger.** `ntm --robot-context=<session> | jq '.panes[] | select(.context_used_pct > 85)'` returns entries.

**Failure mode.** Auto-compact at ≤10% summarizes the crisp "what I just discovered" state into vague bullets; next prompt fetches nothing useful.

**Recipe.**

```bash
# 1. Tell the saturated pane to write a concrete handoff BEFORE compaction
ntm --robot-send=<session> --panes=<pane> --msg="Context at ${pct}%. Immediately: write a 5-line handoff note to bead <id> — current state, one open question, exact next step. Do not do any new work."

# 2. Wait for the handoff to land
ntm --robot-wait=<session> --wait-until=idle --panes=<pane> --timeout=3m

# 3. Restart the pane with fresh context on that bead
ntm --robot-restart-pane=<session> --panes=<pane> --restart-bead=<id>
```

**Validator.** New pane's first commit continues the thread set in the handoff note.

---

## OC-010 — Specific-Terse, Not Generic-Terse

**Trigger.** You've been sending short nudges ("Next review.", "Keep going.", "Continue.") for several ticks.

**Failure mode.** Prompt degradation. Downstream work becomes proportionally shallow; reviews go from finding bugs to rubber-stamping.

**Recipe.** Every nudge must include:

1. **Specific verb** — "Claim and ship bead br-xxx" not "keep working"
2. **Specific target** — the bead ID, file path, or test name
3. **Specific exit condition** — the commit SHA or blocker note

If you cannot name a specific target in one sentence, **stop nudging** — the swarm is done or needs a different scope.

**Prompt module.**

```text
Next bead: <id or 2-sentence problem>. Claim. Reserve <file-pattern>. Code the diff. Run <repo's verify command>. Commit. Close. Report back only with the commit SHA or a concrete blocker.
```

**Validator.** Commits per hour per pane; target ≥1 for active panes.

---

## OC-011 — Saturated Context Replacement Over Rehabilitation

**Trigger.** A cc pane has been running ≥4 days. Work is circular; suggestions recycle earlier ideas.

**Failure mode.** Operator keeps "helping" the saturated pane with more hints. Every hint consumes more of its remaining context.

**Recipe.**

```bash
# Check age
tmux list-panes -t <session> -F '#{pane_index} #{pane_start_time}'

# If pane age > 4 days OR context_used_pct > 90:
ntm --robot-restart-pane=<session> --panes=<pane> --restart-bead=<id>

# Preserve in-flight work by naming the bead the pane was on
```

**Validator.** Fresh pane produces a useful commit on the assigned bead within 30 min.

---

## OC-012 — Source-Health Before Acting

**Trigger.** About to make a state-changing action based on `--robot-snapshot` data.

**Failure mode.** Acting on stale `beads` or `mail` data → wrong bead claimed, wrong assignment, wasted cycle.

**Recipe.**

```bash
# Before any state-changing action, check source freshness
SNAP=$(ntm --robot-snapshot)
echo "$SNAP" | jq '.sources.sources | to_entries[] | {source: .key, fresh: .value.fresh, degraded: .value.degraded, age_ms: .value.age_ms, reason: (.value.degraded_reason // .value.reason_code)}'

# If source is stale (beyond its tier's threshold):
#   beads: re-run `br list --json` locally
#   mail: re-run `ntm --robot-mail-check`
#   quota: re-run `ntm --robot-health-oauth=<session>`
# Only then act.
```

**Validator.** Every major dispatch cycle logs a freshness line; stale sources re-checked before acting on them.

---

## OC-013 — --skip-first Is Not Optional

**Trigger.** Broadcasting a prompt via `ntm send --all`.

**Failure mode.** The prompt lands in your own user pane (pane 0 or 1 depending on tmux pane-base-index) as a zsh command, producing `zsh: command not found: <first-word-of-prompt>`. Worse, the prompt doesn't land on actual agents either if wrapping/quoting collides.

**Recipe.**

```bash
# ALWAYS use --skip-first (or -s) when broadcasting
ntm send <session> --all --skip-first --no-cass-check "<prompt>"

# Or target explicit agent pane list
ntm send <session> --panes=2,3,4,5 "<prompt>"

# Or use --robot-send which excludes the user pane by default
ntm --robot-send=<session> --msg="<prompt>"   # no --all flag → excludes user pane automatically
```

**Validator.** Last line of user pane is not `zsh: command not found: ...`.

---

## OC-014 — Double Ctrl-C For Wedged CLIs

**Trigger.** A cc or gmi CLI is wedged; `C-c` once just clears the current line.

**Failure mode.** Operator pastes more prompts into a pane where the CLI is broken.

**Recipe.**

```bash
# Double Ctrl-C within ~1s exits the wedged CLI
tmux send-keys -t <session>:0.<pane> C-c
sleep 0.3
tmux send-keys -t <session>:0.<pane> C-c

# Then relaunch via the repo alias (preserves --dangerously-bypass flags)
tmux send-keys -t <session>:0.<pane> "cc" Enter        # or "cod" / "gmi"

# Re-send marching orders
ntm --robot-send=<session> --panes=<pane> --msg="$(cat marching_orders.txt)"
```

**Validator.** `ps -ef | grep -E "(claude|codex|gemini)" | grep <pane_pid>` shows a fresh process.

---

## OC-015 — Bead Status Flip Beats Mail Ping-Pong

**Trigger.** You need to reassign a bead currently claimed by a saturated pane.

**Failure mode.** Sending "please release bead br-xxx" via mail waits hours for a response from a pane that may be down.

**Recipe.**

```bash
# Don't wait. Flip the status.
br update <bead_id> --status=open

# Reassign via coordinator
ntm assign <session> --auto --strategy=dependency

# Or direct claim
ntm --robot-bead-claim=<bead_id> --bead-assignee=<new_agent>
```

**Rule.** Mail is for coordination nuance; bead status is for ownership. Don't conflate them.

**Validator.** `br list --status=in_progress --json | jq '.issues[] | select(.id=="<bead_id>") | .assignee'` shows the new agent within 2 min.

---

## OC-016 — Convergence Termination Is Mandatory

**Trigger.** After a tending run, the orchestrator keeps looping even though commits have stopped.

**Failure mode.** Infinite nudging. Every tick produces more "exemplary" prose. Tokens wasted, user annoyed.

**Recipe.** Hard stop when ALL three conditions hold:

1. `git log --since="1 hour ago" --author=<swarm-account>` = 0
2. ≥2 consecutive ticks: every pane's tail contains convergence language (see list in ANTI-PATTERNS.md)
3. `br ready --json` = 0 AND `br list --status=in_progress,claimed` unchanged between ticks

```bash
CONV=0
for tick in $(seq 1 20); do
  commits=$(git -C "$REPO" log --since="1 hour ago" --oneline | wc -l)
  ready=$(br ready --json | jq '.issues | length')
  inflight=$(br list --status=in_progress,claimed --json | jq '.issues | length')

  if [ "$commits" -eq 0 ] && [ "$ready" -eq 0 ] && [ "$inflight" -eq "$PREV_INFLIGHT" ]; then
    CONV=$((CONV+1))
    [ "$CONV" -ge 2 ] && { echo "CONVERGED — exit"; break; }
  else
    CONV=0
  fi
  PREV_INFLIGHT=$inflight
  sleep 600
done
```

**Validator.** Loop exited cleanly; final message to user includes commit count, closed beads, and explicit "swarm converged" statement.

---

## OC-017 — Palette Edit-Before-Send For Template Prompts

**Trigger.** You're about to re-use a saved dispatch prompt that contains a bead ID ("fix bd-3qoly").

**Failure mode.** The bead in the template is already closed; the prompt lands in a session where it's irrelevant; agent wastes a tick investigating.

**Recipe.**

```bash
# Use palette edit-before-send (ntm commit ad20c3e1+)
# Ctrl-P to open palette, select template, 'e' to edit, then send
# OR adjust the message in the command line before dispatch
ntm send <session> --template=next-bead --var bead_id=br-$ACTUAL_ID --no-cass-check
```

**Rule.** Never dispatch templates with stale IDs. Either substitute via `--var` or edit-before-send.

**Validator.** Bead IDs in recent dispatches match currently-open beads, not stale ones.

---

## OC-018 — rch Sync Is Not Universal

**Trigger.** Tests run fine locally but fail on the remote rch worker with "file not found" or stale behavior.

**Failure mode.** Agents assume `rch exec` rsyncs everything; in fact, only paths listed in `transfer.extra_sync_dirs` sync.

**Recipe.**

```bash
# Check config
grep -A10 '^\[transfer\]' ~/.rch/config.toml

# Add missing directory
# (edit ~/.rch/config.toml, append the path to transfer.extra_sync_dirs)

# Or force one-shot sync
rch sync --include-paths=crates/mynew-crate

# Or fall back to local until rch config is updated
cargo test --lib --package mynew-crate
```

**Validator.** Remote tests use the newest local edit; `rch exec md5sum <file>` matches local `md5sum`.

---

## OC-019 — Coordinator Digest Can Lie

**Trigger.** `ntm coordinator digest <session>` returns "no conflicts" but agents report `FILE_RESERVATION_CONFLICT`.

**Failure mode.** Orchestrator trusts digest; conflicts persist; productivity drops without explanation.

**Recipe.**

```bash
# Cross-check digest with conflicts (separate code path)
DIGEST_CONFLICTS=$(ntm coordinator digest <session> | jq '.conflicts | length')
ACTUAL_CONFLICTS=$(ntm coordinator conflicts <session> | jq '.conflicts | length')

if [ "$DIGEST_CONFLICTS" != "$ACTUAL_CONFLICTS" ]; then
  echo "DIGEST LYING: digest=$DIGEST_CONFLICTS actual=$ACTUAL_CONFLICTS"
  # Trust conflicts; investigate digest separately
fi
```

**Rule.** When two `ntm coordinator ...` surfaces disagree, trust the more specific one.

**Validator.** `ntm coordinator digest` and `ntm coordinator conflicts` report consistent counts after resolution.

---

## OC-020 — Narrow Scope Before Widening

**Trigger.** You want to spawn a 7+ repo swarm with 20+ panes.

**Failure mode.** Operator's attention cannot keep up. Panes sit idle at prompt boxes for hours before being nudged. Throughput drops below 3-project baseline.

**Recipe (tier decision).**

| Operator | Max projects | Max panes/project |
| --- | --- | --- |
| Single human orchestrator | 1 | 6 |
| Single AI orchestrator | 3 | 6 (≤18 total) |
| Meta-orchestrator + per-project orchestrators | 7+ | 6 per project |

Beyond these, you need 2-tier orchestration (meta + per-project). Do not exceed the tier limit.

**Rule.** Narrow first. Prove steady-state productivity at 3×6. Then scale with 2-tier.

**Validator.** Every active pane has been nudged at most 1 tick ago; no pane has been idle >2 ticks without diagnosis.

---

## OC-021 — Macro Start-of-Session Beats Granular

**Trigger.** Starting a new swarm session. You're about to run 8+ individual registration/reservation commands.

**Failure mode.** Granular setup is slow, error-prone, and non-idempotent. Different agents' setups drift.

**Recipe (use Agent Mail macros when mail is healthy):**

```python
mcp__mcp-agent-mail__macro_start_session(
    project_key="/data/projects/foo",
    agent_name="GreenCastle",
    program="claude-code",
    model="claude-opus-4-7",
    initial_reservations=["crates/foo/**"],
    ttl_seconds=3600,
    introduce_to=["BlueOcean", "RedCanyon"]  # auto-handshake
)
```

For the orchestrator side:

```bash
ntm spawn <session> --cc=3 --cod=2 --gmi=1 --stagger-mode=smart \
  --prompt "$(cat marching_orders.txt)" \
  --worktrees   # if repo policy allows
```

**Rule.** Macros bundle register + reserve + introduce + inbox-fetch. Use them unless you specifically need granular control.

**Validator.** All agents register within 30s of session start; no registration retry loops.

---

## OC-022 — Cron The Operator Loop (Don't Hand-Tick)

**Trigger.** Manually tending a swarm for >30 min.

**Failure mode.** Operator forgets ticks; misses rate limits; misses stuck panes; swarm drifts.

**Recipe.** Use `CronCreate` (or `/loop 15m` when that slash tool is available) to run the orchestrator loop.

```text
# Example schedule prompt
Every 15 minutes: run one orchestrator tick for session <session>. Use `/vibing-with-ntm` Operator Loop recipe. If convergence detected (see OC-016), report and exit.
```

Or use `/loop` for self-paced iteration if available. Or use a shell cron:

```bash
crontab -l | grep -v "orchestrator-tick" > /tmp/cron.new
echo "*/15 * * * * cd /data/projects/foo && /usr/local/bin/ntm-tick.sh >> /tmp/tick.log 2>&1" >> /tmp/cron.new
crontab /tmp/cron.new
```

**Rule.** If you're orchestrating for ≥30 min, automate the cadence. Every minute you hand-tick is a minute you should be coding.

**Validator.** Cron logs show every tick ran; no gap >20 min.

---

## OC-023 — Pre-Dispatch Hygiene Checklist

**Trigger.** About to send a non-trivial dispatch.

**Failure mode.** Dispatch lands with stale bead ID, wrong pane target, or duplicate of a recent prompt; wastes the tick.

**Recipe (pre-flight):**

```bash
# 1. Freshness
ntm --robot-snapshot | jq '{sources, degraded_sources}'

# 2. Target is actually idle
ntm --robot-is-working=<session> --panes=<pane> | jq '.panes[] | .is_idle'

# 3. Reservation won't collide
ntm coordinator conflicts <session> | jq '.conflicts[] | select(.paths[] | test("<my_pattern>"))'

# 4. No duplicate recent prompt
ntm history search "<first 20 words of prompt>" --days=1 | head -5

# 5. Dispatch with --no-cass-check in orchestrator loops
ntm send <session> --pane=<pane> --no-cass-check "$(cat prompt.txt)"
```

**Validator.** Pre-flight passes before every non-trivial dispatch. Post-flight `ntm --robot-tail` shows the prompt actually rendered.

---

## OC-024 — Handoff Notes Beat Mail Replays

**Trigger.** A pane is ending its session (compaction, restart, shutdown).

**Failure mode.** The next pane on that bead has to re-discover everything from mail history.

**Recipe.**

```text
Before stopping, append a 5-line handoff note to bead <id> description:
  - What I did (commit SHAs if any)
  - Where I am (file, function, line)
  - One open question
  - The exact next step a fresh pane should take
  - Any gotcha (broken build, flaky test, etc.)

Then: `br update <id> --status=open` so the next agent can claim.
```

**Validator.** Fresh pane picks up the bead and advances it within 30 min without re-asking clarifying questions.

---

## OC-025 — "Done" Is Commits + Closed Beads + Push

**Trigger.** A pane reports "I've finished the bead."

**Failure mode.** "Finished" means different things: wrote code / ran tests / committed / pushed / closed bead. Skipping any step = work lost.

**Recipe (definition of done):**

```bash
# 1. Code exists
git status   # working tree clean

# 2. Tests pass locally (or per repo rules)
<repo-specific verify command>

# 3. Committed
git log -1   # shows my commit

# 4. Pushed
git push
git status   # "up to date with origin"

# 5. Beads closed
br close <bead_id> --reason="Completed"

# 6. Reservation released
ntm locks release <session> --paths="<pattern>"  # if using file reservations

# 7. Final mail reply (if thread open)
# send_message(thread_id="beads-<id>", subject="[beads-<id>] Completed", body="<summary>")
```

**Validator.** `br list --status=closed --id=<bead_id>` exists; `git push` succeeded; working tree clean.

---

---

## OC-026 — Verify Agent CLI Is Actually Running

**Trigger.** About to send a prompt or interrupt a pane that looks "stuck."

**Failure mode.** Agent CLI silently exited back to zsh; your prompt lands at the shell as literal text (`zsh: command not found: …`, `zsh: no matches found: (…)`), or your C-c aims at a process that no longer exists.

**Recipe.**

```bash
# Before any send/interrupt, audit the process state
tmux list-panes -t <session>:<win> -F '#{pane_index} #{pane_current_command} #{pane_pid}'

# pane_current_command of "zsh" / "bash" → CLI is dead; relaunch first.
# Expected commands by agent: "claude" (cc), "bun"/"node" (codex), "gemini" (gmi).

# If dead, relaunch via the alias (preserves --dangerously-bypass flags):
tmux send-keys -t <session>:<win>.<pane> "cc" Enter    # or "cod" / "gmi"
sleep 8      # let the CLI boot
# Then dispatch marching orders
```

**Rule.** Check `pane_current_command` (not just the tail) before every state-changing action.

**Validator.** The next `ntm --robot-send` / keypress lands on an agent prompt, not a zsh error line.

---

## OC-027 — Two-Step Relaunch After `--robot-restart-pane`

**Trigger.** You called `ntm --robot-restart-pane … --restart-prompt="…"` to recover a wedged pane.

**Failure mode (observed).** `--robot-restart-pane` uses `tmux respawn-pane -k`, which drops the pane back to **bare zsh**, not to a fresh agent CLI. The `--restart-prompt` text you supplied is then executed by zsh (often as a broken shell command). The pane looks "restarted" but the agent never boots.

**Recipe.** Treat restart-pane as two steps:

```bash
# Step 1 — Nuclear restart (drops to zsh)
ntm --robot-restart-pane=<session> --panes=<pane>

# Step 2 — Relaunch the agent CLI via the alias
tmux send-keys -t <session>:<win>.<pane> "cc" Enter    # or "cod" / "gmi"

# Wait for the CLI to fully boot before dispatching (cc ~6-10s, codex ~3-5s):
sleep 10

# Step 3 — Dispatch fresh marching orders through the normal channel
ntm --robot-send=<session> --panes=<pane> --msg="$(cat marching_orders.txt)"
```

**Same rule applies** to `--robot-restart-pane --restart-prompt="…"` — the prompt is delivered before the CLI is relaunched. Treat `--restart-prompt` as advisory only; always follow with Step 2 + Step 3 above.

Corollary for `--robot-smart-restart --force`: "force" does not necessarily change the pane's PID. Before vs after, compare `tmux list-panes -F '#{pane_index} #{pane_current_command} #{pane_pid}'`. If `pane_pid` is unchanged, the "restart" was soft — escalate to `--robot-restart-pane`.

**Validator.** After Step 2, `pane_current_command` reports the agent process (not zsh). After Step 3, the pane tail shows the prompt rendered inside the agent, not after a `$` shell prompt. And `pane_pid` changed across the restart.

---

## OC-028 — Verify Window Index Before Direct `tmux send-keys`

**Trigger.** About to run `tmux send-keys -t <session>:0.<pane>` or `capture-pane -t <session>:0.<pane>`.

**Failure mode.** `pane-base-index` / `base-index` may be set to `1` in tmux config. Addressing `:0` silently fails with "can't find window: 0"; the keypress disappears and the operator doesn't notice for several ticks.

**Recipe.**

```bash
# Discover the leading window index once per session
WIN=$(tmux list-windows -t <session> -F '#{window_index}' | head -1)

# Then address all panes relative to $WIN
tmux send-keys -t <session>:${WIN}.<pane> "…" Enter
tmux capture-pane -t <session>:${WIN}.<pane> -p -S -20
```

Same applies to `ntm list-panes <session>` — trust the indices it returns over any hardcoded `:0`.

**Validator.** `tmux send-keys … ; tmux capture-pane` shows the keypress echoed in scrollback within 1s.

---

## OC-029 — Recovery-Attempt Budget (2 Strikes, Then Declare Lost)

**Trigger.** You've already tried one intervention on a pane (rotate / restart / interrupt) and it didn't work; you're about to try a third.

**Failure mode.** Recovery attempts on an already-degraded pane often make it worse: cross-pane prompt spill, zsh literal execution, lost in-flight compile work, cascading CLI confusion. Infinite recovery is worse than declared loss.

**Recipe.**

```
Per-pane counter: recovery_attempts
  On each attempted intervention: recovery_attempts += 1
  If recovery_attempts >= 2 AND no observable progress since start: mark pane LOST
  On LOST:
    - Stop intervening
    - ntm kill <session> --pane=<pane>   (or leave it dark if kill is risky)
    - Spawn a fresh replacement: ntm add <session> --cc=1
    - Dispatch the original marching orders to the new pane
```

**Rule.** Two failed interventions is the cap. A fresh pane on a clean account is almost always cheaper than a third rescue attempt.

**Validator.** Replacement pane produces its first commit on the intended bead within 30 min of spawn.

---

## OC-030 — Detach Background-Retry Loops To Keep The Pane Moving

**Trigger.** A state-changing call that depends on a shared resource (bead DB lock, mail server, coordinator) intermittently blocks. The pane is wedged waiting for the call to succeed.

**Failure mode.** Pane sits at "closing bead…" for 10+ minutes waiting for a retryable network/lock error to clear, burning both wall-clock and context on a non-productive wait.

**Recipe.** Detach the retry loop so the pane can pick up its next unit of work immediately:

```bash
# Generic pattern — replace <cmd> with br close / mail send / coordinator update
( for i in 1 2 3 4 5; do
    timeout 60 <cmd> && break
    sleep 20
  done
) &

# Now the pane is free. Dispatch the next nudge:
ntm --robot-send=<session> --panes=<pane> --msg="The close-<id> retry is running in the background. Pick your next bead from bv --robot-triage now."
```

**Rule.** Any call that may retry should detach. Panes should never idle on a retryable failure.

**Validator.** Two panes land their next commit while prior close/send retries are still running.

---

## OC-031 — Sweep Cross-Session Zombies Before Blaming The Swarm

**Trigger.** Bead DB locks, registry locks, or disk churn appear without an obvious cause inside the current swarm.

**Failure mode.** Operator assumes their own agents are the source, restarts panes, finds the same lock reappears. Real cause: a parasitic process from an unrelated session (closed terminal, dead worktree, another project's cargo) still holds the lock.

**Recipe.**

```bash
# Find who is touching the resource across ALL sessions, not just yours
pgrep -af 'br (create|close|update|sync)' | awk '{print $1}' | \
  xargs -I{} sh -c 'echo "PID {}: $(readlink /proc/{}/cwd)"'

pgrep -af 'cargo (test|check|bench|build)' | awk '{print $1}' | \
  xargs -I{} sh -c 'echo "PID {}: $(readlink /proc/{}/cwd)"'

# Long-running rsync / compile / DB write that outlived its session:
ps -eo pid,stat,etime,comm,args | awk '$3 ~ /[0-9]+-/ || $3 ~ /^[0-9]{2,}:/ {print}' | head

# D-state (uninterruptible I/O) processes block SQLite writes filesystem-wide:
ps -eo pid,stat,etime,comm | awk '$2 ~ /D/'
```

Before killing, confirm the PID's `cwd` is NOT one of your live swarm's working dirs. Then:

```bash
kill -15 <pid>       # SIGTERM first
sleep 5
kill -9 <pid>        # SIGKILL if still alive
```

**Rule.** Step 0 of any "something is locked / something is slow" diagnosis is *who is actually holding it, across the whole box*.

**Validator.** Lock count after sweep drops to zero; disk delta trends back down; the originally-failing operation succeeds on next retry.

---

## OC-032 — Per-Pane Build / Artifact Isolation

**Trigger.** Spawning ≥3 agents that will run builds (cargo, go, bun, node, make) against overlapping source trees.

**Failure mode.** Shared target / build / incremental caches create lockfile thrashing and silent cache-key collisions across panes. Throughput collapses even though nothing looks "wrong."

**Recipe.** Give every pane its own artifact root, keyed by project + role + pane:

```bash
# Dispatch as part of marching orders to each pane
ntm --robot-send=<session> --panes=<pane> --msg="Use these build-isolation env vars for this session:
  export CARGO_TARGET_DIR=/tmp/build_<proj>_<role>_p<pane>
  export GOPATH=/tmp/gopath_<proj>_p<pane>
  export BUN_INSTALL_CACHE_DIR=/tmp/bun_<proj>_p<pane>
Make these persistent in your shell rc for the pane's lifetime."
```

Then sweeps can be scoped per-pane (free 19GB by cleaning only one pane's target) and builds stop contending.

**Rule.** Artifact contention is invisible until it isn't; isolate by default for any ≥3-pane build workload.

**Validator.** No two panes share a lock in `lsof +D /tmp/build_*`; no "blocking waiting for file lock" messages in any pane's build output.

---

## OC-033 — Stall-Flip: `in_progress → open` As A Dispatch Signal

**Trigger.** An aged parent/epic bead has been `in_progress` / `claimed` for hours or days with no downstream commits. Sub-bead workers are saying "blocked on parent."

**Failure mode.** Operator waits for the original claimant to return. Sub-bead work silently halts because the dependency graph shows the parent as live.

**Recipe.** Use status-flip as a broadcast signal that the parent is available again:

```bash
# Flip the aged parent back to open
br update <parent_id> --status=open --reason="Stall-flip: no progress 24h, releasing for pickup"

# The graph-visible state change tells every sub-bead worker "parent is claimable."
# Reassign explicitly if you already know who should own it:
ntm --robot-bead-claim=<parent_id> --bead-assignee=<new_agent>
```

**Rule.** Bead status is a broadcast channel, not just bookkeeping. Flipping ownership is cheaper and faster than mailing the old claimant.

**Stalled-bead detection threshold should scale with velocity.** A fixed "45 min stale" threshold is too coarse for a swarm landing commits every few minutes. Parameterize:

```
stale_threshold = max(15m, 3 × median_inter_commit_gap_this_hour)
```

Reopen only when the bead's last-touch-time exceeds this adaptive value AND the assignee has not committed in the same window.

**Validator.** Within 2 ticks of the flip, ≥1 commit lands referencing the parent or a downstream sub-bead.

---

## OC-034 — Depth-Gate Prompts Defeat Shallow-Scan Convergence

**Trigger.** A review / audit pane declares "clean — rotate me" within 2-5 minutes of dispatch.

**Failure mode.** Without a gate, "clean" means "nothing obvious in the first 100 lines I read." Real defects sit untouched; the pane rotates and the next domain gets the same shallow pass.

**Recipe.** Every rotation / "clean" claim must satisfy an evidence gate before it's accepted:

```text
Before declaring <domain> clean, produce evidence:
  1. grep counts: grep -rEn 'unwrap|todo!|unimplemented!|panic!' <scope> | wc -l
     and quote the 3 hottest files by match count.
  2. read-through: quote 3 specific function signatures from the top-3-hottest files
     with a one-sentence note on each.
  3. test run: <repo's verify command> on <scope>, paste last 20 lines of output.

Only after all three → "clean" is accepted and you rotate to the next domain.
```

**Rule.** Accept rotations based on evidence, not prose. Three-item gate is generalizable to any audit / review / coverage claim.

**Validator.** Subsequent ticks on the "cleaned" domain produce zero new HIGH/CRITICAL findings from a fresh reviewer.

---

## OC-035 — Session-Summary Dispatch At Natural Convergence

**Trigger.** A pane or swarm has reached apparent convergence (OC-016) or is about to be shut down.

**Failure mode.** Operator stops without capturing the actual work; next session's operator re-discovers the landscape from scratch.

**Recipe.** Send a structured summary request before pulling the plug:

```text
Produce a ONE-PARAGRAPH session summary (≤8 lines) including:
  - Bead IDs you closed this session (comma-separated list)
  - Commit SHAs you authored (short hash, comma-separated)
  - One concrete thing still open and who/what should pick it up next
  - Any gotcha the next pane on this scope needs to know

Then stop. Do not start new work.
```

**Rule.** Summary prompts self-audit the pane and leave a durable handoff artifact without requiring the operator to parse a long transcript.

**Validator.** The pane's final output is a citable paragraph with real bead IDs and SHAs (grep-verifiable against `git log` and `br list --status=closed`).

---

## OC-036 — "Ready For Validation" Is A Handoff Failure, Not Success

**Trigger.** A pane's tail contains phrases like "Ready for validation via …", "Successfully identified and optimized", "MISSION ACCOMPLISHED", "awaiting review", without a matching `br close` or `git push` in the same window.

**Failure mode.** Agent finished the work in-memory but parked itself waiting for a human / another agent to validate. The bead sits half-done for hours; the commit may never land.

**Recipe.**

```bash
# Detect convergence-language-without-closure as a distinct state
# (separate from the "keep working" convergence in OC-016):
ntm --robot-tail=<session> --lines=30 --panes=<pane> | \
  grep -iE 'ready for validation|mission accomplished|awaiting (review|validation)|handing off'

# Matched → dispatch the self-close nudge:
ntm --robot-send=<session> --panes=<pane> --msg="Don't hand off. Validate your own work now: run the repo's verify command, commit if clean, push, then br close <id>. If validation fails, fix and re-run. Do not stop until the bead is closed and the commit is pushed."
```

**Rule.** "Done" is `OC-025` (push + close). Anything else — including eloquent summaries — is a handoff failure.

**Validator.** Within 1 tick, either a `br close` lands for the referenced bead OR the pane surfaces a concrete blocker (not a review request).

---

## OC-037 — Multi-Enter Submit For CLIs That Don't Auto-Submit Pastes

**Trigger.** Dispatching a multi-line prompt to a codex pane (or any CLI that treats pastes as multi-line edits, not auto-submits).

**Failure mode.** `ntm --robot-send` returns `"success": true` but the prompt sits in the input buffer with only the last line accepted. One Enter may just insert a newline; the second actually submits. `cc` auto-submits on one Enter but `codex` often needs two or three.

**Recipe.** Bake the multi-Enter pattern into the dispatch loop for codex-family panes:

```bash
# Dispatch
ntm --robot-send=<session> --panes=<pane> --msg="$(cat prompt.txt)" --type=codex

# Then force submission (tmux target; respect OC-028 window index)
WIN=$(tmux list-windows -t <session> -F '#{window_index}' | head -1)
for i in 1 2 3; do
  tmux send-keys -t <session>:${WIN}.<pane> Enter
  sleep 2
done

# Verify the prompt rendered
tmux capture-pane -t <session>:${WIN}.<pane> -p -S -10 | grep -i 'working\|thinking\|processing' || \
  echo "codex prompt may not have submitted — check tail"
```

**Rule.** Treat cc as "one Enter auto-submits" and codex as "send then 2-3 trailing Enters with 2s sleeps." Never fire-and-forget on a codex dispatch.

**Validator.** `capture-pane` shows the agent's working/thinking indicator within 5 seconds of the final Enter.

---

## OC-038 — Spawn-Time Model Spec, Or Regret It

**Trigger.** About to `ntm spawn` a swarm with codex / cod panes.

**Failure mode.** Omitting the model pin (`--cod=3` without a model qualifier) can default to a model that the active subscription doesn't support ("The 'gpt-4' model is not supported when using Codex with a ChatGPT account"), instantly breaking every cod pane in the swarm. Costs 10+ min to diagnose, relaunch, and re-dispatch.

**Recipe.**

```bash
# Always pin the model alias at spawn time
ntm spawn <session> --cc=3 --cod=3:gpt-5.4 --gmi=1   # or whatever the current alias is

# Verify post-spawn: each cod pane's tail should show a model banner, not an error
ntm --robot-tail=<session> --lines=5 \
  | jq -r '.panes[] | select(.type=="cod" or .type=="codex") | .lines[]' \
  | grep -iE 'model|error'
```

**Rule.** Every agent type needs an explicit model in the spawn command. Never rely on defaults.

**Validator.** All spawned panes reach an agent prompt (not an error dialog) within 30s of `ntm spawn` returning.

---

## OC-039 — Rotate Domain-Blocked Panes Through Phases, Don't Pause

**Trigger.** A pane declares its domain "fully blocked by dependencies" or "nothing ready in scope."

**Failure mode.** Operator pauses the pane and loses hours of productive capacity. An idle pane is always cheaper to redirect than to pause-and-resume.

**Recipe.** Queue the pane through a phase rotation within 3 ticks of the block-declaration:

```
Tick N:     "Cross-review pass 1 on commits in your domain since HEAD~20"
Tick N+1:   "Cross-review pass 2 on different commits (HEAD~40..HEAD~20)"
Tick N+2:   "Phase C: apply /testing-conformance-harnesses (or /testing-fuzzing / /mock-code-finder) to your domain"
Tick N+3:   "Phase D: write the next missing fuzz target for this domain"
```

Each rotation produces ≥1 review bead or commit. Use `codebase-pattern-extraction`-style skill cycling to generate phase targets.

**Rule.** Never let a healthy pane idle because its first-choice domain is blocked; always have the next phase queued.

**Validator.** Blocked panes stay productive (bead filed or commit landed) for 8+ consecutive ticks after the block was declared.

---

## OC-040 — Never Auto-"1" Destructive Action Dialogs

**Trigger.** Any CLI dialog in a pane that proposes a destructive action: "Remove lock file?", "Force-push?", "Reset branch?", "Delete X?".

**Failure mode.** Orchestrator loops that "just accept the default" auto-confirm destructive actions (per RULE 1: no file deletion without express permission). Work is silently lost; .beads state corrupts; branches disappear.

**Recipe.**

```bash
# Detect destructive-action dialogs in tail before any auto-reply
TAIL=$(tmux capture-pane -t <session>:<win>.<pane> -p -S -20)
echo "$TAIL" | grep -iE 'remove .*(file|lock|dir)|force.push|reset.hard|delete|drop .*table|rm -rf' && \
  echo "DESTRUCTIVE DIALOG DETECTED — send '3' (No) and re-prompt clarifying, do not auto-accept"

# Always choose "No" and clarify via prompt:
tmux send-keys -t <session>:<win>.<pane> "3" Enter   # or whichever option is "No / Cancel"

ntm --robot-send=<session> --panes=<pane> --msg="I declined the destructive-action dialog per RULE 1. Explain why you wanted that action and propose a non-destructive alternative before proposing it again."
```

**Rule.** Match on destructive-action keywords (`remove`, `force-push`, `reset --hard`, `delete`, `drop`, `rm -rf`) and default to decline. Ask the agent to justify before ever re-considering.

**Validator.** No orchestrator-initiated file / branch / table deletion across the session's entire log.

---

## OC-041 — Session-Scoped Skill-Rotation De-Duplication

**Trigger.** Rotating a pane through a skill pool (e.g., `/testing-metamorphic` → `/mock-code-finder` → `/profiling-software-performance`) across multiple rounds.

**Failure mode.** Without a per-pane skill history, round-robin degrades to ad-hoc reselection; the pane runs `/testing-metamorphic` two rounds later and reports "Already covered, no bead filed" — wasted tick.

**Recipe.** Maintain a per-pane rolling-window skill-history and weight the next pick against it:

```bash
# Tracked per pane across the whole session, not just per tick
declare -A SKILL_HIST          # pane -> comma-separated recent-N skills

next_skill_for_pane() {
  local pane=$1
  local pool=(/testing-metamorphic /mock-code-finder /profiling-software-performance \
              /reality-check-for-project /testing-fuzzing /testing-conformance-harnesses)
  local recent="${SKILL_HIST[$pane]}"
  for s in "${pool[@]}"; do
    [[ ",$recent," == *",$s,"* ]] && continue
    echo "$s"
    SKILL_HIST[$pane]="${recent},${s}"
    return
  done
  # Exhausted pool — reset oldest
  SKILL_HIST[$pane]=""
  echo "${pool[0]}"
}
```

For bash-in-zsh-subshell portability, wrap dispatch loops that use associative arrays in `bash -c '…'` (zsh `declare -A` silently fails in some configurations).

**Rule.** Prefer deterministic round-robin over ad-hoc reselection. De-dup against recent N, not last tick.

**Validator.** No pane receives the same skill twice within the pool-size window; "already covered" messages drop to zero.

---

## OC-042 — Orchestrator Hygiene Commits Stimulate Review-Mode Panes

**Trigger.** Review-mode panes are converging (nothing new to find) while the implementer pool has paused.

**Failure mode.** Reviewer panes idle when no fresh diffs exist for them to scrutinize.

**Recipe.** Use the orchestrator's own small hygiene commits (trivial result-type fixes, doc comment touch-ups, compile-unblock patches) as stimulus for the review layer:

```bash
# Land a small hygiene commit yourself
git -C <repo> commit -m "chore: fix Result type in straggler module"
git -C <repo> push

# Immediately dispatch the reviewers at the new HEAD
ntm --robot-send=<session> --type=cc --msg="Fresh commit <short-sha>. Review it — is the Result type the right variant? Is the error context preserved? Any regressions? Apply depth-gate (OC-034) before declaring clean."
```

This also creates material worth critiquing; reviewers sometimes find real bugs in ostensibly trivial commits.

**Rule.** An orchestrator that only delegates starves the review layer. Commit occasionally.

**Validator.** Each hygiene commit produces ≥1 follow-up review observation (comment, bead, or follow-on commit) from reviewer panes within 2 ticks.

---

## OC-043 — Queue-Dry Is A Guarded Transition

**Trigger.** `br ready` is empty, `bv --robot-triage` has no actionable picks, or panes keep asking for "next work" after the backlog appears exhausted.

**Failure mode.** Operator treats "no ready work" as permission to invent tasks, creates duplicate beads, or keeps nudging a genuinely converged swarm.

**Recipe.**

```bash
br ready --json
bv --robot-triage | jq '.quick_ref'
ntm work queue-dry --format=json | jq '{queue_dry, evidence, recommendations, warnings}'

# Preview only, if dry:
ntm work queue-dry --ideate --format=markdown

# Mutate only after human/operator review:
ntm work queue-dry --ideate --create-beads --yes --plan-version="$(git rev-parse --short HEAD)"
br dep cycles --json
bv --robot-triage | jq '.quick_ref'
br sync --flush-only
```

**Prompt module (when the queue is really dry):**

```text
Queue-dry confirmed. Stop generic nudges. Either stand down and report the dry-queue evidence, or review the non-mutating `ntm work queue-dry --ideate` roadmap and create beads only after the guard says creation is safe.
```

**Validator.** New beads are created only after a queue-dry preview, and no duplicate/ready bead was skipped. If `queue_dry=true` and creation is not approved, the orchestrator stops rather than sending "keep going."

---

## OC-044 — Pressure-Aware Assignment Before Bulk Dispatch

**Trigger.** Large swarm, build/test storms, many idle panes, or assignments are about to be pushed to more than 3 panes.

**Failure mode.** Operator floods healthy-looking agents while RCH/build hosts, provider quotas, or reservation queues are already saturated. Throughput drops even though every pane received work.

**Recipe.**

```bash
ntm --robot-agent-health=<session>
ntm --robot-rch-status
ntm --robot-quota-status
ntm locks list <session> --all-agents

# Then assign narrowly:
ntm assign <session> --auto --strategy=dependency --limit=2
```

**Decision rule.** If resource pressure is high, lower assignment fanout, route only one blocker-clearing bead, or wait for builds to finish. If quota pressure is isolated to one provider, add or rotate a different provider pool instead of feeding the saturated one.

**Prompt module:**

```text
Pressure-aware assignment: take exactly one blocker-clearing bead in your domain. Do not start broad test/build sweeps until current build pressure drops. Report commit SHA or a concrete blocker within 60 minutes.
```

**Validator.** Active build/process count and queued assignments do not grow after dispatch; at least one assigned bead closes or reports a concrete blocker within the next tick window.

---

## OC-045 — Pane Identity Beats `session:index` Folklore

**Trigger.** Direct tmux commands, serve/API pane targeting, or a script assumes `session:0.N` / `session:index` is stable.

**Failure mode.** Wrong pane receives a prompt because tmux base-index, window layout, or pane renumbering changed. The operator then diagnoses the wrong agent.

**Recipe.**

```bash
tmux list-panes -t <session> -F '#{pane_index} #{pane_id} #{pane_current_command}'
ntm --robot-agent-names=<session>
ntm --robot-inspect-pane=<session> --inspect-index=<pane>
```

Use NTM's robot/serve pane-targeting paths when possible; current NTM resolves through pane IDs internally. When you must use raw tmux, verify the live pane index immediately before `send-keys`.

**Prompt module:**

```text
Before acting, verify the target pane: list panes with pane_id/current_command, then send only to the intended agent. Do not assume window 0 or a remembered pane index.
```

**Validator.** The intended pane's tail changes and no operator/user pane receives prompt fragments.

---

## OC-046 — Tool Contracts Beat Command Folklore

**Trigger.** A prompt or script references old helper shapes such as `dcg check`, assumes RCH is invoked through NTM, or uses stale robot flag aliases.

**Failure mode.** Agents spend a tick debugging a command that never existed in the current binary, or worse, skip a safety/offload check because the wrapper returned misleading output.

**Recipe.**

```bash
ntm --robot-capabilities | jq '.commands[] | select(.name | test("dcg|rch|docs|schema"))'
ntm --robot-dcg-status
ntm --robot-dcg-check --command='git status'
ntm --robot-rch-status
rch exec -- go test -short ./...
```

Current contracts to remember:

- DCG status: `dcg doctor --format json` under the hood.
- DCG check: `dcg --robot test --format json <command>` under the hood.
- RCH execution: call `rch exec -- <build/test command>` directly; NTM reports RCH health, not a replacement build transport.
- Robot flags: query `--robot-capabilities`; do not rely on remembered aliases.

**Prompt module:**

```text
Before using an integration command, query NTM's registry and the tool's current robot/status surface. If the command shape differs from memory, update the command and proceed; do not debug the stale folklore command.
```

**Validator.** The command exists in `--robot-capabilities`, returns structured output, and the actual build/test path still runs through the repo-required tool (`rch exec -- ...` when required).

---

## OC-047 — First-Dispatch-After-Spawn Must Wait For Input-Ready

**Trigger.** You ran a bare `ntm spawn` (or `atm spawn`) that returned immediately, and you are about to fire the first separate `ntm send` / `--robot-send` seconds later. Or: a freshly-spawned pane shows its CLI running but sits at an empty input box with no working/thinking indicator and never started.

**Failure mode (the boot race, observed).** `spawn` returns **before** the pane agent boots to its input box. A separate `send` in the first few seconds is silently dropped — the keystrokes land before the input is rendered. The pane looks spawned (layer 1 of the Liveness Truth Stack passes: the CLI process is alive) but is **never-engaged**: 0.0% CPU on `pane_pid`, no indicator, no artifact, no acknowledged order. Operators misdiagnose this as "stuck" and restart — throwing away a healthy CLI. This is distinct from the permanent bare-shell case (build left zsh, OC-026/OC-027), where the fix is relaunch.

**Recipe.** After a bare `spawn`, before the first separate `send`, wait for input-ready, then dispatch and verify:

```bash
# Prefer the built-in readiness wait where the path supports it:
#   ntm spawn … --assign            → waits until agents are ready, then assigns
#   ntm spawn … --init-prompt="…"   → sent AFTER agents become ready (pair with --assign)
#   ntm --robot-wait=<session> --wait-until=ready --panes=<pane>

# Manual guard (when you spawned bare and must send separately):
WIN=$(tmux list-windows -t <session> -F '#{window_index}' | head -1)
for i in $(seq 1 10); do
  cmd=$(tmux list-panes -t <session>:${WIN} -F '#{pane_index} #{pane_current_command}' | awk -v p=<pane> '$1==p{print $2}')
  case "$cmd" in claude|bun|node|gemini)
    # process is up; confirm the input box rendered, not still booting
    tmux capture-pane -t <session>:${WIN}.<pane> -p -S -8 | grep -qiE '›|>|esc to|/help|tab to' && break ;;
  esac
  sleep 1   # cc input-ready ~6-10s, codex ~3-5s after process appears
done

# NOW dispatch, then verify it engaged (don't fire-and-forget):
ntm --robot-send=<session> --panes=<pane> --msg="$(cat marching_orders.txt)"
tmux capture-pane -t <session>:${WIN}.<pane> -p -S -10 | grep -iE 'working|thinking|processing' \
  || echo "first dispatch may have raced boot — re-send"
```

**Rule.** A bare `spawn`-then-`send` has no readiness guarantee; `--stagger` only paces prompts *baked into the spawn*, not your separate send. If a pane is never-engaged, **re-dispatch (it never received the order), do NOT restart.** For codex panes, combine with the multi-Enter submit (OC-037).

**Validator.** Within one observation window of the (re-)dispatch, the pane shows a working/thinking indicator and `pane_pid` CPU rises above 0.0%, or an artifact/commit/mail ack lands. A pane that stays at 0.0% after a confirmed input-ready re-send is a genuine stall, not a boot race — escalate per OC-003.

---

## How To Use These Cards

- **In a swarm post-mortem:** "We hit OC-001 (rate limit probe) and OC-003 (stuck pane escalation). Root cause: OC-005 (missed claimed status)."
- **In a prompt:** "Follow OC-004 (Ship-or-Surface) immediately."
- **In a dispatch:** link the card ID from the marching orders.
- **In a new session:** scan the trigger list at the top of each card — any matches? Act preemptively.

Every card has a trigger and a validator. If neither fires, the card doesn't apply. If both fire regularly, it's a load-bearing pattern.
