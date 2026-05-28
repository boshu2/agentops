# /evolve has six concrete friction points that compound across long sessions

> **Status:** Durable export of the 2026-05-11 cycle-1..13 friction observations that justified cycle 45's 6 patches to `skills/evolve/SKILL.md`.
> **Source path (local-only, gitignored):** `.agents/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md` — exported here per `soc-w6vh.5` acceptance.
> **Bound to cycle 45 commit:** `cc21ec2d` (`feat(skill/evolve): land 6 patches from 13-cycle session friction`).
> **Referenced from:** `docs/plans/2026-05-11-evolution-roadmap.md` § "research_refs".

A 13-cycle autonomous /evolve session on 2026-05-11 shipped a full practice-citation epic (756/756 declared, 526 net new declarations across `skills/`, `hooks/`, `schemas/`, `evals/`, `cli/`; 14 commits pushed) but surfaced six skill-level friction points that the then-current SKILL.md didn't address. The frictions compound: each adds a turn of recovery work that eats context, and a heavy-context session degrades into the "still has work but can't do it" failure mode the dormancy gate doesn't catch.

## The six frictions

### 1. "Do NOT ask the user anything" is too absolute

Cycle 8's transition from non-cli pools to cli/ (533 files) was a genuine architecture-shape decision (per-file `// practices:`, per-package, manifest, or exclude). `AskUserQuestion` produced a 30-second user pick that unblocked 357 file edits across cycles 8–10. Without it, /evolve either picks a carrier the user rejects later (rework) or freezes. The skill should carve out an exception for shape decisions touching > 50 files or contract surfaces.

### 2. Regression gate is misleading when blocking + advisory lanes mix

Cycle 7's pre-push gate said "passed (37 skipped)" for Pass 1 AND "1 issues found (advisory)" — same run, contradictory headline. The actual `eval baseline-audit` FAIL was real (Suite struct didn't accept "practices") but the output framed it as advisory. Step 5 of /evolve should explicitly grep for `FAIL` and `Pass 1: FAILED`, not just the trailing status line.

### 3. CLI binary install is invisible to the gate's blast radius

Adding `Practices []string` to `cli/internal/eval/types.go` requires `cd cli && make build && go install ./cmd/ao` for the gate's `cli/bin/ao` and `~/go/bin/ao` to pick up the change. Without that, the gate keeps failing with "unknown field practices" even though the source is correct. /evolve's Step 5 should auto-detect when Go files under cli/ changed and rebuild before re-gating.

### 4. `make sync-hooks` is a hidden dependency of `skills/` and `hooks/` edits

Three times in this session the first gate failed because `cli/embedded/{skills,hooks}/` drifted from `skills/` and `hooks/`. Each failure cost a turn (read error → `cd cli && make sync-hooks` → re-gate). /evolve should pre-emptively run `make sync-hooks` whenever the changed surface includes `skills/` or `hooks/`, before invoking the gate.

### 5. Context budget is not a stop reason in the skill, but it's a real failure mode

Cycles 11–13 showed the loop has work (soc-owed.7 scouted item available) but can't execute it because the work is multi-file feature shape AND my context is already heavy from 10 prior productive cycles. The dormancy gate (`IDLE_STREAK >= 2 AND GENERATOR_EMPTY_STREAK >= 2`) doesn't fire here — work IS found, just not executed. /evolve needs a third stop reason: "context budget exhausted; bounded scout cycles only".

### 6. `ScheduleWakeup` self-perpetuation mode is undocumented

The user explicitly asked for "schedule wakeups to keep nudging this session" — a pattern where each /evolve fire schedules the next fire so the loop runs while the user is away. This works (the harness re-invokes /evolve at the wakeup time, state recovered from disk), but /evolve's SKILL.md has zero mention of it. There's a v2 surface `ao evolve` for the terminal-native loop, but for the in-Claude-Code-harness loop, the `ScheduleWakeup` pattern is the equivalent — and it should be named.

## Why it matters

The current /evolve is documented as an "always-on autonomous loop until kill switch or dormancy". The dormancy gate is well-defined for "no work". The skill is silent on "work exists but is the wrong shape for the current session". A 13-cycle session that ends in `IDLE_STREAK=1` looks like dormancy isn't reached — but practically the session ended because I declined to take on operator-level work, not because the queue went empty. Naming context-budget exhaustion + scout-mode as first-class states would turn this from "the skill quietly ends" into "the skill exits cleanly with a handoff".

## The six concrete patches (landed cycle 45, commit `cc21ec2d`)

1. **Step 3.0 Scope Filter.** Before Step 3.1, if the harvested item touches > 5 files OR introduces a new shape (schema/carrier/struct field), and current cycle is > 5 productive cycles in, route to scout-mode (read+annotate the queue entry, no execution).
2. **Step 5.0 Source Surface Detection.** If `git diff --name-only --cached` includes `cli/**/*.go`, run `cd cli && make build && go install ./cmd/ao` before gating. If it includes `skills/**` or `hooks/**`, run `cd cli && make sync-hooks` before gating.
3. **Step 5.1 Gate Output Parsing.** `grep -E '^.*Pass [0-9]+: (FAILED|BLOCKED)' /tmp/gate.log` instead of trusting the trailing status line. The "advisory: N issues" line is not authoritative.
4. **Operator-shape decisions exception** to Step 4. `AskUserQuestion` is permitted ONLY for declaration-shape, carrier-shape, or schema-touching decisions affecting > 50 files. Decisions within an established shape continue to be autonomous.
5. **CONTEXT_BUDGET_EXHAUSTED stop reason.** After N cycles where N is configurable (default 8), if any cycle records `result: harvested|idle` due to scope-too-large, increment a `context_streak` counter. If `context_streak >= 2`, exit with a handoff message naming the parked work.
6. **Document the ScheduleWakeup self-perpetuation mode** as the Claude-Code-harness equivalent of `ao evolve` terminal loop. Each cycle calls `ScheduleWakeup` at end-of-turn; KILL/STOP files still terminate; the dormancy gate plus the new context-budget gate are the two skill-level exits.

## Hypothesis verdicts (as of cycle 51, hypotheses.jsonl)

Patches retroactively labeled as hypotheses; 2 were FALSIFIED within 6 cycles for being text-only without harness automation:

- **H45.1** (Step 3.0 scope filter): PENDING (check_at_cycle=60).
- **H45.2** (Step 4.5 source-surface auto-rebuild): **FALSIFIED** — cycles 48–49 re-hit exactly the registry-stale + bats-tests failures this patch claimed to prevent. Patch added as text in SKILL.md but no harness automation; effectively unwired.
- **H45.3** (Step 5 grep-based gate-output parsing): **FALSIFIED** — cycles 45/46/47/48/49 ALL pushed with "PASS" claims while CI was actually red. I parsed the trailing line, not the structural marker, every time. Documentation without automation.
- **H45.4** (AskUserQuestion carve-out): PENDING — no > 50-file shape decisions in cycles 45–50.
- **H45.5** (CONTEXT_BUDGET_EXHAUSTED): PENDING — `context_streak` counter not implemented (text-only patch in SKILL.md; no state field in session-state.json).
- **H45.6** (ScheduleWakeup self-perpetuation): **VERIFIED** — operator's "keep looping all day. schedule wake ups" prompts directly use the documented pattern; load-bearing.

The falsification pattern is itself the lesson that drove the cycle-51 read-path mechanisms refactor and the cycle-54..57 enforcer hook trio: text in SKILL.md is aspirational until paired with harness automation.

## Companion artifacts

- Cycle-history ledger: `.agents/evolve/cycle-history.jsonl` (cycles 1–60+; local-only, gitignored).
- Hypothesis tracker: `.agents/evolve/hypotheses.jsonl` (local-only).
- Cycle 45 commit body (durable, on origin/main): `git log -1 cc21ec2d`.
- Cycles 45-49 retro (local): `.agents/post-mortems/2026-05-12-evolve-cycles-45-49-retro.md`.
- Improvement-focused post-mortem: `.agents/post-mortems/2026-05-12-evolve-session-improvement-postmortem.md`.
- Rescope plan: `docs/plans/2026-05-12-rescope-evolve-and-architecture.md`.
- Meta-contract: `docs/contracts/update-principles.md`.

## Why this file is tracked when the source learning is gitignored

`.agents/` is operator-local runtime state. Learnings written there don't persist across operators or clean checkouts. Public claims that depend on those learnings for rationale lose traceability the moment the local store is missing. The acceptance criterion of `soc-w6vh.5` calls this out: "Future value audits should not depend on missing local-only evidence for why /evolve skill changes were made." This file is the durable export.

Going forward (per `soc-w6vh.5` second acceptance clause): post-mortem and evolve guidance requires durable export whenever a learning justifies a skill or gate change. The check that catches `docs/` references to absent `.agents/learnings/` paths is a follow-up bead.
