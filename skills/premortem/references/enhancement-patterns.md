# Enhancement Patterns for Spec Simulation

Patterns for transforming findings into concrete spec improvements.

---

## Pattern 1: Schema from Code

**Problem**: Spec has example JSON that doesn't match actual output.

**Before (Bad)**:
```markdown
The API returns status like:
{
  "status": "running",
  "progress": "50%"
}
```

**After (Good)**:
```markdown
## Status Response Schema

| Field | Type | Values | Description |
|-------|------|--------|-------------|
| status | enum | pending, running, complete, failed | Current state |
| progress | object | {completed: int, total: int} | Progress counts |
| error | string? | null or error message | Present only on failure |

Source: `lib/models.py:WaveStatus`
```

**How to apply**:
1. Find the actual model class in code
2. Extract all fields with types
3. Document enum values explicitly
4. Reference source file

---

## Pattern 2: Error Recovery Matrix

**Problem**: Spec says "handle errors appropriately" without specifics.

**Before (Bad)**:
```markdown
If the operation fails, the system will display an error message.
```

**After (Good)**:
```markdown
## Error Recovery Matrix

| Error Type | User Sees | AI Action | Human Action |
|------------|-----------|-----------|--------------|
| Timeout | "Operation timed out" | Retry once | Check cluster health |
| Auth failure | "401 Unauthorized" | Escalate immediately | Refresh credentials |
| Partial sync | "3/5 apps synced" | Show failed apps | Manual sync remaining |
| Network error | "Connection refused" | Retry 3x with backoff | Check connectivity |
```

**How to apply**:
1. List all error types from code
2. Map each to user-visible message
3. Define AI behavior for each
4. Define human escalation for each

---

## Pattern 3: Mandatory Safety Display

**Problem**: Safety information is optional or buried.

**Before (Bad)**:
```markdown
Wave 6 is forward-only and cannot be rolled back.
```

**After (Good)**:
```markdown
## Safety Classification (ALWAYS DISPLAY)

Every command suggestion MUST show safety level:

| Level | Emoji | Meaning | Display Pattern |
|-------|-------|---------|-----------------|
| Safe | :green_circle: | Read-only, no changes | "**Safe** - This only reads data" |
| Caution | :yellow_circle: | Reversible changes | "**Caution** - This makes changes (reversible)" |
| Dangerous | :red_circle: | Irreversible changes | "**DANGEROUS** - Cannot be undone!" |
| Escalate | :warning: | Requires human expert | "**ESCALATE** - Contact platform team" |

**Wave 6 Example**:
:red_circle: **DANGEROUS - FORWARD ONLY**
This wave migrates Crossplane to v2. There is NO rollback procedure.
```

**How to apply**:
1. Define safety level enum
2. Create visual differentiation (emoji, formatting)
3. Make display MANDATORY in spec
4. Show example for each level

---

## Pattern 4: Per-Tool Timeout Configuration

**Problem**: Single timeout for all operations.

**Before (Bad)**:
```markdown
Tool timeout: 300 seconds
```

**After (Good)**:
```markdown
## Timeout Configuration

| Tool | Operation | Timeout | On Timeout |
|------|-----------|---------|------------|
| get_upgrade_status | Query only | 30s | Retry once |
| preview_wave | Dry run | 60s | Show partial results |
| execute_wave | Wave 1-4 | 300s | Continue polling |
| execute_wave | Wave 5 (operators) | 600s | Show operator status |
| execute_wave | Wave 6 (Crossplane) | 900s | Never auto-retry |
| run_diagnostics | Full scan | 120s | Limit to critical issues |
```

**How to apply**:
1. Profile actual operation durations
2. Add 2x buffer for worst case
3. Define behavior on timeout (retry, partial, escalate)
4. Document differently for different scenarios

---

## Pattern 5: Progress Feedback Specification

**Problem**: Long operations with no feedback.

**Before (Bad)**:
```markdown
Execute the wave and wait for completion.
```

**After (Good)**:
```markdown
## Progress Feedback Requirements

### During Execution
- Status update every 30 seconds minimum
- Show current step: "Approving InstallPlan 3/16..."
- Show elapsed time

### Expected Durations

| Wave | Typical | Maximum | Progress Pattern |
|------|---------|---------|------------------|
| 2-4 | 30s | 2min | Fast, show completion |
| 5 | 5min | 15min | Per-operator status |
| 6 | 10min | 30min | Per-migration step |

### User Guidance
"Wave 5 typically takes 5 minutes. You can check progress with 'get status' or wait for completion notification."
```

**How to apply**:
1. Measure actual durations
2. Define update frequency
3. Specify what to show during wait
4. Give users expected timeframes

---

## Pattern 6: Wave-Specific Handling

**Problem**: All waves treated the same.

**Before (Bad)**:
```markdown
For each wave, call execute_wave with the wave number.
```

**After (Good)**:
```markdown
## Wave-Specific Handling

### Wave 1 (OCP Upgrade)
:red_circle: **HUMAN GATE**
- Requires explicit `human_approved: true` flag
- Display: Full impact assessment before any action
- Duration: 2+ hours, suggest monitoring link
- Never auto-execute

### Wave 6 (Crossplane v2)
:red_circle: **FORWARD ONLY**
- No rollback procedure exists
- Pre-flight: Verify backup exists
- Confirm: Require typed "I understand there is no rollback"
- Post-execution: Verify migration succeeded

### Waves 2-5, 7-10
:yellow_circle: **STANDARD WAVES**
- Preview available (dry-run)
- Confirm before execution
- Rollback guidance available if failed
```

**How to apply**:
1. Identify waves with special requirements
2. Document specific pre-conditions
3. Define confirmation patterns
4. Include post-execution verification

---

## Pattern 7: Escalation Flow

**Problem**: AI tries to handle everything, gets stuck.

**Before (Bad)**:
```markdown
The assistant will help troubleshoot any issues.
```

**After (Good)**:
```markdown
## Escalation Flow

### When to Escalate
1. Error not in known issues database
2. RAG returns no relevant results (score < 0.7)
3. Same error after 2 fix attempts
4. User requests human help
5. Dangerous operation without clear procedure

### Escalation Response Template
:warning: **I need human expertise for this**

I've encountered [specific issue] that I'm not confident handling.

**What I tried:**
- [Action 1 and result]
- [Action 2 and result]

**What I found in docs:**
- [Relevant doc or "no matching documentation"]

**Recommended escalation:**
- Contact: Platform team (#platform-support)
- Include: [specific diagnostic output]
```

**How to apply**:
1. Define escalation triggers
2. Create template response
3. Require "what I tried" summary
4. Provide specific contact/channel

---

## Pattern 8: Audit Trail Requirements

**Problem**: Can't investigate what happened.

**Before (Bad)**:
```markdown
Tool results are displayed to the user.
```

**After (Good)**:
```markdown
## Audit Trail Requirements

### Session Tracking
- Generate unique session_id on first tool call
- Include session_id in all tool calls and responses

### Required Log Fields

| Field | Type | Example |
|-------|------|---------|
| timestamp | ISO8601 | 2026-01-22T10:30:00Z |
| session_id | UUID | abc123-def456 |
| tool_name | string | execute_wave |
| input_params | object | {wave: 5, confirm: true} |
| output_summary | string | "Wave 5 complete, 16 operators updated" |
| duration_ms | int | 45000 |
| safety_level | enum | caution |

### Export Format
```json
{
  "session_id": "abc123",
  "started": "2026-01-22T10:00:00Z",
  "events": [...]
}
```
```

**How to apply**:
1. Define required fields
2. Specify format (JSON, structured log)
3. Include correlation IDs
4. Enable export for postmortem

---

## Applying Patterns

When enhancing a spec:

1. **Match finding to pattern**: Which pattern addresses this failure mode?
2. **Extract concrete details**: What are the actual values, not placeholders?
3. **Apply pattern**: Copy structure, fill in specifics
4. **Verify completeness**: Does this fully address the failure mode?

### Enhancement Checklist

For each finding:
- [ ] Pattern identified
- [ ] Concrete values extracted from code/testing
- [ ] Pattern applied with specifics
- [ ] Cross-referenced with related sections
- [ ] Example included showing pattern in use
