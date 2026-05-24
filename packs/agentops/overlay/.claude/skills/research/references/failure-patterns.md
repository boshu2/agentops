# The 12 Failure Patterns (Research Reference)

> Based on the Vibe-Coding methodology. Load this when you need full pattern details for risk assessment.

---

## Quick Reference

| # | Pattern | Key Symptom | First Action |
|---|---------|-------------|--------------|
| 1 | Fix Spiral | >3 attempts, circles | STOP, revert |
| 2 | Confident Hallucination | Non-existent APIs | Verify docs |
| 3 | Context Amnesia | Forgotten constraints | Save state |
| 4 | Tests Passing Lie | Green but broken | Manual test |
| 5 | Eldritch Horror | >200 line functions | Extract/refactor |
| 6 | Silent Deletion | Missing code | Check git history |
| 7 | Zombie Resurrection | Bugs return | Add regression test |
| 8 | Gold Plating | Unrequested features | Revert extras |
| 9 | Cargo Cult | Copied patterns | Understand why |
| 10 | Premature Abstraction | Generic w/ one use | Inline |
| 11 | Security Theater | Bypassable security | Audit |
| 12 | Documentation Mirage | Docs don't work | Test docs |

---

## Inner Loop Patterns (Seconds-Minutes)

### 1. The Fix Spiral

**Description:** Making a fix that breaks something else, then fixing that break which causes another issue, creating a cascading chain without resolution.

**Symptoms:**
- More than 3 fix attempts without convergence
- Changes oscillating between two states
- "This should work" appearing in explanations
- Error messages changing but not disappearing

**Research Defense:**
- Research root cause BEFORE attempting fix
- Document expected behavior vs actual behavior
- Identify all code paths affected

**Prevention:**
- Set hard limit: 3 attempts then STOP
- State explicit prediction before each fix
- Checkpoint working state before each attempt

---

### 2. The Confident Hallucination

**Description:** Generating plausible-sounding but factually incorrect information about APIs, libraries, or behavior.

**Symptoms:**
- Code references non-existent methods or parameters
- API usage that "looks right" but fails at runtime
- Overly specific technical claims without evidence
- Version-specific features applied to wrong versions

**Research Defense:**
- VERIFY all API claims against actual documentation
- Note confidence levels in provenance table
- Use Tier 6 (external docs) for unfamiliar APIs

**Prevention:**
- Test code in isolation before integration
- Use "I don't know" as valid response
- Run type checkers and linters early

---

### 3. The Context Amnesia

**Description:** As context window fills, losing track of earlier constraints, requirements, or decisions.

**Symptoms:**
- Reintroducing previously fixed bugs
- Contradicting earlier decisions
- Forgetting project-specific conventions
- Repeating completed work

**Research Defense:**
- Stay <40% context utilization
- Write findings to files immediately
- Use targeted reads (offset/limit) not full files

**Prevention:**
- Save progress frequently
- Start fresh sessions for distinct work
- Front-load critical constraints

---

### 4. The Tests Passing Lie

**Description:** Tests pass but code doesn't actually work - too narrow, wrong thing, mocks away behavior.

**Symptoms:**
- Green test suite but broken functionality
- Tests that test mocks instead of real behavior
- Coverage looks good but edge cases fail
- Tests modified in same PR as code they test

**Research Defense:**
- Find actual test coverage in research
- Identify what tests actually verify
- Note mocked vs real dependencies

**Prevention:**
- Run tests yourself; don't trust reported results
- Separate test changes from code changes
- Manual smoke test after suite passes

---

## Middle Loop Patterns (Hours-Days)

### 5. The Eldritch Horror

**Description:** Code becomes incomprehensible - functions spanning hundreds of lines, deeply nested logic, unclear naming.

**Symptoms:**
- Functions exceeding 200 lines
- Nesting depth beyond 4 levels
- Variable names like `temp2`, `data3`
- Comments that don't match behavior

**Research Defense:**
- Document complexity limits in findings
- Note current complexity metrics
- Identify refactoring boundaries

**Prevention:**
- Enforce hard limits: <200 lines per function
- Require meaningful names
- Use explicit interfaces

---

### 6. The Silent Deletion

**Description:** Removing code that appears unused but is actually necessary for edge cases, legacy support, or fallbacks.

**Symptoms:**
- "Cleanup" commits that remove "dead code"
- Features that worked yesterday now fail
- Error handling mysteriously missing
- Comments about "why" deleted along with code

**Research Defense:**
- Research WHY code exists before removal
- Check git history for context
- Trace all references including dynamic calls

**Prevention:**
- Never delete without understanding purpose
- Get human approval for deletion
- Keep deleted code in comments initially

---

### 7. The Zombie Resurrection

**Description:** Previously fixed bugs return because similar code regenerated without fix, or reverts during refactoring.

**Symptoms:**
- Bug reports for issues marked "fixed"
- Same error in different code paths
- Fixes lost during refactoring
- "I thought we fixed this" conversations

**Research Defense:**
- Prior art search prevents re-solving
- Check for existing regression tests
- Document root cause, not just fix

**Prevention:**
- Add regression tests for every fix
- Use automated checks for anti-patterns
- Keep lessons learned file

---

### 8. The Gold Plating

**Description:** Adding unrequested features, extra error handling, additional configurability beyond what was asked.

**Symptoms:**
- PR larger than expected
- New config options no one asked for
- "While I was here, I also..." explanations
- Abstraction layers for single use cases

**Research Defense:**
- Define explicit scope in research
- Note ONLY what's needed for the task
- Separate "nice to have" from "required"

**Prevention:**
- Define explicit scope before starting
- Reject changes outside stated scope
- Prefer boring, obvious solutions

---

## Outer Loop Patterns (Days-Weeks)

### 9. The Cargo Cult

**Description:** Copying patterns from examples without understanding why they work. May be inappropriate for context.

**Symptoms:**
- Copy-pasted code with irrelevant portions
- Patterns from different frameworks mixed
- "Best practices" where they don't fit
- Configuration copied without understanding

**Research Defense:**
- Understand WHY patterns exist
- Ask "why does this pattern exist?" for each
- Verify example matches your context

**Prevention:**
- Test copied code in isolation first
- Adapt patterns to local conventions
- Trace examples to their source

---

### 10. The Premature Abstraction

**Description:** Creating generic abstractions before concrete use cases exist. Abstractions don't match actual needs.

**Symptoms:**
- Generic interfaces with one implementation
- Factory patterns for single classes
- Configuration for cases that don't exist
- "Future-proofing" never used

**Research Defense:**
- Document concrete use cases first
- Require 3+ concrete cases before abstracting
- Note where duplication exists vs speculation

**Prevention:**
- Write concrete implementations first
- Prefer duplication over wrong abstraction
- Extract only when duplication appears

---

### 11. The Security Theater

**Description:** Code appears secure but isn't - validation that misses edge cases, encryption with hardcoded keys.

**Symptoms:**
- Security measures easily circumvented
- Validation on client but not server
- Hardcoded credentials or keys
- "Security by obscurity" approaches

**Research Defense:**
- Include security constraints in research
- Reference external security standards
- Note auth/crypto/access control patterns

**Prevention:**
- Use established security libraries
- Security review by qualified humans
- Static analysis for vulnerabilities

---

### 12. The Documentation Mirage

**Description:** Documentation exists but doesn't match reality - outdated READMEs, incorrect API docs.

**Symptoms:**
- Following docs leads to errors
- Comments contradict adjacent code
- Examples that don't compile
- Setup instructions that don't work

**Research Defense:**
- Verify docs match reality
- Test documentation by following it literally
- Note discrepancies in research findings

**Prevention:**
- Treat docs as code: test them
- Update docs in same PR as code
- Use executable documentation

---

## Pattern Frequency Tracking

Use this in research outputs to track which patterns are relevant:

```markdown
## Failure Pattern Risks

| Pattern | Risk Level | Mitigation |
|---------|------------|------------|
| #2 Confident Hallucination | HIGH | Verify external API claims |
| #5 Eldritch Horror | MEDIUM | Keep functions <200 lines |
| #9 Cargo Cult | MEDIUM | Understand why patterns exist |
```

Risk Levels:
- **HIGH**: Strong indicators in research, requires explicit mitigation
- **MEDIUM**: Some indicators, requires awareness
- **LOW**: Minor indicators, standard practices sufficient

---

## See Also

- `~/.claude/CLAUDE-base.md` - Core Vibe-Coding methodology
- `~/.claude/plugins/marketplaces/agentops-marketplace/reference/failure-patterns.md` - Full pattern reference
- `~/.claude/skills/crank/failure-taxonomy.md` - Execution failure taxonomy
