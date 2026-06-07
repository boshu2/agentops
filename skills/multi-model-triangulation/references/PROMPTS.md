# Triangulation Prompts Library

Copy-paste ready prompts for different triangulation scenarios.

## Table of Contents
- [Idea Evaluation](#idea-evaluation)
- [Code Review](#code-review)
- [Architecture Decision](#architecture-decision)
- [Debugging](#debugging)
- [Risk Assessment](#risk-assessment)
- [Performance Analysis](#performance-analysis)
- [Security Review](#security-review)

---

## Idea Evaluation

### Basic Idea Scoring

```
# COPY TO [Codex/Gemini/Grok]:

Evaluate these ideas for [PROJECT_NAME]. Score each 1-10 on:
- **Quality**: Novel and valuable?
- **Utility**: Solves real problem?
- **Feasibility**: Effort vs payoff?
- **Risk**: What could go wrong?

Ideas to evaluate:
1. [IDEA 1]
2. [IDEA 2]
3. [IDEA 3]

For each:
1. Scores (Q/U/F/R out of 10)
2. One-sentence rationale
3. Final rank best→worst

Be critical. Don't agree with everything.
```

### Feature Prioritization

```
# COPY TO [Codex/Gemini/Grok]:

I'm prioritizing features for [PROJECT]. Help me evaluate:

Features:
1. [FEATURE 1] - [brief description]
2. [FEATURE 2] - [brief description]
3. [FEATURE 3] - [brief description]

For each, assess:
- User impact (1-10): How much does this improve UX?
- Technical complexity (1-10): How hard to build?
- Strategic value (1-10): Alignment with product direction?
- Dependencies: What must exist first?

Recommend top 2 to build first with reasoning.
```

---

## Code Review

### Bug/Security Hunt

```
# COPY TO [Codex/Gemini/Grok]:

Review this code for bugs, security issues, and improvements:

```[LANGUAGE]
[PASTE CODE HERE]
```

Categorize findings:
1. **Critical** (must fix): Security vulnerabilities, data corruption risks
2. **Important** (should fix): Logic errors, edge cases, performance issues
3. **Suggestions** (nice to have): Style, readability, minor optimizations

For each issue:
- Location (line/function)
- Problem description
- Suggested fix

Overall assessment: X/10

Be thorough. Look for edge cases, error handling, injection vulnerabilities.
```

### Refactoring Evaluation

```
# COPY TO [Codex/Gemini/Grok]:

I'm considering refactoring this code:

**Before:**
```[LANGUAGE]
[ORIGINAL CODE]
```

**Proposed After:**
```[LANGUAGE]
[REFACTORED CODE]
```

Evaluate:
1. Is the refactoring worth it? (effort vs improvement)
2. Any bugs introduced?
3. Performance implications?
4. Maintainability: better or worse?
5. Alternative approaches I should consider?

Be honest if the original is fine.
```

---

## Architecture Decision

### Technology Choice

```
# COPY TO [Codex/Gemini/Grok]:

I'm choosing between these options for [PROBLEM]:

**Option A: [NAME]**
[Description, key characteristics]

**Option B: [NAME]**
[Description, key characteristics]

**Option C: [NAME]**
[Description, key characteristics]

Context:
- Team size: [N]
- Timeline: [X months]
- Scale: [expected load/size]
- Constraints: [budget, existing stack, etc.]

Evaluate each on:
- Complexity (simple ↔ complex)
- Maintainability (easy ↔ hard)
- Performance (fast ↔ slow)
- Scalability (scales well ↔ doesn't)
- Learning curve for team
- Long-term vendor/community health

Recommend ONE with clear reasoning. Be opinionated.
```

### System Design Review

```
# COPY TO [Codex/Gemini/Grok]:

Review this system design:

[PASTE DIAGRAM OR DESCRIPTION]

Evaluate:
1. Does it solve the stated problem?
2. Single points of failure?
3. Scalability bottlenecks?
4. Security concerns?
5. Operational complexity?
6. Cost implications?

Suggest specific improvements. Don't just validate—challenge assumptions.
```

---

## Debugging

### Root Cause Analysis

```
# COPY TO [Codex/Gemini/Grok]:

Help debug this issue:

**Symptom:** [What's happening]

**Expected:** [What should happen]

**Context:**
- Environment: [prod/staging/local]
- Recent changes: [deployments, config changes]
- Frequency: [always, intermittent, specific conditions]

**Error/Logs:**
```
[PASTE RELEVANT LOGS]
```

**Code in question:**
```[LANGUAGE]
[RELEVANT CODE]
```

Provide:
1. Top 3 most likely root causes (ranked by probability)
2. For each: diagnostic steps to confirm/rule out
3. Quick fix vs proper fix for most likely cause
```

---

## Risk Assessment

### Project Risk Evaluation

```
# COPY TO [Codex/Gemini/Grok]:

Evaluate risks for this project:

**Project:** [NAME]
**Goal:** [What we're trying to achieve]
**Timeline:** [Deadline]
**Team:** [Size, experience level]

**Planned approach:**
[Description of how we plan to do it]

Identify:
1. **Technical risks**: What could fail technically?
2. **Schedule risks**: What could cause delays?
3. **Resource risks**: Team, budget, dependencies?
4. **Scope risks**: Creep, unclear requirements?

For each risk:
- Probability (high/medium/low)
- Impact (high/medium/low)
- Mitigation strategy

What's the ONE risk I should be most worried about?
```

---

## Performance Analysis

### Optimization Review

```
# COPY TO [Codex/Gemini/Grok]:

Review this code/query for performance:

```[LANGUAGE/SQL]
[PASTE CODE]
```

Context:
- Data size: [rows, objects, etc.]
- Current performance: [time, memory]
- Target: [desired performance]

Analyze:
1. Big-O complexity (time and space)
2. Obvious bottlenecks
3. Optimization opportunities (ranked by impact)
4. Trade-offs of each optimization

Suggest the ONE change with highest impact/effort ratio.
```

---

## Security Review

### Security Assessment

```
# COPY TO [Codex/Gemini/Grok]:

Security review for this [code/API/system]:

```[LANGUAGE]
[PASTE CODE OR API SPEC]
```

Check for:
1. OWASP Top 10 vulnerabilities
2. Authentication/authorization issues
3. Input validation gaps
4. Sensitive data exposure
5. Injection vulnerabilities (SQL, command, XSS)
6. Cryptographic weaknesses

For each finding:
- Severity (critical/high/medium/low)
- Exploitation scenario
- Remediation

What's the most dangerous issue here?
```

---

## Synthesis Template

After receiving responses from multiple models, use this template:

```markdown
## Triangulation Results: [TOPIC]

### Consensus (High Confidence)
Points where ALL models agree:
- [Point 1]
- [Point 2]

### Divergence (Investigate Further)
Points of disagreement:
| Topic | Claude | Model B | Model C |
|-------|--------|---------|---------|
| [X] | [view] | [view] | [view] |

**Why they differ:** [Analysis]

### Unique Insights
- **Claude:** [unique perspective]
- **Model B:** [unique perspective]
- **Model C:** [unique perspective]

### Synthesized Recommendation
[Final recommendation incorporating all perspectives]

### Confidence Level
[High/Medium/Low] based on consensus degree

### Next Steps
1. [Action based on synthesis]
2. [Follow-up investigation if needed]
```
