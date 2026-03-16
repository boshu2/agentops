---
name: doc
description: 'Generates code documentation, validates doc coverage against source, creates code-maps, and syncs documentation with implementation changes. Use when generating documentation, checking doc completeness, or updating stale docs.'
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: product
  dependencies:
    - standards  # loads markdown standards
---

# Doc Skill

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

Generate and validate documentation for any project.

## Execution Steps

Given `/doc [command] [target]`:

### Step 1: Detect Project Type

```bash
# Check for indicators
ls package.json pyproject.toml go.mod Cargo.toml 2>/dev/null

# Check for existing docs
ls -d docs/ doc/ documentation/ 2>/dev/null
```

Classify as:
- **CODING**: Has source code, needs API docs
- **INFORMATIONAL**: Primarily documentation (wiki, knowledge base)
- **OPS**: Infrastructure, deployment, runbooks

### Step 2: Execute Command

**discover** - Find undocumented features:
```bash
# Find public functions without docstrings (Python)
grep -r "^def " --include="*.py" | grep -v '"""' | head -20

# Find exported functions without comments (Go)
grep -r "^func [A-Z]" --include="*.go" | head -20
```

**coverage** - Check documentation coverage:
```bash
# Count documented vs undocumented
TOTAL=$(grep -r "^def \|^func \|^class " --include="*.py" --include="*.go" | wc -l)
DOCUMENTED=$(grep -r '"""' --include="*.py" | wc -l)
echo "Coverage: $DOCUMENTED / $TOTAL"
```

**gen [feature]** - Generate documentation:
1. Read the code for the feature
2. Understand what it does
3. Generate appropriate documentation
4. Write to docs/ directory

**all** - Update all documentation:
1. Run discover to find gaps
2. Generate docs for each undocumented feature
3. Validate existing docs are current

### Step 3: Generate Documentation

When generating docs, include:

**For Functions/Methods:**
```markdown
## function_name

**Purpose:** What it does

**Parameters:**
- `param1` (type): Description
- `param2` (type): Description

**Returns:** What it returns

**Example:**
```python
result = function_name(arg1, arg2)
```

**Notes:** Any important caveats
```

**For Classes:**
```markdown
## ClassName

**Purpose:** What this class represents

**Attributes:**
- `attr1`: Description
- `attr2`: Description

**Methods:**
- `method1()`: What it does
- `method2()`: What it does

**Usage:**
```python
obj = ClassName()
obj.method1()
```
```

### Step 3b: Validate Generated Docs

After generating documentation, run a validation checkpoint before proceeding:

1. **Check accuracy**: Compare generated parameter lists and return types against actual function signatures in source code.
2. **Fix mismatches**: If signatures diverge from generated docs, update the docs to match the code.
3. **Regenerate if needed**: If >20% of entries required fixes, re-run the generation step with corrected context.

This prevents stale or inaccurate docs from being committed.

### Step 4: Create Code-Map (if requested)

**Write to:** `docs/code-map/`

```markdown
# Code Map: <Project>

## Overview
<High-level architecture>

## Directory Structure
```
src/
├── module1/     # Purpose
├── module2/     # Purpose
└── utils/       # Shared utilities
```

## Key Components

### Module 1
- **Purpose:** What it does
- **Entry point:** `main.py`
- **Key files:** `handler.py`, `models.py`

### Module 2
...

## Data Flow
<How data moves through the system>

## Dependencies
<External dependencies and why>
```

### Step 5: Validate Documentation

Check for:
- Out-of-date docs (code changed, docs didn't)
- Missing sections (no examples, no parameters)
- Broken links
- Inconsistent formatting

### Step 6: Write Report

**Write to:** `.agents/doc/YYYY-MM-DD-<target>.md`

```markdown
# Documentation Report: <Target>

**Date:** YYYY-MM-DD
**Project Type:** <CODING/INFORMATIONAL/OPS>

## Coverage
- Total documentable items: <count>
- Documented: <count>
- Coverage: <percentage>%

## Generated
- <list of docs generated>

## Gaps Found
- <undocumented item 1>
- <undocumented item 2>

## Validation Issues
- <issue 1>
- <issue 2>

## Next Steps
- [ ] Document remaining gaps
- [ ] Fix validation issues
```

### Step 7: Report to User

Tell the user:
1. Documentation coverage percentage
2. Docs generated/updated
3. Gaps remaining
4. Location of report

## Key Rules

- **Detect project type first** - approach varies
- **Generate meaningful docs** - not just stubs
- **Include examples** - always show usage
- **Validate existing** - docs can go stale
- **Write the report** - track coverage over time

## Commands Summary

| Command | Action |
|---------|--------|
| `discover` | Find undocumented features |
| `coverage` | Check documentation coverage |
| `gen [feature]` | Generate docs for specific feature |
| `all` | Update all documentation |
| `validate` | Check docs match code |

## Examples

| Input | Output |
|-------|--------|
| `/doc gen authentication` | Generates `docs/api/authentication.md` with full API docs (purpose, parameters, returns, examples) for the authentication module, validated against source signatures. |
| `/doc coverage` | Produces coverage report at `.agents/doc/YYYY-MM-DD-coverage.md` showing percentage of documented items and listing specific undocumented functions. |
| `/doc all` | Discovers all gaps, generates missing docs, validates existing docs against current code, and writes a summary report. |
| `/doc validate` | Checks all existing docs for stale content, broken links, missing sections, and signature mismatches. |

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Coverage calculation inaccurate | Grep pattern doesn't match all code styles | Adjust pattern for project conventions. For Python, check for `async def` and class methods. For Go, check both `func` and `type` definitions. |
| Generated docs lack examples | Missing context about typical usage | Read existing tests to find usage patterns. Check README for code samples. Ask user for typical use case if unclear. |
| Discover command finds too many items | Low existing documentation coverage | Prioritize by running `discover` on specific subdirectories. Focus on public API first, internal utilities later. Use `--limit` to process in batches. |
| Validation shows docs out of sync | Code changed after docs written | Re-run `gen` command for affected features. Consider adding git hook to flag doc updates needed when code changes. |

## Reference Documents

- [references/generation-templates.md](references/generation-templates.md)
- [references/project-types.md](references/project-types.md)
- [references/validation-rules.md](references/validation-rules.md)
