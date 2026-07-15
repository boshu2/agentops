# Validation Script for Research Skill

## Overview

The `validate.sh` script ensures the `/research` skill meets basic quality and completeness standards. It runs a series of checks against the skill's structure, documentation, and references.

## Purpose

This validation script serves as a quality gate for the research skill, ensuring:

- Required files exist with correct structure
- Documentation includes essential patterns and concepts
- References directory contains sufficient resource materials

## Script Location

```
skills/research/scripts/validate.sh
```

## Script Execution

The script performs the following checks:

### Basic Structure Validation
- **SKILL.md exists**: Verifies the primary skill documentation file
- **SKILL.md has YAML frontmatter**: Ensures proper metadata formatting
- **name: research**: Confirms correct skill identification
- **references/ directory exists**: Validates reference materials directory
- **references/ has at least 3 files**: Ensures minimum reference coverage

### Documentation Content Validation
- **SKILL.md mentions .agents/research/ output path**: Confirms documented output location
- **SKILL.md mentions .agents/findings/registry.jsonl**: Confirms the reusable-finding registry bridge
- **SKILL.md mentions reusable findings**: Ensures transient notes are not treated as durable registry entries
- **SKILL.md mentions dedup_key**: Confirms the merge key required by the registry contract
- **SKILL.md mentions temp-file-plus-rename atomic write rule**: Confirms the registry write semantics
- **SKILL.md mentions finding-compiler.sh refresh**: Confirms the follow-up compiler pass is documented
- **SKILL.md mentions Explore agent**: Ensures agent reference is included
- **SKILL.md mentions --auto flag**: Validates feature documentation
- **SKILL.md mentions ao lookup or ao search**: Checks CLI integration documentation
- **SKILL.md mentions knowledge flywheel**: Confirms system architecture coverage
- **SKILL.md mentions backend detection**: Validates technical implementation details
- **SKILL.md mentions quality validation**: Ensures quality assurance documentation

## Usage

### Manual Execution

```bash
# From the project root directory
./skills/research/scripts/validate.sh
```

### Expected Output

```
PASS: SKILL.md exists
PASS: SKILL.md has YAML frontmatter
PASS: SKILL.md has name: research
PASS: references/ directory exists
PASS: references/ has at least 3 files
PASS: SKILL.md mentions .agents/research/ output path
PASS: SKILL.md mentions .agents/findings/registry.jsonl
PASS: SKILL.md mentions reusable findings
PASS: SKILL.md mentions dedup_key
PASS: SKILL.md mentions temp-file-plus-rename atomic write rule
PASS: SKILL.md mentions finding-compiler.sh refresh
PASS: SKILL.md mentions Explore agent
PASS: SKILL.md mentions --auto flag
PASS: SKILL.md mentions ao lookup or ao search
PASS: SKILL.md mentions knowledge flywheel
PASS: SKILL.md mentions backend detection
PASS: SKILL.md mentions quality validation

Results: 17 passed, 0 failed
```

## Integration with CI/CD

This script can be integrated into continuous integration workflows to ensure the research skill meets quality standards before deployment:

```yaml
# Example GitHub Actions workflow
- name: Validate Research Skill
  run: ./skills/research/scripts/validate.sh
```

## Exit Codes

- **0**: All checks passed (success)
- **1**: One or more checks failed
- **2**: Script execution error

## Development Workflow

### Adding New Features to Research Skill

1. **Implement the feature** in the skill's codebase
2. **Update SKILL.md** to document the new functionality
3. **Run validation script** to ensure documentation is complete:
   ```bash
   ./skills/research/scripts/validate.sh
   ```
4. **Address any failures** by updating documentation or code
5. **Commit changes** with confidence the skill meets quality standards

### Updating Validation Criteria

To modify validation criteria:

1. **Edit validate.sh** to add/remove checks as needed
2. **Update this documentation** to reflect new validation requirements
3. **Test the updated script** against the current skill implementation
