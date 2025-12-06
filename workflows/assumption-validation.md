# Assumption Validation Workflow

**Purpose:** Systematically validate research assumptions before planning

**Composes:** `cluster-reality-check` -> `divergence-check`

**Failure Patterns Prevented:** 1 (Tests Passing Lie), 3 (Copy-Pasta Blindspot), 9 (External Dependency Assumption)

---

## Overview

This workflow validates that research findings match reality before planning begins.

```
Extract Assumptions from Research
    ↓
Invoke cluster-reality-check (APIs, images, operators)
    ↓
Invoke divergence-check (upstream vs local)
    ↓
Combine Results
    ↓
Gate Decision: All pass? Proceed to /plan : Return to /research
```

---

## When to Use

- After `/research` completes for infrastructure work
- Before `/plan` starts
- When research used external documentation
- When deploying to unfamiliar cluster
- When using operators with version-specific behavior

## When NOT to Use

- Code-only changes (no infrastructure)
- Well-known patterns you've deployed many times
- Trivial configuration updates
- When assumptions already validated in previous session

---

## Inputs

| Input | Source | Description |
|-------|--------|-------------|
| Research bundle | `/research` output | Findings with assumptions |
| Target cluster | Environment | Where deployment will happen |
| Upstream docs | Research references | External documentation used |

---

## Process

### Step 1: Extract Assumptions from Research

Parse research findings to identify:

```yaml
# assumptions.yaml

api_assumptions:
  - name: "EDB Cluster API"
    api_group: "postgresql.k8s.enterprisedb.io"
    version: "v1"
    kind: "Cluster"
    source: "EDB documentation"

  - name: "Redis API"
    api_group: "apps"
    version: "v1"
    kind: "Deployment"
    source: "Kubernetes standard"

image_assumptions:
  - name: "Dify API"
    image: "langgenius/dify-api:0.11.1"
    registry: "docker.io"
    source: "Dify documentation"

  - name: "Dify Web"
    image: "langgenius/dify-web:0.11.1"
    registry: "docker.io"
    source: "Dify documentation"

operator_assumptions:
  - name: "EDB Postgres"
    csv_pattern: "edb-pg4k"
    expected_status: "Succeeded"
    source: "Cluster inventory"

configuration_assumptions:
  - name: "PostgreSQL parameters"
    parameter: "shared_preload_libraries"
    expected: "Configurable"
    source: "EDB documentation"

feature_assumptions:
  - name: "Image catalog"
    feature: "imageCatalogRef"
    expected: "Available"
    source: "EDB v1.23 docs"
```

### Step 2: Invoke cluster-reality-check

Test each assumption against actual cluster:

```markdown
# Invoke skill
cluster-reality-check:
  assumed_apis:
    - postgresql.k8s.enterprisedb.io/v1/Cluster
    - apps/v1/Deployment
  planned_images:
    - langgenius/dify-api:0.11.1
    - langgenius/dify-web:0.11.1
  operators:
    - edb-pg4k
  namespace: dify
```

**Capture results:**

```yaml
reality_check_results:
  apis:
    - name: "EDB Cluster API"
      status: "PASS"
      evidence: "oc api-resources shows clusters.postgresql.k8s.enterprisedb.io"

    - name: "Deployment API"
      status: "PASS"
      evidence: "Standard Kubernetes API"

  images:
    - name: "Dify API"
      status: "FAIL"
      evidence: "ImagePullBackOff - signature policy"
      error: "image rejected by signature policy"

    - name: "Dify Web"
      status: "FAIL"
      evidence: "ImagePullBackOff - signature policy"

  operators:
    - name: "EDB Postgres"
      status: "PASS"
      evidence: "edb-pg4k.v1.23.0 in Succeeded state"
```

### Step 3: Invoke divergence-check

Compare upstream documentation to local reality:

```markdown
# Invoke skill
divergence-check:
  upstream_source: "Dify Docker Compose documentation"
  local_environment: "OpenShift 4.14"
  features_to_verify:
    - Container images
    - Volume mounts
    - Networking model
    - Environment configuration

divergence-check:
  upstream_source: "EDB CloudNativePG v1.24 docs"
  local_environment: "EDB Postgres v1.23 on OpenShift"
  features_to_verify:
    - imageCatalogRef feature
    - Monitoring configuration
    - Backup options
```

**Capture results:**

```yaml
divergence_check_results:
  dify_docs:
    - feature: "Image registry"
      upstream: "docker.io"
      local: "Signature policy blocks"
      divergence: "HIGH"
      adjustment: "Mirror to internal registry"

    - feature: "Volume mounts"
      upstream: "Docker named volumes"
      local: "PVC required"
      divergence: "MEDIUM"
      adjustment: "Convert to PVC specs"

  edb_docs:
    - feature: "imageCatalogRef"
      upstream: "v1.24 - supported"
      local: "v1.23 - not available"
      divergence: "HIGH"
      adjustment: "Use imageName instead"

    - feature: "Monitoring"
      upstream: "managed.monitoring: true"
      local: "Manual annotation required"
      divergence: "LOW"
      adjustment: "Add prometheus annotations"
```

### Step 4: Combine Results

Build unified validation report:

```markdown
# Assumption Validation Report

## Summary
| Category | Total | Pass | Fail | Diverge |
|----------|-------|------|------|---------|
| APIs | 2 | 2 | 0 | 0 |
| Images | 2 | 0 | 2 | 0 |
| Operators | 1 | 1 | 0 | 0 |
| Divergences | 4 | - | 2 HIGH | 2 LOW |

## Detailed Results

### APIs Validated
| API | Status | Evidence |
|-----|--------|----------|
| EDB Cluster | PASS | oc api-resources confirms |
| Deployment | PASS | Standard K8s API |

### Images Validated
| Image | Status | Issue | Adjustment |
|-------|--------|-------|------------|
| dify-api | FAIL | Signature policy | Mirror to internal |
| dify-web | FAIL | Signature policy | Mirror to internal |

### Operators Validated
| Operator | Status | Version | Evidence |
|----------|--------|---------|----------|
| EDB Postgres | PASS | v1.23.0 | CSV Succeeded |

### Divergences Found
| Feature | Severity | Upstream | Local | Adjustment |
|---------|----------|----------|-------|------------|
| Image registry | HIGH | docker.io | Blocked | Mirror images |
| imageCatalogRef | HIGH | v1.24 | v1.23 | Use imageName |
| Volume model | MEDIUM | Docker | PVC | Convert specs |
| Monitoring | LOW | managed | Manual | Add annotations |

## Required Adjustments
1. [HIGH] Mirror Dify images to internal registry
2. [HIGH] Use imageName instead of imageCatalogRef for EDB
3. [MEDIUM] Convert Docker volumes to PVC specifications
4. [LOW] Add Prometheus annotations for monitoring
```

### Step 5: Gate Decision

```markdown
## Gate Decision

| Condition | Status |
|-----------|--------|
| All APIs validated | PASS |
| All images validated | FAIL |
| All operators validated | PASS |
| No HIGH divergences | FAIL |

### Decision: DO NOT PROCEED TO PLANNING

Blocking Issues:
1. Dify images cannot be pulled (signature policy)
2. EDB imageCatalogRef not available in installed version

### Required Actions Before Proceeding:
1. Request signature policy exception for Dify images
   OR mirror images to internal registry
2. Update research to use imageName instead of imageCatalogRef

### Next Step:
Return to /research with these constraints:
- Images must come from internal registry or allowed list
- EDB configuration must use v1.23 features only
```

---

## Output Formats

### On All Pass

```
ASSUMPTION VALIDATION: PASS

All assumptions validated against cluster reality.
No divergences require adjustment.

Validated:
- 5 APIs confirmed available
- 3 images confirmed pullable
- 2 operators confirmed ready
- 0 divergences with upstream docs

Go/No-Go: PROCEED to /plan
```

### On Failures Found

```
ASSUMPTION VALIDATION: FAIL

Validation Results:
- APIs: 5/5 PASS
- Images: 1/3 FAIL
- Operators: 2/2 PASS
- Divergences: 2 HIGH, 1 MEDIUM

Blocking Issues:
1. [Image] dify-sandbox:0.2.10 - signature policy blocks
2. [Divergence] imageCatalogRef not in v1.23

Required Actions:
1. Mirror dify-sandbox image
2. Research v1.23 image specification syntax

Go/No-Go: DO NOT PROCEED - return to /research
```

---

## Integration

### With /research

```markdown
# Research workflow integration

1. /research produces findings
2. Invoke assumption-validation workflow
3. If PASS: proceed to /plan
4. If FAIL: continue /research with constraints
```

### With /plan

```markdown
# Planning integration

Only invoke /plan when assumption-validation passes.

If you skip assumption-validation:
- Plan may be based on invalid assumptions
- Implementation will fail at phase gates
- Time wasted on invalid plan
```

### With infrastructure-deployment Workflow

```markdown
# assumption-validation is Phase R of infrastructure-deployment

infrastructure-deployment:
  Phase R:
    - /research
    - assumption-validation  # This workflow
    - Gate R
  Phase 0: tracer-bullets
  ...
```

---

## Examples

### Example 1: Simple Validation (All Pass)

**Context:** Deploying well-known pattern to familiar cluster

```yaml
assumptions:
  apis:
    - apps/v1/Deployment
    - v1/Service
    - route.openshift.io/v1/Route
  images:
    - registry.internal/app:v1.2.3  # Internal image
  operators: []  # No operators needed
```

**Result:**
```
ASSUMPTION VALIDATION: PASS
All 4 assumptions validated.
Proceed to planning.
```

### Example 2: Partial Failure

**Context:** New application with external dependencies

```yaml
assumptions:
  apis:
    - postgresql.k8s.enterprisedb.io/v1/Cluster
    - redis.redis.io/v1/Redis  # Doesn't exist!
  images:
    - langgenius/dify-api:0.11.1  # Blocked
  operators:
    - edb-pg4k  # Installed
    - redis-operator  # Not installed!
```

**Result:**
```
ASSUMPTION VALIDATION: FAIL

Failed Assumptions:
1. [API] redis.redis.io/v1/Redis - CRD not found
2. [Image] langgenius/dify-api - signature blocked
3. [Operator] redis-operator - not installed

Actions:
1. Install Redis operator OR use StatefulSet instead
2. Mirror Dify images to internal registry

Return to research with constraints.
```

### Example 3: Divergence Only

**Context:** APIs exist but docs don't match reality

```yaml
assumptions:
  apis:
    - postgresql.k8s.enterprisedb.io/v1/Cluster  # EXISTS
  images: []  # Operator manages
  operators:
    - edb-pg4k  # Installed

divergences:
  - feature: "imageCatalogRef"
    upstream_doc: "v1.24 supports imageCatalogRef"
    local_reality: "v1.23 installed, feature not available"
```

**Result:**
```
ASSUMPTION VALIDATION: FAIL

APIs/Images/Operators: All PASS
Divergences: 1 HIGH

HIGH Divergence:
- imageCatalogRef documented in v1.24
- Local cluster has v1.23
- Feature not available

Action:
Update research to use v1.23 syntax (imageName)

Return to research with version constraint.
```

---

## Quick Reference Checklist

### Before Starting
- [ ] Research bundle available
- [ ] Target cluster accessible
- [ ] Upstream documentation identified

### During Validation
- [ ] All APIs checked
- [ ] All images tested
- [ ] All operators verified
- [ ] Divergences documented

### Gate Decision
- [ ] Zero FAIL status
- [ ] Zero HIGH divergences
- [ ] Required adjustments documented
- [ ] Clear proceed/return decision

---

## Anti-Patterns

### Skipping Validation

```markdown
# BAD
/research
/plan  # Skipped assumption-validation!
```

**Result:** Plan based on invalid assumptions, fails during implementation.

### Partial Validation

```markdown
# BAD: Only checking APIs
cluster-reality-check with apis only
# Skipped images, operators, divergences
```

**Result:** Image pull failures discovered during implementation.

### Ignoring Divergences

```markdown
# BAD: Proceeding despite HIGH divergence
divergence-check shows HIGH: imageCatalogRef not available
"Let's try anyway"
```

**Result:** Admission webhook rejects spec, debug spiral.

---

## Success Criteria

Assumption validation is successful when:

- [ ] All API assumptions verified
- [ ] All image assumptions verified
- [ ] All operator assumptions verified
- [ ] All divergences documented with severity
- [ ] No unresolved HIGH severity issues
- [ ] Clear go/no-go recommendation
- [ ] Required adjustments actionable

---

**Remember:** The cost of 10 minutes validating assumptions is far less than hours debugging why "working" code fails in your environment.
