# Plan Detail Templates

Three tiers of plan detail, auto-selected by issue count and goal complexity. See Step 3.2 in the plan SKILL.md for selection criteria.

## Minimal Template (1-2 issues, fast path)

Use for small, well-understood changes where full spec overhead is wasteful.

```markdown
# Plan: <Goal>

## Issues

### Issue 1: <Title>
- **Files:** `path/to/file.go`
- **Change:** <2-3 sentences>
- **Acceptance:** <1-line test command>
- **Wave:** 1

## Verification
`<single test command>`
```

**Includes:** Title, 2-line description, acceptance criteria, files list.
**Omits:** Boundaries, design briefs, data transformation tables, cross-wave registry, file-conflict matrix.

## Standard Template (3-6 issues)

Full plan format as defined in the main SKILL.md Step 6. This is the default tier for most work.

Includes:
- Context section with applied findings
- Files to Modify table
- Boundaries (Always/Ask First/Never)
- Baseline audit with verification commands
- Implementation with per-section specs
- Tests with named functions
- Conformance checks table
- Verification procedures
- Wave structure with file-conflict matrix

## Deep Template (7+ issues, complex operations)

Everything in Standard, plus:
- **Design briefs** for any rewrite/redesign issues (Purpose, Key artifacts, Workflows)
- **Data transformation mapping tables** for filtering/exclusion logic (source field -> output transformation)
- **Cross-wave shared file registry** with collision mitigations
- **Inline code blocks** for all non-obvious constructs (verified compilable)
- **Per-issue `metadata.files`** for worker file ownership
- **Homogeneous wave grouping** by work type (all Go, all docs, all shell within same wave)
- **Schema strictness pre-flight** for JSON schema changes
