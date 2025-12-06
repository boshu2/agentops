---
name: doc-curator
description: Single entry point for all documentation operations. Routes to 8 make targets for detection/validation and provides LLM-powered generation/improvement. Triggers on "check doc health", "fix links", "generate README", "find orphans", "suggest cross-links", "improve docs".
---

# Documentation Curator

**Single entry point for all documentation operations.** Routes to scripts for detection, uses LLM for intelligent action.

## Quick Commands

| Task | Command | Type |
|------|---------|------|
| **Health audit** | `make docs-index` | Script |
| **Quick metrics** | `make docs-metrics` | Script |
| **Check links** | `make check-links` | Script |
| **Check all links** | `make check-links-all` | Script |
| **Fix links (dry-run)** | `make fix-links` | Script |
| **Fix links (apply)** | `make fix-links-apply` | Script |
| **Validate structure** | `make docs-lint` | Script |
| **Full automation** | `make docs-all` | Script |
| **Generate README** | LLM action | This skill |
| **Suggest cross-links** | LLM action | This skill |
| **Content improvement** | LLM action | This skill |
| **Duplicate detection** | LLM action | This skill |

## When To Use

- "Check documentation health" → `make docs-index`
- "Fix broken links" → `make fix-links-apply`
- "Generate README for this folder" → LLM action
- "Find orphaned documents" → `make docs-index` + review
- "Suggest cross-links" → LLM action
- "Improve this document" → LLM action
- "Find duplicate content" → LLM action

## Capabilities

### 1. Health Report Generation (Detection)

Run the automated health check:

```bash
make docs-index
```

This generates `docs/reference/documentation-search/HEALTH_REPORT.md` containing:
- Missing READMEs (folders without README.md)
- Stale documents (not updated in 90+ days)
- Orphaned documents (no incoming links)

### 2. README Generation (Action)

For each missing README identified:

1. Read all `.md` files in the folder
2. Understand the folder's purpose from its contents
3. Generate a README following this pattern:

```markdown
# [Folder Name]: [One-Line Purpose]

**One sentence:** What this folder contains.

---

## What's Here

| Document | Purpose | ~Tokens |
|----------|---------|---------|
| [file.md](file.md) | Description | Xk |

---

## Start Here

[Point to the first document to read]

---

## Related Sections

- [Link to related folder 1](../related1/)
- [Link to related folder 2](../related2/)

---

**Last Updated:** YYYY-MM-DD
```

### 3. Cross-Link Suggestions (Action)

For orphaned documents:

1. Read the orphan document's content
2. Identify its topic and keywords
3. Search for related documents using semantic understanding
4. Suggest specific cross-links:

```markdown
## Suggested Cross-Links for [orphan.md]

This document about [topic] should be linked from:

1. `docs/related/topic.md` - Add to "See Also" section
2. `docs/guides/workflow.md` - Reference in step 3
3. `README.md` - Add to "Key Documents" table

Reason: [Why these documents are related]
```

### 4. Stale Document Review (Action)

For stale documents:

1. Read the document content
2. Check if information is still accurate (versions, links, procedures)
3. Provide one of:
   - "Still accurate - update timestamp only"
   - "Needs update - [specific issues found]"
   - "Consider archiving - [reason]"

### 5. Content Improvement (Action)

For documents needing quality improvement:

1. Read the document and identify issues:
   - Outdated versions or links
   - Missing sections (Start Here, Related)
   - Inconsistent formatting
   - Token count estimation

2. Provide improvement recommendations:

```markdown
## Improvements for [document.md]

### Quick Fixes
- Line 15: Update version 1.2 → 1.5
- Line 42: Broken link to deprecated.md

### Structure Improvements
- Add "Start Here" section (missing)
- Add "Related Sections" (missing)
- Estimated tokens: 2.3k (add to table)

### Optional Enhancements
- Could benefit from code examples
- Consider splitting into 2 documents (>500 lines)
```

### 6. Duplicate Detection (Action)

Identify content that may be duplicated:

1. Search for similar titles and topics
2. Compare content overlap
3. Recommend consolidation:

```markdown
## Potential Duplicates Found

### High Confidence
- `docs/guides/setup.md` (~80% overlap with)
- `docs/tutorials/getting-started.md`
  - Recommendation: Merge into tutorials, redirect from guides

### Medium Confidence
- `docs/reference/api.md` (partial overlap with)
- `docs/how-to/api-usage.md`
  - Recommendation: Keep both, cross-link, clarify scope
```

## Workflows

### Quick Health Check (2 min)

```bash
make docs-metrics
# Shows: Total docs, broken links, stale count
```

### Full Audit (5 min)

```bash
# Step 1: Generate health report
make docs-index

# Step 2: Read the report
cat docs/reference/documentation-search/HEALTH_REPORT.md

# Step 3: For each issue, take action:
# - Missing README → Generate one (LLM)
# - Orphan → Suggest cross-links (LLM)
# - Stale → Review and recommend (LLM)
```

### Link Validation (3 min)

```bash
make check-links           # Check key files (AGENTS.md, README.md)
make check-links-all       # Check everything
make fix-links             # See what would change (dry-run)
make fix-links-apply       # Apply fixes
```

### Structure Validation (2 min)

```bash
make docs-lint             # Diátaxis compliance check
make docs-lint-changed     # Check only changed files
```

### Complete Automation (10 min)

```bash
make docs-all              # Runs: reorganize + index + validate
```

### Single Folder README (LLM)

```bash
# When user says "generate README for docs/knowledge-corpus/04-research/"

1. List folder contents: ls docs/knowledge-corpus/04-research/
2. Read each .md file to understand purpose
3. Generate README following template in references/
4. Write to docs/knowledge-corpus/04-research/README.md
```

## Scripts

### Health Check (existing)

Location: `docs/scripts/index-automation-suite.sh`

Generates:
- `TOPIC_INDEX.md` - Docs by technology
- `RECENTLY_UPDATED.md` - Last 7 days changes
- `STAR_RATED.md` - Important documents
- `STATISTICS.md` - Doc metrics
- `HEALTH_REPORT.md` - Health issues

### Link Checker (existing)

Location: `tools/scripts/check-doc-links.py`

```bash
make check-links  # Validate all links
```

## Progressive Disclosure

This skill uses three-level loading:

1. **Always loaded:** This SKILL.md (~800 tokens)
2. **On demand:** Health report, specific folders
3. **Never loaded:** Full doc tree (use scripts to query)

## Integration

Works with:
- `make docs-index` - Generate indexes
- `make check-links` - Validate links
- `make docs-all` - Full automation

## Example Session

**User:** "Check doc health and fix any issues"

**Claude:**
1. Run `make docs-index`
2. Read `HEALTH_REPORT.md`
3. Report findings:
   - "Found 3 folders missing READMEs"
   - "Found 5 orphaned documents"
   - "Found 2 stale documents"
4. Ask: "Would you like me to generate READMEs for the 3 folders?"
5. On approval, generate each README
6. Ask: "Would you like cross-link suggestions for the 5 orphans?"
7. Provide suggestions
8. Commit changes

---

**Pattern:** Detection (scripts) + Action (LLM intelligence)
**Philosophy:** Machines detect, LLMs understand and fix
