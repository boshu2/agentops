---
description: Initialize session with standardized get-bearings protocol
---

# /session-start - Begin Long-Running Session

**Source:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

**Purpose:** Run standardized orientation protocol at session start. Required for ongoing projects to maintain continuity.

**Why this matters:** Session initialization prevents lost context between sessions. State files track what's done, what's blocked, and what's next - enabling seamless multi-day work.

---

## Execute Immediately

Run these commands NOW:

```bash
# Detect mono-repo context
# Mono-repos (personal/, work/) have state files in their root, git root is parent
CURRENT_DIR=$(pwd)
CURRENT_NAME=$(basename "$CURRENT_DIR")
GIT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null)

# Determine if we're in a mono-repo subdirectory
if [ "$GIT_ROOT" != "$CURRENT_DIR" ] && [ -n "$GIT_ROOT" ]; then
    # We're in a subdirectory of a git repo
    # Check if this looks like a mono-repo workspace (personal/ or work/)
    if [[ "$CURRENT_NAME" == "personal" || "$CURRENT_NAME" == "work" ]]; then
        echo "📂 Mono-repo detected: $CURRENT_NAME (git root: $GIT_ROOT)"
        MONO_REPO=true
        STATE_DIR="$CURRENT_DIR"
    else
        MONO_REPO=false
        STATE_DIR="$GIT_ROOT"
    fi
else
    MONO_REPO=false
    STATE_DIR="$CURRENT_DIR"
fi

REPO_NAME=$(basename "$STATE_DIR")

# 0. Auto-initialize state files if they don't exist (in STATE_DIR)
cd "$STATE_DIR"

if [ ! -f "feature-list.json" ]; then
    cat > feature-list.json << EOF
{
  "project": "$REPO_NAME",
  "created": "$(date +%Y-%m-%d)",
  "mode": "standard",
  "features": []
}
EOF
    echo "✅ Created feature-list.json in $STATE_DIR"
fi

if [ ! -f "claude-progress.json" ]; then
    cat > claude-progress.json << EOF
{
  "project": "$REPO_NAME",
  "created": "$(date +%Y-%m-%d)",
  "current_state": {
    "working_on": null,
    "blockers": [],
    "next_steps": []
  },
  "sessions": []
}
EOF
    echo "✅ Created claude-progress.json in $STATE_DIR"
fi

if [ ! -f "claude-progress.txt" ]; then
    cat > claude-progress.txt << EOF
# Progress Log - $REPO_NAME
Created: $(date +%Y-%m-%d)

## Session Log

EOF
    echo "✅ Created claude-progress.txt in $STATE_DIR"
fi

# Return to original directory
cd "$CURRENT_DIR"

# 1. Show incomplete work (from STATE_DIR)
echo ""
echo "=== Incomplete Features ==="
cat "$STATE_DIR/feature-list.json" | jq '.features[] | select(.passes == false)' 2>/dev/null || echo "(none)"

# 2. Show recent progress context
echo ""
echo "=== Recent Progress ==="
tail -30 "$STATE_DIR/claude-progress.txt" 2>/dev/null || echo "(no progress log yet)"

# 3. Show recent git activity (from git root for full context)
echo ""
echo "=== Git Activity ==="
git -C "$GIT_ROOT" log --oneline -5 2>/dev/null || echo "(no git history)"
echo ""
git -C "$GIT_ROOT" status --short 2>/dev/null || echo "(git status unavailable)"

# 4. Load learnings (check both STATE_DIR and current dir)
LEARNINGS_DIR=""
if [ -d "$STATE_DIR/.agents/learnings" ]; then
    LEARNINGS_DIR="$STATE_DIR/.agents/learnings"
elif [ -d ".agents/learnings" ]; then
    LEARNINGS_DIR=".agents/learnings"
fi

if [ -n "$LEARNINGS_DIR" ]; then
    echo ""
    echo "📚 Learnings Available:"

    # Count learnings
    PATTERNS=$(find "$LEARNINGS_DIR/patterns" -name "*.md" -not -name "*TEMPLATE*" 2>/dev/null | wc -l | tr -d ' ')
    ANTI_PATTERNS=$(find "$LEARNINGS_DIR/anti-patterns" -name "*.md" -not -name "*TEMPLATE*" 2>/dev/null | wc -l | tr -d ' ')

    echo "   Patterns: $PATTERNS | Anti-patterns: $ANTI_PATTERNS"

    # Show recent (last 7 days)
    RECENT=$(find "$LEARNINGS_DIR" -name "*.md" -mtime -7 -not -name "*TEMPLATE*" -not -name "INDEX.md" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$RECENT" -gt 0 ]; then
        echo "   Recent (7d): $RECENT new learnings"
    fi

    # Show high-severity anti-patterns
    if [ -d "$LEARNINGS_DIR/anti-patterns" ]; then
        HIGH=$(grep -l "Severity.*high" "$LEARNINGS_DIR/anti-patterns"/*.md 2>/dev/null | wc -l | tr -d ' ')
        if [ "$HIGH" -gt 0 ]; then
            echo "   ⚠️  $HIGH high-severity anti-patterns to watch"
        fi
    fi
fi
```

---

## Display Status

After running the above, display:

```
📂 Context: [mono-repo name or repo name]
   State files: [STATE_DIR path]
   Git root: [GIT_ROOT path]

📋 Incomplete features: [count from jq output]
📝 Last session: [date from claude-progress.txt]
🎯 Next recommended: [first item where passes=false]
📚 Learnings: [X patterns, Y anti-patterns]

Git status: [clean / N uncommitted files]
```

---

## Mono-Repo Behavior

When running in `personal/` or `work/`:
- State files (`feature-list.json`, `claude-progress.json`, `claude-progress.txt`) live in the mono-repo root
- Git operations reference the parent workspace root
- Each mono-repo maintains its own session history

This allows you to run `/session-start` from `personal/` and get `personal/`-specific state, even though git root is `workspaces/`.

---

## First-Time Initialization

When state files are created for the first time, display:

```
📁 State files initialized in [STATE_DIR]
   - feature-list.json (feature tracking, passes: true/false)
   - claude-progress.json (structured session data)
   - claude-progress.txt (human-readable session log)

This is a new project. Ready for first session.
```

---

## 2-Agent Harness Mode Detection

If `feature-list.json` exists, check for 2-agent mode:

```bash
MODE=$(cat feature-list.json | jq -r '.mode // "standard"')
if [ "$MODE" = "2-agent-harness" ]; then
    TOTAL=$(cat feature-list.json | jq '.total_features')
    COMPLETE=$(cat feature-list.json | jq '[.features[] | select(.passes == true)] | length')
    PERCENT=$((COMPLETE * 100 / TOTAL))

    echo "2-Agent Harness Mode"
    echo "   Progress: $COMPLETE of $TOTAL features ($PERCENT%)"
fi
```

If in 2-agent mode, display enhanced status:

```
## 2-Agent Harness Mode

Progress: [X] of [Y] features complete ([Z]%)

**Working on ONE feature at a time.**

Next feature: [F00N] [description]
   Priority: [1-3]
   Steps:
   - [Step 1]
   - [Step 2]

**Reminder:** Complete this feature fully before moving on.
When done, run `/session-end` to mark it complete.
```

---

## Example Output

```
## Session Initialized

📋 Incomplete features: 3
📝 Last session: 2025-11-28 - State Infrastructure Session
🎯 Next recommended: INT-004 (Metrics to operations document)

### Incomplete Work
- INT-004: Metrics to operations document
- CLN-001: Remove duplicate lowercase tracer files
- CLN-002: Delete deprecated /work/docs/vibe-coding/

### Recent Context (from claude-progress.txt)
COMPLETED:
- Validated actual corpus status (98 files, most work done)
- Created feature-list.json with current state backfilled
- Updated CLAUDE.md with mandatory state protocol

### Git Status
- Last commit: d01e072 "feat: add file-based state tracking"
- Working tree: clean

Ready to proceed. What would you like to work on?
```

---

## Law 6: Classify Vibe Level

Before starting work, classify the task:

```
🎯 What Vibe Level is this session's work? (0-5)

| Level | Trust | Verify | Use For |
|-------|-------|--------|---------|
| 5 | 95% | Final only | Format, lint |
| 4 | 80% | Spot check | Boilerplate |
| 3 | 60% | Key outputs | Features |
| 2 | 40% | Every change | Integrations |
| 1 | 20% | Every line | Architecture |
| 0 | 0% | N/A | Research |

For Level 1-2: Recommend tracer tests before coding.
```

Record the chosen level - it will be saved in `claude-progress.json` at session end.

---

## Step 6: Capture Baseline Metrics (vibe-check)

After vibe level is established, capture baseline:

```bash
# Capture baseline metrics for session comparison
npx @boshu2/vibe-check session start --level $VIBE_LEVEL --format json 2>/dev/null || echo "vibe-check not available (install with: npm install -g @boshu2/vibe-check)"
```

This stores:
- Baseline metrics from last 7 days
- Session ID for tracking
- Vibe level declared

Display baseline if available:
```
📊 Baseline captured:
   Trust Pass Rate: [X]% (avg last 7 days)
   Rework Ratio: [Y]%
   Velocity: [Z]/hr
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

## Related Commands

- `/session-end` - **MUST run before ending** - saves state
- `/progress-update` - Update progress files mid-session
- `/bundle-load` - Load context bundle before session start
