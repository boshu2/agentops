# Documentation Validation Rules

## Coverage Metrics by Type

| Type | Key Metric | Target | How Measured |
|------|-----------|--------|--------------|
| CODING | Entity Coverage | >= 90% | Documented services / total services |
| CODING | Signpost Accuracy | 100% | Referenced functions exist |
| INFORMATIONAL | Frontmatter Valid | >= 95% | Required fields present |
| INFORMATIONAL | Links Valid | 100% | All internal links resolve |
| OPS | Values.yaml Coverage | >= 80% | Documented keys / total keys |
| OPS | Golden Completeness | 100% | Required sections present |

---

## INFORMATIONAL Validation

No standalone validator script ships with this skill. Run the checks below
manually — for doc-file presence coverage use the shipped
`skills/doc/scripts/audit-oss-docs.sh`; for link/orphan/path checks write a
short throwaway Python script in the target repo (not bash: bash loops are
O(n*m) and time out on large repos, while Python processes 350+ files in
seconds with cleaner regex extraction).

### Checks Performed

1. **Broken Links** - ALL internal .md links resolved
2. **Orphaned Docs** - Files not referenced from any index
3. **Index Completeness** - READMEs reference all subdirectories
4. **Hardcoded Paths** - Absolute paths like /Users/, /home/

### Output Format

```
CRITICAL: Broken Links (81)
   file.md:42 -> missing.md (not found)

MEDIUM: Orphaned Documents (13)
   path/to/orphan.md

LOW: Hardcoded Paths (2)
   file.md:156 -> /Users/...

SUMMARY: 96 issues (81 critical, 13 medium, 2 low)
```

---

## CODING Validation

### Required Sections (16)

From `code-map-standard` skill:

1. Current Status (one-liner with date)
2. Overview (2-3 sentences)
3. State Machine (ASCII diagram if applicable)
4. Inputs/Outputs (table)
5. Data Flow (ASCII diagram)
6. API Endpoints (table with curl examples)
7. Code Signposts (NO line numbers)
8. Configuration (table)
9. Prometheus Metrics (table + PromQL examples)
10. Error Handling (table)
11. Unit Tests (table)
12. Integration Tests (separate from unit)
13. Example Usage (curl + SDK)
14. Related Features (cross-links)
15. Known Limitations
16. Learnings (What Worked + What We'd Change)

### Signpost Rules

- **NO line numbers** - Functions/classes only
- References must exist in source files
- Use semantic names: `authenticate()`, `UserService`

---

## OPS Validation

### Required Sections

1. Overview with Chart.yaml description
2. Quick Start with install command
3. Values Reference table
4. Dependencies table
5. Environment overrides (dev/staging/prod)
6. Troubleshooting table

### Values.yaml Coverage

Every key in values.yaml should have:
- Description comment or doc reference
- Type specification
- Default value explanation

---

## Coverage Report Format

```
===================================================================
              DOCUMENTATION COVERAGE REPORT
===================================================================
Repository: [REPO_NAME]
Type: [CODING|INFORMATIONAL|OPS]
Generated: [date]

SUMMARY
-------------------------------------------------------------------
Total Features: 25
Documented: 22 (88%)
Missing: 3
Orphaned: 1

MISSING DOCUMENTATION
-------------------------------------------------------------------
| Feature | Priority | Source Files |
|---------|----------|--------------|
| auth-service | P1 | services/auth/*.py |

ORPHANED DOCUMENTATION
-------------------------------------------------------------------
| Document | Last Updated | Action |
|----------|--------------|--------|
| legacy-api.md | 2023-06-15 | Remove |

===================================================================
```

---

## Semantic Validation (CODING repos)

**Structure vs Semantic:** Structural validation checks formatting. Semantic validation checks if claims are TRUE.

### Semantic Metrics

| Check | How | Target |
|-------|-----|--------|
| Status Accuracy | Compare "Status: X" to deployment state | 100% |
| Claim Verification | Cross-ref with ground truth file | 100% |
| Validation Freshness | Status includes date | < 30 days |

### Ground Truth Pattern

Establish ONE authoritative file per domain. Other docs MUST reference, not duplicate.

| Domain | Ground Truth | Pattern |
|--------|--------------|---------|
| Agents | `docs/agents/catalog.md` | Reference via link |
| Images | `charts/*/IMAGE-LIST.md` | Reference via link |
| Config | `values.yaml` | Generate docs from source |

### Status Validation

Valid status formats:

```markdown
## Current Status: ✅ RUNNING
Validated: 2026-01-04 against ocppoc cluster

## Current Status: ❌ FAILED
Status: Accepted=False (CRD exists but not running)
Validated: 2026-01-04 against ocppoc cluster

## Current Status: 📝 PLANNED
Not yet deployed - template only
```

### Semantic Validation Commands

Live verification is opt-in. Before contact, obtain a caller authorization ID
for one exact cluster context and namespace, the credential identity (never the
credential value), and the fixed resource allowlist `pods` and
`agents.kagent.dev`. Allow only `get`/`list`; secrets, ConfigMaps, logs,
exec/attach, writes, wildcard resources, current-context inference, and
cross-namespace queries are forbidden. Default bounds are two requests,
10 seconds per request, 30 seconds overall, and 1 MiB combined output. Run each
argv under process-group cleanup and redact secret-like or personal fields
before writing excerpts.

```bash
# Exact examples after <context>, <namespace>, and <caller-id> are approved.
oc --context <context> -n <namespace> --request-timeout=10s get pods -o json
oc --context <context> -n <namespace> --request-timeout=10s get agents.kagent.dev -o json

# Local cross-reference uses only declared docs paths and the same process bounds.
rg '^Status:' docs/code-map/services docs/agents/catalog.md
```

If approval, the context/namespace/resource allowlist, credential identity,
deadline, output cap, or redaction cannot be established, stop before the
request and mark the claim unchecked. The effects report records exact argv,
context/namespace/resources, request/output counts, credential identity, and
zero writes to the cluster.

### --verify-claims Flag

When running `/doc coverage --verify-claims`:

1. Extract all "Status: X" claims from docs
2. After the approval/allowlist gate above, query only the allowlisted deployment state
3. Report mismatches as CRITICAL
4. Flag stale validation dates (>30 days) as WARNING

---

## Anti-Patterns

| DON'T | DO INSTEAD |
|-------|------------|
| Sample 20 files, declare "healthy" | Scan ALL files |
| Say "healthy" with broken links | Report exact issue counts |
| Skip validation for "organized" repos | Validate regardless |
| Use bash loops on large repos | Use Python validator |
| Claim "deployed" without verification | Validate against cluster first |
| Duplicate ground truth data | Reference authoritative file |
| Omit validation dates | Include "Validated: DATE against SOURCE" |
