# Council FAIL Patterns

> Compiled from 946 council verdicts across 14,753 production sessions. 124 FAIL verdicts analyzed for root causes.
> The #1 failure mode is missing mechanical verification — plans that rely on human vigilance instead of automated gates.

## Top 5 Failure Patterns (by frequency)

### 1. Missing Mechanical Verification (38% of FAILs)

Plans assume correctness through convention rather than enforcement.

**Signals:**
- Acceptance criteria say "verify" or "confirm" without a runnable command
- Integration points lack automated conformance checks
- Configuration boundaries have no consistency assertion
- Retry/fallback paths silently diverge from primary paths

**Pre-mortem check:** For each issue, can I run a command that returns 0 (pass) or non-zero (fail)? If not, the acceptance criteria are incomplete.

**Example:** ArgoCD CMP timeout mismatch — three layers (CMP, repo-server, application) with independent timeouts. No test verified alignment. Cache poisoning was invisible for weeks.

---

### 2. Self-Assessment Instead of External Gates (22% of FAILs)

Workers or agents declare their own work complete without independent validation.

**Signals:**
- Issues closed by the same person/agent who implemented them
- "It works on my machine" as the only validation
- Unit tests pass but no integration/E2E coverage
- Manual QA as the sole acceptance gate

**Pre-mortem check:** Is there a validation step performed by a different agent, tool, or process than the implementer?

**Example:** Unit tests found zero production bugs across all analyzed sessions. L3+ tests (integration, E2E) found all real bugs. Self-grading is confirmation bias.

---

### 3. Context Rot and Hallucination (15% of FAILs)

Long sessions or compacted contexts produce incorrect assumptions treated as facts.

**Signals:**
- Session context above 40% causes quality degradation
- Session context above 60% causes 99% information loss
- Multi-phase work in a single session (research -> plan -> implement)
- Learnings or references not verified against current code

**Pre-mortem check:** Does the plan enforce fresh sessions at phase boundaries? Are knowledge artifacts verified before citation?

**Example:** 7 hallucination-contaminated learning files found after a TDD sprint — forensic retro caught 23% baseline hallucination rate.

---

### 4. Propagation Surface Blindness (14% of FAILs)

Changes to shared abstractions (namespaces, directories, CLI surfaces) miss downstream consumers.

**Signals:**
- Renaming or restructuring without full surface enumeration
- Changes to Go source without checking: tests, embedded hooks, external hooks, SKILL.md, docs, scripts
- CLI flag changes without regenerating docs
- Skill directory changes without syncing counts

**Pre-mortem check:** For each structural change, is the full propagation surface enumerated? (Go source, tests, hooks, skills, docs, scripts, CI)

**Example:** CLI namespace restructuring touched 182 files. Missed files caused silent breakage in hooks and embedded skills.

---

### 5. Plan Oscillation (11% of FAILs)

Direction reverses mid-execution, doubling the mechanical cost of propagation.

**Signals:**
- "Create X" in one wave, "flatten X" in a later wave
- Architecture decisions reconsidered after propagation work begins
- Multiple pivots without shipping code between them (Olympus pattern: 5 pivots, zero code)

**Pre-mortem check:** Has the architectural direction been validated (via council or user confirmation) BEFORE propagation work begins?

**Example:** Namespace flattening reversed a prior namespace creation — opposite direction, double the file changes, identical propagation surface.

---

## Pre-Mortem Integration

When running pre-mortem validation, each judge should evaluate the plan against these 5 patterns. Add to the judge prompt:

> Review this plan for the top 5 council FAIL patterns:
> 1. Missing mechanical verification — are all gates automated?
> 2. Self-assessment — is validation external to the implementer?
> 3. Context rot — are phase boundaries enforced with fresh sessions?
> 4. Propagation blindness — is the full change surface enumerated?
> 5. Plan oscillation — is direction validated before propagation?

## Severity Calibration

| Pattern | Blast Radius | Detection Difficulty | Recommended Gate |
|---------|-------------|---------------------|-----------------|
| Missing mechanical verification | HIGH | LOW (grep for "verify manually") | Rewrite acceptance criteria as commands |
| Self-assessment | HIGH | MEDIUM | Add external validator step |
| Context rot | MEDIUM | HIGH (invisible until too late) | Enforce fresh sessions per phase |
| Propagation blindness | HIGH | MEDIUM | Enumerate surface pre-implementation |
| Plan oscillation | HIGH | LOW (visible in plan diff) | Council validate direction first |
