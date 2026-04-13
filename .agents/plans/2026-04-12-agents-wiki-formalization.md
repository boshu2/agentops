---
id: plan-2026-04-12-agents-wiki-formalization
type: plan
date: 2026-04-12
source: "[[.agents/research/2026-04-12-agents-wiki-architecture-synthesis]]"
---

# Plan: Formalize .agents/ as Karpathy-Style Wiki

## Context

.agents/ already implements 80% of the Karpathy wiki pattern: 51 subdirs of structured markdown, wikilinks in 20+ files, BM25 search, maturity lifecycle. What's missing is the navigation layer (INDEX.md + LOG.md) and a dedicated wiki/ directory for LLM-generated content. ao search already scans 10 knowledge surfaces (learnings, patterns, findings, research, compiled, plans, brainstorm, council, design, sessions + dream vault). Adding wiki/ is one function call.

Applied findings:
- `learning-2026-04-12-yagni-bridge-not-clone` — bridge existing systems, don't clone. This plan adds to ao search's existing scan loop, not a new search engine.

## Files to Modify

| File | Change |
|------|--------|
| `.agents/INDEX.md` | **NEW** — canonical catalog of all knowledge artifacts |
| `.agents/LOG.md` | **NEW** — append-only chronological operation log |
| `.agents/wiki/sources/.gitkeep` | **NEW** — tier 1 LLM session summaries |
| `.agents/wiki/synthesis/.gitkeep` | **NEW** — cross-session insights |
| `.agents/wiki/concepts/.gitkeep` | **NEW** — promoted concept pages |
| `.agents/wiki/entities/.gitkeep` | **NEW** — promoted entity pages |
| `cli/cmd/ao/search.go` | Add `wiki` to `searchRepoLocalKnowledge` scan list (1 line) |
| `cli/cmd/ao/forge.go` | Change tier 1 output path from `ao/sessions/` to `wiki/sources/` |
| `cli/internal/llm/forge_tier1.go` | No change — OutputDir is passed by caller |
| `scripts/generate-index.sh` | **NEW** — script to generate INDEX.md from directory scan |

## Boundaries

**Always:** Backward compat — existing `.agents/ao/sessions/` files stay; new writes go to `wiki/sources/`. ao search must find content in both locations.
**Ask First:** Whether to rsync bushido vault/wiki/synthesis/ into local .agents/wiki/synthesis/ (network dependency).
**Never:** Move or rename existing .agents/ subdirectories. Don't break ao search, ao inject, or any hook.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| .agents/ subdirs | `ls -d .agents/*/ \| wc -l` | 51 |
| Learnings count | `ls .agents/learnings/*.md \| wc -l` | 202 |
| Existing sessions | `ls .agents/ao/sessions/*.md \| wc -l` | 65 |
| INDEX.md exists | `ls .agents/INDEX.md` | does not exist |
| LOG.md exists | `ls .agents/LOG.md` | does not exist |
| wiki/ dir exists | `ls -d .agents/wiki/` | does not exist |
| Search scan surfaces | `grep appendKnowledge search.go \| wc -l` | 8 surfaces + vault + sessions |

## Implementation

### 1. Create .agents/wiki/ Directory Structure

Create the four Karpathy-pattern subdirectories:
```bash
mkdir -p .agents/wiki/{sources,synthesis,concepts,entities}
touch .agents/wiki/{sources,synthesis,concepts,entities}/.gitkeep
```

### 2. Generate INDEX.md

Create `scripts/generate-index.sh` that scans .agents/ and produces INDEX.md:

```bash
#!/usr/bin/env bash
# Generate .agents/INDEX.md from directory scan
set -euo pipefail
AGENTS_DIR="${1:-.agents}"
OUTPUT="$AGENTS_DIR/INDEX.md"
```

The script:
- Walks each known subdirectory (learnings/, wiki/sources/, wiki/synthesis/, plans/, research/, council/, patterns/, findings/)
- For each `.md` file, extracts the first H1 heading or filename
- Outputs one wikilink line per file: `- [[subdir/filename]] — first heading`
- Groups by category with H2 headers

### 3. Create LOG.md Seed

```markdown
# .agents/ Operation Log

> Append-only. Never rewrite. Each entry: `YYYY-MM-DD HH:MM | actor | VERB | subject | [[wikilink]]`

2026-04-12 20:00 | ao-plan | INIT | Wiki formalization — INDEX.md + LOG.md + wiki/ directory | [[plans/2026-04-12-agents-wiki-formalization]]
```

### 4. Add wiki/ to ao search Scan Paths

In `cli/cmd/ao/search.go:508`, add after the design line:

```go
results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "wiki/sources", "wiki-source", "wiki-sources", limit)
results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "wiki/synthesis", "wiki-synthesis", "wiki-synthesis", limit)
results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "wiki/concepts", "wiki-concept", "wiki-concepts", limit)
```

### 5. Update forge --tier=1 Output Path

In `cli/cmd/ao/forge.go`, change `runForgeTier1`:

```go
// Before:
outDir := filepath.Join(cwd, ".agents", "ao", "sessions")
// After:
outDir := filepath.Join(cwd, ".agents", "wiki", "sources")
```

### 6. Append to LOG.md from forge/harvest Operations

Add a `appendToLog` helper in the llm package or as a shared utility:

```go
func AppendToLog(agentsDir, actor, verb, subject, wikilink string) error {
    logPath := filepath.Join(agentsDir, "LOG.md")
    entry := fmt.Sprintf("%s | %s | %s | %s | [[%s]]\n",
        time.Now().UTC().Format("2006-01-02 15:04"), actor, verb, subject, wikilink)
    f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil { return err }
    defer f.Close()
    _, err = f.WriteString(entry)
    return err
}
```

Call from `processOneSession` in forge_tier1.go after WriteSessionPage succeeds.

## Tests

**`cli/cmd/ao/search_test.go`** — add:
- `TestSearchRepoLocalKnowledge_IncludesWikiSources`: verify wiki/sources/ is scanned
- `TestSearchRepoLocalKnowledge_IncludesWikiSynthesis`: verify wiki/synthesis/ is scanned

**`scripts/generate-index.sh`** — verify:
- Empty .agents/ produces minimal INDEX.md
- .agents/ with learnings produces correct wikilinks

## Conformance Checks

| Issue | Check Type | Check |
|-------|-----------|-------|
| Issue 1 | files_exist | `[".agents/wiki/sources/.gitkeep", ".agents/wiki/synthesis/.gitkeep", ".agents/wiki/concepts/.gitkeep", ".agents/wiki/entities/.gitkeep"]` |
| Issue 2 | files_exist | `[".agents/INDEX.md"]` |
| Issue 2 | content_check | `{file: ".agents/INDEX.md", pattern: "## Learnings"}` |
| Issue 3 | files_exist | `[".agents/LOG.md"]` |
| Issue 4 | content_check | `{file: "cli/cmd/ao/search.go", pattern: "wiki/sources"}` |
| Issue 5 | content_check | `{file: "cli/cmd/ao/forge.go", pattern: "wiki.*sources"}` |
| Issue 6 | content_check | `{file: "cli/internal/llm/forge_tier1.go", pattern: "AppendToLog\\|LOG.md"}` |

## Verification

1. `go build ./...` — clean build after search.go + forge.go changes
2. `go test ./cmd/ao/ -run "TestSearch"` — search tests pass
3. `go test ./internal/llm/` — llm tests pass
4. `bash scripts/generate-index.sh && head -20 .agents/INDEX.md` — INDEX.md generated
5. `ls .agents/wiki/sources/ .agents/wiki/synthesis/` — dirs exist

## Issues

### Issue 1: Create .agents/wiki/ directory structure
**Dependencies:** None
**Acceptance:** Four subdirectories exist with .gitkeep files
**Description:** `mkdir -p .agents/wiki/{sources,synthesis,concepts,entities}` + .gitkeep in each

### Issue 2: Generate INDEX.md from directory scan
**Dependencies:** Issue 1
**Acceptance:** INDEX.md exists with wikilinks to learnings, wiki sources, plans, research
**Description:** Write `scripts/generate-index.sh` that scans .agents/ and produces categorized INDEX.md. Run it to generate initial INDEX.md.

### Issue 3: Create LOG.md seed
**Dependencies:** None
**Acceptance:** LOG.md exists with header and initial entry
**Description:** Create .agents/LOG.md with the append-only format spec and seed entry.

### Issue 4: Add wiki/ to ao search scan paths
**Dependencies:** Issue 1
**Acceptance:** `ao search` returns results from .agents/wiki/sources/ and wiki/synthesis/
**Description:** Add 3 `appendKnowledgeMarkdownSearch` calls in search.go:508 for wiki/sources, wiki/synthesis, wiki/concepts.

### Issue 5: Update forge --tier=1 output path
**Dependencies:** Issue 1
**Acceptance:** `ao forge transcript --tier=1` writes to .agents/wiki/sources/ instead of ao/sessions/
**Description:** Change outDir in forge.go runForgeTier1 from `ao/sessions` to `wiki/sources`.

### Issue 6: Append to LOG.md from forge operations
**Dependencies:** Issue 3, Issue 5
**Acceptance:** After forge --tier=1 runs, LOG.md has a new INGEST entry
**Description:** Add AppendToLog helper. Call from processOneSession after WriteSessionPage.

## Execution Order

**Wave 1** (parallel): Issue 1 (wiki dirs), Issue 3 (LOG.md seed)
**Wave 2** (after Wave 1): Issue 2 (INDEX.md), Issue 4 (search paths), Issue 5 (forge output path)
**Wave 3** (after Wave 2): Issue 6 (LOG.md integration)

## File-Conflict Matrix

| File | Issues |
|------|--------|
| `cli/cmd/ao/search.go` | Issue 4 |
| `cli/cmd/ao/forge.go` | Issue 5 |
| `.agents/INDEX.md` | Issue 2 |
| `.agents/LOG.md` | Issue 3, Issue 6 |

LOG.md appears in Issues 3 and 6 but they're in different waves (1 and 3) — no conflict.

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical Enforcement | PASS | Each issue has files_exist or content_check conformance |
| PR-002: External Validation | PASS | `go build` + `go test` verify Go changes; `ls` verifies file creation |
| PR-003: Feedback Loops | PASS | INDEX.md is consumed by ao inject; LOG.md by human/agent audit |
| PR-004: Separation Over Layering | PASS | wiki/ is a new directory, not layered onto existing ao/sessions/ |
| PR-005: Process Gates First | N/A | No process gates being added |
| PR-006: Cross-Layer Consistency | PASS | search.go scan paths match directory structure |
| PR-007: Phased Rollout | PASS | 3 waves: dirs → content + search → integration |

Unchecked rules: 0

## Next Steps
- Run `/pre-mortem` to validate plan
- Run `/crank` for autonomous execution
