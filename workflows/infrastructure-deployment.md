# Infrastructure Deployment Workflow

**Purpose:** Orchestrate infrastructure deployment with validation gates at every phase

**Composes:** `cluster-reality-check` -> `tracer-bullet` -> `phase-gate`

**Failure Patterns Prevented:** 1, 3, 4, 5, 9, 11

---

## Overview

This workflow ensures infrastructure deployments succeed by:
1. Validating assumptions before planning
2. Testing critical paths with tracer bullets
3. Gating each phase before proceeding
4. Capturing learnings for future improvements

```
Phase R: Research with Reality Check
    ↓ (Gate: All APIs/images verified?)
Phase 0: Tracer Bullets
    ↓ (Gate: All bullets pass?)
Phase P: Planning
    ↓ (Gate: Plan approved?)
Phase I: Implementation with Gates
    ↓ (Gate: Each phase validated?)
Phase V: Validation & Retrospective
```

---

## Prerequisites

Before starting this workflow:

- [ ] Fresh context window (<20% used)
- [ ] Target cluster accessible (`oc whoami` works)
- [ ] Appropriate permissions (can create resources in namespace)
- [ ] Clear understanding of what you're deploying
- [ ] Time budget established (deployment can take 30min - 2h)

---

## Phase R: Research with Reality Check

**Goal:** Understand what you're deploying AND validate assumptions against cluster

**Time Budget:** 20-40% of total time

### Step R.1: Conduct Research

```markdown
# Use /research command
/research "Deploy [component] on OpenShift"

# Research should produce:
- Understanding of component architecture
- List of required APIs/CRDs
- List of required images
- List of operators needed
- Configuration requirements
- Known constraints/limitations
```

### Step R.2: Extract Assumptions

From research findings, document:

```yaml
assumed_apis:
  - <api-group>/<version>/<kind>
  - ...

planned_images:
  - <registry>/<image>:<tag>
  - ...

operators:
  - <operator-name>
  - ...

configuration_requirements:
  - <requirement-1>
  - ...
```

### Step R.3: Invoke cluster-reality-check

```markdown
# Invoke skill with extracted assumptions
cluster-reality-check:
  assumed_apis: [from step R.2]
  planned_images: [from step R.2]
  operators: [from step R.2]
  namespace: <target-namespace>
```

### Step R.4: Invoke divergence-check

```markdown
# If research used external documentation
divergence-check:
  upstream_source: <documentation-url>
  local_environment: "OpenShift <version>"
  features_to_verify: [from research]
```

### Gate R: Research Complete?

| Check | Status |
|-------|--------|
| Research findings documented | [ ] |
| All APIs verified exist | [ ] |
| All images verified pullable | [ ] |
| All operators verified ready | [ ] |
| No HIGH severity divergences | [ ] |
| Required adjustments documented | [ ] |

**Decision:**
- All checks pass -> Proceed to Phase 0
- Any check fails -> Continue research with new constraints

---

## Phase 0: Tracer Bullets

**Goal:** Validate critical assumptions with minimal deployments before full planning

**Time Budget:** 10-15% of total time

### Step 0.1: Identify Critical Assumptions

From research, identify assumptions that if wrong, would invalidate the entire plan:

```markdown
Critical Assumptions:
1. [Assumption]: [Impact if wrong]
2. [Assumption]: [Impact if wrong]
3. [Assumption]: [Impact if wrong]
```

**Common critical assumptions:**
- Operator accepts expected API version
- Image can be pulled and runs
- Admission webhooks accept configuration
- Storage class works as expected
- Network policies allow required traffic

### Step 0.2: Fire Tracer Bullets

For each critical assumption, invoke tracer-bullet skill:

```markdown
# Tracer Bullet 1: API Version
tracer-bullet:
  assumption: "EDB accepts postgresql.k8s.enterprisedb.io/v1"
  minimal_spec: |
    apiVersion: postgresql.k8s.enterprisedb.io/v1
    kind: Cluster
    metadata:
      name: tracer-api-test
    spec:
      instances: 1
      storage:
        size: 1Gi
  success_criteria: "Cluster reaches Ready state"
  timeout: 120s
  cleanup: true

# Tracer Bullet 2: Image Pull
tracer-bullet:
  assumption: "dify-api image is pullable"
  minimal_spec: |
    apiVersion: v1
    kind: Pod
    metadata:
      name: tracer-image-test
    spec:
      containers:
        - name: test
          image: langgenius/dify-api:0.11.1
          command: ["sleep", "10"]
      restartPolicy: Never
  success_criteria: "Pod reaches Running state"
  timeout: 60s
  cleanup: true
```

### Step 0.3: Analyze Results

Document tracer bullet outcomes:

```markdown
Tracer Bullet Results:
| Bullet | Assumption | Result | Evidence |
|--------|------------|--------|----------|
| 1 | EDB API v1 | PASS | Cluster Ready in 45s |
| 2 | dify-api image | FAIL | ImagePullBackOff |
| 3 | ... | ... | ... |
```

### Gate 0: All Tracer Bullets Pass?

**Decision:**
- All PASS -> Proceed to Phase P
- Any FAIL -> Return to Phase R with findings

```markdown
# If bullet fails:
Failed Assumption: [assumption]
Evidence: [error message, events]
Required Action: [what must change]

# Return to research to find alternative approach
```

---

## Phase P: Planning

**Goal:** Create detailed implementation plan with validated assumptions

**Time Budget:** 15-25% of total time

### Step P.1: Create Plan

With validated assumptions, create implementation plan:

```markdown
# Use /plan command
/plan [component] deployment

# Plan should include:
- Per-phase breakdown
- Exact file:line specifications
- Validation commands per phase
- Rollback procedure
- Success criteria
```

### Step P.2: Include Phase Validation

Every phase in plan MUST have:

```yaml
phase_N:
  name: "[Phase Name]"
  resources:
    - <resource-1>
    - <resource-2>
  validation_commands:
    - "<command-1>"
    - "<command-2>"
  success_criteria:
    - "<criteria-1>"
    - "<criteria-2>"
  rollback:
    - "<rollback-command>"
```

### Step P.3: Human Review

Present plan for approval:

```markdown
## Plan Summary

**Component:** [what]
**Target:** [where]
**Phases:** [how many]
**Estimated Time:** [duration]

### Phase Breakdown
| Phase | Name | Resources | Validation |
|-------|------|-----------|------------|
| 1 | ... | ... | ... |
| 2 | ... | ... | ... |

### Risk Assessment
- [Risk 1]: [Mitigation]
- [Risk 2]: [Mitigation]

### Rollback Strategy
[How to undo if needed]

---
Approve? (yes/no/revise)
```

### Gate P: Plan Approved?

**Decision:**
- Approved -> Proceed to Phase I
- Revise -> Update plan, re-present
- Rejected -> Return to research

---

## Phase I: Implementation with Gates

**Goal:** Execute plan with validation after every phase

**Time Budget:** 30-40% of total time

### Implementation Loop

```
For each phase in plan:
  1. Implement phase resources
  2. Invoke phase-gate skill
  3. If PASS: Commit, continue
  4. If FAIL: Stop, debug
```

### Step I.N: Implement Phase N

```bash
# Create/modify resources as specified in plan
# Example:
oc apply -f phase-N-resources.yaml
```

### Step I.N+1: Phase Gate

```markdown
# Invoke phase-gate skill
phase-gate:
  phase_number: N
  phase_name: "[Name from plan]"
  resources_deployed: [list from plan]
  validation_commands: [from plan]
  success_criteria: [from plan]
  rollback_procedure: [from plan]
```

### Step I.N+2: Commit or Stop

**If phase-gate PASS:**
```bash
git add .
git commit -m "phase N: [description]"
# Continue to phase N+1
```

**If phase-gate FAIL:**
```markdown
STOP. Do not proceed.

Failure Details:
- Phase: N
- Failed Validation: [which]
- Error: [message]
- Evidence: [events, logs]

Options:
1. Fix issue, re-validate phase N
2. Rollback phase N, investigate
3. Return to planning with findings
```

### Gate I: All Phases Complete?

Continue loop until all phases pass.

---

## Phase V: Validation & Retrospective

**Goal:** Final validation and learning capture

**Time Budget:** 10-15% of total time

### Step V.1: Full Validation Suite

Run comprehensive validation:

```bash
# Syntax validation
make ci-all

# Resource validation
oc get all -n <namespace>

# Health checks
curl -f http://<service>/health

# Functional test
[component-specific tests]
```

### Step V.2: Rollback Test

Verify rollback procedure works:

```bash
# Test rollback of last phase
git revert HEAD --no-commit

# Verify resources can be removed
oc delete -f phase-N-resources.yaml --dry-run=server

# Abort revert (we're just testing)
git checkout .
```

### Step V.3: Run Retrospective

```markdown
# Invoke /retro command
/retro infrastructure-deployment

# Capture:
- What went well
- What diverged from plan
- Unexpected issues encountered
- Skills/workflows that helped
- Skills/workflows that were missing
- Learnings for next deployment
```

### Step V.4: Document Deployment

Create deployment documentation:

```markdown
# [Component] Deployment

**Deployed:** YYYY-MM-DD
**Target:** <cluster>/<namespace>
**Version:** <version>

## Architecture
[Diagram or description]

## Components
- [Component 1]: [Status]
- [Component 2]: [Status]

## Access
- URL: <route>
- Credentials: <secret-reference>

## Operations
- Health check: <command>
- Logs: <command>
- Restart: <command>

## Known Issues
- [Issue]: [Workaround]
```

### Gate V: Deployment Complete?

| Check | Status |
|-------|--------|
| Full validation passes | [ ] |
| Rollback tested | [ ] |
| Retrospective complete | [ ] |
| Documentation created | [ ] |
| Learnings captured | [ ] |

**Decision:**
- All pass -> Deployment complete
- Any fail -> Address before declaring done

---

## Failure Handling

### Research Phase Failure

```markdown
Symptom: Reality check shows divergences
Action:
1. Document divergences
2. Continue research for alternatives
3. Update assumptions
4. Re-run reality check
5. Repeat until all validated
```

### Tracer Bullet Failure

```markdown
Symptom: Critical assumption invalid
Action:
1. Document failure evidence
2. Identify root cause
3. Return to research phase
4. Find alternative approach
5. Create new tracer bullet
6. Repeat until pass
```

### Phase Gate Failure

```markdown
Symptom: Phase validation fails
Action:
1. STOP immediately
2. Capture state (events, logs)
3. Identify root cause
4. Options:
   a. Fix issue, re-validate
   b. Rollback phase, investigate
   c. Return to planning
5. Never proceed with failing gate
```

### Implementation Rollback

```markdown
# If rollback needed:
1. Identify rollback point (which phase)
2. Execute rollback commands from plan
3. Verify resources removed
4. Document what happened
5. Return to appropriate phase
```

---

## Time Budget Guidelines

| Phase | Simple Deploy | Medium Deploy | Complex Deploy |
|-------|---------------|---------------|----------------|
| R (Research) | 10 min | 30 min | 60 min |
| 0 (Tracer) | 5 min | 15 min | 30 min |
| P (Plan) | 10 min | 20 min | 40 min |
| I (Implement) | 15 min | 45 min | 90 min |
| V (Validate) | 5 min | 15 min | 30 min |
| **Total** | **45 min** | **2 hours** | **4 hours** |

---

## Integration Points

### Skills Used

| Phase | Skills |
|-------|--------|
| R | cluster-reality-check, divergence-check |
| 0 | tracer-bullet |
| P | (none - Claude orchestration) |
| I | phase-gate |
| V | (none - validation commands) |

### Commands Used

| Phase | Commands |
|-------|----------|
| R | /research |
| P | /plan |
| V | /retro |

### Related Workflows

- `assumption-validation` - Subset of Phase R
- `post-work-retro` - Detailed version of Phase V

---

## Example: Dify Deployment

### Phase R Summary
```
Research: Dify multi-container application
Reality Check: 2 HIGH divergences (images, volumes)
Divergence Check: Docker -> OpenShift translation needed
Gate R: FAIL - Must resolve image and volume issues
```

### Phase 0 Summary
```
Tracer 1: EDB database cluster - PASS
Tracer 2: Redis pod - PASS
Tracer 3: Dify API image - FAIL (signature policy)
Gate 0: FAIL - Return to research for image solution
```

### Phase R (Iteration 2)
```
Solution: Mirror images to internal registry
Reality Check: All images now pullable
Gate R: PASS
```

### Phase 0 (Iteration 2)
```
Tracer 3 (retry): Dify API image - PASS
Gate 0: PASS
```

### Phase P Summary
```
Plan: 4 phases
- Phase 1: Namespace and secrets
- Phase 2: Database (EDB cluster)
- Phase 3: Supporting services (Redis, Weaviate)
- Phase 4: Application deployments
Human Review: APPROVED
Gate P: PASS
```

### Phase I Summary
```
Phase 1: Namespace/secrets - Gate PASS
Phase 2: Database - Gate PASS (cluster ready in 90s)
Phase 3: Supporting services - Gate PASS
Phase 4: Application - Gate FAIL (quota exceeded)
  -> Fix: Request quota increase
  -> Retry Phase 4 - Gate PASS
Gate I: PASS (all phases complete)
```

### Phase V Summary
```
Full validation: PASS
Rollback test: PASS
Retrospective: Completed
Documentation: Created
Gate V: PASS - Deployment complete
```

---

## Success Criteria

Infrastructure deployment is successful when:

- [ ] All research assumptions validated
- [ ] All tracer bullets passed
- [ ] Plan approved by human
- [ ] All phases passed gates
- [ ] Full validation suite passes
- [ ] Rollback procedure verified
- [ ] Retrospective completed
- [ ] Documentation created
- [ ] Zero unresolved issues

---

**Remember:** Every gate exists to catch problems early. A failing gate is not a failure - it's the workflow working as designed. The failure would be proceeding despite a failing gate.
