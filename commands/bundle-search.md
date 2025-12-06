# /bundle-search - Semantic Search for Context Bundles

**Purpose:** Find relevant bundles using Memory MCP semantic search

**When to use:**
- You need a bundle but don't know the exact filename
- You want to find bundles by topic/keyword
- You need bundles related to a specific concept
- You want to discover co-loaded bundle patterns

**Prerequisites:** Memory MCP indexed bundles (run `/bundle-index-all` first)

---

## How It Works

### Traditional Search (Filename-based)
```bash
ls .agents/bundles/ | grep redis
# Only finds bundles with "redis" in filename
# Misses: "caching-optimization.md" (uses Redis but not in name)
```

### Semantic Search (Memory MCP)
```bash
/bundle-search "caching performance optimization"
# Finds:
# - redis-caching-research.md
# - caching-optimization.md
# - performance-tuning-complete.md
# All mention caching, even if "caching" not in filename
```

---

## Usage

### Basic Search
```bash
/bundle-search "kubernetes deployment strategies"

# Returns:
# 1. k8s-migration-research.md (95% relevance)
#    Topics: Kubernetes, deployment, migration, strategies
#    Created: 2025-10-15, Accessed: 12 times
#
# 2. container-orchestration-plan.md (87% relevance)
#    Topics: K8s, orchestration, deployment patterns
#    Created: 2025-09-20, Accessed: 5 times
#
# 3. helm-chart-optimization.md (73% relevance)
#    Topics: Helm, Kubernetes, deployment
#    Created: 2025-08-10, Accessed: 3 times
```

### Filter by Type
```bash
/bundle-search "redis" --type research

# Only returns research bundles about Redis
```

### Filter by Date
```bash
/bundle-search "agentops" --since 7d

# Bundles created in last 7 days
```

### Filter by Tags
```bash
/bundle-search "performance" --tags redis,database

# Bundles tagged with both "redis" AND "database"
```

### Multi-Select Load
```bash
/bundle-search "kubernetes deployment"

# Interactive selection:
# [x] 1. k8s-migration-research.md
# [x] 2. container-orchestration-plan.md
# [ ] 3. helm-chart-optimization.md
#
# Load selected? (y/n): y
#
# Loading:
# - k8s-migration-research.md (2.1K tokens)
# - container-orchestration-plan.md (1.8K tokens)
# Total: 3.9K tokens (1.95% context)
```

---

## Search Strategies

### By Topic (Semantic)
```bash
/bundle-search "improving application response times"

# Memory MCP semantic search finds:
# - "redis-caching-research.md" (mentions response time optimization)
# - "performance-tuning-complete.md" (focuses on response times)
# - "api-optimization-plan.md" (includes latency reduction)
```

### By Problem Statement
```bash
/bundle-search "authentication failing intermittently"

# Finds bundles addressing similar issues:
# - "auth-service-debugging.md"
# - "session-timeout-fix.md"
# - "redis-connection-pool-issue.md"
```

### By Technology
```bash
/bundle-search "Redis pub/sub patterns"

# Technology-focused search:
# - "redis-caching-research.md"
# - "event-driven-architecture.md"
# - "message-queue-comparison.md"
```

### By Related Concepts
```bash
/bundle-search "related to agentops-roadmap-complete"

# Memory MCP finds:
# - Bundles that depend on agentops-roadmap
# - Bundles frequently co-loaded with it
# - Bundles from same time period/context
```

---

## How Search Works (Under the Hood)

### Step 1: Query Memory MCP
```typescript
const results = await mcp_memory_search_nodes({
  query: "kubernetes deployment strategies"
});

// Memory MCP searches:
// - Entity names (bundle IDs)
// - Observations (topics, tags, learnings)
// - Related entities (dependencies, co-loaded)
```

### Step 2: Rank Results
```typescript
// Memory MCP returns entities, we rank by:
// 1. Semantic relevance (how well query matches observations)
// 2. Access frequency (popular bundles ranked higher)
// 3. Recency (newer bundles ranked higher if tie)
// 4. Co-loading patterns (bundles loaded together)
```

### Step 3: Display Summaries
```typescript
// For each result, show:
// - Bundle name + relevance score
// - Topics/tags
// - Creation date + access count
// - Token size
// - Dependencies
```

### Step 4: Interactive Selection
```typescript
// User can:
// - Select single bundle (load it)
// - Select multiple bundles (load all)
// - View bundle details (cat bundle)
// - Load dependencies automatically
```

---

## Example Workflows

### Workflow 1: "I need bundles about caching"

```bash
/bundle-search "caching"

# Results:
# 1. redis-caching-research.md (98% relevance)
# 2. memcached-evaluation.md (85% relevance)
# 3. local-cache-patterns.md (72% relevance)
#
# Select: 1
# Load redis-caching-research.md (1.9K tokens)
```

### Workflow 2: "What bundles relate to my current work?"

```bash
# Currently working on Redis implementation
/bundle-search "related to redis-implementation-progress"

# Results:
# 1. redis-caching-research.md (dependency)
# 2. redis-caching-plan.md (dependency)
# 3. circuit-breaker-pattern.md (frequently co-loaded)
#
# Select: all
# Load all 3 bundles (5.2K tokens)
```

### Workflow 3: "Find bundles I haven't used recently"

```bash
/bundle-search "performance" --not-accessed-since 30d

# Results:
# 1. database-optimization.md (not accessed in 45 days)
# 2. query-tuning-guide.md (not accessed in 60 days)
#
# Discover forgotten knowledge!
```

### Workflow 4: "What did we learn about X?"

```bash
/bundle-search "learnings about Redis failures"

# Memory MCP searches observations with "learning" prefix:
# 1. redis-caching-research.md
#    Learning: "Circuit breaker pattern prevents cascade failures"
#
# 2. redis-connection-pool-issue.md
#    Learning: "Connection pooling failed, use sentinel pattern"
```

---

## Advanced Features

### Co-Loading Suggestions
```bash
/bundle-search "redis-caching-plan" --suggest-related

# Bundle: redis-caching-plan.md
# Frequently loaded with:
# - redis-caching-research.md (85% of loads)
# - circuit-breaker-pattern.md (60% of loads)
# - framework-sanitization.md (40% of loads)
#
# Load all? (y/n)
```

### Dependency Resolution
```bash
/bundle-search "redis-implementation" --with-dependencies

# Bundle: redis-implementation-complete.md
# Dependencies (auto-selected):
# ✓ redis-caching-research.md
# ✓ redis-caching-plan.md
#
# Total: 3 bundles, 6.8K tokens (3.4% context)
```

### Version Discovery
```bash
/bundle-search "agentops-roadmap" --all-versions

# Results:
# 1. agentops-roadmap-v3.md (current, 2025-11-07)
# 2. agentops-roadmap-v2.md (superseded, 2025-10-15)
# 3. agentops-roadmap-v1.md (superseded, 2025-09-01)
#
# Show diff between v2 and v3? (y/n)
```

---

## Memory MCP Integration

### What Gets Indexed

Every bundle stored in Memory MCP has:

**Entity:**
```typescript
{
  name: "bundle-redis-caching-research-2025-11-08",
  entityType: "context_bundle",
  observations: [
    "Created: 2025-11-08T10:30:00Z",
    "Type: research",
    "Topics: Redis caching, performance optimization, distributed systems",
    "Tags: redis, caching, performance, database",
    "Original: 45K tokens, Compressed: 1.9K, Ratio: 23.7:1",
    "Accessed 3 times, last: 2025-11-08T14:00:00Z",
    "Key learning: Circuit breaker pattern prevents cascade failures"
  ]
}
```

**Relations:**
```typescript
// Dependencies
{from: "bundle-redis-plan", to: "bundle-redis-research", relationType: "depends_on"}

// Versioning
{from: "bundle-redis-v2", to: "bundle-redis-v1", relationType: "supersedes"}

// Co-loading
{from: "bundle-redis-plan", to: "bundle-circuit-breaker", relationType: "frequently_loaded_with"}

// Creator
{from: "User_Fullerbt", to: "bundle-redis-research", relationType: "created"}
```

### Indexing Bundles

**Automatic (recommended):**
```bash
# When creating bundle
/bundle-save redis-caching-research
# Automatically creates Memory MCP entity
```

**Manual (for existing bundles):**
```bash
# Index all existing bundles
/bundle-index-all

# Index specific bundle
/bundle-index redis-caching-research.md
```

**Batch (for migration):**
```bash
# Index all bundles in .agents/bundles/
/bundle-index-all

# Shows progress:
# Indexing 47 bundles...
# [████████████████████] 100% (47/47)
# Created 47 entities, 128 relations
# Total time: 3.2 seconds
```

---

## Search Tips

### 1. Use Natural Language
```bash
# Good
/bundle-search "how to optimize database queries"

# Works, but less effective
/bundle-search "database"
```

### 2. Be Specific for Narrow Results
```bash
# Narrow (5-10 results)
/bundle-search "Redis pub/sub for cache invalidation"

# Broad (30+ results)
/bundle-search "Redis"
```

### 3. Use Filters to Refine
```bash
# Start broad
/bundle-search "kubernetes"
# 50 results

# Add filters
/bundle-search "kubernetes" --type research --since 30d
# 5 results
```

### 4. Search by Problem, Not Solution
```bash
# Good (finds multiple solutions)
/bundle-search "slow API response times"

# Less useful (finds only Redis solutions)
/bundle-search "Redis caching solution"
```

### 5. Discover Related Work
```bash
# Find what others loaded with this bundle
/bundle-search "related to {current-bundle}"

# Discover co-loading patterns
```

---

## Integration with Other Commands

### With /bundle-load
```bash
# Search, then load interactively
/bundle-search "kubernetes deployment"
# Select results → auto-loads selected bundles
```

### With /bundle-save
```bash
# Save bundle → auto-indexes in Memory MCP
/bundle-save redis-caching-research
# Creates entity + relations automatically
```

### With /research-multi
```bash
# Research creates findings → search for related bundles
/research-multi "authentication system"
# Synthesis suggests: "Related bundles: auth-patterns.md, security-guide.md"
```

### With /plan
```bash
# Planning phase → search for similar plans
/bundle-search "related to current plan"
# Discover proven patterns from past work
```

---

## Output Format

### Standard Output
```
Found 3 bundles matching "kubernetes deployment strategies":

1. k8s-migration-research.md (95% relevance) ⭐
   Topics: Kubernetes, deployment, migration strategies, helm
   Created: 2025-10-15 | Accessed: 12 times (last: 2025-11-01)
   Size: 2.1K tokens | Ratio: 21:1
   Dependencies: None
   Status: complete

2. container-orchestration-plan.md (87% relevance)
   Topics: K8s, orchestration, deployment patterns, scaling
   Created: 2025-09-20 | Accessed: 5 times (last: 2025-10-25)
   Size: 1.8K tokens | Ratio: 18:1
   Dependencies: k8s-migration-research.md
   Status: approved

3. helm-chart-optimization.md (73% relevance)
   Topics: Helm, Kubernetes, deployment, performance
   Created: 2025-08-10 | Accessed: 3 times (last: 2025-09-15)
   Size: 1.5K tokens | Ratio: 15:1
   Dependencies: None
   Status: complete

Total: 3 bundles, 5.4K tokens (2.7% context if all loaded)

Actions:
[L]oad selected | [A]ll | [D]etails | [Q]uit
```

### JSON Output (for scripting)
```bash
/bundle-search "redis" --json

{
  "query": "redis",
  "results": [
    {
      "bundle_id": "bundle-redis-caching-research-2025-11-08",
      "filename": "redis-caching-research.md",
      "relevance": 0.95,
      "topics": ["Redis", "caching", "performance"],
      "created": "2025-11-08T10:30:00Z",
      "accessed_count": 3,
      "size_tokens": 1900,
      "dependencies": []
    }
  ]
}
```

---

## Configuration

### Search Settings (in .claude/settings.json)
```json
{
  "bundle_search": {
    "max_results": 10,
    "min_relevance": 0.5,
    "sort_by": "relevance",
    "show_related": true,
    "auto_load_dependencies": false
  }
}
```

---

## Success Criteria

Search is successful when:
- ✅ Finds relevant bundles (not just exact filename matches)
- ✅ Results ranked by relevance
- ✅ Shows bundle metadata (topics, size, access count)
- ✅ Enables multi-select loading
- ✅ Suggests related bundles (co-loading patterns)
- ✅ Respects 40% rule (shows total token cost)

---

## Troubleshooting

### "No bundles found"
```bash
# Possible causes:
# 1. Bundles not indexed yet
/bundle-index-all

# 2. Query too specific
/bundle-search "redis" --min-relevance 0.3

# 3. Memory MCP not running
# Check: .mcp.json has memory server configured
```

### "Search returns irrelevant results"
```bash
# Solution: Use more specific query
/bundle-search "Redis pub/sub pattern for cache invalidation"

# Or add filters
/bundle-search "redis" --type research --tags performance
```

### "Can't find recently created bundle"
```bash
# Bundle not indexed yet
/bundle-index redis-caching-research.md

# Or wait for auto-index (runs every hour)
```

---

**Memory MCP semantic search enables discovery of bundles by meaning, not just filename.**
**Result: Find relevant knowledge faster, discover related work, avoid duplicate research.**
