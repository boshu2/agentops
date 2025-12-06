---
description: Phase 1 - Research deeply before planning (Outer→Middle Loop)
---

# /research - Phase 1: Gather & Understand

**Purpose:** Conduct deep research before planning. This is Outer/Middle Loop work.

**Why this matters:** Research is 80% of cognitive work. If you're shouting at Claude during implementation, your research was inadequate. Thorough research prevents wasted effort and wrong directions.

**Loop:** Primarily Outer Loop (wks-mos decisions), touches Middle Loop (hrs-days exploration)

**Token budget:** 40-60k tokens (20-30% of context)

**Output:** research.md bundle → fresh session → `/plan`

---

## Opus 4.5 Behavioral Standards

<investigate_before_planning>
Before designing solutions, thoroughly explore the codebase and problem space. Read relevant files, understand existing patterns, and identify constraints. Do not speculate about code you have not opened.
</investigate_before_planning>

<use_parallel_tool_calls>
When gathering context, read multiple related files in parallel. If exploring authentication, read auth middleware, user model, and routes simultaneously rather than sequentially.
</use_parallel_tool_calls>

<avoid_confirmation_bias>
Actively seek evidence against your initial hypothesis. Explore alternatives even when the first approach seems obvious. Document why alternatives were rejected.
</avoid_confirmation_bias>

---

## FAAFO Alignment

| Dimension | How Research Delivers |
|-----------|----------------------|
| **Fast** | Prevents rework from poor understanding |
| **Ambitious** | Enables tackling complex projects |
| **Autonomous** | Builds context for independent execution |
| **Fun** | Reduces frustration from wrong directions |
| **Optionality** | Explores multiple approaches (N×K×σ/t) |

---

## Three Loops Context

```
┌─────────────────────────────────────────────────────────┐
│              OUTER LOOP (Weeks-Months)                   │
│   ┌─────────────────────────────────────────────────┐   │
│   │          MIDDLE LOOP (Hours-Days)                │   │
│   │   ┌─────────────────────────────────────────┐   │   │
│   │   │       INNER LOOP (Sec-Min)              │   │   │
│   │   │   [Implementation happens here]          │   │   │
│   │   └─────────────────────────────────────────┘   │   │
│   │   [Planning happens here]                       │   │
│   └─────────────────────────────────────────────────┘   │
│   [RESEARCH HAPPENS HERE] ← YOU ARE HERE                │
└─────────────────────────────────────────────────────────┘
```

**Why Outer Loop:** Research shapes architecture, approach, constraints—decisions that affect weeks/months of work.

---

## When to Use

**Use /research when:**
- Task spans multiple sessions
- Multiple valid approaches exist
- Significant risk of mistakes
- Complex multi-step work
- You'd say "I need to consider this carefully"

**Skip /research when:**
- Simple, obvious implementation
- Single-session work (<2 hours)
- Low-risk changes
- Trivial fixes

---

## PDC Framework for Research

### Prevent (Before Research)

| Prevention | Action |
|------------|--------|
| **Context rot** | Fresh session for research |
| **Scope creep** | Define research boundaries upfront |
| **Analysis paralysis** | Set time/token limits |
| **Missing patterns** | Check institutional memory first |
| **Assumption blindness** | Plan tracer tests for critical assumptions |

**Pre-Research Checklist:**
- [ ] Fresh context window (<20% used)?
- [ ] Research scope clearly defined?
- [ ] Existing bundles checked (`/bundle-search`)?
- [ ] Token budget allocated (40-60k max)?
- [ ] **Tracer testing planned for infrastructure work?**

### Detect (During Research)

| Detection | Watch For |
|-----------|-----------|
| **Context amnesia** | AI forgets earlier findings |
| **Instruction drift** | Research expanding beyond scope |
| **Shallow exploration** | Missing edge cases, alternatives |
| **Confirmation bias** | Only finding evidence for first idea |

**Mid-Research Checks:**
- "What are we researching?" (test AI memory)
- "What alternatives have we NOT explored?"
- "What could go wrong with this approach?"

### Correct (After Issues)

| Issue | Correction |
|-------|------------|
| **Context degraded** | Save findings, start fresh session |
| **Wrong direction** | Pivot research focus explicitly |
| **Missing depth** | `/research --extend` on specific area |
| **Too broad** | Narrow scope, defer secondary topics |

---

## How It Works

### Step 1: Define Research Goal

**Provide:**
- What you want to understand
- Why it matters (context, urgency)
- Constraints and requirements
- What success looks like

**Good research requests:**
```
"Research how to implement Redis caching for API, considering
 10k req/sec load and <100ms latency requirement"

"Explore Postgres 12→13 migration options with zero downtime,
 current schema has 50 tables with foreign keys"

"Investigate EDB Cluster direct pattern vs Crossplane XRD
 for our pgvector use case"
```

**Poor research requests:**
```
"Research caching" (too vague)
"How to migrate databases?" (no constraints)
"What's the best architecture?" (undefined "best")
```

### Step 2: Research Execution

**I will:**

1. **Map the system** (5-10k tokens)
   - Identify relevant files, components, patterns
   - Find similar implementations in your codebase
   - Check institutional memory (bundles, git history)

2. **Explore solutions** (15-25k tokens)
   - Search for multiple approaches (Option Value: increase N)
   - Evaluate pros/cons of each
   - Find edge cases and constraints
   - Identify proven patterns

3. **Gather examples** (10-15k tokens)
   - Find code examples (your codebase, public)
   - Document implementation patterns
   - Note common gotchas and failure modes

4. **Analyze findings** (5-10k tokens)
   - Synthesize discoveries
   - Identify recommended approach
   - Document constraints and risks
   - Capture learning for institutional memory

### Step 3: Output Research Bundle

```markdown
# [Topic] Research Findings

**Type:** Research
**Created:** [Date]
**Loop:** Outer (architecture decision) / Middle (approach selection)
**Tags:** [relevant tags]

---

## Executive Summary
[1-2 sentence key finding]

## Problem Statement
[Why this research matters, constraints]

## Option Space Explored
**Option Value Context:** N=[approaches] × K=[parallel paths] × σ=[uncertainty]

### Approach A: [Name]
- **Pros:** [benefits]
- **Cons:** [drawbacks]
- **Effort:** [estimate]
- **Risk:** [PDC assessment]

### Approach B: [Name]
- **Pros:** [benefits]
- **Cons:** [drawbacks]
- **Effort:** [estimate]
- **Risk:** [PDC assessment]

## Recommended Approach
[The approach I recommend, with rationale]

## Tracer Tests Required

**Before full implementation, validate these assumptions:**

| Assumption | Tracer Test | Time | Validates |
|------------|-------------|------|-----------|
| [e.g., API version exists] | [e.g., Deploy minimal CR] | 15m | [What it proves] |
| [e.g., Auth works] | [e.g., curl test] | 10m | [What it proves] |

**Total tracer investment:** [X minutes]
**Risk if skipped:** [Hours potentially lost]

## Failure Pattern Risks
| Pattern | Risk Level | Mitigation |
|---------|------------|------------|
| [Relevant pattern] | High/Med/Low | [Prevention] |

## PDC Strategy
- **Prevent:** [What we'll do before implementation]
- **Detect:** [How we'll catch problems]
- **Correct:** [How we'll recover if needed]

## Constraints & Edge Cases
1. [Constraint/edge case]
2. [Constraint/edge case]

## File Locations (Your Codebase)
- [file:line] - Existing related code
- [file:line] - Pattern to follow

## Open Questions
- [ ] [Question for user]

## Token Stats
- Research tokens: [X]k
- Bundle tokens: [Y]k
- Compression ratio: [Z]:1
```

### Step 4: Review & Decide

**You review:**
- Does research answer the question?
- Are all approaches considered?
- Is recommendation sound?
- Are failure patterns addressed?

**Then:**
- **Approve** → Proceed to `/plan`
- **Extend** → `/research --extend "specific area"`
- **Reject** → Different research direction
- **Save** → `/bundle-save` for later

---

## Failure Pattern Awareness

**During research, watch for these patterns:**

### Inner Loop (shouldn't happen in research, but watch for)
- **Context Amnesia:** AI forgets earlier findings → re-state key constraints
- **Instruction Drift:** Research expanding beyond scope → redirect

### Middle Loop (relevant to research)
- **Memory Tattoo Decay:** Previous research outdated → verify bundle dates
- **Eldritch Horror Setup:** Research leading to over-complex solution → simplify

### Outer Loop (most relevant)
- **Bridge Torching Risk:** Research ignoring API compatibility → add as constraint
- **Stewnami Setup:** Research ignoring workspace boundaries → clarify scope

---

## Option Value in Research

**Research maximizes Option Value by:**

```
Option Value = (N × K × σ) / t

N = Number of approaches explored (increase during research)
K = Parallel paths identified (find independent modules)
σ = Uncertainty reduced (clarify unknowns)
t = Time invested (bounded by token budget)
```

**Good research:** Explores 3-5 approaches (N=5), identifies parallel work (K=3), reduces uncertainty (σ reduced), within budget (t=1 session)

**Poor research:** Single approach (N=1), sequential only (K=1), uncertainty remains (σ high), over budget (t=2+ sessions)

---

## Integration with RPI Flow

```
RESEARCH (Outer Loop) ← YOU ARE HERE
    │
    ↓ research.md bundle
    │
    ↓ [Fresh Session - Context Reset]
    │
PLAN (Middle Loop)
    │
    ↓ plan.md bundle (approved)
    │
    ↓ [Fresh Session - Context Reset]
    │
IMPLEMENT (Inner Loop)
    │
    ↓ Code changes + commit
```

**Why fresh sessions?** Prevents context rot. Research context pollutes planning. Planning context pollutes implementation.

---

## Command Options

```bash
# Default research (40-60k tokens)
/research "Your question here"

# Quick research (20-30k tokens)
/research --quick "Your question"
# Good for: Already know direction, need details

# Deep research (55-70k tokens)
/research --deep "Your question"
# Good for: Critical decisions, unfamiliar domains

# Extend previous research
/research --extend "Add analysis of [specific area]"
# Requires: Previous research bundle loaded
```

---

## After Research

**Automatic transition:**
1. Research completes
2. I ask: "Ready to create implementation plan?"
3. Yes → I start `/plan` automatically
4. No → Save bundle for later

**Manual transition:**
1. `/bundle-save [research-name]`
2. Start fresh session
3. `/bundle-load [research-name]`
4. `/plan [research-name].md`

---

## Best Practices

### Do
- Invest time in research (80% of cognitive work)
- Explore multiple approaches (increase N)
- Document failure pattern risks
- Save bundles for team
- Check institutional memory first

### Don't
- Rush through research
- Assume single approach
- Skip edge case analysis
- Plan before research complete
- Ignore existing patterns

---

## Emergency Procedures

**If research goes wrong:**

1. **Context degraded (>50% used)**
   - Save current findings
   - Start fresh session
   - Load findings, continue

2. **Wrong direction discovered**
   - Document why direction was wrong (learning!)
   - Pivot explicitly
   - Continue research

3. **Scope too large**
   - Split into multiple research sessions
   - Save partial findings
   - Continue with narrower scope

**Universal emergency:**
```
1. STOP all AI activity
2. SAVE current context
3. ASSESS what went wrong
4. DOCUMENT for prevention
5. RESTART with lessons learned
```

---

**Ready?** Describe your research goal and I'll explore comprehensively.
