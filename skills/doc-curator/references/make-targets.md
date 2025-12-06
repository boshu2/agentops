# Documentation Make Targets

Quick reference for all doc-related make targets in gitops repository.

## Index & Health

| Target | Purpose | Time |
|--------|---------|------|
| `docs-index` | Generate all indexes + health report | 30s |
| `docs-metrics` | Show quick health metrics | 5s |
| `docs-validate` | Validate structure | 10s |
| `docs-all` | Run everything (reorganize + index + validate) | 60s |

## Link Management

| Target | Purpose | Time |
|--------|---------|------|
| `check-links` | Check key files (AGENTS.md, README.md) | 10s |
| `check-links-all` | Check all .md files comprehensively | 60s |
| `fix-links` | Dry-run link fixes (shows what would change) | 10s |
| `fix-links-apply` | Apply link fixes (modifies files) | 10s |
| `fix-links-report` | Generate manual review list | 10s |

## Structure Validation

| Target | Purpose | Time |
|--------|---------|------|
| `docs-lint` | Diátaxis compliance check | 15s |
| `docs-lint-changed` | Check only changed files (faster) | 5s |
| `docs-consolidate` | Check length thresholds | 10s |

## Output Locations

| Make Target | Output File |
|-------------|-------------|
| `docs-index` | `docs/reference/documentation-search/HEALTH_REPORT.md` |
| `docs-index` | `docs/reference/documentation-search/TOPIC_INDEX.md` |
| `docs-index` | `docs/reference/documentation-search/RECENTLY_UPDATED.md` |
| `docs-index` | `docs/reference/documentation-search/STAR_RATED.md` |
| `docs-index` | `docs/reference/documentation-search/STATISTICS.md` |
| `fix-links-report` | stdout (review manually) |

## Common Workflows

### Daily Check
```bash
make docs-metrics
```

### Before PR
```bash
make check-links && make docs-lint-changed
```

### Weekly Audit
```bash
make docs-all
cat docs/reference/documentation-search/HEALTH_REPORT.md
```

### Fix All Issues
```bash
make fix-links-apply && make docs-index
```

---

**Location:** All targets defined in `/path/to/work/gitops/Makefile`
