---
name: toil-mining
description: Mine usage history (cass, rtk, shell) for
---
# $toil-mining — Mine usage history for automation candidates

> **Purpose:** the operator's own history is the highest-signal backlog nobody
> reads. This skill mines it — past agent sessions, command analytics, shell
> history — clusters the repeated prompts and command sequences, scores each
> cluster by frequency x pain, and hands a ranked candidate list to
> `$automation-shape-routing`, which decides the shape (workflow vs swarm vs
> skill). Toil-mining is that router's missing feeder: routing is useless
> without a measured queue of things worth routing.

**Use when:** the same prompt keeps getting hand-pasted across sessions; the
operator asks what to automate next; or the scheduled compounding sweep fires.
Non-goals: building the automation, choosing its shape, or editing history.

(`user-invocable: false` is interim — promotion to invocable needs catalog +
dispositions rows, a separate one-line change outside this skill's directory.)

## ⚠️ Critical Constraints

- **⛔ NEVER dispatch via `claude -p` / `claude --print` — scheduled ticks and headless workers use `codex exec` (or the local llama lane).** **Why:** because of LAW 0 — `claude -p` bills the API / burns the Max quota, is hook-blocked on this host, and a sweep that fires on a timer would burn it on a schedule.
- **Measure before believing.** Candidates come from counted history, never from "I feel like I do this a lot." **Why:** because intuition about repetition is unreliable — the measured record shows the inversion that intuition misses (see worked example: rituals at 94x while skills sat at ≤9x).
- **This skill emits candidates; it never builds the automation or picks its shape.** **Why:** because shape (workflow vs swarm vs skill) is `$automation-shape-routing`'s single job — a miner that also builds grades its own homework and duplicates the router.
- **Filter machine echoes before clustering.** Tool results, file-update confirmations, and error strings repeat far more than any human ritual and are not toil. **Why:** because the top raw counts are almost always harness noise; ranking them wastes the whole run (in the fixture, the top 4 raw entries are all machine echoes).
- **Hand-pasted prompts are the loudest toil signal.** A long, structured prompt pasted N times is a skill or tick that does not exist yet. **Why:** because each paste costs operator attention and drifts a little — variant copies of the same ritual diverge until no canonical version exists.
- **Read sources read-only.** Session archives, analytics DBs, and shell history are evidence; never rewrite or prune them. **Why:** because the mine must stay replayable — next month's sweep diffs against the same record.

## Sources

| Source | How to read it | What it yields |
|---|---|---|
| cass (session archaeology) | `$cass` queries over past agent sessions | Repeated hand-typed prompts, recurring rescue patterns, abandoned-then-retried tasks |
| rtk analytics | `rtk gain --history`, `rtk discover` | Command-level frequency, token-savings misses, commands worth proxying |
| Shell history | `history` / `~/.zsh_history` timestamps | Repeated command sequences and multi-step chains outside agent sessions |
| Usage-data scans | ritual-scan JSON (e.g. `~/.codex/usage-data/rituals-*.json`) | Pre-counted repeated prompts across all projects, with first/last-seen windows |

## Execution Steps

### Step 1: Collect

Pull from every available source above; skip missing ones gracefully and note
which were consulted. Capture per item: text, count, first/last seen, source.

**Checkpoint:** at least one source produced counted data. If none did, stop
and report "no measurable history" — do not fall back to guessing.

### Step 2: Cluster

Group near-duplicates (the same ritual pasted with small edits, the same
command chain with different arguments) into clusters. Then split each cluster:
human-initiated toil vs machine echo (tool results, success confirmations,
error strings). Discard the echoes from ranking; keep their counts as context.

**Checkpoint:** every surviving cluster traces to something a human typed,
pasted, or deliberately re-ran.

### Step 3: Score — frequency x pain

`score = frequency x pain`, where pain weighs: length/structure of the pasted
text, whether it runs unattended (overnight > interactive), consequence of
getting it wrong (safety gates, money, external sends), and drift risk (how
many variants the cluster contains). Rank descending. A high-frequency
one-word nudge ("ok") can still rank low: frequency alone is not the verdict.

**Checkpoint:** the ranking would survive the operator reading it — each score
has a one-line justification naming both factors.

### Step 4: Emit and hand off

Write the ranked candidate list (see Output Specification), each with a
suggested next step. Hand the list to `$automation-shape-routing` for the
shape decision; durable candidates worth tracking become bead candidates via
`$beads-br`. The miner stops there.

## Modes

| Mode | Trigger | Behavior |
|---|---|---|
| (default) | invoked on demand | One mining pass over all sources, full report |
| `--compounding` | scheduled tick (launchd on Mac, systemd user timer on bushido) | Proactive sweep: diff against the previous findings file, report only new/grown clusters, append to the findings ledger, queue bead candidates. Dispatched headlessly via `codex exec` — never `claude -p`. Design: [references/tick-design.md](references/tick-design.md) |

Compounding is the point: a one-off mine pays once; a scheduled sweep keeps
catching new rituals as they form, before they fossilize into 90-paste habits.

## Worked Example

Input: [fixtures/rituals-excerpt.json](fixtures/rituals-excerpt.json) — a
20-entry excerpt derived from a real ritual scan (71,191 prompts scanned).
Step 2 discards the top raw counts (1545x "Bash completed with no output",
498x, 337x, 319x — all machine echoes). Step 3 ranks the surviving human toil:

```markdown
# Toil-mining candidates — 2026-06-12

| Rank | Cluster | Freq | Pain | Why it ranks here | Suggested next step |
|---|---|---|---|---|---|
| 1 | overnight factory supervisor ritual ("[/loop overnight factory supervisor — Bo is asleep...]") | 94 | very high | ~150-word safety-critical prompt, hand-pasted, runs unattended overnight, gates external actions | route to automation-shape-routing; likely a scheduled tick + skill |
| 2 | META-ORCHESTRATOR TICK v2 | 81 | very high | multi-section orchestration pass, pasted 81x in ~20h, drift-prone across variants | same — strongest tick candidate |
| 3 | one-work-tick factory prompt | 39 | high | structured single-unit work dispatch with hard gates | fold into the same tick family as #1/#2 |
| 4 | "keep going towards goal" | 44 | medium | pure nudge; cheap to type but signals a missing supervisor loop | symptom of #1-#3; closes when they ship |
| — | "ok" (112x) | 112 | low | one-word approval; frequency high, pain near zero | not a candidate — note only |

Frequency inversion: these hand-pasted rituals ran 94/81/44/39 times while no
skill invocation exceeded 9 in the same window. The automation that exists is
not the automation that is needed.
```

Candidate #1 is the 94x overnight-supervisor ritual: highest combined score
because it is long, safety-gated, unattended, and pasted nearly a hundred
times — exactly the profile of a tick that should already exist.

## Output Specification

- **Artifact directory:** `$REPO/.agents/toil-mining/` in the mined repo.
- **Filename convention:** `YYYY-MM-DD-candidates.md`; machine output, when
  requested, is the same basename with `.json`. Compounding also updates its
  ledger but still emits this run report.
- **Serialization/schema format:** Markdown contains exactly one sources line,
  discarded-echo note, six-column ranked table with at least one valid row,
  and terminal handoff line. JSON is an array of ranked candidate objects.
- **Validator command:** with `$REPO`, `$date`, and `$json_output` (`0|1`) set:

  ```bash
  set -euo pipefail
  [[ "$date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]
  python3 -c 'import datetime, sys; datetime.date.fromisoformat(sys.argv[1])' "$date"
  physical_repo="$(cd "$REPO" && pwd -P)"
  report_dir="$physical_repo/.agents/toil-mining"
  test "$(cd "$report_dir" && pwd -P)" = "$report_dir"
  report="$report_dir/$date-candidates.md"
  test -f "$report"
  test ! -L "$report"
  test -s "$report"
  test "$(grep -Ec '^Sources consulted: .+$' "$report")" -eq 1
  test "$(grep -Ec '^Discarded echoes: .+$' "$report")" -eq 1
  test "$(grep -Fxc '| Rank | Cluster | Freq | Pain | Why it ranks here | Suggested next step |' "$report")" -eq 1
  test "$(grep -Fxc '|---|---|---|---|---|---|' "$report")" -eq 1
  rows="$(grep -Ec '^\| [0-9]+ \|' "$report")"
  valid="$(grep -Ec '^\| [0-9]+ \| [^|]+ \| [1-9][0-9]* \| [^|]+ \| [^|]+ \| [^|]+ \|$' "$report")"
  test "$rows" -gt 0
  test "$valid" -eq "$rows"
  handoff='^Handoff: automation-shape-routing; bead candidates: .+$'
  test "$(grep -Ec "$handoff" "$report")" -eq 1
  last_nonempty="$(awk 'NF { line = $0 } END { print line }' "$report")"
  printf '%s\n' "$last_nonempty" | grep -Eq "$handoff"
  if [[ "$json_output" == 1 ]]; then
    json="${report%.md}.json"
    test -f "$json"
    test ! -L "$json"
    test -s "$json"
    jq -e 'type == "array" and length > 0 and all(.[];
      (.rank | type) == "number" and .rank > 0 and
      (.cluster | type) == "string" and (.cluster | length) > 0 and
      (.frequency | type) == "number" and .frequency > 0 and
      (.pain | type) == "string" and (.pain | length) > 0 and
      (.justification | type) == "string" and (.justification | length) > 0 and
      (.suggested_next_step | type) == "string" and
      (.suggested_next_step | length) > 0)' "$json" >/dev/null
  else
    [[ "$json_output" == 0 ]]
  fi
  ```
- **Downstream handoff:** pass the validated report to
  `$automation-shape-routing`; only operator-accepted keepers become bead
  candidates via `$beads-br`.

## Quality Rubric

- [ ] Every candidate carries a measured count from a named source, not an estimate
- [ ] Machine echoes were filtered and noted, never ranked
- [ ] Each score names both factors (frequency AND pain) in one line
- [ ] The report ends with a handoff to automation-shape-routing, not a built automation
- [ ] No history source was modified
- [ ] Compounding runs diffed against the prior findings file instead of re-reporting old clusters

## See Also

- [automation-shape-routing](../automation-shape-routing/SKILL.md) — downstream consumer; decides the shape of each candidate
- [cass](../cass/SKILL.md) — the session-archaeology source
- [beads-br](../beads-br/SKILL.md) — lands accepted candidates on the tracker

## Reference Documents

- [references/tick-design.md](references/tick-design.md) — the compounding-mode scheduled-tick design (launchd / systemd user timer dispatching codex exec; findings ledger + bead candidates)
