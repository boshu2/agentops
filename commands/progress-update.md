---
description: Interactively update progress files without manual JSON editing
---

# /progress-update - Update Progress Files

**Purpose:** Safely update `claude-progress.json` and `feature-list.json` without manual editing

**Why this exists:** "The model is less likely to inappropriately change or overwrite JSON files" - but humans can still make errors. This command provides guided updates.

**Token budget:** 3-5k tokens (1-2% context)

---

## Quick Start

```bash
# Update current state
/progress-update

# Mark feature complete
/progress-update --complete feature-005

# Add blocker
/progress-update --blocker "Waiting on API access"

# Clear blockers
/progress-update --clear-blockers
```

---

## Interactive Mode

```bash
/progress-update
```

```
## Progress Update

Current project: dify-deployment
Last updated: 2025-11-27T14:00:00Z

What would you like to update?

1. Mark feature complete
2. Update current work item
3. Add/remove blocker
4. Add next step
5. Update session notes

> 1

### Mark Feature Complete

Incomplete features:
1. feature-005: User can send chat message
2. feature-006: System responds with AI
3. feature-007: Chat history persists

Which feature? > 1

Marking feature-005 as complete...
✅ Updated feature-list.json

Anything else? [y/n] > n

✅ Progress files updated
```

---

## Command Options

### Mark Feature Complete

```bash
/progress-update --complete feature-005
```

Updates `feature-list.json`:
```json
{
  "id": "feature-005",
  "passes": true,
  "completed_date": "2025-11-27T16:30:00Z",
  "completed_session": "session-004"
}
```

### Update Work Item

```bash
/progress-update --working-on "feature-006"
```

Updates `claude-progress.json`:
```json
{
  "current_state": {
    "working_on": "feature-006"
  }
}
```

### Manage Blockers

```bash
# Add blocker
/progress-update --blocker "Waiting on API credentials"

# Clear all blockers
/progress-update --clear-blockers
```

### Add Next Steps

```bash
/progress-update --next-step "Write integration tests"
```

---

## Safety Rules

**This command WILL:**
- Update `passes` field on features
- Update `current_state` fields
- Add session entries
- Add/remove blockers and next steps

**This command will NOT:**
- Delete features
- Modify feature descriptions
- Change feature steps
- Reorder features

> "It is unacceptable to remove or edit tests because this could lead to missing or buggy functionality."

---

## File Locations

Progress files should be at project root:

```
myproject/
├── claude-progress.json    # Session state
├── feature-list.json       # Feature tracking
├── src/
└── ...
```

**Templates available at:** `.claude/templates/`

---

## Integration

### Session Lifecycle

```
/session-start
     ↓
  [work on features]
     ↓
/progress-update --complete feature-X
     ↓
  [more work]
     ↓
/session-end
```

### With Other Commands

- `/session-start` - Reads progress state
- `/session-end` - Updates progress state (calls this internally)
- `progress-tracker` skill - Underlying implementation

---

## Examples

### Mark Multiple Features Complete

```bash
/progress-update --complete feature-001
/progress-update --complete feature-002
```

### Full State Update

```bash
/progress-update --working-on "feature-003" --next-step "Add validation" --next-step "Write tests"
```

### Clear and Reset

```bash
/progress-update --clear-blockers --working-on "feature-001"
```

---

## Troubleshooting

### Files Not Found

```
❌ Progress files not found in current directory

Create them?
1. Yes, initialize from templates
2. No, I'll create them manually

> 1

✅ Created claude-progress.json
✅ Created feature-list.json
```

### Invalid JSON

```
⚠️  feature-list.json has invalid JSON

Options:
1. Show error details
2. Reset from template (loses data)
3. Open for manual fix

> 1

Error: Unexpected token at line 15
```

---

## Related Commands

- `/session-start` - Begin session, reads progress
- `/session-end` - End session, updates progress
- `progress-tracker` skill - Underlying JSON operations

---

**Best Practice:** Use this command instead of manually editing JSON files.
