# Example Session: Platform Operations Profile

**Profile**: `platform-ops`
**Scenario**: Production incident - Redis cache cluster failing, causing API timeouts
**Duration**: ~45 minutes
**Outcome**: Incident resolved, postmortem completed

---

## Session Start (2:17 PM - PagerDuty Alert)

**Alert**: 🚨 P1 - API latency p99 > 5s (SLO: 500ms)

**User**: "Read CLAUDE.md - APIs are timing out, users reporting errors"

**System Auto-Detects**:
- Keywords: "incident", "timeouts", "errors" → platform-ops profile
- Priority: P1 (auto-load monitoring + incident agents)
- Context loaded: 3k (foundational) + 8k (platform-ops incident response) = 11k tokens (5.5%)

**Claude**: "Loading platform-ops profile for incident response. Let me gather diagnostics."

**Loaded artifacts**:
- Incident agents (incidents-response, monitoring-alerts-runbooks)
- Monitoring agents (monitoring-slo-dashboards)
- Deployment agents (argocd-debug-sync, deployments-rollback-automation)
- Skills: validate.sh, sync.sh

---

## Phase 1: Triage (2:17-2:22 PM) - 5 minutes

### 1.1 Gather Symptoms

**Claude runs diagnostics**:

```bash
# Check API health
kubectl get pods -n production | grep api
# api-server-1: Running but CrashLoopBackOff recently
# api-server-2: Running
# api-server-3: CrashLoopBackOff

# Check recent events
kubectl get events -n production --sort-by='.lastTimestamp' | tail -20
# Multiple "Failed to connect to redis" errors
```

**Prometheus queries**:
```promql
# API latency
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
# Result: 8.4 seconds (16x SLO breach)

# Error rate
rate(http_requests_total{status=~"5.."}[5m])
# Result: 12% (120x SLO breach)
```

### 1.2 Initial Hypothesis

**Claude analyzes**:
- ✅ Symptom: API timeouts
- ✅ Recent events: Redis connection failures
- ✅ Pattern: API pods restarting
- 🔍 Hypothesis: Redis cluster is down or unreachable

**Severity assessment**: P1 (major service degradation, 12% error rate)

---

## Phase 2: Debug Root Cause (2:22-2:28 PM) - 6 minutes

### 2.1 Check Redis Cluster

**Claude investigates**:

```bash
# Check Redis pods
kubectl get pods -n production | grep redis
# redis-cluster-0: Running
# redis-cluster-1: Running
# redis-cluster-2: CrashLoopBackOff ← Problem!

# Check Redis logs
kubectl logs redis-cluster-2 -n production --tail=50
# Error: "Insufficient memory to allocate 1024MB"
# Error: "OOM kill by kernel"
```

**Root cause found**: Redis pod OOM killed (out of memory)

### 2.2 Verify Impact Scope

**Claude checks**:

```bash
# How many API requests use cache?
# Grafana dashboard: 85% of requests hit Redis

# What happens when Redis is down?
# API code: Falls back to database queries (slower)
# Result: 10x latency increase (50ms → 500ms → 5000ms cascade)
```

**Impact**: 85% of traffic affected, cascading failures

---

## Phase 3: Immediate Mitigation (2:28-2:32 PM) - 4 minutes

### 3.1 Restore Redis Cluster

**Claude executes**:

```bash
# Check Redis resource limits
kubectl describe pod redis-cluster-2 -n production
# Memory limit: 512Mi
# Memory request: 512Mi
# Actual usage before crash: 498Mi → 512Mi (hit limit)

# Quick fix: Increase memory limit temporarily
kubectl patch deployment redis-cluster -n production --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value":"1Gi"}]'

# Verify pods restart successfully
kubectl rollout status deployment/redis-cluster -n production
# ✅ deployment "redis-cluster" successfully rolled out
```

### 3.2 Verify Recovery

**Claude monitors**:

```bash
# Check API latency
# Prometheus: p99 latency drops from 8.4s → 520ms (within SLO)

# Check error rate
# Prometheus: Error rate drops from 12% → 0.3% (within SLO)

# Check Redis health
kubectl exec redis-cluster-0 -n production -- redis-cli cluster info
# cluster_state: ok
# cluster_slots_ok: 16384
```

**Recovery confirmed**: All metrics back to normal

**Time to mitigation**: 15 minutes (2:17 PM → 2:32 PM)

---

## Phase 4: Permanent Fix (2:32-2:40 PM) - 8 minutes

### 4.1 Analyze Memory Growth

**Claude loads**: monitoring-slo-dashboards agent

**Investigation**:

```bash
# Query Prometheus for Redis memory growth
rate(redis_memory_used_bytes[7d])
# Result: Steady 2MB/hour growth over past 2 weeks

# Root cause: Cache eviction policy not configured
kubectl exec redis-cluster-0 -n production -- redis-cli CONFIG GET maxmemory-policy
# maxmemory-policy: noeviction ← Problem!
```

**Findings**:
- Redis set to `noeviction` (fills up, then fails)
- Should be `allkeys-lru` (evict least recently used when full)
- Memory limit too low for current traffic

### 4.2 Implement Proper Configuration

**Claude loads**: sites-harmonize agent

**Changes to config.env**:

```bash
# Edit sites/production/config.env
REDIS_MEMORY_LIMIT=2Gi  # Was: 512Mi
REDIS_EVICTION_POLICY=allkeys-lru  # Was: noeviction
REDIS_MAXMEMORY_SAMPLES=5  # LRU sampling
```

**Claude runs harmonize**:

```bash
# Render new values.yaml
python3 releases/1.15/render_values.py \
  --env-file sites/production/config.env \
  --output apps/redis-cluster/values.yaml

# Validate changes
make test-app APP=redis-cluster
# ✅ Validation passed
```

**Git commit**:

```bash
git add sites/production/config.env apps/redis-cluster/values.yaml
git commit -m "$(cat <<'EOF'
fix(redis): increase memory limit and enable LRU eviction

## Context
P1 incident: Redis OOM killed causing 12% API error rate.
Memory limit (512Mi) insufficient for current traffic + noeviction policy.

## Solution
- Increased memory limit: 512Mi → 2Gi (4x headroom)
- Enabled LRU eviction: noeviction → allkeys-lru
- Added maxmemory-samples config for eviction quality

## Learning
- Redis memory growth: 2MB/hour = ~1.4GB/month
- Noeviction policy causes hard failures (OOM kill)
- LRU eviction prevents memory exhaustion gracefully
- Should have monitoring alerts for memory growth trends

## Impact
- Incident resolution time: 15 min (triage + mitigation)
- Prevents future OOM incidents
- Headroom for 60 days of traffic growth
- SLO compliance restored (p99: 520ms, errors: 0.3%)

EOF
)"

git push origin main
```

**ArgoCD sync**: Deploys new Redis configuration automatically

---

## Phase 5: Verification & Monitoring (2:40-2:50 PM) - 10 minutes

### 5.1 Post-Deployment Validation

**Claude monitors rollout**:

```bash
# Verify deployment
kubectl rollout status deployment/redis-cluster -n production
# ✅ Successfully rolled out

# Check Redis config applied
kubectl exec redis-cluster-0 -n production -- redis-cli CONFIG GET maxmemory
# maxmemory: 2147483648 (2GB) ✅

kubectl exec redis-cluster-0 -n production -- redis-cli CONFIG GET maxmemory-policy
# maxmemory-policy: allkeys-lru ✅
```

### 5.2 SLO Compliance Check

**Claude queries Prometheus**:

```promql
# API latency p99 (target: <500ms)
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[10m]))
# Result: 485ms ✅

# Error rate (target: <0.1%)
100 * rate(http_requests_total{status=~"5.."}[10m]) / rate(http_requests_total[10m])
# Result: 0.08% ✅

# Redis memory usage (should be under 80% of limit)
100 * redis_memory_used_bytes / redis_memory_limit_bytes
# Result: 24% (480MB / 2GB) ✅
```

**All SLOs met** ✅

### 5.3 Alert Tuning

**Claude loads**: monitoring-alerts-runbooks agent

**New alert added**:

```yaml
# prometheus-rules.yaml
- alert: RedisMemoryGrowthTrend
  expr: |
    predict_linear(redis_memory_used_bytes[7d], 86400 * 30) >
    redis_memory_limit_bytes * 0.8
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "Redis memory projected to hit 80% in 30 days"
    description: "Current growth rate will exhaust memory. Review eviction policy or increase limit."
```

---

## Phase 6: Postmortem (2:50-3:02 PM) - 12 minutes

### 6.1 Timeline

**Claude loads**: incidents-postmortems agent

**Generated timeline**:

```markdown
## Incident Timeline

**Incident ID**: INC-2025-11-09-001
**Severity**: P1
**Duration**: 45 minutes (2:17 PM - 3:02 PM)
**Impact**: 12% error rate, 8.4s p99 latency

### Timeline

| Time | Event |
|------|-------|
| 2:17 PM | PagerDuty alert: API latency SLO breach |
| 2:22 PM | Root cause identified: Redis OOM |
| 2:28 PM | Mitigation applied: Increased memory limit |
| 2:32 PM | Services recovered, SLOs restored |
| 2:40 PM | Permanent fix deployed (config.env update) |
| 2:50 PM | Monitoring and alerting improved |
| 3:02 PM | Postmortem completed |

### Root Cause

**Primary**: Redis memory limit (512Mi) too low for traffic growth
**Secondary**: Eviction policy (`noeviction`) caused hard failure instead of graceful degradation

### Impact

- **Users affected**: ~15,000 users (12% error rate for 15 minutes)
- **Requests failed**: ~18,000 requests
- **Revenue impact**: Minimal (cached data, not transactions)
- **SLO breach**: p99 latency (500ms → 8400ms), error rate (0.1% → 12%)
```

### 6.2 What Went Well

1. ✅ **Fast detection**: Alert fired immediately when SLO breached
2. ✅ **Clear monitoring**: Prometheus + Grafana showed exact problem
3. ✅ **Quick mitigation**: Memory increase applied in 11 minutes
4. ✅ **Root cause fix**: Config.env updated for permanent solution
5. ✅ **Documentation**: Commit message captured full context

### 6.3 What Went Wrong

1. ❌ **No proactive monitoring**: Memory growth should have triggered alert before OOM
2. ❌ **Incorrect eviction policy**: `noeviction` caused hard failure
3. ❌ **Undersized resources**: 512Mi too small for production traffic

### 6.4 Action Items

**Claude generates**:

| Action | Owner | Due Date | Priority |
|--------|-------|----------|----------|
| Add memory growth trend alert | SRE Team | 2025-11-10 | P0 |
| Review all Redis clusters for eviction policy | Platform Team | 2025-11-12 | P1 |
| Capacity planning review (3-month projection) | SRE Lead | 2025-11-15 | P1 |
| Update runbook with Redis OOM procedure | On-call rotation | 2025-11-16 | P2 |

**Postmortem doc saved**: `docs/incidents/2025-11-09-redis-oom.md`

---

## Session Summary

### Time Breakdown
- Triage: 5 min
- Debug: 6 min
- Mitigation: 4 min
- Permanent fix: 8 min
- Verification: 10 min
- Postmortem: 12 min
- **Total**: 45 minutes

### Incident Metrics
- **Detection time**: <1 min (automated alert)
- **Time to triage**: 5 min
- **Time to mitigation**: 15 min (P1 target: 30 min) ✅
- **Time to resolution**: 23 min (with permanent fix)
- **Time to postmortem**: 45 min (target: 2 hours) ✅

### Agents Used
1. `incidents-response` (triage + coordination)
2. `monitoring-alerts-runbooks` (diagnostics)
3. `monitoring-slo-dashboards` (metrics analysis)
4. `sites-harmonize` (config.env updates)
5. `incidents-postmortems` (postmortem generation)
6. Skills: `validate.sh`, `sync.sh`

### Context Usage
- Peak: 14k tokens (7% of window)
- Average: 11k tokens (5.5% of window)
- Well under 40% rule ✅

### SLO Impact
- **Before**: p99=8.4s, errors=12% (massive breach)
- **After**: p99=485ms, errors=0.08% (within SLO) ✅
- **Recovery time**: 15 minutes

### Learnings Captured
1. Redis eviction policies critical (noeviction → LRU)
2. Proactive memory growth monitoring needed
3. Resource limits should have 3-6 month headroom
4. Pattern: Cache OOM → API cascade failure (add to runbook)

---

## What Made This Efficient

### 1. Profile Auto-Detection
Keywords "incident" + "timeouts" loaded platform-ops profile with incident response agents immediately

### 2. Incident Response Workflow
Built-in workflow: Triage → Debug → Mitigate → Fix → Postmortem

### 3. Integrated Monitoring
Prometheus + Grafana queries built into agents (no context switching)

### 4. Harmonize Pattern
Config.env changes → automated rendering → GitOps deployment

### 5. Runbook Automation
Common incident patterns (Redis OOM) have documented procedures

### 6. Blameless Postmortem
Auto-generated from timeline + root cause analysis

---

## Alternative Without Profile

**Estimated time without platform-ops profile**: ~2-3 hours

**Why slower?**
- ❌ Manual incident procedure lookup ("what's the runbook?")
- ❌ Tool switching (Prometheus → Grafana → kubectl → docs)
- ❌ Missing patterns (Redis OOM not previously documented)
- ❌ Postmortem manual (template not auto-populated)

**With profile**: 45 minutes (3-4x faster)

---

## Follow-Up Actions

**Next Day**:
- Review all Redis clusters (action item from postmortem)
- Implement memory growth trend alert
- Share incident learnings in team retro

**Next Week**:
- Capacity planning review
- Update SRE runbooks with Redis OOM pattern

**Next Month**:
- Quarterly review of resource limits across all services
- Extract "cache cluster sizing" pattern for reuse
