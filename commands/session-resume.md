---
description: Single-command session resume with auto-detection
---

# /session-resume - Unified Session Resume

**Purpose:** Resume work with intelligent context detection. Replaces `/session-start` + `/bundle-load`.

---

## Execute Immediately

```bash
# 1. Show current location
pwd

# 2. Check for progress files
if [ -f "claude-progress.json" ]; then
    echo "📋 Progress file found"
    cat claude-progress.json | jq '.current_state'
else
    echo "📋 No progress file (new project or fresh start)"
fi

# 3. Check for feature list
if [ -f "feature-list.json" ]; then
    INCOMPLETE=$(cat feature-list.json | jq '[.features[] | select(.passes == false)] | length')
    NEXT=$(cat feature-list.json | jq -r '.features[] | select(.passes == false) | .id' | head -1)
    echo "🎯 Incomplete features: $INCOMPLETE"
    echo "🎯 Next: $NEXT"
fi

# 4. Show recent bundles
if [ -d ".agents/bundles" ]; then
    echo "📦 Recent bundles:"
    ls -t .agents/bundles/*.md 2>/dev/null | head -3
fi

# 4.5. Load learnings
if [ -d ".agents/learnings" ]; then
    PATTERNS=$(find .agents/learnings/patterns -name "*.md" -not -name "*TEMPLATE*" 2>/dev/null | wc -l | tr -d ' ')
    ANTI_PATTERNS=$(find .agents/learnings/anti-patterns -name "*.md" -not -name "*TEMPLATE*" 2>/dev/null | wc -l | tr -d ' ')
    echo "📚 Learnings: $PATTERNS patterns, $ANTI_PATTERNS anti-patterns"

    # Warn about high-severity anti-patterns
    if [ -d ".agents/learnings/anti-patterns" ]; then
        grep -l "Severity.*high" .agents/learnings/anti-patterns/*.md 2>/dev/null | while read f; do
            NAME=$(head -1 "$f" | sed 's/# Anti-Pattern: //')
            echo "   ⚠️  Watch: $NAME"
        done
    fi
fi

# 5. Show git context
git log --oneline -3
git status --short
```

---

## Display Format

```
## Session Resumed

📍 **Project:** [from progress or pwd]
📋 **Last work:** [from current_state.working_on or "N/A"]
📝 **Resume summary:** [from current_state.resume_summary or "N/A"]

🎯 **Next feature:** [first incomplete from feature-list]
📦 **Recent bundles:**
   - [bundle-1.md] (2 hours ago)
   - [bundle-2.md] (1 day ago)
📚 **Learnings:** [X patterns, Y anti-patterns]
   ⚠️  Watch: [high-severity anti-pattern names]

**Git:** [clean | N uncommitted files]

---

Continue current work, or load a bundle?
(Type "continue" or bundle name)
```

---

## If User Chooses Bundle

Load the bundle content into context:
```bash
cat .agents/bundles/[bundle-name].md
```

Then display the bundle's overview section.

---

## Why This Exists

**Before (3+ commands):**
```
/session-start
/bundle-load my-research
# ...now you can work
```

**After (1 command):**
```
/resume
# Shows state, offers bundles, ready to work
```

---

## Related Commands

- `/session-end` - Still use this to save state explicitly
- `/bundle-save` - Still use this for explicit checkpoints
- This command is for STARTING, not ENDING
