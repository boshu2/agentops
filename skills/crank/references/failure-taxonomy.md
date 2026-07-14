# Failure Taxonomy

> **Classification and handling of failures in autonomous execution.**

## Overview

Failures in crank execution fall into distinct categories, each with specific detection methods and remediation strategies. The goal is always: **continue the epic, escalate what can't be fixed**.

## Failure Categories

### 1. Polecat Stuck

**Symptoms**:
- No status change for 5+ poll intervals (2.5 min)
- Convoy shows `running` but no progress
- gc session shows no progress

**Detection**:

```bash
# Check session status
gc session list --json | jq '.[] | select(.state == "active")'

# Peek at agent output
gc session peek <agent> --lines 30
```

**Causes**:
- Waiting for user input
- Infinite loop in code
- External service timeout
- Claude usage limit hit

**Remediation**:

```bash
# Step 1: Nudge the agent
gc session nudge <agent> "continue with your assigned task"

# Step 2: Wait one poll interval (30s)

# Step 3: If still stuck, check for usage limit
gc session peek <agent> --lines 50 | grep -i "limit"

# Step 4: If usage limit, nuke and re-sling after cooldown
# WARNING: This destroys the polecat session. Ensure work is saved.
gt polecat nuke <rig>/<name> --force
# Wait for limit reset, then:
gt sling <issue> <rig>

# Step 5: If other cause, nuke and re-sling immediately
# WARNING: This destroys the polecat session. Ensure work is saved.
gt polecat nuke <rig>/<name> --force
gt sling <issue> <rig>
```

**Control boundary**: return the stuck-agent evidence to RPI. Crank does not
count nudges or create a breaker.

---

### 2. Validation Failure

**Symptoms**:
- Polecat completes but issue not closed
- `.agents/validations/` contains failure artifacts
- Commit exists but tests/lint failing

**Detection**:

```bash
# Check polecat output
gc session peek <agent> --lines 50 | grep -i "fail\|error"

# Check validation artifacts
ls ./polecats/<polecat>/.agents/validations/

# Check CI if applicable
git -C ./polecats/<polecat> log -1 --format="%H" | xargs gh run list --commit
```

**Causes**:
- Tests failing
- Lint errors
- Type check failures
- Security scan findings
- Build failures

**Remediation**:

```bash
# Step 1: Add failure context to issue
bd comments add <issue> "Validation failed: $(cat validation-output.txt | head -50)"

# Step 2: Re-sling with hint
bd comments add <issue> "HINT: Focus on fixing <specific failure>"
gt sling <issue> <rig>

# Step 3: If second failure, be more specific
bd comments add <issue> "EXPLICIT: The test_auth_flow test fails because X. Fix by Y."
gt sling <issue> <rig>
```

**Control boundary**: return the validation evidence to RPI. The persistent
governor owns any no-progress/oscillation breaker and helper eligibility.

---

### 3. Dependency Deadlock

**Symptoms**:
- Multiple issues show as `blocked`
- No issues in `ready` state
- Circular dependency detected

**Detection**:

```bash
# Check for circular deps
bd blocked --parent=<epic> --show-deps

# Manual trace
bd show <issue-a> | grep "blocked by"
bd show <issue-b> | grep "blocked by"
# If A -> B -> A, deadlock exists
```

**Causes**:
- Incorrectly specified dependencies
- Missing issue that should break the cycle
- Overly aggressive blocking

**Remediation**:

```bash
# Step 1: Identify the cycle
bd dep graph <epic>  # Visual if available

# Step 2: Remove weakest dependency
bd dep remove <issue> <blocking-issue>

# Step 3: Add comment explaining
bd comments add <issue> "Removed dep on <blocking> to break deadlock.
May need manual integration after both complete."

# Step 4: Continue cranking
# Issues should now become ready
```

**Escalation trigger**: Immediate if auto-resolution fails

---

### 4. Context Limit

**Symptoms**:
- Polecat stops mid-work
- Message about "context limit" or "token limit"
- Partial work committed

**Detection**:

```bash
gc session peek <agent> --lines 50 | grep -i "context\|token\|limit"
```

**Causes**:
- Large files read into context
- Long conversation history
- Complex multi-file changes

**Remediation**:

```bash
# Step 1: Checkpoint current progress
git -C ./polecats/<polecat> stash  # If uncommitted work

# Step 2: Check what was accomplished
git -C ./polecats/<polecat> log --oneline -5

# Step 3: Update issue with progress
bd comments add <issue> "Partial progress: <what was done>. Remaining: <what's left>"

# Step 4: Fresh polecat
gt polecat nuke <rig>/<name> --force
gt sling <issue> <rig>

# The new polecat reads the comment and continues from there
```

**Control boundary**: return context-limit evidence and a proposed
decomposition; Crank owns no local threshold.

---

### 5. Git Conflict

**Symptoms**:
- Merge/rebase fails
- `.beads/` conflicts
- Branch divergence

**Detection**:

```bash
git -C ./polecats/<polecat> status | grep -i "conflict\|diverged"
```

**Causes**:
- Parallel work on same files
- Stale branch
- Beads sync race

**Remediation**:

```bash
# For beads conflicts (most common)
git -C ./polecats/<polecat> checkout --theirs .beads/issues.jsonl
git -C ./polecats/<polecat> add .beads/issues.jsonl
git -C ./polecats/<polecat> commit -m "merge: resolve beads conflict"

# For code conflicts
# Step 1: Check if conflict is trivial
git -C ./polecats/<polecat> diff --name-only --diff-filter=U

# Step 2: If simple, nudge polecat to resolve
tmux send-keys -t gt-<rig>-<polecat> "resolve the git conflicts and continue" Enter

# Step 3: If complex, abort and re-sling with fresh base
git -C ./polecats/<polecat> merge --abort
git -C ./polecats/<polecat> fetch origin
git -C ./polecats/<polecat> reset --hard origin/main
gt sling <issue> <rig>
```

**Control boundary**: return the conflicting paths and proposed serialization
or decomposition to the orchestrator.

---

### 6. External Service Failure

**Symptoms**:
- Timeouts in polecat output
- API errors (429, 500, etc.)
- Network connectivity issues

**Detection**:

```bash
gc session peek <agent> --lines 50 | grep -i "timeout\|429\|500\|network\|connection"
```

**Causes**:
- Rate limiting
- Service outage
- Network partition
- API credential expiry

**Remediation**:

```bash
# Step 1: Identify the service
# (from polecat output)

# Step 2: Check service status
# (manual or via status page)

# Step 3: If rate limited, preserve the service response and retry-after hint
# Return the evidence; any later dispatch requires a governor admission

# Step 4: If outage, pause affected issues
bd update <issue> --labels=WAITING_EXTERNAL
bd comments add <issue> "Paused: <service> outage. Resume when service recovers."

# Step 5: Continue other issues
# External failures shouldn't block entire epic
```

**Escalation trigger**: Immediate for credential issues, after recovery for outages

---

### 7. Polecat Crash

**Symptoms**:
- gc session disappeared from session list
- Agent not in `gc status --json`
- Issue still shows `in_progress`

**Detection**:

```bash
gc session list --json | jq '.[] | select(.alias == "<agent>")'
gc status --json | jq '.agents[] | select(.name == "<agent>")'
```

**Causes**:
- OOM kill
- Segfault in tooling
- System restart
- Manual termination

**Remediation**:

```bash
# Step 1: Clean up orphaned state
gt polecat nuke <rig>/<name> --force 2>/dev/null || true

# Step 2: Reset issue status
bd update <issue> --status=open

# Step 3: Add crash context
bd comments add <issue> "Previous polecat crashed. No partial work recovered."

# Step 4: Re-sling
gt sling <issue> <rig>
```

**Control boundary**: return crash evidence and recovered state to RPI; Crank
does not count crashes or self-authorize re-dispatch.

---

## Failure Handling Matrix

| Failure Type | Detection evidence | Proposed next move |
|--------------|--------------------|--------------------|
| Worker stuck | Last output and process state | Repair or no-progress breaker candidate |
| Validation fail | Command, exit status, and focused output | Repair candidate |
| Dependency deadlock | Dependency cycle | Re-plan candidate |
| Context limit | Checkpoint and unfinished scope | Decomposition candidate |
| Git conflict | Conflicting paths and owners | Serialize or re-plan candidate |
| External service | Service response and retry-after hint | Wait/repair candidate |
| Worker crash | Exit state and recovered files | Repair or systemic-breaker candidate |

## Escalation Evidence

When Crank cannot complete an admitted action, return one evidence packet:

```bash
# Write the detailed failure report into the wave evidence packet
cat <<'EOF'
## BLOCKED ACTION REPORT

**Issue**: <issue-id>
**Epic**: <epic-id>
**Failure Type**: <type>
**Admitted action**: <admission-id>
**Approach**: <what ran>
**Failure**: <reason and evidence reference>
**Blocker class candidate**: <class or none>

### Recommendation
<what human should investigate>
EOF

# Return the packet to RPI without mailing, invoking a helper, or dispatching
# another worker. The governor owns any protected transition.
```

## Post-Failure Analysis

After epic completion (or major milestone), analyze failures:

```bash
# List issues that returned blocked or waiting evidence
bd list --parent=<epic> --has-label=BLOCKER
bd list --parent=<epic> --has-label=WAITING_EXTERNAL

# Check recurring failure signatures
# (requires custom tooling or log analysis)

# Feed into retrospective
/postmortem --topic="crank failures on <epic>"
```

## Prevention Strategies

Based on failure patterns:

| Pattern | Prevention |
|---------|------------|
| Frequent context limits | Decompose large issues |
| Repeated validation fails | Add pre-validation to issues |
| Git conflicts | Smaller, focused changes |
| External service issues | Add circuit breaker patterns |
| Polecat crashes | Monitor system resources |
