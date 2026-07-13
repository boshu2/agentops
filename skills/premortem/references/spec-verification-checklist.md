# Spec Verification Checklist

Use this checklist to verify spec completeness before implementation.

## Mandatory Items

Every spec MUST have answers to these questions:

### 1. Interface Definition
- [ ] Input format defined (schema/types)
- [ ] Output format defined
- [ ] Error response format defined
- [ ] API versioning strategy

### 2. Error Handling
- [ ] What errors can occur?
- [ ] How is each error communicated?
- [ ] What should user do for each error?
- [ ] Retry logic (if applicable)

### 3. Timing
- [ ] Timeout values specified
- [ ] Rate limits (if applicable)
- [ ] Expected latency bounds
- [ ] What happens on timeout?

### 4. Safety
- [ ] Destructive operations require confirmation
- [ ] Rollback procedure documented
- [ ] Data backup strategy (if applicable)
- [ ] Permission requirements

### 5. Dependencies
- [ ] External services listed
- [ ] Version requirements
- [ ] Fallback behavior if dependency unavailable
- [ ] Authentication/authorization requirements

### 6. State Management
- [ ] Initial state defined
- [ ] State transitions listed
- [ ] How to recover from inconsistent state
- [ ] Cleanup procedures

### 7. Wire Input Validation
- [ ] Enum/classification fields validated against explicit allowlist on parse
- [ ] Invalid wire values handled (reclassify or reject, never trust blindly)
- [ ] Contradictory field combinations normalized (e.g., `is_error=false` + `error_class=timeout`)
- [ ] Default/fallback case is semantically meaningful (not just "unknown")

### 8. Struct Contract Completeness
- [ ] Every code path that creates a struct populates ALL fields (grep `StructName{`)
- [ ] Synthesized/summary instances use tracked metadata, not zero values
- [ ] Index fields refer to original input positions when data is sorted internally
- [ ] Structural sweep test exists: iterates all outputs, asserts required fields non-zero

### 9. Classification Taxonomy
- [ ] Pattern matching uses contextual co-occurrence, not bare substrings for numbers/keywords
- [ ] 5+ realistic false-positive inputs tested against each pattern
- [ ] Default case is distinct from empty/malformed (e.g., `execution_error` vs `unknown`)
- [ ] Tests assert exact expected class, never just `!= wrong_class`

## Verification Template

| Category | Checklist Item | Present? | Location | Notes |
|----------|----------------|----------|----------|-------|
| Interface | Input schema | yes/no | line N | |
| Interface | Output schema | yes/no | line N | |
| Interface | Error format | yes/no | line N | |
| Error | Error list | yes/no | line N | |
| Error | Recovery steps | yes/no | line N | |
| Timing | Timeouts | yes/no | line N | |
| Timing | Rate limits | yes/no | line N | |
| Safety | Rollback | yes/no | line N | |
| Safety | Confirmation | yes/no | line N | |
| Deps | Dep list | yes/no | line N | |
| Deps | Fallbacks | yes/no | line N | |
| State | Initial state | yes/no | line N | |
| State | Transitions | yes/no | line N | |
| Wire | Enum allowlist | yes/no | line N | |
| Wire | Invalid value handling | yes/no | line N | |
| Wire | Contradictory combos | yes/no | line N | |
| Struct | All paths populate fields | yes/no | line N | |
| Struct | Synthesized instances | yes/no | line N | |
| Struct | Sweep test exists | yes/no | line N | |
| Classify | Contextual patterns | yes/no | line N | |
| Classify | False-positive tests | yes/no | line N | |
| Classify | Exact class assertions | yes/no | line N | |

## Gap Severity

| Missing Item | Severity | Rationale |
|--------------|----------|-----------|
| Rollback procedure | CRITICAL | Can't recover from failures |
| Error handling | CRITICAL | Users stranded on errors |
| Input validation | HIGH | Security and reliability risk |
| Timeouts | HIGH | Can hang indefinitely |
| Dependencies | HIGH | Silent failures when deps unavailable |
| Rate limits | MEDIUM | Performance issues at scale |
| Cleanup procedures | MEDIUM | Resource leaks |
| Wire enum validation | HIGH | Untrusted input bypasses classification |
| Struct field completeness | HIGH | Inconsistent contract for consumers |
| Classification patterns | HIGH | False positives cause wrong retry/escalation behavior |
| Exact test assertions | MEDIUM | Regression detection fails silently |
| Version strategy | LOW | Future compatibility |

## Quick Reference

**Minimum Viable Spec** must have:
1. Input/output schema (what goes in, what comes out)
2. Error handling (what can go wrong, what user does)
3. Rollback procedure (how to undo)
4. Dependencies (what this needs to work)

If any of these 4 are missing → CRITICAL gap, spec is not ready for implementation.
