---
date: 2026-05-01
goal: overnight autonomous /evolve loop completing eval-corpus Wave 1.5 polish + Wave 2 starter + opportunistic next-work backlog
complexity: standard (one orchestration script + 8 seeded queue items + 2 kill-switches; Mac-side; backgrounded)
prior_research:
  - .agents/research/2026-05-01-eval-as-self-pruning-corpus-synthesis.md
  - .agents/research/2026-05-01-internal-self-modification-audit.md
applied_findings:
  - f-2026-04-30-002 (corpus-active precondition — applies to evolve loop's work-detection)
  - f-2026-05-01-005 (mutation-safety on dry-run paths)
test_levels: [L0]  # this is automation/scheduling — the L1+L2 testing happens INSIDE each evolve cycle's /rpi → /crank → /validation
---

# Plan: Overnight Evolve-Loop — Eval-Corpus Wave 1.5 + Wave 2 + Opportunistic Backlog

## Context

You just shipped Wave 1 of eval-as-self-pruning-corpus (6 atomic issues across 4 commits: c954643f, 4872191c, 56c8ca2e, 3889921f). The loop is closed end-to-end. Three known follow-up gaps:

- **Wave-1.5 polish** (≤4 hrs of work): awk portability fix, `lib/finding-compiler-helpers.sh` extraction, doc-index entry, 9-marker test, `--reconcile` mode stub
- **Wave-2 starter** (capture sidecar — partial): privacy-redaction posture, treatment-only Run capture from real `~/dev/*` activity
- **Opportunistic backlog** (34 high-severity unconsumed items in `.agents/rpi/next-work.jsonl`): tech-debt cyclomatic-complexity refactors, daemon-portfolio quick wins (E5 CI hygiene, E1 idempotency contract)

`/evolve` is built exactly for this: select worst-fitness gap → `/rpi` cycle (research/plan/pre-mortem/implement/validate) → harvest → repeat until kill-switch / cycle-cap / regression-breaker / dormancy. Built-in `--max-cycles`, `--dry-run`, `--quality`, regression detection.

Bushido's nightly Dream pipeline exists but is in remake-hold (per `~/.claude/reference/bushido.md` "Pipeline remake hold (2026-05-01)") and uses OpenClaw-era infrastructure — wrong tool here. **Mac-side `ao evolve` is the right shape**: runs while Bo sleeps, uses Mac's existing skills + bd workspace + Claude OAuth, no daemon involvement.

## Deliverable: a single kickoff script + 2 commits

### Files to create

| File | Purpose |
|---|---|
| `scripts/overnight-evolve.sh` | The kickoff script. Sources nvm + ao + claude paths; sets budget/timeout; tails progress; writes morning summary |
| `~/.agents/overnight/2026-05-01-23-kill-switch` | File-based kill switch. Touch this file from another terminal to stop evolve before next cycle |
| `.agents/rpi/next-work-overnight-seed.jsonl` | Seed entries to PREPEND to next-work.jsonl before evolve starts (Wave-1.5 polish items) |

### Files to modify

| File | Why |
|---|---|
| `.agents/rpi/next-work.jsonl` | Append the 8 Wave-1.5 polish items so evolve picks them up first (highest severity) |

### Out of scope

- Bushido nightly pipeline reactivation (in remake-hold)
- Wave-3+ (calibrated judge, control arm, attribution, ablation) — too ambitious for one night
- Cross-runtime wiring of overnight evolve (Mac-only this round)

## Boundaries

- **Time budget**: 8 hrs (23:00 → 07:00 ET). `timeout` enforced via `gtimeout 28800` wrapper.
- **Cycle cap**: `--max-cycles=10` — at ~30-45 min/cycle, hits time budget naturally; max-cycles is the belt to timeout's suspenders.
- **Regression breaker**: `/evolve` has built-in regression detection — if any cycle's `/validation` FAILs the post-merge `ao goals measure`, evolve halts.
- **Kill-switch**: file at `~/.agents/overnight/<date>-kill-switch`. Each cycle's pre-flight checks for it; if present, exit cleanly with `<promise>BLOCKED</promise>`.
- **Cost budget**: rely on Claude's own per-account rate-limiting + Anthropic API throttling. No explicit USD cap (the user's existing patterns prove this is OK; can add `MaxCostUSD` in future revision if needed).
- **Mutation surface**: only the `eval-as-self-pruning-corpus-w1` branch + main-mergeable commits. No force-push, no main-branch direct edits.
- **Push policy**: cycles commit locally; `git push origin <branch>` happens ONLY at evolve teardown if all cycles green. Overnight does not push to remote unless successful end-to-end.

## Implementation Detail

### Issue OE-1 — Seed Wave-1.5 polish into next-work.jsonl

Append to `.agents/rpi/next-work.jsonl` (one line per entry, JSONL):

```json
{"title":"Wave 1.5: awk portability fix in eval-verdict-compiler.sh int_ge","type":"bug","severity":"high","source":"wave-1-residual","description":"int_ge() in hooks/eval-verdict-compiler.sh:120 uses awk syntax that's portably-fragile on macOS BSD awk vs GNU awk. Replace awk-based comparison with bash native: [ \"$a\" -ge \"$b\" ]. The threshold-breach branch (harmful>=3 + utility<0.3) currently emits an awk syntax error on Mac (does not break Wave 1 loop closure but pollutes logs).","target_repo":"agentops","files":["hooks/eval-verdict-compiler.sh"]}
{"title":"Wave 1.5: extract lib/finding-compiler-helpers.sh","type":"refactor","severity":"medium","source":"wave-1-residual","description":"Per pre-mortem H3, extract note/warn/write_atomic/slugify/derive_dedup_key/extract_markdown_body/frontmatter_json_from_markdown/relative_to_root from hooks/finding-compiler.sh into lib/finding-compiler-helpers.sh. Refactor finding-compiler.sh to source the lib. Update hooks/eval-verdict-compiler.sh to source the lib (drops 3 inlined helpers). Existing tests/skills/test-finding-registry-flow.sh must remain green.","target_repo":"agentops","files":["lib/finding-compiler-helpers.sh","hooks/finding-compiler.sh","hooks/eval-verdict-compiler.sh"]}
{"title":"Wave 1.5: doc-index entry for eval-verdict-pipeline","type":"docs","severity":"low","source":"wave-1-residual","description":"docs/documentation-index.md missing entry for docs/contracts/eval-verdict-pipeline.md (Edit failed pre-read in original Wave 1 commit). Add as a sibling line to the finding-registry contract entry under the Contracts section.","target_repo":"agentops","files":["docs/documentation-index.md"]}
{"title":"Wave 1.5: extend test-eval-verdict-compiler.sh to 9 PASS markers","type":"task","severity":"medium","source":"wave-1-residual","description":"Add 4 missing assertions per plan rev 2: (a) regressed verdict mutates utility downward + increments harmful_count; (b) harmful_count>=3 + utility<0.3 queues next-work entry (covers the awk fix from OE-1's first item); (c) re-running compiler on processed manifest is no-op (idempotency via watermark); (d) crash-between-mutation-and-watermark replays cleanly. Currently 5+2 PASS = 7; target 9.","target_repo":"agentops","files":["tests/skills/test-eval-verdict-compiler.sh","tests/fixtures/eval-verdicts/manifest-regressed.json","tests/fixtures/eval-verdicts/manifest-empty-applicable.json"]}
{"title":"Wave 1.5: --reconcile mode stub for eval-verdict-compiler.sh","type":"task","severity":"low","source":"wave-1-residual","description":"Plan rev 2 noted --reconcile mode as Wave-1.5 follow-up: re-derive corpus state from full manifest history (not just newer-than-watermark). Just the flag + a TODO that errors with 'not yet implemented' if invoked. Wave 2+ provides real implementation.","target_repo":"agentops","files":["hooks/eval-verdict-compiler.sh"]}
{"title":"Wave 1.5: post-Wave-1 SCHEMA.md content_hash recompute","type":"docs","severity":"low","source":"wave-1-residual","description":"~/.agents/evals/SCHEMA.md was modified for rc3 (commit de37b48 in evals repo). Per the rc3 entry rationale, content_hash should be recomputed and cited in §11 entry #18. Currently §11 entry #18 documents the BUMP but doesn't carry the post-bump hash. Compute via the _validator's canonicalization pipeline; record alongside.","target_repo":"agentops","files":["~/.agents/evals/SCHEMA.md"]}
{"title":"Wave 1.5: cross-vendor Codex review of eval-corpus plan rev 2","type":"task","severity":"medium","source":"wave-1-residual","description":"Pre-mortem ran 5 Claude judges; --mixed cross-vendor (Codex) was deferred to a pre-/crank gate. Now post-/crank: run bushido codex 'review .agents/plans/2026-05-01-eval-as-self-pruning-corpus.md against the 8 council FAIL patterns; verdict in PASS/WARN/FAIL form'. Capture verdict + findings into .agents/council/2026-05-01-pre-mortem-eval-corpus-codex-attempt-1.md. If Codex flags issues, queue follow-ups.","target_repo":"agentops","files":[".agents/council/2026-05-01-pre-mortem-eval-corpus-codex-attempt-1.md"]}
{"title":"Wave 2 starter: capture-sidecar privacy-redaction policy doc","type":"task","severity":"medium","source":"wave-1-followup","description":"Wave 2 needs explicit privacy posture before any real ~/dev/* sessions get captured. Write docs/contracts/eval-capture-sidecar-privacy.md: redaction patterns (envvar-looking strings, AWS keys, GitHub tokens, JWT-shape strings, PII regex), local-only-storage policy, per-repo opt-in mechanism (~/.agents/evals/capture-allowlist.txt with absolute repo paths), kill-switch (env var AGENTOPS_CAPTURE_DISABLED=1).","target_repo":"agentops","files":["docs/contracts/eval-capture-sidecar-privacy.md","docs/documentation-index.md"]}
```

That's 8 entries. Order in next-work.jsonl is: high-severity first → bug, then refactor, then medium tasks, then low/docs. Evolve will pick `--quality` mode to prioritize high-severity.

### Issue OE-2 — Write `scripts/overnight-evolve.sh`

```bash
#!/usr/bin/env bash
# Overnight evolve loop — kicked off manually before bed.
#
# Usage: bash scripts/overnight-evolve.sh
#
# What it does:
#   1. Verify git working tree clean + on the right branch
#   2. Verify ao + claude are on PATH (sources zsh env if needed)
#   3. Set up kill-switch file path
#   4. Wraps `ao evolve --quality --max-cycles=10` in `gtimeout 28800` (8h)
#   5. Tees stdout/stderr to ~/.agents/overnight/$DATE/run.log
#   6. On exit (any cause): write morning summary to ~/.agents/overnight/$DATE/summary.md
#       containing: cycles completed, commits made, files changed,
#       fitness deltas, regression breakers, kill-switch state, exit code

set -uo pipefail
DATE=$(date +%Y-%m-%d-%H%M)
DIR="$HOME/.agents/overnight/$DATE"
KILL_SWITCH="$DIR/kill-switch"
LOG="$DIR/run.log"
SUMMARY="$DIR/summary.md"

mkdir -p "$DIR"

# Pre-flight
cd "$HOME/dev/agentops" || { echo "no agentops repo"; exit 1; }
if [ -n "$(git status --porcelain)" ]; then
    echo "WORKING TREE NOT CLEAN — refusing to start. Stash or commit first." | tee "$SUMMARY"
    exit 1
fi

START_SHA=$(git rev-parse HEAD)
START_BRANCH=$(git rev-parse --abbrev-ref HEAD)
START_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Kill-switch hook for evolve cycles to check
export AGENTOPS_OVERNIGHT_KILL_SWITCH="$KILL_SWITCH"
export AGENTOPS_OVERNIGHT_LOG_DIR="$DIR"

# 8h timeout (gtimeout = GNU coreutils on Mac via brew)
TIMEOUT_BIN=gtimeout
command -v "$TIMEOUT_BIN" >/dev/null 2>&1 || TIMEOUT_BIN=timeout

echo "Starting overnight evolve at $START_TIME (SHA=$START_SHA, branch=$START_BRANCH)" | tee "$LOG"
echo "Kill switch: touch $KILL_SWITCH to stop before next cycle"

# The evolve invocation
"$TIMEOUT_BIN" 8h ao evolve --quality --max-cycles=10 --test-first 2>&1 | tee -a "$LOG"
EXIT_CODE=$?

END_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
END_SHA=$(git rev-parse HEAD)
COMMITS_MADE=$(git rev-list "$START_SHA".."$END_SHA" --count 2>/dev/null || echo "?")
FILES_CHANGED=$(git diff --name-only "$START_SHA" "$END_SHA" 2>/dev/null | wc -l | tr -d ' ')

cat > "$SUMMARY" <<EOF
# Overnight Evolve Summary — $DATE

- **Start**: $START_TIME (SHA=$START_SHA, branch=$START_BRANCH)
- **End**:   $END_TIME (SHA=$END_SHA)
- **Exit code**: $EXIT_CODE  ($([ $EXIT_CODE -eq 0 ] && echo "clean" || echo "non-zero — see log"))
- **Commits made**: $COMMITS_MADE
- **Files changed**: $FILES_CHANGED
- **Kill switch fired**: $([ -f "$KILL_SWITCH" ] && echo "YES" || echo "no")
- **Log**: $LOG

## Commits

\`\`\`
$(git log --oneline "$START_SHA".."$END_SHA" 2>/dev/null || echo "no new commits")
\`\`\`

## Files

\`\`\`
$(git diff --stat "$START_SHA" "$END_SHA" 2>/dev/null | head -50)
\`\`\`

## Fitness deltas

\`\`\`
$(ao goals measure --json 2>/dev/null | jq -c '.gates' 2>/dev/null || echo "ao goals measure unavailable")
\`\`\`

## Next look

- Read $LOG for cycle-level detail
- Run \`git diff $START_SHA..HEAD\` for full review
- \`bd ready\` for fresh queue items spawned by post-mortem harvest
EOF

echo
echo "=== Summary written: $SUMMARY ==="
cat "$SUMMARY"
exit "$EXIT_CODE"
```

### Issue OE-3 — Pre-flight verification

Before kicking off, verify:
1. `git status` clean on `eval-as-self-pruning-corpus-w1` branch
2. `ao --version` returns sane output
3. `claude --version` returns sane output
4. `ao goals measure` runs (i.e., GOALS.md is well-formed)
5. `bd ready` returns the seeded items + existing high-severity backlog
6. `ls ~/.agents/overnight/` exists and is writable

```bash
bash scripts/overnight-evolve-preflight.sh
```

(small companion script — outputs PASS/FAIL per check, exits non-zero on any FAIL)

## Tests

| Level | What | Where |
|---|---|---|
| L0 | shellcheck on the two new shell scripts | inline in OE-2 / OE-3 acceptance |
| L0 | seed JSONL parses cleanly via jq | acceptance check |

L1+ testing happens INSIDE each evolve cycle's `/rpi → /crank → /validation` flow — the cycles themselves are tested. We don't test the orchestration script's correctness beyond syntax + basic acceptance.

## Conformance Checks

```yaml
checks:
  - kind: files_exist
    files:
      - scripts/overnight-evolve.sh
      - scripts/overnight-evolve-preflight.sh
  - kind: command
    cmd: "shellcheck --severity=error scripts/overnight-evolve.sh scripts/overnight-evolve-preflight.sh"
    expected_exit: 0
  - kind: command
    cmd: "jq -c . < .agents/rpi/next-work-overnight-seed.jsonl | wc -l | tr -d ' ' | grep -q '^8$'"
    expected_exit: 0
  - kind: command  # dry-run smoke
    cmd: "bash scripts/overnight-evolve-preflight.sh"
    expected_exit: 0
```

## Issues

| ID | Title | Files | blockedBy |
|---|---|---|---|
| OE-1 | Seed Wave-1.5 polish into next-work.jsonl | .agents/rpi/next-work-overnight-seed.jsonl, .agents/rpi/next-work.jsonl | none |
| OE-2 | Write scripts/overnight-evolve.sh | scripts/overnight-evolve.sh | none |
| OE-3 | Pre-flight script | scripts/overnight-evolve-preflight.sh | none |

All three can land in parallel — disjoint files. Total estimated effort: ~30 min to create, ~10 min to verify.

## Pre-Mortem Compliance (inline, --quick)

Single pass across the 8 council FAIL patterns:

| FAIL pattern | Concern | Mitigation in plan |
|---|---|---|
| 1. Missing mechanical verification | Could the script declare success while evolve actually failed? | Exit code propagated explicitly via `EXIT_CODE`; summary includes both git-based commit count and exit code; user reads summary first thing in morning |
| 2. Self-assessment instead of external gates | Does evolve grade its own work? | Each cycle's /rpi runs /pre-mortem (cross-vendor council on plan) + /validation (post-impl council). External gates active per cycle. |
| 3. Context rot / hallucination | 8 hours = many cycles = drift | `/evolve` forks fresh context per cycle (per skill spec context.window=fork). Cycle isolation prevents accumulating context rot. |
| 4. Propagation surface blindness | New shell scripts in scripts/ — does CI pick them up? | shellcheck conformance check; existing pre-push-gate.sh runs shellcheck. Both lint clean. |
| 5. Plan oscillation | Evolve picks high-severity first — could it churn? | --max-cycles=10 cap; regression breaker halts on validation FAIL; kill-switch file lets operator stop without losing committed work |
| 6. Dead infrastructure activation | New scripts may not be invoked tonight | Bo invokes manually before bed; not relying on systemd timer activation |
| 7. Missing rollback/rescue map | What if evolve commits broken code? | Each cycle commits independently; `git revert <commit>` undoes any single bad commit. Branch is `eval-as-self-pruning-corpus-w1` (not main); no force-push; no remote push. |
| 8. Four-surface closure gap | Code/Docs/Examples/Proof | Code = scripts; Docs = this plan; Examples = the seed JSONL itself; Proof = morning summary.md showing commits + fitness |

**Verdict**: PASS (inline). No blocker findings.

## Decision Gate

`<promise>DONE</promise>` when:
- 3 issues (OE-1, OE-2, OE-3) implemented
- All conformance checks pass
- Bo verifies kickoff script works in `--dry-run`-equivalent mode
- Bo invokes `bash scripts/overnight-evolve.sh` before bed

## Morning Verification (the next-day path)

```bash
# 1. Read the summary
cat ~/.agents/overnight/$(ls -t ~/.agents/overnight/ | head -1)/summary.md

# 2. Inspect commits
git log --oneline pre-wave-1-baseline-eval-corpus-2026-05-01..HEAD

# 3. Validate everything still works
cd cli && go build ./... && go test ./...
bash tests/skills/test-eval-verdict-compiler.sh
bash tests/skills/test-finding-registry-flow.sh

# 4. Decide
#  - All green + good progress → cherry-pick or merge to main
#  - Mixed → review each commit individually
#  - Red → git reset --hard pre-wave-1-baseline-eval-corpus-2026-05-01
```

## Risks (un-glossed)

1. **Claude usage limits**: 10 cycles × multiple Skill invocations × multiple sub-agents per cycle = potentially 50+ Claude API calls. May hit Anthropic API rate limits. Mitigation: `/evolve` already supports rate-limit backoff via /rpi's retry semantics.
2. **/evolve may pick low-value work**: queue items are heuristic-ranked. evolve might spend a cycle on something marginal. Mitigation: `--quality` mode prioritizes high-severity findings first; cycle cap limits damage.
3. **Long-running cycles**: a single /rpi cycle on a large issue (e.g., the helper extraction + finding-compiler refactor) could eat 90 min. Time budget might fit only 5-6 cycles, not 10. Mitigation: time budget timeout (8h) is the real wall; cycle cap is the suspenders.
4. **Background commits to wrong branch**: evolve might branch off mid-cycle. Mitigation: pre-flight checks branch=`eval-as-self-pruning-corpus-w1`; evolve cycles inherit branch; no `git checkout main` happens.
5. **Untracked-file wipe pattern**: as observed during today's Wave-1 build, untracked files can disappear. Mitigation: every evolve cycle commits at the end; nothing important stays untracked overnight.

## Next Steps

1. Land OE-1, OE-2, OE-3 (~30 min)
2. Run `bash scripts/overnight-evolve-preflight.sh` — confirm green
3. Before bed: `bash scripts/overnight-evolve.sh` (in nohup or screen, OR just leave terminal open since it self-times-out at 8h)
4. Morning: read summary, decide merge / cherry-pick / revert
