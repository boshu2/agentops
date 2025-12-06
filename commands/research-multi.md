---
description: Launch 3 parallel agents for 3x faster research
---

# /research-multi - Parallel Multi-Agent Research

**Purpose:** Conduct deep research 3x faster via parallel agent execution

**When to use:**
- Complex systems requiring broad exploration
- Time-critical research (deadlines approaching)
- Multi-faceted problems (technical + business + security)
- Large codebases (too much to explore sequentially)

**Token budget:** 40-60k tokens (same as single-agent, but 3x wall-clock speedup)

**Output:** Combined research bundle (500-1k tokens compressed)

---

## The Parallel Research Philosophy

**Same token budget. Faster results.**

Traditional research is sequential:
1. Map the landscape (20k tokens, 30 minutes)
2. Find similar implementations (20k tokens, 30 minutes)
3. Identify constraints (20k tokens, 30 minutes)
**Total:** 60k tokens, 90 minutes

Parallel research uses 3 agents simultaneously:
1. Agent 1: Map landscape (20k tokens, 30 minutes)
2. Agent 2: Find similar implementations (20k tokens, 30 minutes)
3. Agent 3: Identify constraints (20k tokens, 30 minutes)
**Total:** 60k tokens, 30 minutes (3x speedup!)

**Key insight:** Wall-clock time drops by 3x, token budget stays the same.

---

## How It Works

### Step 1: Task Distribution

**I will automatically split your research into 3 parallel tracks:**

**Agent 1: Landscape Mapper**
- Find all relevant files and components
- Understand directory structure
- Map dependencies and relationships
- Document naming conventions

**Agent 2: Pattern Finder**
- Search for similar implementations
- Identify reusable patterns
- Find examples and references
- Extract proven approaches

**Agent 3: Constraint Analyst**
- Identify technical constraints
- Find edge cases and gotchas
- Check dependencies and versions
- Document security considerations

### Step 2: Parallel Execution

**All 3 agents work simultaneously:**

```
Time: 0 min
├─ Agent 1 starts: Mapping landscape
├─ Agent 2 starts: Finding patterns
└─ Agent 3 starts: Analyzing constraints

Time: 30 min
├─ Agent 1 completes: Landscape map (20k tokens)
├─ Agent 2 completes: Pattern catalog (20k tokens)
└─ Agent 3 completes: Constraint report (20k tokens)

Time: 35 min
└─ Synthesis: Combined findings (5k tokens)
```

**Total:** 65k tokens, 35 minutes (vs 90 minutes sequential)

### Step 3: Synthesis

**I combine findings from all 3 agents:**

- Cross-reference discoveries (Agent 1's files + Agent 2's patterns)
- Resolve conflicts (Agent 2 vs Agent 3 recommendations)
- Prioritize findings (most important insights first)
- Create unified research bundle

---

## Usage

### Basic Syntax

```bash
/research-multi "[research topic]"
```

### Examples

```bash
# Investigate complex system
/research-multi "How does the authentication and authorization system work?"

# Evaluate technology decision
/research-multi "Should we use Kyverno or OPA for policy enforcement?"

# Debug complex issue
/research-multi "Why are Redis connections being exhausted?"

# Understand unfamiliar codebase
/research-multi "How is the CI/CD pipeline configured?"
```

---

## What I Do Automatically

### 1. Analyze Research Scope

**I determine:**
- Is this problem complex enough for 3 agents? (Yes → continue, No → suggest /research)
- What are the 3 main research areas?
- How should I split the work?

### 2. Launch 3 Agents

**I create Task tool calls:**

```
Task 1 (Agent 1): Landscape Mapper
Task 2 (Agent 2): Pattern Finder
Task 3 (Agent 3): Constraint Analyst
```

**Each agent:**
- Loads constitutional foundation (CONSTITUTION.md)
- Gets specific research focus
- Works independently
- Returns structured findings

### 3. Synthesize Results

**I combine findings:**

```markdown
# Combined Research: [Topic]

## Summary (from all 3 agents)
[TL;DR of key findings]

## Landscape (Agent 1)
- Files: [relevant paths]
- Structure: [organization]
- Dependencies: [what connects to what]

## Patterns (Agent 2)
- Similar implementation 1: [location + approach]
- Similar implementation 2: [location + approach]
- Reusable pattern: [what to adapt]

## Constraints (Agent 3)
- Technical: [limitations, versions]
- Edge cases: [scenarios to handle]
- Security: [considerations]

## Recommended Approach
[Based on all findings, what's the best path?]

## Next Steps
1. [Planning phase activities]
2. [Validation needed]
```

### 4. Compress to Bundle

**I automatically save combined findings:**

```bash
# Auto-generated bundle
.agents/bundles/[topic]-research-multi.md
```

**Bundle structure:**
- Compressed 60k research → 500-1k token bundle
- Includes contributions from all 3 agents
- Ready for planning phase
- Shareable with team

---

## When to Use Multi-Agent vs Single-Agent

### Use /research-multi when:

✅ **Problem is complex** - Multiple facets to explore
✅ **Time is critical** - Need results faster
✅ **Codebase is large** - Too much to explore sequentially
✅ **Multiple domains** - Technical + business + security
✅ **Broad exploration** - Need comprehensive understanding

**Example:** "Understand authentication system across 10+ files, find similar patterns, identify security constraints"

### Use /research when:

✅ **Problem is focused** - Single area to investigate
✅ **Time is flexible** - No deadline pressure
✅ **Codebase is small** - Can explore thoroughly alone
✅ **Single domain** - Pure technical or pure business
✅ **Deep dive needed** - Sequential exploration preferred

**Example:** "Debug single configuration file causing validation errors"

---

## Agent Specialization

### Agent 1: Landscape Mapper (Structure Specialist)

**Strengths:**
- Finding files and components
- Understanding organization
- Mapping dependencies
- Identifying relationships

**Tools used:**
- find, ls, tree (file discovery)
- grep (quick scans)
- git log (history exploration)
- Read (file inspection)

**Output:**
```markdown
## Landscape Map

### Files Discovered
- [path/to/file1.yaml] - [purpose]
- [path/to/file2.go] - [purpose]

### Directory Structure
- /apps - Application manifests
- /config - Configuration files
- /docs - Documentation

### Dependencies
- Component A → Component B
- Component C → External Service

### Key Observations
- Naming convention: [pattern]
- Organization: [principle]
```

### Agent 2: Pattern Finder (Similarity Specialist)

**Strengths:**
- Finding similar implementations
- Identifying reusable patterns
- Comparing approaches
- Extracting best practices

**Tools used:**
- grep (pattern search)
- git log (historical patterns)
- Read (example inspection)
- Glob (file matching)

**Output:**
```markdown
## Patterns Discovered

### Similar Implementation 1
- **Location:** [path/to/example1]
- **Pattern:** [approach used]
- **Adaptation:** [what to change for our use]

### Similar Implementation 2
- **Location:** [path/to/example2]
- **Pattern:** [approach used]
- **Differences:** [how it differs]

### Reusable Pattern
- **Name:** [pattern-name]
- **When to use:** [scenarios]
- **How to adapt:** [customization points]
```

### Agent 3: Constraint Analyst (Risk Specialist)

**Strengths:**
- Identifying constraints
- Finding edge cases
- Checking dependencies
- Assessing risks

**Tools used:**
- Read (file analysis)
- grep (constraint search)
- Bash (validation commands)
- git log (historical issues)

**Output:**
```markdown
## Constraints Identified

### Technical Constraints
- API version: [requirement]
- Dependencies: [required packages/versions]
- Platform: [OS, runtime requirements]

### Edge Cases
- Scenario 1: [what if X happens]
- Scenario 2: [boundary condition]
- Scenario 3: [failure mode]

### Security Considerations
- Authentication: [requirements]
- Authorization: [access control]
- Data handling: [encryption, PII]

### Known Gotchas
- [Pitfall 1 and how to avoid]
- [Pitfall 2 and how to avoid]
```

---

## Synthesis Process

**How I combine 3 agent outputs:**

### 1. Cross-Reference Findings

**Agent 1 found files → Agent 2 searched those files for patterns**
- Result: "File X uses Pattern Y (confirmed by both agents)"

**Agent 2 found pattern → Agent 3 checked constraints**
- Result: "Pattern Y works IF constraint Z is satisfied"

### 2. Resolve Conflicts

**If agents disagree:**
- Agent 2: "Use approach A"
- Agent 3: "Approach A violates constraint B"
- **Synthesis:** "Use approach A with modification C to satisfy constraint B"

### 3. Prioritize Findings

**Most important insights first:**
1. Critical blockers (Agent 3 constraints)
2. Proven patterns (Agent 2 similarities)
3. Structural understanding (Agent 1 landscape)

### 4. Create Unified Recommendation

**Based on all findings:**
```markdown
## Recommended Approach

Given:
- Landscape: [Agent 1 found X structure]
- Patterns: [Agent 2 found Y similar implementation]
- Constraints: [Agent 3 identified Z limitation]

Recommend:
- Adapt pattern Y to our structure X
- Modify to satisfy constraint Z
- Follow this sequence: [steps]

Rationale:
- [Why this approach is best given all findings]
```

---

## Token Budget Management

**Same token budget as single-agent research:**
- Total: 40-60k tokens (20-30% of 200k window)
- Per agent: ~20k tokens each
- Synthesis: 5-10k tokens
- Reserved: 135-150k tokens (67-75%)

**Key difference:** Wall-clock time is 3x faster

**If budget is tight:**
```bash
# Option 1: Smaller scope per agent
/research-multi "[focused-topic]" --quick
# Each agent: ~10k tokens, faster execution

# Option 2: Checkpoint mid-research
# Agents automatically checkpoint if approaching 40%

# Option 3: Sequential fallback
# If parallel not possible, auto-switches to /research
```

---

## Success Criteria

**Multi-agent research succeeds when:**

✅ All 3 agents complete their research tracks
✅ Findings are cross-referenced and synthesized
✅ Unified recommendation is clear
✅ Combined bundle is <1k tokens
✅ Ready for planning phase

**Quality checks:**
- Agent 1's files match Agent 2's pattern locations
- Agent 2's patterns satisfy Agent 3's constraints
- No major conflicts between agent findings
- Recommendation is actionable

---

## Advanced: Custom Agent Assignment

**If you want specific research tracks:**

```bash
/research-multi "[topic]" --tracks "track1,track2,track3"
```

**Example:**
```bash
/research-multi "Authentication system" --tracks "frontend,backend,database"

# Agent 1: Research frontend authentication (React, JWT handling)
# Agent 2: Research backend authentication (API, middleware)
# Agent 3: Research database authentication (user schema, sessions)
```

**Default tracks (if not specified):**
- Landscape (structure, files, dependencies)
- Patterns (similar implementations, reusable approaches)
- Constraints (technical limits, edge cases, security)

---

## Integration with Other Commands

**Before multi-agent research:**
```bash
Read CLAUDE.md  # Load constitutional foundation
# Agent suggests /research-multi for complex exploration
```

**After multi-agent research:**
```bash
# Automatic bundle creation
# File: .agents/bundles/[topic]-research-multi.md

# Load in planning phase
/plan [topic]-research-multi.md

# Or load later
/bundle-load [topic]-research-multi
```

---

## Examples

### Example 1: Complex System Investigation

```bash
/research-multi "How does the GitOps application sync system work?"

# Agent 1 (Landscape):
# - Maps: apps/, sites/, overlays/, base/
# - Dependencies: Kustomize, Argo CD, Helm
# - Files: 50+ YAML files, 10+ directories

# Agent 2 (Patterns):
# - Similar: 7 example apps using same pattern
# - Pattern: Base + overlay inheritance
# - Reusable: Kustomization template

# Agent 3 (Constraints):
# - Kustomize version: >=4.5.0
# - Argo CD sync waves required
# - Edge case: Namespace ordering

# Synthesis (5 min):
# Combined findings → "Use base+overlay pattern with sync waves"
# Bundle: 0.9k tokens
# Time saved: 60 minutes vs sequential research
```

### Example 2: Technology Evaluation

```bash
/research-multi "Evaluate Prometheus vs Grafana Mimir for metrics"

# Agent 1 (Landscape):
# - Current: Prometheus (standalone)
# - Scale: 100k metrics/sec
# - Storage: Local disk, 15 day retention

# Agent 2 (Patterns):
# - Example 1: SaaS using Mimir (better scaling)
# - Example 2: On-prem using Prometheus (simpler ops)
# - Pattern: Start Prometheus, migrate to Mimir at scale

# Agent 3 (Constraints):
# - Prometheus: Limited retention, disk I/O
# - Mimir: Requires object storage (S3)
# - Cost: Mimir $$$ vs Prometheus $

# Synthesis:
# Recommendation: "Start Prometheus, plan Mimir migration at 1M metrics/sec"
# Bundle: 1.1k tokens
# Time saved: 45 minutes
```

### Example 3: Debugging Complex Issue

```bash
/research-multi "Why is the auth service intermittently failing?"

# Agent 1 (Landscape):
# - Services: auth-api, auth-db, redis-cache
# - Logs: auth-api shows timeouts
# - Dependencies: PostgreSQL, Redis

# Agent 2 (Patterns):
# - Similar issue: user-service (fixed via connection pooling)
# - Pattern: Increase pool size, add health checks
# - Related: Redis connection exhaustion (seen before)

# Agent 3 (Constraints):
# - Connection limits: PostgreSQL 100 max, Redis 10k max
# - Edge case: Burst traffic at 5pm daily
# - Observed: Pool exhaustion during bursts

# Synthesis:
# Root cause: "Connection pool too small for burst traffic"
# Solution: "Increase pool size + add health checks + rate limiting"
# Bundle: 0.8k tokens
# Time saved: 75 minutes (debug was urgent)
```

---

## Limitations

**When multi-agent research WON'T help:**

❌ **Problem is too simple** - 3 agents is overkill for straightforward tasks
❌ **Sequential dependencies** - If Agent 2 needs Agent 1's output to proceed
❌ **Narrow scope** - Single file or configuration change
❌ **Context is already high** - If already at 35%+ context, too risky

**In these cases, use:**
- `/research` for focused investigation
- `Read CLAUDE.md-simple` for straightforward tasks
- Sequential exploration when dependencies exist

---

## Performance Metrics

**Proven speedups from parallel research:**

| Metric | Single-Agent | Multi-Agent | Improvement |
|--------|--------------|-------------|-------------|
| Wall-clock time | 90 min | 30 min | **3x faster** |
| Token budget | 60k | 65k | Similar |
| Coverage | Good | Excellent | Broader |
| Quality | High | High | Maintained |

**Real examples:**
- GitOps system exploration: 90 min → 30 min (3x)
- Technology evaluation: 60 min → 20 min (3x)
- Complex debugging: 120 min → 40 min (3x)

---

## Related Commands

- **Read CLAUDE.md** - Load constitutional foundation before research
- **/research** - Single-agent focused research
- **/bundle-save** - Compress research findings (auto-called)
- **/bundle-load** - Resume research in fresh session
- **/plan** - Next phase after research
- **/validate-multi** - 3x parallel validation

---

**Ready for 3x faster research? What complex system are you investigating?**
