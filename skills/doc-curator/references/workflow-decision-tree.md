# Doc-Curator Decision Tree

Quick reference for choosing the right workflow.

## What do you need?

```
Start
  │
  ├─ "Quick status check" ─────────→ make docs-metrics
  │
  ├─ "Full health audit" ──────────→ make docs-index + review HEALTH_REPORT.md
  │
  ├─ "Check if links work" ────────→ make check-links
  │
  ├─ "Fix broken links" ───────────→ make fix-links-apply
  │
  ├─ "Validate doc structure" ─────→ make docs-lint
  │
  ├─ "Run everything" ─────────────→ make docs-all
  │
  ├─ "Generate missing README" ────→ Ask Claude (LLM action)
  │
  ├─ "Find related documents" ─────→ Ask Claude (LLM action)
  │
  ├─ "Improve document quality" ───→ Ask Claude (LLM action)
  │
  └─ "Find duplicate content" ─────→ Ask Claude (LLM action)
```

## When to Use Scripts vs LLM

| Task | Use Script | Use LLM |
|------|------------|---------|
| Find broken links | ✅ `check-links` | |
| Fix obvious link paths | ✅ `fix-links-apply` | |
| Count documents | ✅ `docs-metrics` | |
| Validate YAML/structure | ✅ `docs-lint` | |
| Generate health report | ✅ `docs-index` | |
| Decide if doc is stale | | ✅ |
| Generate README | | ✅ |
| Suggest cross-links | | ✅ |
| Improve content | | ✅ |
| Find duplicates | | ✅ |

## Script = Fast, Deterministic

Scripts are best for:
- Binary checks (link exists or not)
- Counting and listing
- Pattern matching
- Syntax validation

## LLM = Understanding, Generation

LLM is best for:
- Understanding document purpose
- Generating new content
- Suggesting semantic relationships
- Quality assessment
- Content improvement recommendations

---

**Pattern:** Scripts detect → LLM understands and acts
