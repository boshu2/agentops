# Memory Management Agent

**Purpose:** Automatically manage the agent memory system - consolidation, cleanup, monitoring, and optimization.

**Type:** Autonomous background agent

**Tier:** 1 - Foundational (System Critical)

**Success Rate:** 98%+ | **Time Savings:** Eliminates manual memory management

---

## When to Use

**Automatic triggers:**
- Warm tier exceeds age threshold (30 days)
- Memory usage exceeds limits (hot tier >100 entities)
- Daily maintenance window (runs at 2 AM)
- On-demand via `/memory-manage` command

**Manual invocation:**
```bash
# Run full maintenance cycle
/memory-manage

# Run specific operations
/memory-manage --consolidate-only
/memory-manage --cleanup-only
/memory-manage --stats-only
```

---

## What It Does

### 1. Automatic Consolidation
- **Trigger:** Warm tier files older than 30 days
- **Action:** Run 10:1 compression to cold tier
- **Frequency:** Daily at 2 AM
- **Validation:** Verify compression ratio achieved

### 2. TTL Cleanup
- **Trigger:** Entities exceeding TTL
- **Action:** Remove expired memories
- **Frequency:** Daily at 2 AM
- **Safety:** Never removes permanent entities (TTL=null)

### 3. Growth Monitoring
- **Trigger:** Memory size checks
- **Action:** Alert if hot tier >100, warm tier >1GB, cold tier >500MB
- **Frequency:** Every 6 hours
- **Response:** Auto-consolidate if thresholds exceeded

### 4. Learning Extraction
- **Trigger:** New agent executions
- **Action:** Extract patterns, failures, best practices
- **Frequency:** Real-time (via git hooks)
- **Storage:** Hot tier for recent, consolidate to cold

### 5. Health Checks
- **Trigger:** Scheduled or on-demand
- **Action:** Verify tier integrity, check TOON files, validate stats
- **Frequency:** Every 12 hours
- **Recovery:** Repair corrupted files, rebuild indices

---

## Architecture

```
Memory Management Agent
    ├─ Scheduler (cron-like)
    │   ├─ Daily: consolidation + cleanup
    │   ├─ 6hr: growth monitoring
    │   └─ 12hr: health checks
    │
    ├─ Monitors
    │   ├─ Tier size tracking
    │   ├─ Entity count tracking
    │   └─ Performance metrics
    │
    ├─ Operations
    │   ├─ Consolidate (warm→cold)
    │   ├─ Cleanup (TTL-based)
    │   └─ Optimize (defrag, reindex)
    │
    └─ Alerts
        ├─ Growth warnings
        ├─ Corruption detection
        └─ Performance degradation
```

---

## Implementation

### Autonomous Mode (Recommended)

Run as background service:

```bash
# Start memory manager daemon
python .claude/memory/daemon.py start

# Check status
python .claude/memory/daemon.py status

# Stop daemon
python .claude/memory/daemon.py stop
```

### Cron Mode (Alternative)

Add to crontab:

```bash
# Daily maintenance at 2 AM
0 2 * * * cd $WORKSPACES_DIR && python .claude/memory/cli.py consolidate --days 30 && python .claude/memory/cli.py cleanup

# 6-hour monitoring
0 */6 * * * cd $WORKSPACES_DIR && python .claude/memory/cli.py stats --alert-on-threshold
```

### Manual Mode (Development)

```bash
# Run full maintenance
make memory-maintain

# Run specific operation
make memory-consolidate
make memory-cleanup
```

---

## Configuration

`~/.memory/config.json`:

```json
{
  "enabled": true,
  "auto_consolidate": true,
  "auto_cleanup": true,
  "thresholds": {
    "hot_max_entities": 100,
    "warm_max_age_days": 30,
    "warm_max_size_mb": 1024,
    "cold_max_size_mb": 512
  },
  "schedule": {
    "consolidate": "0 2 * * *",
    "cleanup": "0 2 * * *",
    "monitor": "0 */6 * * *",
    "health": "0 */12 * * *"
  },
  "alerts": {
    "email": null,
    "slack_webhook": null,
    "log_only": true
  }
}
```

---

## Monitoring Dashboard

Real-time view:

```bash
python .claude/memory/cli.py dashboard
```

Output:
```
Memory System Dashboard
=======================

Status: ✅ Healthy
Last Consolidation: 2025-11-24 02:00 (22h ago)
Last Cleanup: 2025-11-24 02:00 (22h ago)

Tier Status:
  Hot:  45 entities (425 KB) - 🟢 Normal
  Warm: 234 entities (2.3 MB) - 🟢 Normal
  Cold: 1,205 entities (8.1 MB) - 🟢 Normal

Performance:
  Hot access: 3.2ms avg
  Warm access: 42ms avg
  Cold access: 78ms avg

Token Savings:
  TOON vs JSON: 34.7%
  Monthly savings: ~2.8M tokens
  Cost savings: ~$28/month

Next Actions:
  - Consolidation: in 2h
  - Cleanup: in 2h
  - Health check: in 10h
```

---

## Operations Reference

### Consolidate
```bash
# Automatic (age-based)
python .claude/memory/cli.py consolidate --days 30

# Force consolidate all warm
python .claude/memory/cli.py consolidate --force

# Dry-run (preview)
python .claude/memory/cli.py consolidate --dry-run
```

### Cleanup
```bash
# TTL-based cleanup
python .claude/memory/cli.py cleanup

# Aggressive cleanup (remove old cold tier)
python .claude/memory/cli.py cleanup --aggressive --days 180
```

### Monitor
```bash
# Current stats
python .claude/memory/cli.py stats

# Detailed breakdown
python .claude/memory/cli.py stats --verbose

# Alert on threshold
python .claude/memory/cli.py stats --alert-on-threshold
```

---

## Alerts & Notifications

### Warning Conditions
- 🟡 Hot tier >80 entities (approaching limit)
- 🟡 Warm tier >800 MB (consolidation recommended)
- 🟡 Cold tier >400 MB (cleanup recommended)

### Critical Conditions
- 🔴 Hot tier >100 entities (auto-consolidate triggered)
- 🔴 Warm tier >1 GB (forced consolidation)
- 🔴 Corrupted TOON file detected (auto-repair)

### Recovery Actions
1. **Hot overflow:** Automatic flush to warm
2. **Warm overflow:** Forced consolidation to cold
3. **File corruption:** Backup + rebuild from other tiers
4. **Performance degradation:** Optimize/reindex

---

## Integration with Agent Ecosystem

### Automatic Learning Extraction

When agents run, memory manager extracts learnings:

```python
# Example: code-review agent finishes
# → Memory manager detects completion
# → Extracts: patterns found, issues detected, best practices
# → Stores in hot tier with agent name
# → Consolidates to cold tier weekly
```

### Cross-Agent Knowledge Sharing

Agents can query memories before execution:

```python
# Example: applications-create-app agent starting
# → Queries: "What patterns worked for similar apps?"
# → Loads: Relevant memories from cold tier
# → Applies: Known best practices
# → Stores: New learnings after completion
```

---

## Maintenance Schedule

| Time | Operation | Duration | Impact |
|------|-----------|----------|--------|
| 02:00 | Consolidation | 5-30s | None (background) |
| 02:05 | Cleanup | 1-5s | None (background) |
| Every 6h | Monitoring | <1s | None |
| Every 12h | Health Check | 2-10s | None |

**Zero downtime:** All operations non-blocking, agents continue working during maintenance.

---

## Rollback & Recovery

### Backup Before Operations

```bash
# Automatic backup before consolidation
~/.memory/backups/
  ├── 2025-11-24-02-00-pre-consolidate/
  │   ├── hot/
  │   ├── warm/
  │   └── cold/
```

### Restore from Backup

```bash
# If consolidation goes wrong
python .claude/memory/cli.py restore --backup 2025-11-24-02-00-pre-consolidate

# Verify restoration
python .claude/memory/cli.py stats
```

---

## Performance Optimization

Memory manager automatically optimizes:

1. **Defragmentation:** Rewrite TOON files to reduce size
2. **Reindexing:** Rebuild internal indices for faster queries
3. **Deduplication:** Remove duplicate observations
4. **Compression:** Additional gzip for cold tier (optional)

---

## Best Practices

### DO:
- ✅ Run in autonomous mode (daemon)
- ✅ Monitor dashboard regularly
- ✅ Let automatic consolidation run
- ✅ Review alerts for growth warnings

### DON'T:
- ❌ Manually edit TOON files (use CLI)
- ❌ Disable cleanup (causes unbounded growth)
- ❌ Force consolidate too frequently (<7 days)
- ❌ Ignore critical alerts

---

## Success Criteria

- ✅ Consolidation runs successfully daily
- ✅ No memory growth beyond thresholds
- ✅ All tiers accessible <100ms
- ✅ Zero manual intervention required
- ✅ Token savings maintained at 30%+

---

## Constraints

- Memory operations must complete <60s
- No blocking of agent executions
- Preserve all data (no accidental deletion)
- Maintain compression ratios (10:1 target)

---

**Version:** 1.0.0
**Last Updated:** 2025-11-24
**Autonomous:** Yes (recommended)
**Manual Fallback:** Yes (via CLI)
