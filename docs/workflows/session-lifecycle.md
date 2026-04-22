# Session Lifecycle Workflow

**Purpose:** Runtime-aware guide to working across sessions with hook-capable runtimes (including Codex v0.115.0+ native hooks) and the Codex hookless fallback for older versions

**Philosophy:** Talk naturally when lifecycle hooks exist. Codex v0.115.0+ supports native hooks (installed by `scripts/install-codex-plugin.sh`) and works like Claude Code. Older Codex versions use the explicit lifecycle commands instead of assuming hidden automation.

---

## Quick Start

### Option 1: Natural Language (Hook-Capable Runtimes)

Just describe what you want:

| Say This | What Happens |
|----------|--------------|
| "Continue working on X" | `CLAUDE.md` provides the startup surface; hooks recover handoff and stage any factory state silently |
| "I need to add Y" | If no startup goal exists yet, the first substantive prompt can be captured as silent factory intake for later explicit `/rpi` or `ao knowledge brief` use |
| "I'm done for today" | Saves progress, offers retrospective |
| "What should I work on?" | Shows status, suggests next task |
| "Where was I?" | Shows last session, current state, blockers |

### Option 2: Software Factory Lane (Recommended in Codex)

```bash
# Start session with a goal-time briefing when possible
ao factory start --goal "fix auth startup"

# Run the delivery lane
/rpi "fix auth startup"
# or: ao rpi phased "fix auth startup"

# Monitor long-running phased work
ao rpi status

# End session
ao codex stop

# Inspect lifecycle + flywheel health
ao codex status
```

`ao factory start` keeps the operator lane explicit: build a bounded briefing if
the corpus can support it, run Codex startup, then move into RPI. The lower
level lifecycle commands still exist when you want direct control.

### Option 3: Bedtime Dream Run

```bash
# One-time bootstrap / scheduler guidance
ao overnight setup --apply --runner codex --runner claude --at 01:30

# Start a private overnight run now
ao overnight start --goal "close the loop on today's auth work"

# In the morning
ao overnight report --from .agents/overnight/latest
```

Use this when you want AgentOps to work against the real local `.agents` corpus
while you are away. This is not the same as the GitHub nightly workflow:

- GitHub nightly is the public CI proof harness
- `ao overnight` is the private local compounding engine

Scheduling is still external, but Dream now helps you bootstrap it honestly.
Use `ao overnight setup` to detect the host, persist `dream.*` config, and
generate scheduler assistance artifacts for `launchd`, `cron`, or `systemd`
without pretending sleeping laptops have guaranteed wake behavior.

### Option 4: Lower-Level Codex Lifecycle

```bash
# Start session
ao codex start

# During work
ao lookup --query "topic"
ao search "topic" --cite retrieved

# End session
ao codex stop
```

For normal Codex skill usage, entry skills drive the same startup path with
`ao codex ensure-start`, and closeout-owner skills drive the same closeout path
with `ao codex ensure-stop`, including the post-close maintenance that hook-capable
SessionEnd would normally run.

### Option 5: Slash Commands (Hook-Capable Power Users)

```bash
# Start session
/session-start

# During work
/progress-update --complete feature-005

# End session
/session-end
```

**Use the mode your runtime actually supports.** Claude/OpenCode can drive lifecycle via hooks. Codex users should usually prefer `ao factory start --goal "<goal>"` as the briefing-first operator surface, while `ao codex start` / `ao codex stop` / `ao codex status` remain the lower-level lifecycle primitives. Codex skills automate the same boundaries via `ao codex ensure-start` / `ao codex ensure-stop`.

---

## Runtime Modes

| Mode | Start | Closeout | Notes |
|------|-------|----------|-------|
| Hook-capable | Natural language, `/session-start`, or startup hooks | Natural language, `/session-end`, or session-end hooks | Best fit for Claude/OpenCode when hooks are installed; `CLAUDE.md` is the startup surface and hooks stage state silently |
| Codex native hooks (v0.115.0+) | Same startup path as hook-capable — native hooks installed by `scripts/install-codex-plugin.sh` | Native `Stop` hook for turn-scope close-loop; explicit `ao codex stop` / `ao codex ensure-stop` for transcript-driven closeout | Native `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, and `PermissionRequest`; no native `SessionEnd` event today |
| Codex hookless fallback (pre-v0.115.0) | `ao factory start --goal "<goal>"`, `ao codex start`, or skill-driven `ao codex ensure-start` | `ao codex stop` or skill-driven `ao codex ensure-stop` | No startup/session-end hook surface under `~/.codex`; lifecycle is explicit, and closeout owns the same curation hygiene as SessionEnd |
| Dream overnight run | `ao overnight setup --apply` then `ao overnight start --goal "<goal>"` | `ao overnight report --from <dir>` | Private local overnight mode. Dream bootstraps config and scheduler assistance, while the host OS still owns actual scheduling semantics |
| Manual fallback | `ao inject`, `ao lookup` | `ao forge transcript`, `ao flywheel close-loop` | Lowest-level portable path |

---

## Hook-Capable Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│                     SESSION START                           │
│  "Continue the API work" or /session-start                  │
├─────────────────────────────────────────────────────────────┤
│  • Recover handoff / tracker goal when available            │
│  • Keep operator framing in CLAUDE.md                       │
│  • Stage factory goal / briefing files silently             │
│  • Mark missing-goal sessions for prompt-time intake        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                        WORK                                 │
│  "Add the validation layer" or just start coding            │
├─────────────────────────────────────────────────────────────┤
│  • First substantive prompt can become silent factory intake│
│  • Run /rpi or ao knowledge brief explicitly when needed    │
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

## Codex Hookless Lifecycle (pre-v0.115.0)

> Codex v0.115.0+ supports native hooks and follows the hook-capable lifecycle above. The diagram below applies to older Codex versions using the hookless fallback.

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
| `ao codex start` | Start an explicit Codex hookless session | Start of work in Codex pre-v0.115.0 (v0.115.0+ uses native hooks automatically) |
| `ao codex stop` | Close out a Codex session without runtime hooks | End of work in Codex pre-v0.115.0 |
| `ao codex status` | Inspect Codex lifecycle and flywheel health | Any time in Codex |
| Natural-language goal | Native factory intake when hooks are installed | Preferred operator entrypoint in Claude/OpenCode |
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
