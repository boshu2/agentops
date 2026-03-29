# Session Lifecycle Workflow

**Purpose:** Runtime-aware guide to working across sessions with hook-capable runtimes and Codex hookless fallback

**Philosophy:** Talk naturally when lifecycle hooks exist. In Codex, use the explicit lifecycle commands instead of assuming hidden automation.

---

## Quick Start

### Option 1: Natural Language (Hook-Capable Runtimes)

Just describe what you want:

| Say This | What Happens |
|----------|--------------|
| "Continue working on X" | Loads context, shows progress, finds next task when runtime hooks are available |
| "I need to add Y" | Checks for plan, guides you if needed |
| "I'm done for today" | Saves progress, offers retrospective |
| "What should I work on?" | Shows status, suggests next task |
| "Where was I?" | Shows last session, current state, blockers |

### Option 2: Codex Explicit Lifecycle (Recommended in Codex)

```bash
# Start session
ao codex start

# During work
ao lookup --query "topic"
ao search "topic" --cite retrieved

# End session
ao codex stop

# Inspect lifecycle + flywheel health
ao codex status
```

For normal Codex skill usage, entry skills drive the same startup path with
`ao codex ensure-start`, and closeout-owner skills drive the same closeout path
with `ao codex ensure-stop`.

### Option 3: Slash Commands (Hook-Capable Power Users)

```bash
# Start session
/session-start

# During work
/progress-update --complete feature-005

# End session
/session-end
```

**Use the mode your runtime actually supports.** Claude/OpenCode can drive lifecycle via hooks. Codex uses explicit `ao codex start` / `ao codex stop` / `ao codex status`, while Codex skills automate the same boundaries via `ao codex ensure-start` / `ao codex ensure-stop`.

---

## Runtime Modes

| Mode | Start | Closeout | Notes |
|------|-------|----------|-------|
| Hook-capable | Natural language, `/session-start`, or startup hooks | Natural language, `/session-end`, or session-end hooks | Best fit for Claude/OpenCode when hooks are installed |
| Codex hookless fallback | `ao codex start` or skill-driven `ao codex ensure-start` | `ao codex stop` or skill-driven `ao codex ensure-stop` | No startup/session-end hook surface under `~/.codex`; lifecycle is explicit |
| Manual fallback | `ao inject`, `ao lookup` | `ao forge transcript`, `ao flywheel close-loop` | Lowest-level portable path |

---

## Hook-Capable Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│                     SESSION START                           │
│  "Continue the API work" or /session-start                  │
├─────────────────────────────────────────────────────────────┤
│  • Load bundle (if exists)                                  │
│  • Read claude-progress.json                                │
│  • Show: last session, current work, next steps             │
│  • Suggest: next feature to work on                         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                        WORK                                 │
│  "Add the validation layer" or just start coding            │
├─────────────────────────────────────────────────────────────┤
│  • Implement features                                       │
│  • Fix bugs                                                 │
│  • Research when needed                                     │
│  • Commit frequently                                        │
│                                                             │
│  Track progress:                                            │
│  • "Feature X is done" or /progress-update --complete X     │
│  • "I'm blocked on Y" or /progress-update --blocker "Y"     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     SESSION END                             │
│  "Done for today" or /session-end                           │
├─────────────────────────────────────────────────────────────┤
│  • Check for uncommitted changes                            │
│  • Prompt for session summary                               │
│  • Update claude-progress.json                              │
│  • Offer to save bundle                                     │
│  • Suggest /retro for learning extraction                   │
└─────────────────────────────────────────────────────────────┘
```

## Codex Hookless Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│                    CODEX SESSION START                      │
│  ao codex start                                             │
├─────────────────────────────────────────────────────────────┤
│  • Inspect .agents/ and surfaced learnings                  │
│  • Run safe close-loop maintenance                          │
│  • Sync MEMORY.md and write startup context                 │
│  • Record retrieved citations for adopted startup artifacts │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                           WORK                              │
│  ao lookup / ao search --cite / skills                      │
├─────────────────────────────────────────────────────────────┤
│  • Retrieve learnings and findings on demand                │
│  • Record citations when search results are actually used   │
│  • Build, debug, research, plan, validate                   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    CODEX SESSION END                        │
│  ao codex stop                                              │
├─────────────────────────────────────────────────────────────┤
│  • Resolve archived transcript or history fallback          │
│  • Forge/queue learnings safely                             │
│  • Run close-loop maintenance and sync MEMORY.md            │
│  • Persist lifecycle state for status/recovery              │
└─────────────────────────────────────────────────────────────┘
```

---

## Progress Files

Two JSON files track state across sessions:

### claude-progress.json

```json
{
  "project": "my-api",
  "last_updated": "2025-11-27T16:00:00Z",
  "sessions": [
    {
      "session_id": "session-001",
      "started": "2025-11-27T14:00:00Z",
      "ended": "2025-11-27T16:00:00Z",
      "summary": "Implemented auth endpoints",
      "commits": ["abc123", "def456"],
      "features_completed": ["feature-001", "feature-002"]
    }
  ],
  "current_state": {
    "last_commit": "def456",
    "working_on": "feature-003",
    "blockers": [],
    "next_steps": ["Add rate limiting", "Write tests"]
  }
}
```

### feature-list.json

```json
{
  "project": "my-api",
  "total_features": 8,
  "features": [
    {
      "id": "feature-001",
      "description": "User can authenticate",
      "passes": true,
      "completed_date": "2025-11-27T15:00:00Z"
    },
    {
      "id": "feature-002",
      "description": "User can refresh token",
      "passes": true
    },
    {
      "id": "feature-003",
      "description": "API has rate limiting",
      "passes": false
    }
  ]
}
```

**Templates available:** `.claude/templates/`

---

## Natural Language Examples

### Starting Work

```
User: "Let's continue the caching work"

Claude: Welcome back! Last session you completed the Redis connection setup.

📋 Current state:
   Features complete: 3/8
   Working on: Cache invalidation (feature-004)
   No blockers

Ready to continue with cache invalidation?
```

### During Work

```
User: "The cache invalidation is done"

Claude: Great! I'll mark feature-004 as complete.

✅ Updated feature-list.json
   Features complete: 4/8

Next up: feature-005 (Cache warming on startup)
Want to continue, or take a break?
```

### Hitting a Blocker

```
User: "I'm stuck - need the Redis credentials from DevOps"

Claude: I'll note that as a blocker.

⚠️ Blocker added: "Need Redis credentials from DevOps"

Want to:
1. Work on something else while waiting?
2. Draft a message to DevOps?
3. End the session for now?
```

### Ending Session

```
User: "That's it for today"

Claude: Wrapping up your session.

📊 This session:
   Duration: ~2 hours
   Commits: 3
   Features completed: feature-004, feature-005

⚠️ You have 2 uncommitted files. Commit them? [y/n]

Quick summary of what you accomplished?
> Finished cache invalidation and warming

✅ Session saved to claude-progress.json

Next time, just say "continue the caching work" and I'll pick up where you left off.
```

---

## Command Reference

### Session Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `ao codex start` | Start an explicit Codex hookless session | Start of work in Codex |
| `ao codex stop` | Close out a Codex session without runtime hooks | End of work in Codex |
| `ao codex status` | Inspect Codex lifecycle and flywheel health | Any time in Codex |
| `/session-start` | Initialize session, load context | Start of work in hook-capable runtimes |
| `/session-end` | Save progress, close gracefully | End of work in hook-capable runtimes |
| `/progress-update` | Update progress files | During work |

### Workflow Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/research` | Deep exploration of a topic | Before planning complex work |
| `/plan` | Create implementation plan | After research, before coding |
| `/implement` | Execute approved plan | After plan is approved |

### Context Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/bundle-save` | Save context for later | End of session or milestone |
| `/bundle-load` | Load saved context | Start of session |
| `/bundle-search` | Find bundles by topic | When you forget bundle name |

### Learning Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/retro` | Session retrospective | After significant work |
| `/retro --quick` | Extract reusable patterns | After solving problems |

---

## Workflows by Scenario

### Scenario 1: Continuing Previous Work

**Natural:**
```
"Continue the API work"
```

**Commands:**
```bash
/bundle-load api-implementation
/session-start
```

### Scenario 2: Starting New Feature

**Natural:**
```
"I need to add user authentication"
```

**Commands:**
```bash
/research "authentication approaches"
# ... research output ...
/plan authentication-research.md
# ... plan output ...
/implement authentication-plan.md
```

### Scenario 3: Quick Bug Fix

**Natural:**
```
"The login endpoint is returning 500 errors"
```

**Commands:**
```bash
# No special commands needed - just debug
```

### Scenario 4: End of Day

**Natural:**
```
"Done for today"
```

**Commands:**
```bash
/session-end
# optionally:
/retro
```

---

## Best Practices

### Do

- **Talk naturally** - Commands are optional
- **Commit frequently** - Preserve recovery points
- **Write meaningful summaries** - Future you will thank you
- **Mark features complete** - Track progress as you go
- **End sessions gracefully** - Don't let context expire

### Don't

- **Force commands** - Natural language works
- **Skip progress updates** - Tracking helps continuity
- **Let sessions expire** - Save state before context fills
- **Ignore blockers** - Document them for future sessions

---

## Troubleshooting

### "I don't see my progress"

```bash
# Check for progress files
ls claude-progress.json feature-list.json

# If missing, create from templates
cp .claude/templates/claude-progress.json .
cp .claude/templates/feature-list.json .
```

### "My bundle didn't load"

```bash
# Search for bundles
/bundle-search "your topic"

# List all bundles
/bundle-list
```

### "Context is getting full"

```bash
# End session gracefully
/session-end

# Or save and start fresh
/bundle-save my-progress
# Start new session
/bundle-load my-progress
```

---

## Integration with RPI Workflow

For complex features, use the phased RPI flow:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ DISCOVERY   │ ──▶ │ IMPLEMENT   │ ──▶ │ VALIDATION  │
│ /research   │     │  /crank     │     │    /vibe    │
│ /plan       │     │ /implement  │     │ /post-mortem│
│ /pre-mortem │     │  (execute)  │     │  (learn)    │
└─────────────┘     └─────────────┘     └─────────────┘
      │                   │                   │
      ▼                   ▼                   ▼
research + plan      validated code      learnings + next work
    bundle              bundle              + commit
```

**Or just say:** "I need to add a complex feature" and I'll guide you through it.

---

## Files Reference

| File | Location | Purpose |
|------|----------|---------|
| Progress template | `.claude/templates/claude-progress.json` | Session state template |
| Feature template | `.claude/templates/feature-list.json` | Feature tracking template |
| Intent router | `.claude/skills/intent-router.md` | Natural language routing |
| Session autostart | `.claude/hooks/session-autostart.sh` | Auto-show context |
| Session start cmd | `.claude/commands/session-start.md` | Manual session start |
| Session end cmd | `.claude/commands/session-end.md` | Manual session end |
| Progress update cmd | `.claude/commands/progress-update.md` | Manual progress update |

---

**Remember:** Just talk naturally. The system handles the rest.
