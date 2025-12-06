---
name: test-gap-scanner
description: Scan Python codebases to identify modules without corresponding test files. This skill should be used when enforcing Law 7 (TDD), auditing test coverage gaps, or before starting new development to understand testing debt. Outputs a report of untested modules with recommendations.
---

# Test Gap Scanner

## Overview

Scan a Python codebase to find modules that lack corresponding test files. Enforces Law 7 (TDD) by identifying testing gaps before they become technical debt.

## When to Use

- Before starting new development (understand existing gaps)
- After completing a feature (verify tests were created)
- During code review (ensure Law 7 compliance)
- Sprint planning (prioritize testing debt)
- CI/CD integration (fail builds with untested code)

## Quick Start

```bash
# Scan current directory
python scripts/scan_test_gaps.py .

# Scan specific directory with custom test path
python scripts/scan_test_gaps.py ./tools --test-dir ./tests

# Output as JSON for CI integration
python scripts/scan_test_gaps.py . --format json

# Fail if coverage below threshold (for CI)
python scripts/scan_test_gaps.py . --min-coverage 80

# Scan ALL git repositories under a root directory
python scripts/scan_test_gaps.py ~/workspaces --all-repos
```

## How It Works

### Detection Algorithm

1. **Find all Python modules** (excluding `__init__.py`, tests, migrations)
2. **Search for corresponding test files** using patterns:
   - `module.py` → `test_module.py`
   - `module.py` → `tests/test_module.py`
   - `module.py` → `tests/unit/test_module.py`
   - `path/to/module.py` → `tests/path/to/test_module.py`
3. **Calculate coverage percentage** (modules with tests / total modules)
4. **Generate report** with gaps and recommendations

### Output Format

**Console (default):**
```
Test Gap Scanner Report
=======================

Directory: ./tools/parallel-learning
Total modules: 12
Modules with tests: 8
Coverage: 66.7%

MISSING TESTS (4 modules):
  ❌ tools/scripts/validate_context.py
     Expected: tests/test_validate_context.py
  ❌ tools/scripts/check_links.py
     Expected: tests/test_check_links.py
  ❌ memory/__init__.py
     Expected: tests/test_memory.py (or skip __init__.py)
  ❌ utils/helpers.py
     Expected: tests/unit/test_helpers.py

RECOMMENDATIONS:
  1. Create test files for 4 missing modules
  2. Run: pytest tests/ --cov to verify coverage
  3. Add to CI: --min-coverage 80 to enforce threshold
```

**JSON (for CI):**
```json
{
  "directory": "./tools/parallel-learning",
  "total_modules": 12,
  "tested_modules": 8,
  "coverage_percent": 66.7,
  "missing_tests": [
    {
      "module": "tools/scripts/validate_context.py",
      "expected_test": "tests/test_validate_context.py"
    }
  ],
  "status": "FAIL",
  "threshold": 80
}
```

**Multi-Repo Output (--all-repos):**
```
Test Gap Scanner - Multi-Repo Report
============================================================

Root: /Users/me/workspaces
Repos scanned: 32
Repos with Python: 18
Total modules: 227
Total tested: 8
Overall coverage: 3.5%

COVERAGE BY REPOSITORY:
------------------------------------------------------------
Repository                           Modules   Tested Coverage
------------------------------------------------------------
❌ personal/agent-builder                   2        0     0.0%
❌ work/gitops                            100        0     0.0%
✔ release-engineering                      5        3    60.0%
✔ example-repo                                  3        2    66.7%
------------------------------------------------------------

HIGH PRIORITY GAPS (0% coverage, 3+ modules):
  ❌ gitops: 100 untested modules
  ❌ claude-code-templates: 71 untested modules

Status: ❌ FAIL (threshold: 80%)
```

## Configuration

### Exclude Patterns

By default, these are excluded from scanning:
- `__init__.py` (can be included with `--include-init`)
- `**/tests/**` (test files themselves)
- `**/migrations/**` (database migrations)
- `**/.venv/**` (virtual environments)
- `**/node_modules/**` (JS dependencies)

### Custom Test Patterns

```bash
# Custom test file pattern
python scripts/scan_test_gaps.py . --test-pattern "spec_{module}.py"

# Multiple test directories
python scripts/scan_test_gaps.py . --test-dir tests --test-dir spec
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Check test coverage gaps
  run: |
    python .claude/skills/test-gap-scanner/scripts/scan_test_gaps.py \
      ./src --min-coverage 80 --format json > test-gaps.json

- name: Fail if gaps exceed threshold
  run: |
    if grep -q '"status": "FAIL"' test-gaps.json; then
      echo "Test coverage below threshold!"
      cat test-gaps.json
      exit 1
    fi
```

### GitLab CI

```yaml
test-gap-scan:
  script:
    - python .claude/skills/test-gap-scanner/scripts/scan_test_gaps.py . --min-coverage 80
  allow_failure: false
```

### Pre-commit Hook

```yaml
# .pre-commit-config.yaml
- repo: local
  hooks:
    - id: test-gap-scanner
      name: Check for untested modules
      entry: python .claude/skills/test-gap-scanner/scripts/scan_test_gaps.py
      args: [--min-coverage, "70"]
      language: python
      pass_filenames: false
```

## Law 7 Enforcement

This skill helps enforce **Law 7: ALWAYS TDD (Test-Driven Development)**:

| Scenario | Action |
|----------|--------|
| New module created | Scanner detects missing test, blocks commit |
| Coverage drops | CI fails, requires test before merge |
| Technical debt audit | Generate report of all gaps |
| Sprint planning | Prioritize untested modules |

## Related Skills

- **testing**: Run existing tests (`uv run pytest`)
- **test-pipeline**: Full CI validation pipeline
- **manifest-validation**: YAML/schema validation

## Scripts

### scan_test_gaps.py

Main scanner script. Run with `--help` for all options:

```bash
python scripts/scan_test_gaps.py --help
```
