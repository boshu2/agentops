# Comprehensive Skill Pattern

**Proven pattern for creating production-grade skills based on `gitops-app-creation` and `release-engineering`**

This pattern emerged from building two comprehensive skills that successfully manage complex workflows across multiple repositories.

---

## Pattern Overview

**Name:** Comprehensive Multi-Workflow Skill Pattern

**Use When:**
- Skill covers multiple related workflows (3-5+)
- Skill manages complete lifecycle (not just one operation)
- Skill requires extensive documentation (500+ lines)
- Skill needs JIT-loadable references (<40% context rule)
- Skill integrates with external systems (repositories, APIs, tools)

**Examples:**
- `gitops-app-creation` - 5 app patterns, 54 production apps, Kustomize/Helm
- `release-engineering` - 5 workflows (scaffold/harmonize/bootstrap/validate/provision), 67 config variables

---

## Skill Structure (The Sacred Pattern)

```
skill-name/
├── SKILL.md (500-1000 lines)          # Complete guide with all workflows
├── references/                         # JIT-loadable guides (200-500 lines each)
│   ├── workflow-1-guide.md            # Deep dive into workflow 1
│   ├── workflow-2-guide.md            # Deep dive into workflow 2
│   ├── error-handling.md              # Exit codes, common errors
│   └── schema-reference.md            # Variables, configuration, types
├── templates/                          # Example files users can copy
│   ├── workflow-1/                    # Template for workflow 1
│   │   └── example.yaml               # Working example
│   ├── workflow-2/                    # Template for workflow 2
│   │   └── example.yaml               # Working example
│   └── config.example                 # Configuration template
└── scripts/                            # Optional automation helpers
    ├── validate.py                    # Validation script
    └── helper.sh                      # Utility script
```

---

## SKILL.md Structure (The Anatomy)

### 1. Frontmatter (YAML)

```yaml
---
name: skill-name
description: Complete <domain> lifecycle using X proven <workflows/patterns> - covers A, B, C, D, E with built-in validation and examples
tags: [domain, workflow1, workflow2, key-tech, automation]
---
```

**Key principles:**
- Description explains COMPLETE scope (not just one feature)
- Lists number of workflows/patterns explicitly
- Mentions key capabilities (validation, examples, etc.)
- Tags cover domain + workflows + technologies

### 2. Critical Warnings (⚠️ Section)

**Immediately after title**, before anything else:

```markdown
## ⚠️ CRITICAL: The Golden Rule

**ALWAYS do X (source of truth), NEVER do Y (anti-pattern)**

```
This rule X → Y (correct workflow)
    ↓
This is WRONG (wrong approach)
```

**Breaking this rule causes:**
- Consequence 1
- Consequence 2
- Consequence 3
```

**Why this works:**
- Prevents most common mistake upfront
- Shows correct workflow visually
- Explains impact (not just "don't do this")

**Examples:**
- `release-engineering`: "ALWAYS edit config.env, NEVER edit values.yaml"
- `gitops-app-creation`: "Pattern 5 uses helmCharts, NOT resources"

### 3. When to Use (Clear Triggers)

```markdown
## When to Use

Use this skill when you need to:

- **Action 1** - Specific use case description
- **Action 2** - Another specific use case
- **Action 3** - Debug/troubleshoot scenario
- **Action 4** - Validation scenario
```

**Key principles:**
- Each bullet starts with bold action verb
- Covers creation, modification, debugging, validation
- Specific, not vague ("Create new app" not "Work with apps")

### 4. Quick Start (Fastest Path)

**NEW: Start with the easiest/most common workflow**

```markdown
## Quick Start: <Primary Workflow>

**Fastest way to <accomplish goal>:** Use the `<command>` target:

```bash
# Complete workflow (one command)
make primary-workflow PARAM=value

# Step-by-step
make step1
make step2
make step3
```

**What `primary-workflow` does (N steps):**
1. 🏗️ **Step 1**: Description
2. 🔄 **Step 2**: Description
3. ➕ **Step 3**: Description

**This is the recommended workflow for most users.**

For manual control or advanced use cases, see workflows below.
```

**Why this works:**
- Shows success path immediately (don't make users hunt)
- One-liner for common case
- Explains what happens (transparency)
- Points to detailed workflows for advanced users

**Examples:**
- `release-engineering`: `make site-setup` (scaffold + harmonize + workspace)
- `gitops-app-creation`: Could add `make create-app PATTERN=5`

### 5. The N Core Workflows (Main Content)

```markdown
## The N Core Workflows

### Workflow 1: <Name> (<Percentage>% of usage - <description>)

**Use When:** Specific scenario that triggers this workflow

**What it does:**
1. Step 1 description
2. Step 2 description
3. Step 3 description
4. Step N description

**Structure:**
```
directory/
├── file1
├── file2
└── directory/
    └── file3
```

**Example Usage:**
```bash
# Via Command Line
command --flag value

# Via GitLab CI
VARIABLE=value

# Via Makefile
make target PARAM=value
```

**Real Examples:**
- Location 1: `path/to/example1`
- Location 2: `path/to/example2`

**Common Scenarios:**

**Scenario 1: <Description>**
```bash
# Step 1: Do something
command1

# Step 2: Do something else
command2

# Result: What happened
```

**Exit Codes:**
- `0` - Success
- `1` - Error type 1
- `2` - Error type 2
```

**Key principles:**
- Start with percentage of usage (helps users prioritize)
- "What it does" is numbered list (clear steps)
- Show directory structure (visual understanding)
- Multiple usage methods (CLI, CI, Makefile, etc.)
- Real examples point to actual files
- Common scenarios show realistic usage
- Exit codes for automation

**Pattern:** Repeat for each workflow

### 6. Universal/Core Tool (If Applicable)

If workflows share a common engine:

```markdown
## The Universal <Tool> (Core Engine)

**`<tool-name>`** is the heart of ALL workflows - a <description>.

### Capabilities

- Capability 1 with ANY input
- Capability 2
- Capability 3
- Clear exit codes (0-N) for automation

### Usage Pattern

```bash
<tool> \
  --param1 value1 \
  --param2 value2 \
  [--optional-flag]           # Optional parameter
```

### Exit Codes (CRITICAL for automation)

| Code | Meaning | Action |
|------|---------|--------|
| `0` | Success | Proceed |
| `1` | Error 1 | Fix X |
| `2` | Error 2 | Fix Y |

### Examples

**Example 1: <Use case>**
```bash
<tool> --param value
```

**Example 2: <Another use case>**
```bash
<tool> --other-param value
```
```

**Why this works:**
- Centralizes shared tool documentation
- Clear exit codes table (automation-friendly)
- Multiple examples show flexibility

**Examples:**
- `release-engineering`: `render_values.py` (universal Jinja2 renderer)

### 7. Configuration/Schema (If Applicable)

For skills with many variables/options:

```markdown
## Configuration <Variables/Schema> (N total)

All configuration lives in `<file>` with N documented variables across M categories:

| Category | Count | Examples |
|----------|-------|----------|
| **Category 1** | X | `VAR1`, `VAR2`, `VAR3` |
| **Category 2** | Y | `VAR4`, `VAR5`, `VAR6` |

**See:** `references/<schema>.md` for complete variable documentation (JIT load)

**Example `<config-file>`:**
```
# Category 1
VAR1=value1
VAR2=value2

# Category 2
VAR4=value4
```
```

**Why this works:**
- Shows total count upfront (scope awareness)
- Categorizes variables (findability)
- Points to reference (JIT loading)
- Shows minimal example (quick start)

**Examples:**
- `release-engineering`: 67 config variables across 8 categories

### 8. Advanced Examples Section

```markdown
## Advanced Examples (<repository/path>)

For advanced use cases beyond the N core workflows, see working examples in `<path>`:

| Example | Pattern/Workflow | Use Case | Location |
|---------|------------------|----------|----------|
| **example-1** | Pattern/Workflow X | Description | `path/to/example1` |
| **example-2** | Pattern/Workflow Y | Description | `path/to/example2` |

**Note:** These examples are fully functional and can be rendered/run with:
```bash
command example-1
command example-2
```
```

**Why this works:**
- Points to real, working examples
- Categorizes by pattern/workflow
- Shows how to run examples
- Separates advanced from core (progressive disclosure)

**Examples:**
- `gitops-app-creation`: Points to `gitops/examples/` with 8 apps
- `release-engineering`: Points to `pipelines/` with real CI/CD

### 9. References (JIT Loadable)

```markdown
## References (Loaded JIT)

When needed, I'll load:

- `references/workflow-1-guide.md` - Complete workflow 1 documentation
- `references/workflow-2-guide.md` - Complete workflow 2 documentation
- `references/error-handling.md` - Exit codes and troubleshooting
- `references/schema-reference.md` - Configuration variables

**External Documentation:**
- `../../<repo>/README.md` - Repository overview
- `../../<repo>/docs/` - Complete documentation tree
```

**Why this works:**
- Tells agent exactly what references exist
- Shows when to load each (scoped to workflow)
- Points to external docs (repository integration)
- Maintains <40% context rule

**Examples:**
- `gitops-app-creation`: 5 references (patterns, kustomize, helm, files, validation)
- `release-engineering`: 4 references (harmonize, scaffold, exit-codes, config-schema)

### 10. Common Scenarios (Real-World Usage)

```markdown
## Common Scenarios

### Scenario 1: <Real-World Task> (Full Lifecycle)

```bash
# Step 1: <Action>
command1

# Step 2: <Action>
command2

# Step 3: <Action>
command3

# Done! <Result>
```

### Scenario 2: <Another Task>

```bash
# Step 1: <Action>
command1

# Step 2: Preview changes (safe)
command2 --preview

# Step 3: Review diff
command3

# Step 4: Apply if satisfied
command4

# Done! <Result>
```
```

**Why this works:**
- Shows complete, realistic workflows
- Includes safety steps (preview, review)
- Explains result at end
- Users can copy-paste

**Examples:**
- `release-engineering`: 4 scenarios (create site, update config, debug errors, validate upgrade)
- `gitops-app-creation`: 3 scenarios (simple app, app with configmap, custom app)

### 11. Error Handling (Troubleshooting)

```markdown
## Error Handling

### Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `Error message 1` | Root cause | Step-by-step fix |
| `Error message 2` | Root cause | Step-by-step fix |

### Error Recovery

**If <workflow> fails mid-way:**
```bash
# 1. Check status
command1

# 2. Review changes
command2

# 3. Reset if needed
command3

# 4. Fix issue
command4

# 5. Retry
command5
```
```

**Why this works:**
- Table format (scannable)
- Specific error messages (searchable)
- Recovery procedures (actionable)

**Examples:**
- `release-engineering`: 6 common errors + recovery procedures

### 12. Safety Checks (Before/After/Never)

```markdown
## Safety Checks

### Before Running <Workflow>

- ☑️ Check X
- ☑️ Review Y
- ☑️ Backup Z

### After Running <Workflow>

- ☑️ Verify output with `command`
- ☑️ Check for errors
- ☑️ Test in environment

### Never

- ❌ Anti-pattern 1
- ❌ Anti-pattern 2
- ❌ Anti-pattern 3
```

**Why this works:**
- Checkbox format (actionable)
- Covers before/after/never (complete safety net)
- Specific commands (executable)

**Examples:**
- `release-engineering`: 3 sections per workflow (scaffold, harmonize, bootstrap)

### 13. Integration with Other Skills

```markdown
## Integration with Other Skills

**Works with:**

- **skill-1**: How they integrate
- **skill-2**: Workflow example
- **skill-3**: Cross-skill pattern

**Workflow Example:**
```bash
# 1. Use skill-1
command1

# 2. Use this skill
command2

# 3. Use skill-3
command3
```
```

**Why this works:**
- Shows ecosystem integration
- Provides cross-skill workflow
- Helps users discover related skills

**Examples:**
- `release-engineering`: Integrates with gitops-app-creation, manifest-validation, git-workflow, testing

---

## References Directory Structure

Each reference file follows this pattern:

### `references/workflow-guide.md`

```markdown
# <Workflow> Guide

**Source:** Where this workflow comes from
**Reference:** Official documentation link

---

## Step-by-Step Logic Flow

### Step 1: <Action>

* Description
* Commands
* Output

**Example:**
```bash
command
```

---

### Step 2: <Action>

...

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|

---

## Related Documentation

- Link 1
- Link 2
```

### `references/exit-codes.md`

```markdown
# Exit Codes Reference

Complete guide to all exit codes used by <tool/script>.

## <Tool> Exit Codes (0-N)

| Code | Meaning | Description | Action |
|------|---------|-------------|--------|
| **0** | Success | ... | ... |
| **1** | Error 1 | ... | ... |

---

## Handling Exit Codes in Scripts

### Bash Example

```bash
command
EXIT_CODE=$?

case $EXIT_CODE in
  0) echo "Success" ;;
  1) echo "Error" ; exit 1 ;;
  *) echo "Unknown" ; exit 1 ;;
esac
```

---

## Exit Code Flow Chart

```
tool
    |
    ├─ Exit 0 → Success
    ├─ Exit 1 → Error → Fix X
    └─ Exit 2 → Error → Fix Y
```
```

### `references/schema-reference.md`

```markdown
# <Configuration> Schema (N Variables)

Complete reference for all `<config-file>` variables.

**Source:** `<path/to/template>`

---

## Variable Categories

| Category | Count | Description |
|----------|-------|-------------|
| Category 1 | X | ... |
| Category 2 | Y | ... |

**Total:** N variables

---

## Category 1 Configuration (X variables)

| Variable | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `VAR1` | string | ✅ | - | ... |
| `VAR2` | number | | `10` | ... |

**Example:**
```bash
VAR1=value1
VAR2=20
```

---

## Variable Types

### String
```bash
VAR=value
```

### Boolean
```bash
VAR=true  # or false
```

---

## Variable Validation

**Blank Check:**
```bash
# ❌ Will fail
VAR=

# ✅ Will pass
VAR=value
```

---

## Best Practices

### Organize by Category
```bash
################################################################################
# Category 1
################################################################################

VAR1=value1
```

---

## Related Documentation

- Link 1
- Link 2
```

---

## Templates Directory Structure

### Pattern: One template per workflow

```
templates/
├── workflow-1/
│   ├── example-file-1.yaml
│   └── example-file-2.yaml
├── workflow-2/
│   ├── example-file-1.yaml
│   └── example-file-2.yaml
└── config.example          # Shared configuration template
```

### Template Content Pattern

Each template should be:
- ✅ **Working example** (not just skeleton)
- ✅ **Well-commented** (explains each section)
- ✅ **Copy-pasteable** (users can use directly)
- ✅ **Representative** (shows common usage)

**Example:**
```yaml
# This is the STANDARD PATTERN used by 90%+ of apps
# It inflates the chart with site-specific values

helmGlobals:
  chartHome: ../../../path/to/charts

helmCharts:
  - name: app-name
    valuesFile: values.yaml
```

---

## Key Metrics That Indicate Success

**Your comprehensive skill is working when:**

1. ✅ **SKILL.md is 500-1000 lines** (comprehensive but not overwhelming)
2. ✅ **References total 1000-2000 lines** (JIT-loadable depth)
3. ✅ **Users can start immediately** (Quick Start section)
4. ✅ **Common mistakes are prevented** (Critical Warnings section)
5. ✅ **Error handling is clear** (Exit codes + troubleshooting)
6. ✅ **Examples are working** (Users can copy-paste)
7. ✅ **Safety is built-in** (Before/After/Never checks)
8. ✅ **Integration is documented** (Works with other skills)

**Token counts:**
- SKILL.md: 2k-4k tokens (loaded when skill triggers)
- Each reference: 500-1k tokens (loaded JIT)
- Total context: <40% when fully loaded (8k-16k tokens)

---

## Common Mistakes to Avoid

### ❌ Mistake 1: No Quick Start

**Problem:** Users must read entire SKILL.md to get started

**Fix:** Add Quick Start section with one-liner for most common workflow

### ❌ Mistake 2: Critical Warnings Buried

**Problem:** Most common mistake appears halfway through doc

**Fix:** Put ⚠️ CRITICAL section immediately after title

### ❌ Mistake 3: Missing Real Examples

**Problem:** Only shows templates, no working examples

**Fix:** Point to actual working files in repositories

### ❌ Mistake 4: No Exit Codes

**Problem:** Automation can't handle errors properly

**Fix:** Document all exit codes in references/exit-codes.md

### ❌ Mistake 5: Vague "When to Use"

**Problem:** Users don't know when to invoke skill

**Fix:** Specific action-based bullets (Create X, Debug Y, Validate Z)

### ❌ Mistake 6: No Error Recovery

**Problem:** When workflow fails mid-way, users don't know how to recover

**Fix:** Add "Error Recovery" section with reset/retry procedures

### ❌ Mistake 7: Everything in SKILL.md

**Problem:** SKILL.md is 3000+ lines, violates <40% rule

**Fix:** Extract to references/ (workflow guides, schemas, exit codes)

### ❌ Mistake 8: No Safety Checks

**Problem:** Users run dangerous operations without validation

**Fix:** Add Before/After/Never sections for each workflow

---

## Checklist: Is Your Skill Comprehensive?

**Frontmatter:**
- [ ] Name is descriptive
- [ ] Description explains COMPLETE scope
- [ ] Lists number of workflows/patterns
- [ ] Tags cover domain + workflows + tech

**Structure:**
- [ ] ⚠️ CRITICAL section immediately after title
- [ ] When to Use has 5+ specific triggers
- [ ] Quick Start shows one-liner
- [ ] N Core Workflows section (3-5 workflows)
- [ ] Each workflow has structure/examples/scenarios
- [ ] Universal tool section (if applicable)
- [ ] Configuration/schema section (if applicable)
- [ ] Advanced Examples section
- [ ] References (JIT loadable) section
- [ ] Common Scenarios (3-5 realistic workflows)
- [ ] Error Handling (table + recovery)
- [ ] Safety Checks (before/after/never)
- [ ] Integration with Other Skills

**References:**
- [ ] Workflow guides (one per major workflow)
- [ ] Exit codes reference
- [ ] Schema/configuration reference
- [ ] Each reference is 200-500 lines

**Templates:**
- [ ] One directory per workflow (if applicable)
- [ ] Working examples (not just skeletons)
- [ ] Well-commented
- [ ] Config example file

**Quality:**
- [ ] SKILL.md is 500-1000 lines
- [ ] Total references are 1000-2000 lines
- [ ] Points to real working examples
- [ ] Fully loaded context <40% (8k-16k tokens)
- [ ] Exit codes documented for automation
- [ ] Error recovery procedures included

---

## Examples to Study

### gitops-app-creation (418 lines SKILL.md)

**Structure:**
- ⚠️ CRITICAL: Pattern 5 helmCharts warning
- When to Use: 7 specific triggers
- Quick Start: Pattern selection decision tree
- The 5 Proven Patterns: Each with examples
- Kustomize Patch Techniques
- File Organization Rules
- Validation Workflow
- Integration with Other Skills
- Common Scenarios: 3 realistic workflows
- Script Execution
- References (JIT): 5 references
- **NEW:** Advanced Examples section (gitops/examples/)

**References:**
- patterns-catalog.md (551 lines)
- kustomize-techniques.md (182 lines)
- helm-values-guide.md (266 lines)
- file-organization.md (224 lines)
- validation-guide.md (114 lines)

**Templates:**
- 5 pattern directories with kustomization.yaml examples

**Total:** ~2,000 lines (SKILL + references)

### release-engineering (893 lines SKILL.md)

**Structure:**
- ⚠️ CRITICAL: The Golden Rule (config.env vs values.yaml)
- When to Use: 7 specific triggers
- **NEW:** Quick Start: Makefile orchestration (site-setup)
- The 5 Core Workflows: Each with structure/examples/scenarios
- The Universal Renderer: render_values.py
- Configuration Variables: 67 total
- GitLab CI Integration
- Advanced Examples: Points to pipelines/
- References (JIT): 4 references
- Common Scenarios: 4 realistic workflows
- Error Handling: 6 common errors + recovery
- Safety Checks: Before/after/never for each workflow
- Integration with Other Skills
- Python Standards & Tooling

**References:**
- harmonize-guide.md (280 lines)
- scaffold-guide.md (306 lines)
- exit-codes.md (257 lines)
- config-schema.md (439 lines)

**Templates:**
- config.env.example (full template)

**Total:** ~2,175 lines (SKILL + references)

---

## When NOT to Use This Pattern

**Use simpler patterns when:**

- ❌ Skill has only 1-2 workflows (use basic pattern)
- ❌ Skill is <200 lines total (no need for references/)
- ❌ Skill is purely AI-powered (no deterministic workflows)
- ❌ Skill has no configuration (no schema needed)
- ❌ Skill has no error codes (no exit-codes.md needed)

**This comprehensive pattern is for:**
- ✅ Multi-workflow skills (3-5+ workflows)
- ✅ Complete lifecycle management
- ✅ Integration with external systems
- ✅ Automation-friendly (CI/CD)
- ✅ 500-1000 line SKILL.md
- ✅ 1000-2000 lines total (with references)

---

## Summary: The Comprehensive Skill Formula

```
Comprehensive Skill =
  ⚠️ CRITICAL Warning
  + When to Use (specific triggers)
  + Quick Start (one-liner)
  + N Core Workflows (structure/examples/scenarios)
  + Universal Tool (if applicable)
  + Configuration (schema/variables)
  + Advanced Examples (real files)
  + References (JIT loadable)
  + Common Scenarios (realistic workflows)
  + Error Handling (codes + recovery)
  + Safety Checks (before/after/never)
  + Integration (other skills)
```

**Result:** Users can:
1. Start immediately (Quick Start)
2. Avoid common mistakes (Critical Warnings)
3. Execute complete workflows (N Core Workflows)
4. Handle errors gracefully (Error Handling)
5. Work safely (Safety Checks)
6. Integrate with ecosystem (Other Skills)
7. Learn progressively (JIT References)

**This pattern creates production-grade skills that scale from beginner to expert.**
