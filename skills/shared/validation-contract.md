# Validation Contract

> **The Trust Problem**: Agent completion claims cannot be trusted. Verify then trust.

## Overview

This document specifies how validation requirements are defined, executed, and enforced in the swarm/crank architecture.

```
BROKEN (old):
<task-notification> --> TaskUpdate(completed) --> bd close
                        ^ TRUST (no verification)

CORRECT (new):
<task-notification> --> RUN VALIDATION --> IF PASS --> complete
                                       --> IF FAIL --> retry/escalate
                        ^ VERIFY then trust
```

---

## Specifying Validation Requirements

### TaskCreate Metadata

Validation requirements are specified in the `metadata.validation` field when creating tasks:

```
TaskCreate(
  subject="Implement feature X",
  description="...",
  metadata={
    "validation": {
      "files_exist": ["path/to/file1", "path/to/file2"],
      "command": "npm test",
      "content_check": {"file": "path/to/file", "pattern": "expected_pattern"},
      "tests": "pytest tests/test_feature.py",
      "lint": "eslint src/"
    }
  }
)
```

### Validation Types

| Type | Schema | Description |
|------|--------|-------------|
| `files_exist` | `string[]` | List of file paths that must exist after task completion |
| `command` | `string` | Shell command that must exit with code 0 |
| `content_check` | `{file: string, pattern: string}` | File must contain pattern (regex supported) |
| `tests` | `string` | Test command that must pass |
| `lint` | `string` | Lint command that must pass |
| `custom` | `{name: string, command: string}` | Named custom check |
| `cross_cutting` | `{name, type, ...}[]` | Epic-level constraints from "Always" boundaries, applied to every task |

### Multiple Checks

All specified checks must pass. Order of execution:

1. `files_exist` - fastest, fail-fast
2. `content_check` - fast pattern matching
3. `lint` - catch style issues before tests
4. `tests` - comprehensive verification
5. `command` - custom commands last
6. `custom` - any additional custom checks
7. `cross_cutting` - epic-level constraints last

---

## Validation Check Details

### files_exist

Verifies that specified files exist after task completion.

**Schema:**
```json
{
  "files_exist": ["src/auth.py", "tests/test_auth.py"]
}
```

**Execution:**
```bash
for file in files_exist:
    if not os.path.exists(file):
        FAIL("File not found: " + file)
PASS
```

**Use when:** Task creates new files.

### command

Runs arbitrary shell command, checks exit code.

**Schema:**
```json
{
  "command": "make build"
}
```

**Execution:**
```bash
result = subprocess.run(command, shell=True)
if result.returncode != 0:
    FAIL("Command failed with exit code: " + result.returncode)
PASS
```

**Use when:** Need custom build/validation step.

### content_check

Verifies file contains expected content.

**Schema:**
```json
{
  "content_check": {
    "file": "src/config.py",
    "pattern": "API_VERSION = \"2.0\""
  }
}
```

**Multiple patterns:**
```json
{
  "content_check": [
    {"file": "src/auth.py", "pattern": "def authenticate"},
    {"file": "src/auth.py", "pattern": "def authorize"}
  ]
}
```

**Execution:**
```bash
content = read_file(file)
if not regex.search(pattern, content):
    FAIL("Pattern not found in " + file + ": " + pattern)
PASS
```

**Use when:** Task must implement specific functions/patterns.

### tests

Runs test suite, checks for passing tests.

**Schema:**
```json
{
  "tests": "pytest tests/test_feature.py -v"
}
```

**Execution:**
```bash
result = subprocess.run(tests, shell=True)
if result.returncode != 0:
    FAIL("Tests failed")
PASS
```

**Use when:** Task has associated tests.

### lint

Runs linter, checks for clean output.

**Schema:**
```json
{
  "lint": "ruff check src/"
}
```

**Execution:**
```bash
result = subprocess.run(lint, shell=True)
if result.returncode != 0:
    FAIL("Lint errors found")
PASS
```

**Use when:** Code quality must be maintained.

### custom

Named custom check for documentation.

**Schema:**
```json
{
  "custom": {
    "name": "Database migration",
    "command": "python manage.py migrate --check"
  }
}
```

**Use when:** Domain-specific validation needed.

### cross_cutting

Epic-level constraints applied to EVERY task. Derived from "Always" boundaries in the plan.

**Schema:**
```json
{
  "cross_cutting": [
    {"name": "auth-required", "type": "content_check", "file": "src/middleware.go", "pattern": "AuthMiddleware"},
    {"name": "tests-pass", "type": "tests", "command": "go test ./..."},
    {"name": "builds-clean", "type": "command", "command": "go build ./..."}
  ]
}
```

Each entry is a **flat object** with:
- `name` (string): Human-readable label for the constraint
- `type` (string): One of `files_exist`, `content_check`, `command`, `tests`, `lint`
- Remaining fields: Same as the corresponding validation type above

**Execution:**
```bash
# Run AFTER all per-task checks pass
for check in cross_cutting:
    run_check(check.type, check)  # Same execution logic as per-task checks
    if FAIL:
        FAIL(f"Cross-cutting constraint '{check.name}' failed")
PASS
```

**Use when:** Plan defines "Always" boundaries that apply to every issue in the epic. /crank reads these from the epic description and injects into every worker task.

**Source:** Cross-cutting constraints flow from plan boundaries:
```
Plan "Always" boundaries → Epic description → /crank extracts → TaskCreate metadata
```

---

## Failure Handling

### Retry Strategy

| Failure Type | Retry Action | Max Retries |
|--------------|--------------|-------------|
| `files_exist` | Re-spawn agent with explicit file list | 2 |
| `command` | Re-spawn with command output as context | 3 |
| `content_check` | Re-spawn with exact pattern required | 2 |
| `tests` | Re-spawn with test failure details | 3 |
| `lint` | Re-spawn with lint errors | 2 |

### Retry Context

When retrying, include failure context in the agent's prompt:

```
RETRY task #<id>: Previous attempt failed validation.

## Original Task
<original description>

## Validation Failure
Type: <check_type>
Details: <failure_output>

## Required Fix
<specific guidance based on failure>

Complete the task and ensure validation passes."
)
```

### Escalation

After MAX_RETRIES failures:

1. Mark task as blocked:
   ```
   TaskUpdate(taskId="<id>", status="blocked")
   ```

2. Record failure history:
   ```
   TaskUpdate(taskId="<id>", description="<original>

   ## ESCALATED - Validation Failures
   Attempt 1: <failure>
   Attempt 2: <failure>
   Attempt 3: <failure>

   Requires human review.")
   ```

3. Continue with other tasks (don't block entire swarm)

---

## Default Validation

When no explicit validation is specified, apply minimal checks:

```python
def default_validation(task_id, worker_artifacts):
    # Check agent didn't end with errors
    # (parse task notification / SendMessage envelope for failure indicators)

    # Check worker reported artifacts exist
    # Workers do NOT commit — they write files and report via SendMessage.
    # The team lead validates artifacts exist before committing.
    for artifact in worker_artifacts:
        if not os.path.exists(artifact):
            return FAIL(f"Reported artifact not found: {artifact}")

    # Check for modified files (workers write in main tree or isolated worktrees)
    result = subprocess.run("git status --porcelain", shell=True, capture_output=True)
    if not result.stdout.strip():
        return WARN("No file changes detected — worker may not have written anything")

    return PASS
```

> **Note:** Workers MUST NOT commit. The team lead is the sole committer.
> Validation checks for file existence and content, not commit history.
> The lead commits all validated changes after the wave completes.
> See `skills/swarm/SKILL.md` "Git Commit Policy" for details.

---

## Integration with Crank

When crank invokes swarm, it can specify validation at the epic level:

```python
# Crank creates tasks from beads issues
for issue in ready_issues:
    TaskCreate(
        subject=f"{issue.id}: {issue.title}",
        description=issue.description,
        metadata={
            "beads_id": issue.id,
            "validation": build_validation_from_issue(issue)
        }
    )
```

### Building Validation from Issue

```python
def build_validation_from_issue(issue):
    validation = {}

    # Check for test requirements in issue
    if "test" in issue.labels or "tests/" in issue.description:
        validation["tests"] = detect_test_command(issue)

    # Check for file creation requirements
    files_mentioned = extract_file_paths(issue.description)
    if files_mentioned:
        validation["files_exist"] = files_mentioned

    # Check for function/method requirements
    patterns = extract_code_patterns(issue.description)
    if patterns:
        validation["content_check"] = patterns

    return validation
```

---

## Examples

### Example 1: New Feature with Tests

```
TaskCreate(
  subject="Add user authentication",
  description="Implement JWT-based authentication...",
  metadata={
    "validation": {
      "files_exist": [
        "src/auth/jwt.py",
        "src/auth/__init__.py",
        "tests/test_auth.py"
      ],
      "content_check": [
        {"file": "src/auth/jwt.py", "pattern": "def create_token"},
        {"file": "src/auth/jwt.py", "pattern": "def verify_token"}
      ],
      "tests": "pytest tests/test_auth.py -v",
      "lint": "ruff check src/auth/"
    }
  }
)
```

### Example 2: Bug Fix

```
TaskCreate(
  subject="Fix null pointer in user lookup",
  description="Handle case where user not found...",
  metadata={
    "validation": {
      "content_check": {
        "file": "src/users/lookup.py",
        "pattern": "if user is None"
      },
      "tests": "pytest tests/test_users.py::test_user_not_found -v"
    }
  }
)
```

### Example 3: Documentation Update

```
TaskCreate(
  subject="Update API docs for v2",
  description="Update README with new endpoints...",
  metadata={
    "validation": {
      "files_exist": ["docs/api/v2.md"],
      "content_check": {
        "file": "docs/api/v2.md",
        "pattern": "## Authentication"
      }
    }
  }
)
```

### Example 4: Infrastructure Change

```
TaskCreate(
  subject="Add Redis caching layer",
  description="Configure Redis for session caching...",
  metadata={
    "validation": {
      "files_exist": ["docker-compose.yml", "src/cache/redis.py"],
      "command": "docker-compose config --quiet",
      "content_check": {
        "file": "docker-compose.yml",
        "pattern": "redis:"
      }
    }
  }
)
```

---

## Validation in Distributed Mode

In distributed mode (tmux + Agent Mail), validation works the same but with message-based coordination:

1. **Demigod completes work** -> sends `OFFERING_READY` message
2. **Mayor receives message** -> runs validation checks
3. **On PASS** -> sends `OFFERING_ACCEPTED`, closes beads issue
4. **On FAIL** -> sends `RETRY_REQUIRED` with failure context

See `skills/swarm/SKILL.md` for distributed mode details.

---

## See Also

- `skills/swarm/SKILL.md` - Main swarm skill with validation integration
- `skills/crank/SKILL.md` - Crank orchestration with validation loop
- `skills/crank/failure-taxonomy.md` - Comprehensive failure handling
- `skills/vibe/SKILL.md` - Comprehensive validation skill
