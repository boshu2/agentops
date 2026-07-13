# Simulation Prompts for Spec Simulation

Prompts to ask yourself during each iteration of spec simulation.

---

## The Core 10 Prompts

Use these to drive each iteration:

### 1. Input Validation
> "What if the input isn't what we expect?"

- Wrong format
- Missing fields
- Null values
- Out of range
- Malicious input

### 2. Dependency Failure
> "What if the external dependency fails?"

- API returns 500
- Service is down
- Network timeout
- Auth token expired
- Rate limited

### 3. Scale Issues
> "What if this takes 10x longer or 10x more resources?"

- Slow network
- Large dataset
- Many concurrent users
- Resource exhaustion
- Memory pressure

### 4. User Behavior
> "What if the user skips reading instructions?"

- Copy-paste without reading
- Clicks confirm without checking
- Ignores warnings
- Does steps out of order
- Cancels mid-operation

### 5. Rollback Need
> "What if we need to undo this?"

- Partial completion
- Wrong environment
- Changed requirements
- Bug discovered after
- User regret

### 6. Debugging Scenario
> "What if we're debugging this at 2 AM?"

- No access to original user
- Logs are unclear
- State is inconsistent
- Multiple possible causes
- Time pressure

### 7. Partial Failure
> "What happens on partial failure?"

- Step 3 of 5 fails
- Some items succeed, some fail
- Inconsistent state
- Unknown progress
- Recovery unclear

### 8. Repetition
> "What if the user does this 100 times?"

- Accumulating errors
- Resource leaks
- State drift
- Performance degradation
- User fatigue

### 9. Environment Difference
> "What if the environment is different?"

- Different version
- Missing tool
- Different permissions
- Different config
- Different network

### 10. Audit & Compliance
> "What does the audit trail look like?"

- Who did what when
- What was the state before/after
- Why was this action taken
- Can we prove compliance
- Can we investigate later

---

## Domain-Specific Prompts

### For API Specs

- "What if the client sends an older API version?"
- "What if response size exceeds limits?"
- "What if pagination is needed?"
- "What if the API is called twice rapidly?"

### For CLI Tool Specs

- "What if run from wrong directory?"
- "What if environment variables missing?"
- "What if output is piped vs TTY?"
- "What if user Ctrl+C mid-execution?"

### For Workflow Specs

- "What if user skips a required step?"
- "What if workflow is interrupted and resumed?"
- "What if two users run simultaneously?"
- "What if approval times out?"

### For Integration Specs

- "What if the other system's schema changed?"
- "What if webhook delivery fails?"
- "What if timestamps are in different zones?"
- "What if retry creates duplicates?"

### For AI/LLM Specs

- "What if the model hallucinates?"
- "What if context window exceeded?"
- "What if model refuses the request?"
- "What if RAG returns wrong context?"

---

## Iteration Structure

For each prompt, document:

```markdown
## Iteration N: [Category]

**Prompt used**: "[The question asked]"

**Scenario imagined**:
[Specific failure scenario in detail]

**What goes wrong**:
- [Specific symptom 1]
- [Specific symptom 2]

**Root cause**:
[Why this happens]

**Lesson learned**:
[What assumption was wrong or what was missing]

**Enhancement needed**:
- [ ] [Concrete spec change with details]
```

---

## Quick Iteration Guide

| Iteration | Primary Focus | Secondary Focus |
|-----------|---------------|-----------------|
| 1 | Input validation | Interface mismatch |
| 2 | Timeout/performance | Scale issues |
| 3 | Error handling | Dependency failure |
| 4 | Safety/security | Rollback need |
| 5 | User experience | User behavior |
| 6 | Integration | Environment difference |
| 7 | State management | Partial failure |
| 8 | Documentation | Debugging scenario |
| 9 | Tooling/CLI | Repetition |
| 10 | Operational | Audit & compliance |

---

## Severity Assessment

After each iteration, assess:

| Question | If Yes → |
|----------|----------|
| Blocks basic functionality? | Critical |
| Causes data loss? | Critical |
| Significant UX degradation? | Important |
| Hard to debug/fix later? | Important |
| Would be nice to have? | Nice-to-have |

---

## Output Summary Template

After all iterations:

```markdown
# Simulation Summary

**Spec**: [Name]
**Iterations**: 10
**Date**: YYYY-MM-DD

## Findings by Severity

### Critical (4)
1. Iteration 1: [Brief description]
2. Iteration 4: [Brief description]
3. Iteration 6: [Brief description]
4. Iteration 7: [Brief description]

### Important (3)
5. Iteration 2: [Brief description]
6. Iteration 5: [Brief description]
7. Iteration 9: [Brief description]

### Nice-to-Have (3)
8. Iteration 3: [Brief description]
9. Iteration 8: [Brief description]
10. Iteration 10: [Brief description]

## Top Enhancements Needed

1. [Most impactful enhancement]
2. [Second most impactful]
3. [Third most impactful]

## Questions to Answer Before Implementation

1. [Question from simulation that needs real answer]
2. [Another question]
```
