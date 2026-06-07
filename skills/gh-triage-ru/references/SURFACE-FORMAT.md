# Surface Formats

When to surface an issue/PR to the user for judgment, and how to format the request.

---

## When to Surface

| Situation | Surface? | Reason |
|-----------|----------|--------|
| Simple bug with clear fix | No | Just fix it |
| Feature that fits scope | No | Just implement |
| Complex feature request | **Yes** | Scope implications |
| Interesting PR ideas | **Yes** | Need approval to integrate |
| Ambiguous intended behavior | **Yes** | Could break things |
| Security-related | **Yes** | High stakes |
| Breaking changes | **Yes** | User impact |
| Multiple valid approaches | **Yes** | Design decision |

---

## Format: Feature Request

```
FEATURE REQUEST REQUIRING JUDGMENT: owner/repo#42

**Request:** "[Title from issue]"

**User's Problem:** [What they're trying to accomplish]

**Analysis:**
+ [Benefit 1]
+ [Benefit 2]
- [Drawback 1]
- [Drawback 2]
- Scope Risk: [What this might lead to]

**Implementation Options:**
1. **Full implementation**: [What it would take, LOC estimate]
2. **Simplified version**: [Reduced scope alternative]
3. **Decline**: [Polite response]

**My recommendation:** Option [X] because [reasoning]

**If approved:** I'll implement and close with: "[proposed response]"
```

---

## Format: Bug with Unclear Resolution

```
BUG REQUIRING JUDGMENT: owner/repo#42

**Report:** "[Title]"

**User claims:** [What they say is broken]

**My findings:** [What I found during verification]

**Uncertainty:**
- [ ] Is this actually a bug or intended behavior?
- [ ] If bug, what's the correct behavior?
- [ ] Could fixing this break other use cases?

**Options:**
1. Fix as user requests: [implications]
2. Fix differently: [alternative approach]
3. Document as intended: [explanation]

**My recommendation:** [X] because [reasoning]
```

---

## Format: PR with Good Ideas

```
PR WITH POTENTIALLY USEFUL IDEAS: owner/repo#42

**PR Title:** "[Title]"
**Author's intent:** [What they're trying to achieve]

**The diff shows:** [Summary of changes]

**My assessment:**
+ [Good idea 1]
+ [Good idea 2]
- [Concern 1]
- [Concern 2]

**Options:**
1. Implement similar approach independently
2. Take partial idea (just X, not Y)
3. Note for future, close for now
4. Decline entirely (current behavior intentional)

**My recommendation:** [X] because [reasoning]

**Note:** Will close PR regardless—just need guidance on whether to implement.
```

---

## Format: Security-Related

```
SECURITY-RELATED ITEM: owner/repo#42

**Report:** "[Title]"

**Potential impact:** [What could go wrong]

**My analysis:**
- Exploitability: [Low/Medium/High]
- Impact if exploited: [What happens]
- Current mitigations: [What's already in place]

**Proposed fix:** [Technical approach]

**Questions:**
1. Is this worth addressing given impact level?
2. Should we disclose timeline or keep private?
3. Is the proposed fix appropriate?

**Urgency:** [Low/Medium/High/Critical]
```

---

## Format: Breaking Change Consideration

```
POTENTIAL BREAKING CHANGE: owner/repo#42

**Request:** "[Title]"

**Current behavior:** [How it works now]
**Proposed behavior:** [How it would change]

**Who's affected:**
- Users doing X: [impact]
- Users doing Y: [impact]
- Scripts/automation: [impact]

**Migration path:**
- [ ] Can we deprecate first?
- [ ] Is there a flag for gradual rollout?
- [ ] How would users update their usage?

**Options:**
1. Break now (simpler, rip the bandaid)
2. Deprecation period (add warning, break later)
3. Add new feature, keep old (complexity)
4. Decline (not worth disruption)

**My recommendation:** [X] because [reasoning]
```

---

## Response After Decision

Once you've received guidance:

```bash
# If implementing
gh issue close N -R owner/repo -c "[Brief summary of decision and fix]

Fixed in abc123."

# If declining
gh issue close N -R owner/repo -c "[Use appropriate decline template]"

# If needs more info
gh issue comment N -R owner/repo -b "[Use appropriate request-info template]"
```
