---
description: Initialize a new multi-day project with 2-Agent Harness (50-200 features)
---

# /project-init - Initialize Multi-Day Project

**Source:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

**Purpose:** Run the Initializer Agent for greenfield applications that qualify for 2-Agent Harness.

**When to Use:** Project is greenfield + >3 sessions expected + vague scope + output is application.

---

## Prerequisites Check

Before proceeding, verify this project qualifies:

| Condition | Check |
|-----------|-------|
| Greenfield | Is this a NEW codebase/application? |
| Duration | Will this span multiple days? (>3 sessions) |
| Scope | Are requirements vague/expansive? (>10 features) |
| Deliverable | Is output a working application? |

If ANY condition is false, suggest standard RPI workflow instead.

---

## Step 1: Check for Existing Project

```bash
if [ -f "feature-list.json" ]; then
    echo "feature-list.json already exists"
    echo "Options:"
    echo "  1. View existing features: cat feature-list.json | jq '.features | length' features"
    echo "  2. Overwrite (loses existing): rm feature-list.json && /project-init"
    echo "  3. Continue existing project: /session-start"
    exit 1
fi
```

If files exist, ask user whether to overwrite or continue.

---

## Step 2: Gather Project Brief

Ask the user:

```
## Project Brief Required

Please provide a 1-2 paragraph description of what you want to build.

Include:
- What the application does
- Who will use it
- Key capabilities (even if vague)

Example:
"Build a dashboard app that shows team metrics. Should have login,
show graphs of productivity data, let managers filter by team member
and date range, and export reports to PDF."
```

Wait for user input.

---

## Step 3: Generate Feature List

**Invoke the `feature-expander` skill with the brief.**

The feature-expander will:
1. Analyze the brief
2. Generate 50-200 features across categories:
   - Functional (40-60%)
   - UI/UX (20-30%)
   - Edge Cases (10-15%)
   - Security (5-10%)
   - Performance (5-10%)

---

## Step 4: Create Project Files

### feature-list.json

Write the generated features to `feature-list.json`:

```json
{
  "project": "[PROJECT_NAME from brief]",
  "plan_bundle": "2-agent-harness-[project]-[date]",
  "created": "[ISO timestamp]",
  "mode": "2-agent-harness",
  "total_features": [count],
  "features": [
    {
      "id": "F001",
      "category": "functional",
      "description": "User can...",
      "priority": 1,
      "steps": ["Step 1", "Step 2"],
      "passes": false,
      "completed_date": null
    }
  ]
}
```

### claude-progress.json

Initialize progress tracking:

```json
{
  "project": "[PROJECT_NAME]",
  "mode": "2-agent-harness",
  "plan_bundle": "2-agent-harness-[project]-[date]",
  "created": "[ISO timestamp]",
  "last_updated": "[ISO timestamp]",
  "sessions": [],
  "current_state": {
    "last_commit": null,
    "working_on": null,
    "vibe_level": null,
    "blockers": [],
    "next_steps": ["Start with F001"]
  }
}
```

### claude-progress.txt

Create initial entry:

```
================================================================================
PROJECT INITIALIZED: [PROJECT_NAME]
Mode: 2-Agent Harness
Created: [DATE]
Total Features: [COUNT]
================================================================================

--- [DATE] [TIME] - Project Initialization ---

CREATED:
- feature-list.json with [COUNT] features
- claude-progress.json for session tracking
- Initial git commit

NEXT SESSION SHOULD:
- Run /session-start to see first feature
- Work on ONE feature at a time
- Run /session-end when done

================================================================================
```

---

## Step 5: Optional - Generate init.sh

Ask user:

```
## Environment Setup Script (Optional)

Would you like me to generate an init.sh for environment setup?
This runs at the start of each session to ensure consistent environment.

1. Yes - Generate basic init.sh
2. No - Skip for now
```

If yes, create `init.sh`:

```bash
#!/bin/bash
# Project: [PROJECT_NAME]
# Generated: [DATE]
# Purpose: Run at start of each session

set -e

echo "Setting up environment..."

# Add project-specific setup here
# Example:
# npm install
# python -m venv .venv && source .venv/bin/activate

echo "Environment ready"
```

---

## Step 6: Initial Commit

```bash
git add feature-list.json claude-progress.json claude-progress.txt
# Add init.sh if created
[ -f "init.sh" ] && git add init.sh

git commit -m "feat: initialize 2-agent harness project

- Created feature-list.json with [COUNT] features
- Created progress tracking files
- Mode: 2-agent-harness
- Ready for /session-start"
```

---

## Step 7: Display Summary

```
## Project Initialized

**Project:** [NAME]
**Mode:** 2-Agent Harness
**Features:** [COUNT] (all passes: false)

### Feature Breakdown
- Functional: [N] features
- UI/UX: [N] features
- Edge Cases: [N] features
- Security: [N] features
- Performance: [N] features

### Files Created
- feature-list.json (feature tracking)
- claude-progress.json (session state)
- claude-progress.txt (session log)
[- init.sh (environment setup)] if created

### How It Works
1. Each session: Run `/session-start` to see next feature
2. Work on ONE feature at a time
3. End session: Run `/session-end` to mark feature complete
4. Repeat until all features pass

### Next Step
Run `/session-start` to begin working on the first feature.

Or review features first:
cat feature-list.json | jq '.features[0:5]'
```

---

## Command Options

| Flag | Purpose |
|------|---------|
| `--quick` | Generate 20-50 features (faster, less comprehensive) |
| `--comprehensive` | Generate 100-200 features (thorough) |
| `--retrofit` | Generate features from existing codebase |

---

## Related Commands

- `/session-start` - Begin working session
- `/session-end` - End session, update progress
- `/progress-update` - Update progress mid-session

---

## Why 2-Agent Harness?

From Anthropic's research on long-running agents:

> "The first agent session is an 'initializer' agent, while all subsequent sessions are 'coding' agents."

> "The initializer agent is given a project description and tasked with generating a comprehensive feature list."

> "The model is less likely to inappropriately change or overwrite JSON files compared to Markdown files."

This prevents:
- **One-shotting** - Trying to build everything at once
- **Premature completion** - Declaring done with features missing
- **Context loss** - Forgetting what was done across sessions
