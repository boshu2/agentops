---
description: List all saved bundles across repositories (prefer /bundle-search for discovery)
---

# /bundle-list - Browse All Bundles

**Purpose:** Show all available bundles across repositories with filtering options.

**TIP:** For better discovery, use `/bundle-search` with semantic queries instead!

**Multi-Repository:** Searches ALL repositories:
- `workspace/.agents/bundles/`
- `gitops/.agents/bundles/`
- `example-repo/.agents/bundles/`
- `12-factor-agentops/.agents/bundles/`
- `life/.agents/bundles/`
- All other repos with `.agents/bundles/`

**When to use:**
- Browsing all available bundles
- Filtering by date/tag/size
- Understanding what exists workspace-wide

**Better alternative:** Use `/bundle-search "topic"` for semantic discovery

**Token budget:** 1-5k total (depends on number of bundles shown)

---

## How It Works

### Default: Show All Bundles Across All Repositories

**Multi-Repository Search:** I will search across all repositories in `/path/to/workspace/`:

```bash
# Repositories searched:
# - workspace/.agents/bundles/ (workspace root)
# - gitops/.agents/bundles/ (team bundles)
# - 12-factor-agentops/.agents/bundles/ (framework bundles)
# - example-repo/.agents/bundles/ (application bundles)
# - life/.agents/bundles/ (personal bundles)
# - [all other repos with .agents/bundles/]
```

```bash
$ /bundle-list
```

**Output:**
```
📦 Available Bundles (12 total across 5 repositories)

🔴 redis-caching-research (1.2k) - 2025-11-05 14:30
   Repository: gitops
   Tags: redis, caching, architecture
   UUID: bundle-abc123def456
   Accessed: 3 times (last: 2025-11-06 09:15)
   Status: ACTIVE (2 days old)

🔴 k8s-orchestration-plan (4.5k) - 2025-11-06 13:35
   Repository: 12-factor-agentops
   Tags: kubernetes, orchestration, k8s
   UUID: bundle-k8s-orchestration-plan-2025-11-06
   Accessed: 1 time (last: 2025-11-06 13:35)
   Status: ACTIVE (today)

🔴 life-flavor-integration-plan (2.0k) - 2025-11-06 13:19
   Repository: workspace
   Tags: life, flavor, agentops
   UUID: bundle-life-flavor-integration-2025-11-06
   Accessed: 1 time (last: 2025-11-06 13:19)
   Status: ACTIVE (today)

🟠 kyverno-policy-pattern (0.9k) - 2025-11-03 16:45
   Repository: gitops
   Tags: kyverno, security, policy
   UUID: bundle-ghi789jkl012
   Accessed: 2 times
   Status: STALE (4 days old) - Consider refreshing

[... more bundles ...]

Legend:
  🔴 = Active (used in last 3 days)
  🟠 = Stale (not used in 3-7 days)
  🟡 = Old (not used in 7+ days)

Repository Distribution:
  - workspace: 1 bundle
  - gitops: 5 bundles
  - 12-factor-agentops: 3 bundles
  - example-repo: 2 bundles
  - life: 1 bundle
```

---

## Search Options

### By Name (Contains)
```bash
$ /bundle-list redis
# Shows all bundles with "redis" in name across ALL repositories
```

Output:
```
Found 3 bundles matching "redis" (across 2 repositories):

📦 redis-caching-research
   Repository: gitops

📦 redis-cluster-failover
   Repository: gitops

📦 redis-memory-optimization
   Repository: example-repo
```

### By Tag
```bash
$ /bundle-list --tag redis
# All bundles tagged with redis

$ /bundle-list --tag "redis|caching"
# All bundles tagged redis OR caching

$ /bundle-list --tag redis --tag architecture
# All bundles tagged redis AND architecture
```

Output:
```
Found 5 bundles with tag "redis":

📦 redis-caching-research (1.2k)
📦 redis-cluster-failover (1.5k)
📦 redis-memory-optimization (0.8k)
📦 redis-pubsub-pattern (0.7k)
📦 redis-backup-strategy (1.1k)

Total size: 5.3k tokens (manageable)
```

### By Date
```bash
$ /bundle-list --recent
# 5 most recently accessed bundles

$ /bundle-list --recent 10
# 10 most recent

$ /bundle-list --since "2025-11-01"
# Created on or after Nov 1

$ /bundle-list --before "2025-10-01"
# Created before Oct 1

$ /bundle-list --today
# Created today
```

Output:
```
Most recent bundles (accessed):

1. redis-caching-research (2025-11-06 09:15)
2. argocd-debugging-plan (2025-11-05 14:30)
3. kyverno-policy-pattern (2025-11-05 12:00)
4. caching-implementation-plan (2025-11-04 16:45)
5. applications-create-app (2025-11-04 14:20)
```

### By Size
```bash
$ /bundle-list --small
# <0.5k tokens (quick reference)

$ /bundle-list --medium
# 0.5k-1.5k tokens

$ /bundle-list --large
# >1.5k tokens

$ /bundle-list --largest
# Sort by size, biggest first
```

Output:
```
Largest bundles (by token count):

📦 database-migration-plan (2.0k)
📦 argocd-debugging-plan (1.8k)
📦 redis-caching-research (1.2k)
📦 redis-cluster-failover (1.5k)
📦 kyverno-policy-pattern (0.9k)
```

### Combined Filters
```bash
$ /bundle-list --tag redis --since "2025-11-01" --large
# Redis bundles, created in last 6 days, >1.5k tokens

$ /bundle-list --tag "security|compliance" --recent 5
# Recent security/compliance bundles (5 most)

$ /bundle-list redis --small --recent
# Recent redis bundles under 0.5k tokens
```

---

## Output Formats

### Default: Human-Readable Table
```bash
$ /bundle-list
```

```
📦 redis-caching-research (1.2k) - 2025-11-05
   Tags: redis, caching, architecture
   UUID: bundle-abc123def456
   ...
```

### Detailed View
```bash
$ /bundle-list --detailed
```

```
Name: redis-caching-research
UUID: bundle-abc123def456
Size: 1.2k tokens
Created: 2025-11-05 14:30:00 UTC
Last accessed: 2025-11-06 09:15:00 UTC
Access count: 3
Tags: redis, caching, architecture
Compression ratio: 5:1 (original 6k tokens)
Source: research-phase
Status: ACTIVE
Related bundles:
  - redis-cluster-failover (same domain)
  - caching-implementation-plan (next phase)

First 100 chars of content:
"Redis caching architecture pattern. Key finding: use pub/sub..."
```

### CSV Export
```bash
$ /bundle-list --export csv
```

```
name,uuid,size_tokens,created,tags,access_count
redis-caching-research,bundle-abc123def456,1200,2025-11-05,redis|caching|architecture,3
argocd-debugging-plan,bundle-def456ghi789,1800,2025-11-04,argocd|debugging|operations,1
...
```

### JSON Export
```bash
$ /bundle-list --export json
```

```json
{
  "bundles": [
    {
      "name": "redis-caching-research",
      "uuid": "bundle-abc123def456",
      "size_tokens": 1200,
      "created": "2025-11-05T14:30:00Z",
      "tags": ["redis", "caching", "architecture"],
      "access_count": 3
    },
    ...
  ],
  "total_bundles": 12,
  "total_tokens": 14500
}
```

### Summary Only
```bash
$ /bundle-list --summary-only
# Just names, tags, and dates (ultra-fast, minimal tokens)
```

```
redis-caching-research | redis, caching | 2025-11-05
argocd-debugging-plan | argocd, debugging | 2025-11-04
kyverno-policy-pattern | kyverno, security | 2025-11-03
```

---

## Statistics & Analysis

### Show Statistics
```bash
$ /bundle-list --stats
```

Output:
```
Bundle Statistics (Multi-Repository)

Total bundles: 12 (across 5 repositories)
Total size: 14.5k tokens
Average size: 1.2k tokens
Largest bundle: database-migration-plan (2.0k, gitops)
Smallest bundle: redis-pubsub-pattern (0.7k, gitops)

Repository Distribution:
  workspace: 1 bundle (2.0k tokens)
  gitops: 5 bundles (6.5k tokens)
  12-factor-agentops: 3 bundles (8.5k tokens)
  example-repo: 2 bundles (1.8k tokens)
  life: 1 bundle (2.0k tokens)

Activity:
  Active (last 3 days): 4 bundles
  Stale (3-7 days): 5 bundles
  Old (7+ days): 3 bundles

Top tags:
  redis (5 bundles)
  architecture (4 bundles)
  operations (3 bundles)
  caching (2 bundles)
  security (2 bundles)

Most accessed:
  1. redis-caching-research (3 times, gitops)
  2. kyverno-policy-pattern (2 times, gitops)
  3. argocd-debugging-plan (1 time, gitops)

Storage (per repository):
  workspace/.agents/bundles/: 8k bytes (1 file)
  gitops/.agents/bundles/: 26k bytes (5 files)
  12-factor-agentops/.agents/bundles/: 34k bytes (3 files)
  example-repo/.agents/bundles/: 7k bytes (2 files)
  life/.agents/bundles/: 8k bytes (1 file)
  Total: 83k bytes (12 files)
```

### Show Gaps
```bash
$ /bundle-list --analyze
# Identify missing bundles or duplicates
```

Output:
```
Bundle Analysis

✅ Well-covered areas:
  - Redis architecture (5 bundles)
  - Debugging workflows (3 bundles)
  - Security patterns (2 bundles)

⚠️ Gaps identified:
  - Postgres optimization (0 bundles)
  - Load testing patterns (0 bundles)
  - Cost analysis (0 bundles)

🔄 Potentially duplicate:
  - "redis-caching-research" and "redis-cache-pattern"
    (Consider consolidating if >3 days old)

Recommendations:
  - Create postgres-optimization bundle
  - Consolidate redis bundles (too many variants)
```

---

## Related Bundles

### Find Connected Work

```bash
$ /bundle-list redis-caching-research --related
# Show bundles connected to this one
```

Output:
```
📦 redis-caching-research
   Created: 2025-11-05

Related bundles:

Parent/Source:
  - None (this is root research)

Children/Next Phase:
  - caching-implementation-plan (ready for /plan phase)

Same Domain:
  - redis-cluster-failover (architecture)
  - redis-memory-optimization (performance)
  - redis-backup-strategy (operations)

Suggested Path:
  1. redis-caching-research ← You are here
  2. caching-implementation-plan ← Next step
  3. redis-cluster-failover ← Advanced topic
```

---

## Using Results

### Load a Bundle from List
```bash
$ /bundle-list redis
# Shows: redis-caching-research, redis-cluster-failover, redis-pubsub-pattern

# Pick one:
/bundle-load redis-caching-research
# ✅ Loaded!
```

### Share Bundle UUID
```bash
$ /bundle-list redis-caching-research
# Output includes: UUID: bundle-abc123def456

# Copy UUID and share: "Load this: bundle-abc123def456"
```

### Decide if Duplicate Work
```bash
$ /bundle-list redis
# Shows: redis-caching-research (accessed 3 times, very recent)

Output:
"This topic has been researched 3 times.
 Suggestion: Use existing bundle first before new research."

/bundle-load redis-caching-research
# Start with existing research, extend if needed
```

---

## Integration with Commands

### Research Workflow
```bash
# Step 1: Check if already researched
$ /bundle-list redis
# Found: redis-caching-research (2 days old)

# Step 2: Load existing
$ /bundle-load redis-caching-research

# Step 3: Extend if needed
$ /research "Updates to Redis caching (2025 improvements)"

# Step 4: Save new findings
$ /bundle-save redis-caching-updated
```

### Planning Workflow
```bash
# Step 1: Find research
$ /bundle-list --tag redis --tag caching

# Step 2: Load for context
$ /bundle-load redis-caching-research

# Step 3: Create plan
$ /plan redis-caching-research.md

# Step 4: Save plan
$ /bundle-save caching-implementation-plan
```

### Multi-Agent Workflow
```bash
# Step 1: Check what's been researched
$ /bundle-list --analyze
# Shows: Redis well-covered, Postgres gaps

# Step 2: Run multi-agent research on gap
$ Read CLAUDE.md-multi "Postgres optimization patterns"

# Step 3: Save findings
$ /bundle-save postgres-optimization-research

# Step 4: Future: Load and plan
$ /bundle-load postgres-optimization-research
```

---

## Best Practices

### Do
- ✅ Check /bundle-list before starting research
- ✅ Use tags to organize bundles
- ✅ Use --related to understand workflow
- ✅ Share UUIDs with team
- ✅ Archive old bundles (--analyze shows candidates)

### Don't
- ❌ Create duplicate bundles without checking
- ❌ Ignore "potentially duplicate" warnings
- ❌ Leave thousands of old bundles (maintain archive)
- ❌ Use non-descriptive names

---

## Storage & Performance

### Where Bundles Live (Multi-Repository Architecture)

```
/path/to/workspace/
├── .agents/bundles/                      # Workspace-wide bundles
│   ├── life-flavor-integration-plan.md
│   └── .bundle-metadata.json
│
├── gitops/.agents/bundles/               # Team bundles
│   ├── redis-caching-research.md
│   ├── argocd-debugging-plan.md
│   └── kyverno-policy-pattern.md
│
├── 12-factor-agentops/.agents/bundles/   # Framework bundles
│   ├── k8s-orchestration-plan.md
│   ├── repo-split-phase2-completion.md
│   └── index.json
│
├── example-repo/.agents/bundles/              # Application bundles
│   └── [application-specific bundles]
│
└── life/.agents/bundles/                 # Personal bundles
    └── [personal bundles]
```

**Search strategy:**
1. Search all `.agents/bundles/` directories in workspace
2. Aggregate results across repositories
3. Display with repository location
4. Load from correct repository path

### Search Performance
- <100 bundles: Instant search
- 100-500 bundles: <1 second
- 500+ bundles: Use filters (--tag, --recent)

### Automated Maintenance

**NEW: Automated bundle cleanup script** (`bin/bundle-maintenance.sh`)

```bash
# Scan all bundles for issues
bin/bundle-maintenance.sh

# Archive old bundles (>30 days)
bin/bundle-maintenance.sh --archive

# Archive and commit to git
bin/bundle-maintenance.sh --commit

# Delete (not archive) old bundles
bin/bundle-maintenance.sh --prune --commit
```

**What it detects:**
- 🟡 Old bundles (>30 days) - suggests archiving
- 🟠 Stale bundles (7-30 days) - monitor
- ⚠️  Oversized bundles (>2k tokens) - suggests compression
- 🔄 Duplicate bundles - exact hash matches
- 📝 Similar names - potential consolidation

**Example output:**
```
Total bundles scanned: 10
Active (<7 days): 10
Stale (7-30 days): 0
Old (>30 days): 0
Oversized (>2k tokens): 8

Recommendations:
  2. Review 8 oversized bundles:
     - Compress content (remove verbose outputs)
     - Split into multiple smaller bundles
```

### Manual Cleanup (Legacy)

```bash
# List all bundles not accessed in 30 days
$ /bundle-list --before "2025-10-06"

# Manual deletion
$ rm .agents/bundles/old-bundle.md
$ git add -A && git commit -m "chore(bundles): remove old bundle"
```

---

## Next Steps

1. **Explore bundles:** `/bundle-list` to see what's available
2. **Find related work:** `/bundle-list --tag [topic]` to find your domain
3. **Load research:** `/bundle-load [bundle-name]` to use existing findings
4. **Continue work:** Use loaded bundle with `/plan`, `/implement`, etc.

**Workflow:**
```bash
/bundle-list redis           # Find redis bundles
/bundle-load redis-caching   # Load research
/plan redis-caching.md       # Create plan
/bundle-save redis-plan      # Save plan
```

**Questions?** Use `/bundle-list --help` for more options.
