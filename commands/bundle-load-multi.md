# /bundle-load-multi - Load Multiple Bundles Simultaneously

**Purpose:** Load 2-5 bundles at once for cross-flavor work with token budget management

**Philosophy:** Multi-flavor coordination enables work spanning multiple domains simultaneously

**Token budget:** 5-20k tokens (combined bundle size, managed automatically)

**Output:** Multiple contexts loaded, cross-bundle analysis, token budget status

---

## When to Use

Use `/bundle-load-multi` to:

### 1. Cross-Flavor Work (Technical + Personal)
```
# Load team work + personal tracking
/bundle-load-multi agentops-roadmap life-career-2025

# Result: Technical work feeds personal metrics automatically
```

### 2. Implementation Chain (Research → Plan → Progress)
```
# Load full implementation history
/bundle-load-multi research-bundle plan-bundle implementation-progress

# Result: Complete context from start to current state
```

### 3. Related Domains (Infrastructure + Application)
```
# Load related work across repositories
/bundle-load-multi k8s-infrastructure redis-app-deployment

# Result: Infrastructure and application context together
```

### 4. Multi-Repository Coordination
```
# Load bundles from different repos
/bundle-load-multi gitops-workflows example-repo-patterns release-automation

# Result: Cross-repository patterns and practices
```

---

## How It Works

### Basic Usage

```bash
/bundle-load-multi {bundle1} {bundle2} [bundle3] [options]

# Examples:
/bundle-load-multi agentops-roadmap life-career-2025
/bundle-load-multi research plan implementation-progress
/bundle-load-multi k8s-infra redis-cache monitoring-setup
```

### Step 1: Validate Request

**Token Budget Pre-Check:**

```
=== Multi-Bundle Load Request ===

Bundles requested: 3
1. agentops-roadmap-complete (35k tokens estimated)
2. life-career-2025 (12k tokens estimated)
3. infrastructure-ops (18k tokens estimated)

--- Pre-Load Validation ---

Current context:        5k tokens (2.5%)
Requested bundles:     65k tokens (estimated)
After load:            70k tokens (35%)

Status: ✅ SAFE TO LOAD
Reason: Under 40% threshold with margin
Margin: 10% remaining (20k tokens to threshold)

Proceeding with load...
```

**If Too Large:**

```
=== Multi-Bundle Load Request ===

Bundles requested: 4
1. agentops-roadmap-complete (35k tokens)
2. framework-sanitization (38k tokens)
3. launch-complete (20k tokens)
4. infrastructure-ops (18k tokens)

--- Pre-Load Validation ---

Current context:        5k tokens (2.5%)
Requested bundles:    111k tokens (estimated)
After load:           116k tokens (58%)

⚠️ EXCEEDS 40% THRESHOLD

Recommendation: Load fewer bundles
- Option 1: Load just roadmap + framework (73k = 36.5%)
- Option 2: Load just roadmap (40k = 20%)
- Option 3: Use --dry-run to see individual sizes

Try: /bundle-load-multi agentops-roadmap framework-sanitization
```

### Step 2: Search and Resolve Bundles

**Multi-Repository Search:**

For each bundle name:

```bash
# Search workspace-wide
find $WORKSPACES_DIR -type f -name "*${BUNDLE_NAME}*" -path "*/.agents/bundles/*"
```

**Resolution Output:**

```
--- Resolving Bundles ---

1. agentops-roadmap
   ✅ Found: .agents/bundles/agentops-roadmap-complete.md
   Size: 35k tokens
   Repository: workspace
   Type: roadmap

2. life-career-2025
   ✅ Found: personal/life/.agents/bundles/life-career-2025.md
   Size: 12k tokens
   Repository: personal/life
   Type: career-tracking

3. infrastructure-ops
   ✅ Found: .agents/bundles/infrastructure-operations-complete.md
   Size: 18k tokens
   Repository: workspace
   Type: infrastructure

All bundles resolved ✅
Total: 65k tokens (32.5% of context window)
```

**If Multiple Matches:**

```
--- Resolving Bundles ---

1. agentops-roadmap ✅ (35k tokens)
2. career (multiple matches found):

   📦 Available matches for 'career':
   a. life-career-2025 (12k tokens) - personal/life - 2025-11-08
   b. career-strategy (8k tokens) - personal/life - 2025-10-15
   c. career-growth-metrics (6k tokens) - personal/life - 2025-09-20

   Which bundle? [a/b/c/skip]

User selects: a

2. career → life-career-2025 ✅ (12k tokens)
3. infrastructure-ops ✅ (18k tokens)

All bundles resolved ✅
```

### Step 3: Load Bundles Sequentially

**Loading Progress:**

```
=== Loading Bundles ===

[1/3] Loading agentops-roadmap-complete...
      ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 35k tokens
      ✅ Loaded (current: 40k tokens, 20%)

[2/3] Loading life-career-2025...
      ━━━━━━━━━━━━ 12k tokens
      ✅ Loaded (current: 52k tokens, 26%)

[3/3] Loading infrastructure-operations-complete...
      ━━━━━━━━━━━━━━━━━━ 18k tokens
      ✅ Loaded (current: 70k tokens, 35%)

=== Load Complete ===

Total loaded: 65k tokens (3 bundles)
Current context: 70k tokens (35%)
Safe to continue: YES ✅
Margin to threshold: 10% (20k tokens)
```

### Step 4: Cross-Bundle Analysis

**Detect Dependencies and Relationships:**

```
=== Cross-Bundle Insights ===

🔗 Dependencies Detected:
- agentops-roadmap references infrastructure-ops (3 times)
- infrastructure-ops prerequisite for agentops-roadmap
- life-career-2025 tracks work from agentops-roadmap

📊 Temporal Relationships:
- agentops-roadmap: Q4 2025 goals
- life-career-2025: Q4 2025 tracking
- infrastructure-ops: Q3-Q4 2025 implementation
- Timeline: All within current quarter ✅

🏷️ Tag Overlap:
- Common tags: agentops, q4-2025, production-ready
- Shared context: Framework development, career growth

📁 Repository Spread:
- workspace: 2 bundles (agentops-roadmap, infrastructure-ops)
- personal/life: 1 bundle (life-career-2025)
- Cross-flavor coordination: ✅ ENABLED

💡 Suggested Actions:
- Use agentops-roadmap for technical planning
- Track accomplishments in life-career-2025
- Reference infrastructure-ops for implementation details
```

### Step 5: Context Ready for Work

```
=== Context Loaded ===

You now have:
✅ Full roadmap and vision (agentops-roadmap)
✅ Career tracking context (life-career-2025)
✅ Infrastructure patterns (infrastructure-ops)

Context utilization: 70k tokens (35%)
Recommended work types:
- Planning new features (use roadmap + infrastructure)
- Implementing features (track in career metrics)
- Multi-repository coordination

Ready for: /plan, /implement, or continue current work
```

---

## Loading Strategies

### Strategy 1: Priority Order (Default)

**Load bundles by importance:**

```bash
/bundle-load-multi bundle1 bundle2 bundle3 --priority

# Loads in order specified (most important first)
# Stops if 40% threshold approached
```

**Example:**

```
Priority loading enabled ✅

[1/4] Loading critical-bundle (highest priority)...
      ✅ Loaded (current: 35k, 17.5%)

[2/4] Loading important-bundle...
      ✅ Loaded (current: 55k, 27.5%)

[3/4] Loading nice-to-have-bundle...
      ✅ Loaded (current: 72k, 36%)

[4/4] Loading optional-bundle...
      ⚠️ Would exceed threshold (would reach 85k, 42.5%)
      ⏭️ SKIPPED (optional bundle)

Result: 3/4 bundles loaded, stayed under threshold ✅
```

### Strategy 2: Smart Dependency Order

**Automatically reorder based on dependencies:**

```bash
/bundle-load-multi plan implementation research --smart-order

# Reorders to: research → plan → implementation
```

**Example:**

```
Smart ordering enabled ✅

Analyzing dependencies...
- implementation requires plan
- plan requires research

Reordered load sequence:
1. research (prerequisite)
2. plan (depends on research)
3. implementation (depends on plan)

Loading in dependency order...
✅ All bundles loaded in correct sequence
```

### Strategy 3: Size-Optimized

**Load smallest bundles first:**

```bash
/bundle-load-multi bundle1 bundle2 bundle3 --size-optimized

# Loads smallest → largest (maximize number of bundles loaded)
```

**Example:**

```
Size optimization enabled ✅

Bundle sizes:
- bundle3: 8k tokens (smallest)
- bundle2: 15k tokens (medium)
- bundle1: 35k tokens (largest)

Loading order: bundle3 → bundle2 → bundle1

✅ All 3 loaded (total: 58k, 29%)
```

---

## Token Budget Management

### Automatic Enforcement

**Always check before loading:**

```python
def check_token_budget(current_tokens, requested_bundles):
    THRESHOLD = 200000 * 0.40  # 40% of 200k context window = 80k

    total_requested = sum([b.size for b in requested_bundles])
    projected_total = current_tokens + total_requested

    if projected_total > THRESHOLD:
        return {
            'safe': False,
            'reason': 'Would exceed 40% threshold',
            'projected': projected_total,
            'threshold': THRESHOLD,
            'overage': projected_total - THRESHOLD
        }

    return {
        'safe': True,
        'margin': THRESHOLD - projected_total,
        'percentage': (projected_total / 200000) * 100
    }
```

### Threshold Warning Levels

**Green Zone (0-30%):**
```
Current: 45k tokens (22.5%)
Status: ✅ SAFE - Plenty of room
Recommendation: Can load more bundles if needed
```

**Yellow Zone (30-35%):**
```
Current: 65k tokens (32.5%)
Status: ⚠️ CAUTION - Approaching threshold
Recommendation: Monitor remaining work carefully
```

**Orange Zone (35-40%):**
```
Current: 75k tokens (37.5%)
Status: ⚠️ WARNING - Near threshold
Recommendation: Avoid loading more bundles
```

**Red Zone (40%+):**
```
Current: 85k tokens (42.5%)
Status: ❌ EXCEEDS - Over threshold
Action: BLOCKED - Cannot load more
```

### Progressive Loading

**Load bundles until threshold:**

```bash
/bundle-load-multi bundle1 bundle2 bundle3 bundle4 --progressive

# Loads as many as possible without exceeding threshold
```

**Example:**

```
Progressive loading enabled ✅

[1/4] bundle1 (25k) → Loaded ✅ (current: 30k, 15%)
[2/4] bundle2 (20k) → Loaded ✅ (current: 50k, 25%)
[3/4] bundle3 (15k) → Loaded ✅ (current: 65k, 32.5%)
[4/4] bundle4 (18k) → Would exceed (would be 83k, 41.5%)
                    → STOPPED ⏸️

Result: 3/4 bundles loaded
Status: At safe capacity (65k, 32.5%)
```

---

## Dry Run Mode

### Preview Without Loading

```bash
/bundle-load-multi bundle1 bundle2 bundle3 --dry-run

# Shows what would be loaded, without actually loading
```

**Output:**

```
=== DRY RUN MODE (Preview Only) ===

Bundles to load: 3

1. agentops-roadmap-complete
   Size: 35k tokens
   Repository: workspace
   Type: roadmap
   Impact: +35k tokens (17.5% → 35%)

2. life-career-2025
   Size: 12k tokens
   Repository: personal/life
   Type: career-tracking
   Impact: +12k tokens (35% → 41%)

3. infrastructure-ops
   Size: 18k tokens
   Repository: workspace
   Type: infrastructure
   Impact: +18k tokens (41% → 50%)

--- Summary ---

Current context:    5k tokens (2.5%)
After loading:     70k tokens (35%)
Status:            ✅ SAFE (under 40%)

Cross-bundle analysis:
- 2 bundles share tags: agentops, q4-2025
- 1 dependency: infrastructure-ops → agentops-roadmap
- 2 repositories: workspace, personal/life

Ready to load? (this was a dry run)

To actually load: /bundle-load-multi bundle1 bundle2 bundle3
```

### Dry Run Use Cases

**1. Planning Multi-Bundle Loads:**
```bash
# Preview large load
/bundle-load-multi roadmap framework launch infra --dry-run

# See impact before committing
```

**2. Token Budget Exploration:**
```bash
# Try different combinations
/bundle-load-multi set1 set2 set3 --dry-run
/bundle-load-multi set1 set2 --dry-run
/bundle-load-multi set1 --dry-run

# Find optimal load
```

**3. Dependency Discovery:**
```bash
# See relationships before loading
/bundle-load-multi research plan implementation --dry-run

# Understand load order
```

---

## Cross-Bundle Features

### Dependency Detection

**Automatic Analysis:**

```
🔗 Dependencies Detected:

Strong dependencies (explicit references):
- implementation-bundle requires plan-bundle (5 references)
- plan-bundle requires research-bundle (3 references)

Weak dependencies (related topics):
- infrastructure-ops relates to agentops-roadmap (shared tags)
- life-career-2025 relates to agentops-roadmap (temporal overlap)

Recommendation: Load in dependency order for best understanding
```

### Tag Analysis

**Common Themes:**

```
🏷️ Tag Overlap Analysis:

Shared tags across bundles:
- #agentops: 3 bundles (agentops-roadmap, infrastructure-ops, life-career-2025)
- #q4-2025: 2 bundles (agentops-roadmap, life-career-2025)
- #production-ready: 2 bundles (agentops-roadmap, infrastructure-ops)

Unique tags per bundle:
- agentops-roadmap: #vision, #strategy, #framework
- life-career-2025: #career, #growth, #metrics
- infrastructure-ops: #bootstrap, #validation, #tooling

Insight: All bundles relate to Q4 2025 production goals
```

### Temporal Analysis

**Timeline Relationships:**

```
📊 Temporal Analysis:

Bundle timeline:
- research-bundle: 2025-10-15 (oldest)
- plan-bundle: 2025-10-20
- implementation-progress: 2025-11-05 (newest)

Chronological flow: ✅ CORRECT
- Research → Plan → Implementation (proper sequence)

Date gaps:
- Research to Plan: 5 days (normal)
- Plan to Implementation: 16 days (within expected range)

Status: Timeline makes sense for multi-session work ✅
```

### Repository Distribution

**Cross-Repository Analysis:**

```
📁 Repository Distribution:

workspace: 2 bundles
- agentops-roadmap-complete (35k tokens)
- infrastructure-operations-complete (18k tokens)

personal/life: 1 bundle
- life-career-2025 (12k tokens)

work/gitops: 0 bundles (none requested)
work/example-repo: 0 bundles (none requested)

Cross-flavor coordination: ✅ ENABLED
- Technical bundles: 2 (workspace)
- Personal bundles: 1 (personal/life)
- Integration points: Q4 2025 goals, AgentOps framework
```

---

## Advanced Options

### --priority (Priority Loading)

**Load in specified order, stop if threshold approached:**

```bash
/bundle-load-multi critical important optional --priority
```

### --smart-order (Dependency Order)

**Automatically reorder by dependencies:**

```bash
/bundle-load-multi implementation plan research --smart-order
# Loads: research → plan → implementation
```

### --size-optimized (Smallest First)

**Maximize number of bundles loaded:**

```bash
/bundle-load-multi large medium small --size-optimized
# Loads: small → medium → large
```

### --progressive (Load Until Threshold)

**Load as many as possible:**

```bash
/bundle-load-multi b1 b2 b3 b4 b5 --progressive
# Loads bundles until 40% threshold reached
```

### --dry-run (Preview Only)

**See impact without loading:**

```bash
/bundle-load-multi bundle1 bundle2 --dry-run
```

### --max-tokens (Custom Limit)

**Set custom token limit:**

```bash
/bundle-load-multi bundle1 bundle2 --max-tokens 50000
# Uses 50k instead of 80k (40% of 200k)
```

### --skip-analysis (Fast Load)

**Skip cross-bundle analysis:**

```bash
/bundle-load-multi bundle1 bundle2 --skip-analysis
# Loads bundles without dependency/tag/temporal analysis
```

---

## Common Use Cases

### Use Case 1: Technical + Personal Tracking

**Scenario:** Implement feature while tracking career growth

```bash
/bundle-load-multi agentops-roadmap life-career-2025

# Result:
# - agentops-roadmap: Technical plans and vision
# - life-career-2025: Career tracking for accomplishments
# - Cross-reference: Technical work feeds career metrics
```

**Output:**

```
=== Loading Multiple Bundles ===

Bundles: 2
Total: 47k tokens (23.5%)

Cross-flavor coordination enabled ✅
- Technical: agentops-roadmap (workspace)
- Personal: life-career-2025 (personal/life)

Integration points:
- Track AgentOps accomplishments in career metrics
- Reference roadmap milestones in capability inventory
- Q4 2025 goals span both flavors

Ready for: Technical implementation with automatic career tracking
```

### Use Case 2: Full Implementation History

**Scenario:** Resume complex work with full context

```bash
/bundle-load-multi redis-research redis-plan redis-progress

# Result:
# - Full history from research through current progress
# - Understand all decisions made
# - Resume implementation from exact stopping point
```

**Output:**

```
=== Loading Implementation Chain ===

Bundles: 3 (research → plan → progress)
Total: 45k tokens (22.5%)

Timeline:
- redis-research: 2025-10-15 (research phase)
- redis-plan: 2025-10-20 (planning phase)
- redis-progress: 2025-11-05 (implementation phase)

Flow verified: ✅ CORRECT
- Research informed plan
- Plan guided implementation
- Progress shows 7/20 files complete

Ready for: /implement --resume
Git state: 7 files staged (matches progress bundle)
```

### Use Case 3: Related Infrastructure + Application

**Scenario:** Coordinate infrastructure and application work

```bash
/bundle-load-multi k8s-infrastructure redis-app-deployment monitoring-setup

# Result:
# - Infrastructure patterns
# - Application deployment specifics
# - Monitoring configuration
# - All three work together
```

**Output:**

```
=== Loading Related Domains ===

Bundles: 3
Total: 62k tokens (31%)

Dependencies detected:
- redis-app-deployment requires k8s-infrastructure
- monitoring-setup integrates with both

Recommendation: Build infrastructure first, then app, then monitoring

Repository spread:
- work/gitops: k8s-infrastructure
- work/example-repo: redis-app-deployment
- work/gitops: monitoring-setup

Ready for: Multi-repository coordinated implementation
```

### Use Case 4: Quarterly Planning Review

**Scenario:** Review all Q4 2025 work

```bash
/bundle-load-multi agentops-roadmap career-q4-goals framework-sanitization

# Result:
# - Technical roadmap for Q4
# - Personal career goals for Q4
# - Framework work supporting goals
# - Complete quarterly context
```

---

## Error Handling

### Bundle Not Found

```
=== Multi-Bundle Load Request ===

Bundles requested: 3

[1/3] agentops-roadmap
      ✅ Found: .agents/bundles/agentops-roadmap-complete.md

[2/3] nonexistent-bundle
      ❌ NOT FOUND

      Searched locations:
      - .agents/bundles/
      - work/gitops/.agents/bundles/
      - personal/*/.agents/bundles/

      Suggestions:
      - Check spelling: nonexistent-bundle
      - Use /bundle-list to see available bundles
      - Try partial name

[3/3] infrastructure-ops
      ✅ Found: .agents/bundles/infrastructure-operations-complete.md

--- Results ---

Found: 2/3 bundles
Missing: 1 bundle (nonexistent-bundle)

Options:
1. Continue with 2 bundles
2. Abort and fix bundle name
3. Skip missing bundle

What would you like to do? [1/2/3]
```

### Token Budget Exceeded

```
=== Multi-Bundle Load Request ===

Bundles: 4
Estimated total: 125k tokens

Current: 10k tokens (5%)
After load: 135k tokens (67.5%)

❌ EXCEEDS 40% THRESHOLD

Overage: 55k tokens over threshold (80k limit)

Recommendations:

Option 1: Load fewer bundles
- Load 2 bundles: agentops-roadmap + framework (73k = 36.5%)
- Skip: launch + infrastructure

Option 2: Start fresh session
- Archive current context
- Load all 4 bundles fresh

Option 3: Use progressive loading
- Load bundles until threshold
- Skip remaining

Try: /bundle-load-multi agentops-roadmap framework --progressive
```

### Git State Mismatch (Implementation Bundles)

```
=== Loading Implementation Bundles ===

[1/3] plan-bundle ✅ (loaded)
[2/3] implementation-progress...

⚠️ GIT STATE MISMATCH DETECTED

Bundle says:
- 7 files staged
- 0 files modified
- Branch: main

Git shows:
- 5 files staged
- 2 files modified
- Branch: main

Difference:
- 2 files modified since bundle created
- May indicate work done after bundle save

Options:
1. Abort load (safest)
2. Show diff (investigate)
3. Force load (risky - ignore mismatch)

Recommended: Abort and investigate

What would you like to do? [1/2/3]
```

---

## Success Criteria

Multi-bundle load is successful when:

- ✅ All bundles found and loaded
- ✅ Token budget under 40% after load
- ✅ Git state verified (for implementation bundles)
- ✅ Cross-bundle dependencies understood
- ✅ Integration points identified
- ✅ Ready to continue work across all loaded contexts

---

## Performance Notes

**Load Time:**

- 2 bundles: ~2-3 seconds
- 3 bundles: ~3-5 seconds
- 4+ bundles: ~5-8 seconds

**Optimization:**

- Bundles load sequentially (safer than parallel)
- Token counting happens during search
- Cross-analysis happens after all loads
- Dry run mode is instant (no actual loading)

**Best Practices:**

- Load 2-3 bundles typically (sweet spot)
- Use --dry-run for 4+ bundles
- Use --progressive for uncertain loads
- Use --priority for critical context

---

## Integration with Workflow

### Complete Multi-Flavor Cycle

```
Session 1: Research (Team Flavor)
  Read CLAUDE.md
  /research redis-caching
  /bundle-save redis-research
  [End session]

Session 2: Plan (Team Flavor)
  Read CLAUDE.md
  /bundle-load redis-research
  /plan
  /bundle-save redis-plan
  [End session]

Session 3: Implement (Team + Personal Flavors)
  Read CLAUDE.md
  /bundle-load-multi redis-plan life-career-2025 ← Multi-bundle
  /implement
  [Track accomplishments in career bundle automatically]
  /bundle-save redis-progress
  [End session]

Session 4: Resume (Multi-Flavor Context)
  Read CLAUDE.md
  /bundle-load-multi redis-progress life-career-2025 ← Resume both
  /implement --resume
  Complete ✅
  [Career metrics updated automatically]
```

---

## Commands Reference

```bash
# Basic multi-bundle load
/bundle-load-multi {bundle1} {bundle2} [bundle3...]

# Preview without loading
/bundle-load-multi {bundle1} {bundle2} --dry-run

# Priority loading (order matters)
/bundle-load-multi {bundle1} {bundle2} --priority

# Smart dependency order
/bundle-load-multi {bundle1} {bundle2} --smart-order

# Size-optimized (smallest first)
/bundle-load-multi {bundle1} {bundle2} --size-optimized

# Progressive (load until threshold)
/bundle-load-multi {bundle1} {bundle2} {bundle3} --progressive

# Custom token limit
/bundle-load-multi {bundle1} {bundle2} --max-tokens 50000

# Skip cross-analysis (faster)
/bundle-load-multi {bundle1} {bundle2} --skip-analysis
```

---

## Examples

### Example 1: Cross-Flavor Work

```bash
/bundle-load-multi agentops-roadmap life-career-2025
```

**Output:**
```
=== Loading Multiple Bundles ===

Bundles: 2
Total: 47k tokens (23.5%)

[1/2] agentops-roadmap-complete ✅ (35k, workspace)
[2/2] life-career-2025 ✅ (12k, personal/life)

Cross-flavor coordination: ✅ ENABLED
Integration points:
- Q4 2025 goals span both bundles
- Technical accomplishments → career metrics
- Framework development → capability growth

Ready for: Technical work with career tracking
```

### Example 2: Implementation Chain

```bash
/bundle-load-multi research plan progress --smart-order
```

**Output:**
```
=== Loading Implementation Chain ===

Smart ordering: research → plan → progress

[1/3] research ✅ (18k)
[2/3] plan ✅ (15k)
[3/3] progress ✅ (20k)

Total: 53k tokens (26.5%)

Timeline verified:
- research: 2025-10-15
- plan: 2025-10-20 (5 days later)
- progress: 2025-11-05 (16 days later)

Ready for: /implement --resume
```

### Example 3: Dry Run Large Load

```bash
/bundle-load-multi roadmap framework launch infra --dry-run
```

**Output:**
```
=== DRY RUN MODE ===

Bundles: 4
Total: 111k tokens (estimated)

Current: 5k tokens (2.5%)
After: 116k tokens (58%)

❌ WOULD EXCEED THRESHOLD (40% = 80k)

Recommendations:
- Load 2 bundles: roadmap + framework (73k = 36.5%)
- Or load 3 bundles: roadmap + launch + infra (73k = 36.5%)

This was a dry run. Nothing loaded.
```

---

## Related Commands

- **/bundle-load** - Load single bundle
- **/bundle-save** - Save current context as bundle
- **/bundle-list** - Browse available bundles

---

**Multi-flavor work enabled.**
**Load 2-5 bundles simultaneously.**
**Stay under 40% threshold automatically.**
**Cross-bundle coordination built-in.**

**Ready for:** Work spanning multiple domains without context collapse

*Enables: Cross-flavor coordination, multi-repository work, implementation chains*
*Token budget: Automatically managed (40% rule enforced)*
*Compression: 35:1 average across all bundles*
