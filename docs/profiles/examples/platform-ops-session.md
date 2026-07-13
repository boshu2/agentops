# Example Session: Platform Operations

**Profile**: platform-ops
**Scenario**: Respond to production incident
**Duration**: ~45 min

---

## Session Flow

### 1. Incident Detection (5 min)

```
User: Alert: High error rate on order-service
```

**Skills loaded**: operations

**Immediate Actions**:
- Assess severity (P1 - major functionality)
- Check user impact (orders failing)
- Start incident timeline

---

### 2. Investigation (15 min)

```
User: Investigate the error spike
```

**Skills loaded**: operations, monitoring

**Actions**:
- Check error logs for patterns
- Review recent deployments
- Analyze metrics dashboards
- Identify root cause hypothesis

**Findings**:
- Database connection pool exhausted
- Started after 10am traffic spike
- Pool sized for normal load, not peak

---

### 3. Mitigation (10 min)

```
User: Fix the connection pool issue
```

**Skills loaded**: operations, validation

**Actions**:
- Increase connection pool size
- Deploy configuration change
- Monitor recovery
- Validate tracer request succeeds

**Result**: Error rate back to normal

---

### 4. Documentation (10 min)

```
User: Document this incident
```

**Skills loaded**: meta

**Actions**:
- Create incident timeline
- Document root cause
- Write action items
- Update runbook

**Output**: Causal report in `.agents/council/` when using `/postmortem`

---

### 5. Prevention (5 min)

```
User: Add alerting for this
```

**Skills loaded**: monitoring

**Actions**:
- Add connection pool utilization alert
- Link to new runbook
- Set appropriate threshold

---

## Skills Used Summary

| Skill | When | Purpose |
|-------|------|---------|
| operations | Detection, Investigation | Incident response |
| monitoring | Investigation, Prevention | Metrics and alerts |
| validation | Mitigation | Verify fix works |
| meta | Documentation | Postmortem capture |

---

## Session Outcome

- ✅ Incident mitigated
- ✅ Root cause identified
- ✅ Postmortem documented
- ✅ Runbook updated
- ✅ Alert added

**Time**: ~45 min (MTTR)
