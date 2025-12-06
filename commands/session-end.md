---
description: End session gracefully with progress update and optional retrospective
---

# /session-end - Close Long-Running Session

**Source:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

**Purpose:** Update state files before ending. Required for session continuity.

**Why this matters:** Proper session endings preserve institutional memory. Future sessions (including different agents) can resume exactly where you left off without re-discovering context.

---

## Execute Immediately

### Step 0: Verify State Files Exist

```bash
# Check all required state files exist
MISSING=""
[ ! -f "feature-list.json" ] && MISSING="$MISSING feature-list.json"
[ ! -f "claude-progress.json" ] && MISSING="$MISSING claude-progress.json"
[ ! -f "claude-progress.txt" ] && MISSING="$MISSING claude-progress.txt"

if [ -n "$MISSING" ]; then
    echo "⚠️  Missing state files:$MISSING"
    echo "Run /session-start first to initialize them."
fi
```

If files are missing, **run `/session-start` first** - it will create them automatically.

### Step 1: Check Git Status

```bash
git status
```

If uncommitted changes exist, commit them first:
```bash
git add -A && git commit -m "session: [brief description]"
```

### Step 2: Update feature-list.json

For each feature completed this session, change `passes: false` to `passes: true`.

**Important:** Only modify the `passes` field. Do not delete, reorder, or restructure features.

```bash
# Show what needs updating
cat feature-list.json | jq '.features[] | select(.passes == false) | .id + ": " + .description'
```

#### Standard Mode

Ask user: "Which features did we complete this session?"

Then edit feature-list.json to set `passes: true` for completed items.

#### 2-Agent Harness Mode

If `mode: "2-agent-harness"` in feature-list.json:

1. **Identify the worked feature:**
   ```bash
   CURRENT=$(cat claude-progress.json | jq -r '.current_state.working_on')
   ```

2. **Confirm completion:**
   ```
   Did you complete feature $CURRENT?

   Feature: [description]
   Steps:
   - [Step 1]
   - [Step 2]

   (y/n)
   ```

3. **If yes, update:**
   - Set `passes: true` in feature-list.json
   - Add `completed_date: [ISO timestamp]`
   - Update `current_state.working_on` to next feature

4. **Show progress:**
   ```
   Feature $CURRENT marked complete

   Progress: [X+1] of [Y] features ([Z]%)

   Next feature: [F00N+1] [description]

   Ready for next session!
   ```

5. **If no (not complete):**
   - Keep `passes: false`
   - Ask about blockers
   - Add to `current_state.blockers` if blocked

### Step 3: Record Vibe Level

Ask: "What vibe level was this session? (0-5)"

Update `claude-progress.json` to add/update session entry with vibe_level:

```json
{
  "session_id": "YYYY-MM-DD-NNN",
  "started": "SESSION_START_TIME",
  "ended": "NOW",
  "vibe_level": LEVEL,
  "summary": "SESSION_SUMMARY",
  "commits": ["COMMIT_HASHES"],
  "features_completed": ["FEATURE_IDS"]
}
```

### Step 4: Append to claude-progress.txt

Add a new session entry:

```
--- YYYY-MM-DD HH:MM - [Session Description] ---

COMPLETED:
- [What was accomplished]

REMAINING (from feature-list.json where passes=false):
- [List remaining items]

NEXT SESSION SHOULD:
- [Recommendation for next session]

================================================================================
```

### Step 5: Commit State Files

```bash
git add feature-list.json claude-progress.txt claude-progress.json
git commit -m "session: [summary of work done]"
```

### Step 6: Capture Session Metrics (vibe-check)

Capture session metrics with failure pattern detection:

```bash
# Get session metrics with failure pattern detection
VIBE_METRICS=$(npx @boshu2/vibe-check session end --format json 2>/dev/null)

# If successful, parse and display
if [ -n "$VIBE_METRICS" ]; then
    echo "$VIBE_METRICS" | jq '.'
fi
```

This returns metrics and retro data to merge into the session entry:

```json
{
  "metrics": {
    "trust_pass_rate": 92,
    "rework_ratio": 11,
    "iteration_velocity": 4.2,
    "flow_efficiency": 85,
    "vibe_score": 76
  },
  "retro": {
    "failure_patterns_hit": [],
    "failure_patterns_avoided": ["Debug Spiral", "Context Amnesia"],
    "learnings": ["Test-first approach prevented spirals"]
  },
  "baseline_comparison": {
    "trust_delta": 0,
    "rework_delta": -6,
    "verdict": "normal",
    "message": "Typical session for you"
  }
}
```

Display to user:
```
📊 Session Metrics:
   Trust Pass Rate: 92% (+0% vs baseline)
   Rework Ratio: 11% (-6% vs baseline)
   Flow Efficiency: 85%
   VibeScore: 76%

✅ Patterns Avoided: Debug Spiral, Context Amnesia
💡 Learnings: Test-first approach prevented spirals
```

**Merge these fields into the session entry in claude-progress.json.**

If vibe-check is not installed:
```bash
npm install -g @boshu2/vibe-check
```

### Step 7: Check for Milestone Completion

```bash
# Check if all features are complete
if [ -f "feature-list.json" ]; then
    INCOMPLETE=$(jq '[.features[] | select(.passes == false)] | length' feature-list.json)
    TOTAL=$(jq '.features | length' feature-list.json)

    if [ "$TOTAL" -gt 0 ] && [ "$INCOMPLETE" -eq 0 ]; then
        echo "🎉 MILESTONE COMPLETE!"
        echo "   All $TOTAL features done."
        echo ""
        echo "   This is a good time to capture learnings:"
        echo "   → /retro  - Analyze what worked and what didn't"
        echo "   → /learn  - Extract reusable patterns"
    else
        echo "📋 Progress: $((TOTAL - INCOMPLETE)) of $TOTAL features complete"
        echo "   $INCOMPLETE features remaining - continue next session"
    fi
fi
```

**Retro/Learn triggers:**
- **Milestone complete** (all features pass) → Prompt for `/retro` then `/learn`
- **Work in progress** → No prompt, keep focus on features
- **Major blocker resolved** → Consider `/learn --failure` for the debugging pattern
- **Weekly** → `/maintain` catches anything missed

Learnings are saved to `.agents/learnings/` and loaded automatically in future sessions.

---

## Interactive Flow

```
## Session End

### Git Status
[Show git status output]

### Features Completed
Which features did we complete? (comma-separated IDs or "none")
> INT-004, CLN-001

### Session Summary
One sentence describing this session:
> Completed metrics doc and cleaned up duplicate files

### Updating State Files...

✅ feature-list.json: Updated INT-004, CLN-001 to passes: true
✅ claude-progress.txt: Appended session entry
✅ Committed: "session: Completed metrics doc and cleanup"

### Remaining Work
- CLN-002: Delete deprecated /work/docs/vibe-coding/

Next session: /session-start
```

---

## Example Progress Entry

```
--- 2025-11-28 15:30 - Corpus Completion Session ---

COMPLETED:
- Created metrics-to-operations.md (INT-004)
- Removed duplicate tracer files (CLN-001)

REMAINING (from feature-list.json where passes=false):
- CLN-002: Delete deprecated /work/docs/vibe-coding/

NEXT SESSION SHOULD:
- Verify no dependencies on /work/docs/vibe-coding/
- Delete the deprecated directory
- Run make check-links to validate

================================================================================
```

---

## The Workflow

```
/session-start     →  [work]  →  /session-end
      ↓                              ↓
  Read state                    Write state
  Show next item                Update passes
  Orient                        Append log
                                Commit
```

---

## Why This Matters

From Anthropic's research:
> "the model is less likely to inappropriately change or overwrite JSON files compared to Markdown files"

- **JSON for tasks** (`feature-list.json`) - Stable, only `passes` changes
- **TXT for logs** (`claude-progress.txt`) - Append-only, preserves history
- **Git for recovery** - Every state change is committed

---

## Failure Patterns Tracked

vibe-check automatically detects these failure patterns:

| Pattern | Triggered When |
|---------|---------------|
| Debug Spiral | 2+ spirals detected |
| Context Amnesia | Rework ratio >50% |
| Velocity Crash | Iteration velocity <1/hr |
| Trust Erosion | Trust pass rate <60% |
| Flow Disruption | Flow efficiency <50% |

Avoiding patterns generates positive learnings. Hitting patterns suggests retro is needed.

---

## Dependencies

- `@boshu2/vibe-check` npm package (required for session metrics)

Install globally:
```bash
npm install -g @boshu2/vibe-check
```

---

## Related Commands

- `/session-start` - **Run this to begin next session**
- `/progress-update` - Update progress files mid-session
- `/retro` - **Recommended:** Run retrospective for learning extraction
- `/learn` - Extract patterns to `.agents/learnings/`
- `/bundle-save` - Save context bundle if needed
