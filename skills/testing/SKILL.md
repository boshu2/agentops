---
name: testing
description: Execute testing workflows including unit tests, manifest validation, security scanning, and integration tests with proper reporting
tags: [testing, validation, ci/cd, quality]
---

# Testing Skill

Provides comprehensive testing workflows for validation, security scanning, and quality assurance across the GitOps infrastructure.

## When to Use

Use this skill when you need to:
- Run unit tests (pytest) against Python code
- Validate manifests against schema and policies
- Perform security scanning (SAST, dependency checks)
- Run integration tests (Kustomization, Helm, Argo CD)
- Generate coverage reports
- Perform pre-commit validation
- Execute full CI suite before merging

## How to Invoke

**Automatic (Claude decides):**
```
Run the test suite to ensure the changes are correct
```

**Explicit:**
```
Use the testing skill to validate the manifests and run tests
```

## Testing Types

### 1. Unit Tests (Python)

**Run all tests:**
```bash
uv run pytest tests/ -v
```

**Run specific test file:**
```bash
uv run pytest tests/test_harmonize.py -v
```

**Run with coverage:**
```bash
uv run pytest tests/ --cov=tools/scripts --cov-report=html
```

**TDD Workflow:**
```bash
# 1. Write test (RED - fails)
uv run pytest tests/test_new_feature.py -v

# 2. Write minimal code (GREEN - passes)
uv run pytest tests/test_new_feature.py -v

# 3. Refactor with confidence
uv run pytest tests/ --cov=mymodule --cov-report=term
```

### 2. Manifest Validation

**Quick syntax check:**
```bash
make quick              # YAML syntax (5 seconds)
```

**Single app test:**
```bash
make test-app APP=myapp  # Render single app (3 seconds)
```

**Full validation:**
```bash
make ci-all             # Complete CI suite (30 seconds)
```

### 3. Kustomization Tests

**Verify kustomization builds:**
```bash
kustomize build apps/myapp/
```

**Check patches apply correctly:**
```bash
kustomize build apps/myapp/ | grep -A 5 "patchedResource"
```

**Validate referenced resources exist:**
```bash
kustomize cfg print apps/myapp/kustomization.yaml
```

### 4. Helm Tests

**Validate Helm charts:**
```bash
helm lint apps/myapp/    # Check chart syntax
helm template myapp apps/myapp/  # Preview rendered output
```

**Test Helm values:**
```bash
helm template myapp apps/myapp/ \
  --values apps/myapp/values.yaml \
  --validate
```

### 5. Security Scanning

**SAST (Static Application Security Testing):**
```bash
# Bandit: Find Python security issues
bandit -r tools/scripts/ -f json > security-report.json

# Semgrep: Advanced pattern matching
semgrep --config=p/security-audit .
```

**Dependency Scanning:**
```bash
# Check for vulnerable dependencies
pip-audit                    # Python packages
npm audit                    # JavaScript packages
```

**Secret Detection:**
```bash
# Scan for hardcoded secrets
git log -p -S "password" .
git log -p -S "secret" .
gitleaks detect --verbose    # With tool installed
```

### 6. Policy Validation (Kyverno)

**Dry-run policy against manifests:**
```bash
kyverno apply policies/ \
  --resource apps/myapp/manifest.yaml \
  --resource-kind Deployment \
  --values config.env
```

**List all policies:**
```bash
kyverno apply policies/ --list
```

## Pre-Commit Validation

**Quick checks before committing:**

```bash
# 1. Quick YAML syntax (5 sec)
make quick

# 2. Single app render (3 sec)
make test-app APP=myapp

# 3. Commit when ready
git add apps/ config.env
git commit -m "..."
```

## Pre-Push Validation

**Full CI before pushing to main:**

```bash
# Run entire CI suite (30 sec)
make ci-all

# If all checks pass, safe to push
git push origin feature/my-feature
```

## CI/CD Integration

### GitHub Actions

CI runs automatically on:
- Pull requests to main
- Commits to feature branches
- Tags for releases

**Test pipeline stages:**
1. YAML syntax validation (quick)
2. Kustomization builds (build)
3. Helm template validation (build)
4. Security scanning (sast)
5. Python unit tests (unit)
6. Coverage reports (reporting)

### GitLab CI

**Pipeline stages:**
```yaml
stages:
  - validate      # Syntax checks
  - build         # Kustomize, Helm renders
  - test          # Unit tests, integration tests
  - security      # SAST, dependency scanning
  - report        # Coverage, artifacts
```

## Test Results Interpretation

### Green (Passing)

```
✅ All 47 tests PASSED
✅ 92% code coverage (threshold: 85%)
✅ No security issues found
✅ All manifests valid
```

**Next:** Safe to commit/push.

### Yellow (Warnings)

```
⚠️  3 tests PASSED, 2 SKIPPED
⚠️  87% code coverage (threshold: 85%) - close!
⚠️  2 low-severity security issues (review before merging)
✅ All manifests valid
```

**Next:** Review warnings, decide if acceptable, document decision.

### Red (Failing)

```
❌ 5 tests FAILED out of 47
❌ 72% code coverage (threshold: 85%)
❌ Critical security issue: hardcoded credentials
❌ 3 manifests invalid
```

**Next:** Fix issues, re-run tests, don't commit until green.

## Test Coverage

**Minimum requirements:**
- Unit tests: ≥85% coverage
- Manifests: 100% validation pass
- Security: Zero critical issues

**Coverage report:**
```bash
uv run pytest tests/ --cov --cov-report=html
open htmlcov/index.html
```

## Common Test Patterns

### Pattern 1: Test-Driven Development (TDD)

```bash
# 1. Write failing test
cat > tests/test_new_feature.py << 'EOF'
def test_feature():
    assert new_function(5) == 10
EOF

# 2. Run (RED - fails)
uv run pytest tests/test_new_feature.py

# 3. Write minimal code (GREEN)
def new_function(x):
    return x * 2

# 4. Verify (GREEN)
uv run pytest tests/test_new_feature.py

# 5. Refactor + re-test
```

### Pattern 2: Integration Test

```bash
# 1. Render config
make harmonize SITE=nyc DRY_RUN=true

# 2. Validate output
kustomize build apps/nyc/ > /tmp/manifest.yaml

# 3. Check against policies
kyverno apply policies/ --resource /tmp/manifest.yaml

# 4. Commit if clean
```

### Pattern 3: Regression Test

```bash
# 1. Save baseline
kubectl get deployment -o yaml > /tmp/baseline.yaml

# 2. Apply changes
git apply patch.diff

# 3. Compare
diff /tmp/baseline.yaml <(kubectl get deployment -o yaml)
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| Tests fail locally but pass in CI | Python version mismatch | Use `uv run pytest` (venv isolated) |
| Coverage below threshold | New code not tested | Add tests before committing |
| Manifest validation fails | Invalid YAML or API version | Run `make quick` to debug |
| Security scan timeout | Large codebase | Run on specific directories |

## Integration with Other Skills

Works with:
- **manifest-validation**: Automated during tests
- **config-rendering**: Test rendered output
- **git-workflow**: Run tests before commit

## Related Documentation

- [Testing guide](../../docs/how-to/testing/testing-guide.md)
- [CI/CD pipeline](../../docs/how-to/ci-cd/pipeline-reference.md)
- [Coverage requirements](../../docs/reference/coverage-standards.md)
- [Makefile reference](../../docs/reference/makefile-reference.md)
