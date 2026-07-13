# Failure Taxonomy for Spec Validation

Comprehensive catalog of failure modes to check during premortem validation.

---

## How to Use This Taxonomy

When running premortem, use each category as a checklist item:

1. For each category (Interface Mismatch, Timing, Error Handling, etc.)
2. Ask the Detection Question against the spec
3. If the answer is "no" or "unclear", that's a GAP
4. Apply the Enhancement Pattern to fix it

The taxonomy covers 12 categories. Minimum viable premortem covers at least:
- **Interface Mismatch** - API/schema defined?
- **Error Handling** - Error states defined?
- **Safety** - Rollback defined?
- **Integration** - Dependencies defined?

For comprehensive validation, walk through all 10 categories. See `enhancement-patterns.md` for how to fix gaps.

---

## Category 1: Interface Mismatch

**Description**: What the spec says vs what the system actually does.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Wrong JSON schema | "What does the actual output look like?" | Extract schema from code |
| Missing fields | "What fields are we assuming exist?" | Document all expected fields |
| Different types | "Is this a string or enum?" | Add type constraints |
| Versioning issues | "What if API version changes?" | Add version handling |

**Simulation Prompt**: "What if I actually run this command right now and compare output to spec?"

---

## Category 2: Timing & Performance

**Description**: Operations take longer or behave differently under load.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Timeout | "What if this takes 10x longer?" | Per-operation timeouts |
| Race condition | "What if two requests overlap?" | Add locking/ordering |
| Resource exhaustion | "What if we hit rate limits?" | Add backoff/retry |
| Cascading delays | "What if dependency is slow?" | Add circuit breakers |

**Simulation Prompt**: "What happens if I run this during peak load with degraded network?"

---

## Category 3: Error Handling

**Description**: What happens when things go wrong.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Unclear error message | "Can user understand this?" | Add actionable messages |
| Missing recovery | "What does user do after error?" | Add recovery steps |
| Silent failure | "How do we know this failed?" | Add explicit error states |
| Partial failure | "What if step 3 of 5 fails?" | Add checkpoint/resume |

**Simulation Prompt**: "What if every external call fails? What does the user see?"

---

## Category 4: Safety & Security

**Description**: Dangerous operations without adequate protection.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Missing confirmation | "Can this delete prod data?" | Add explicit confirm gate |
| Unclear severity | "Does user know this is dangerous?" | Add visual safety levels |
| No rollback | "What if we need to undo?" | Document rollback procedure |
| Privilege escalation | "Can this exceed permissions?" | Add permission checks |

**Simulation Prompt**: "What's the worst thing a user could do by accident?"

---

## Category 5: User Experience

**Description**: How users interact with the system.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Skipped instructions | "What if user doesn't read?" | Put warnings before actions |
| Confusing flow | "Is the next step obvious?" | Add explicit next actions |
| Missing feedback | "Does user know it's working?" | Add progress indicators |
| Information overload | "Is this scannable?" | Limit to 2-3 sentences |

**Simulation Prompt**: "What if the user is stressed and just wants to copy-paste?"

---

## Category 6: Integration Points

**Description**: Interactions with external systems.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Dependency unavailable | "What if API is down?" | Add fallback behavior |
| Changed behavior | "What if upstream updates?" | Version pin dependencies |
| Auth failure | "What if token expires?" | Add re-auth flow |
| Data format change | "What if schema evolves?" | Add schema validation |

**Simulation Prompt**: "What if every external system is having a bad day?"

---

## Category 7: State Management

**Description**: Keeping track of where we are.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Lost state | "What if session ends mid-operation?" | Add checkpointing |
| Inconsistent state | "What if DB and cache differ?" | Add reconciliation |
| Stale state | "What if data changed since read?" | Add refresh/optimistic locking |
| Orphaned resources | "What if create succeeds but record fails?" | Add cleanup procedures |

**Simulation Prompt**: "What if power goes out halfway through?"

---

## Category 8: Documentation Gap

**Description**: Spec doesn't match reality.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Outdated example | "Does this actually work?" | Test all examples |
| Missing prerequisite | "What else needs to be true?" | Document prerequisites |
| Implicit assumption | "What am I assuming is already done?" | Make assumptions explicit |
| Wrong version | "Does this work on current version?" | Add version requirements |

**Simulation Prompt**: "Could a new team member follow this spec from scratch?"

---

## Category 9: Tooling & CLI

**Description**: Command-line and tool behavior.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Different flags | "Are these the actual flags?" | Verify against --help |
| Path issues | "What if running from different dir?" | Use absolute paths |
| Missing tools | "Is this tool installed?" | Add tool prerequisites |
| Output format varies | "Is output consistent?" | Parse defensively |

**Simulation Prompt**: "What if I run this on a fresh machine?"

---

## Category 10: Operational

**Description**: Running in production.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| No audit trail | "Can we investigate later?" | Add structured logging |
| Missing metrics | "How do we know it's healthy?" | Add observability |
| No runbook | "What do we do at 2 AM?" | Add troubleshooting guide |
| Unclear ownership | "Who gets paged?" | Add escalation path |

**Simulation Prompt**: "What if this breaks on Sunday at 3 AM?"

---

## Category 11: Classification & Pattern Matching

**Description**: When code classifies inputs into categories (error types, event kinds, status codes), the taxonomy and matching patterns are a common source of subtle bugs.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| False-positive patterns | "Can a benign input match this pattern?" (e.g., port `4290` matching HTTP `429`) | Require contextual co-occurrence: bare substring matches for numbers/keywords need surrounding context (HTTP prefix, error suffix) |
| Incomplete taxonomy | "What falls through to the default case?" List 5 real-world inputs that would hit `default`. | Expand patterns or make the default case semantically meaningful (e.g., `execution_error` vs `unknown`) |
| Overly broad keyword | "Does `sandbox` always mean sandbox violation? Or could `sandbox startup failed` be an execution error?" | Require compound matches: keyword + policy phrase (denied, violation, not permitted) |
| Untrusted wire enums | "What if the JSON says `error_class: bogus`?" | Validate against an explicit allowlist; reclassify invalid values from content |
| Impossible state combinations | "Can `is_error=false` coexist with `error_class=timeout`?" | Normalize after parse: clear contradictory fields |
| Weak test assertions | "Does the test assert `!= wrong_class` or `== expected_class`?" | Always assert exact expected value. `!= X` silently passes when result drifts to a third wrong value. |

**Simulation Prompt**: "Generate 10 realistic error messages from production. How many match the wrong pattern? How many fall through to default?"

---

## Category 12: Struct & Contract Completeness

**Description**: When structs gain new fields, not all code paths populate them — creating inconsistent contracts for consumers.

| Failure Mode | Detection Question | Enhancement Pattern |
|--------------|-------------------|---------------------|
| Partial field population | "Does every code path that creates this struct set ALL fields?" | Grep `StructName{` across package; verify each literal sets new fields |
| Synthesized instances skip fields | "Are end-of-batch/summary instances populated from tracked metadata, or do they use zero values?" | Store provenance (last-seen timestamp, agent, index) alongside state; use it for synthesized instances |
| Index after sort | "If events are sorted, does `EventIndex` refer to the original position or the sorted position?" | Wrap with original index before sorting; emit original index |
| No structural sweep test | "Is there a test that iterates ALL output instances and asserts non-zero for required fields?" | Add invariant test: for all outputs, assert required fields are populated |

**Simulation Prompt**: "Add a new field to this struct. Which code paths would produce a zero-value? Would any consumer break?"

---

## Quick Reference

During validation, for each category:

| Step | Action |
|------|--------|
| 1 | Ask the Detection Question against the spec |
| 2 | Answer: yes (present), no (missing), partial (incomplete) |
| 3 | If missing/partial: log as GAP with line number |
| 4 | Apply Enhancement Pattern from `enhancement-patterns.md` |

Not every category will yield findings for every spec. Focus on categories relevant to your spec's domain.
