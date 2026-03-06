# TaskCreate Examples

> Copy-paste-ready TaskCreate patterns for each crank mode.

---

## SPEC WAVE TaskCreate

Use when `--test-first` is set and issue is spec-eligible (`feature`/`bug`/`task`).

```
TaskCreate(
  subject="SPEC: <issue-title>",
  description="Generate contract for beads issue <issue-id>.

Details from beads:
<paste issue details from bd show>

You are a spec writer. Generate a contract for this issue.

FIRST: Explore the codebase to understand existing patterns, types, and interfaces
relevant to this issue. Use Glob and Read to examine the code.

THEN: Read the contract template at skills/crank/references/contract-template.md.

Generate a contract following the template. Include:
- At least 3 invariants
- At least 3 test cases mapped to invariants
- Concrete types and interfaces from the actual codebase

If inputs are missing or the issue is underspecified, write BLOCKED with reason.

Output: .agents/specs/contract-<issue-id>.md

```validation
files_exist:
  - .agents/specs/contract-<issue-id>.md
content_check:
  - file: .agents/specs/contract-<issue-id>.md
    pattern: "## Invariants"
  - file: .agents/specs/contract-<issue-id>.md
    pattern: "## Test Cases"
```

Mark task complete when contract is written and validation passes.",
  activeForm="Writing spec for <issue-id>"
)
```

---

## TEST WAVE TaskCreate

Use when `--test-first` is set, SPEC WAVE is complete, and issue is spec-eligible.

```
TaskCreate(
  subject="TEST: <issue-title>",
  description="Generate FAILING tests for beads issue <issue-id>.

Details from beads:
<paste issue details from bd show>

You are a test writer. Generate FAILING tests from the contract.

Read ONLY the contract at .agents/specs/contract-<issue-id>.md.
You may read codebase structure (imports, types, interfaces) but NOT existing
implementation details.

Generate tests that:
- Cover ALL test cases from the contract's Test Cases table
- Cover ALL invariants (at least one test per invariant)
- All tests MUST FAIL when run (RED state)
- Follow existing test patterns in the codebase

Do NOT read or reference existing implementation code.
Do NOT write implementation code.

Output: test files in the appropriate location for the project's test framework.

```validation
files_exist:
  - <test-file-path-1>
  - <test-file-path-2>
```

> **RED verification note:** The validation gate runs commands directly (no shell wrappers).
> To confirm tests FAIL, the worker must run the test command manually and verify non-zero
> exit before marking complete. Do NOT put negated commands in validation metadata — the
> gate would treat test failure as a validation failure. Workers self-verify RED state.

Mark task complete when tests are written and ALL tests FAIL.",
  activeForm="Writing tests for <issue-id>"
)
```

---

## GREEN Mode TaskCreate

Use when `--test-first` is set, SPEC and TEST waves are complete, and issue is spec-eligible.

```
TaskCreate(
  subject="<issue-id>: <issue-title>",
  description="Implement beads issue <issue-id> (GREEN mode).

Details from beads:
<paste issue details from bd show>

**GREEN Mode:** Failing tests exist. Make them pass. Do NOT modify test files.

Failing tests are at:
- <test-file-path-1>
- <test-file-path-2>

Contract is at: .agents/specs/contract-<issue-id>.md

Follow GREEN Mode rules from /implement SKILL.md:
1. Read failing tests and contract FIRST
2. Write minimal implementation to pass tests
3. Do NOT modify test files
4. Do NOT add tests (already written)
5. Validate by running test suite

Execute using /implement <issue-id>. Mark complete when all tests pass.",
  activeForm="Implementing <issue-id> (GREEN)"
)
```

---

## Standard IMPL TaskCreate (`feature`/`bug`/`task`)

Use when `--test-first` is NOT set for implementation issues. `metadata.validation` is required and MUST include:
- `tests`
- At least one structural check: `files_exist` or `content_check`

```
TaskCreate(
  subject="<issue-id>: <issue-title>",
  description="Implement beads issue <issue-id>.

Details from beads:
<paste issue details from bd show>

Execute using /implement <issue-id>. Mark complete when done.

```validation
tests: "<test-command>"
files_exist:
  - <expected-output-file-1>
content_check:
  - file: <expected-output-file-1>
    pattern: "<required-structure-pattern>"
command: "<optional-build-or-smoke-command>"
```

> **Allowlist-safe commands:** Validation commands run via `run_restricted()` which only
> permits: `go`, `pytest`, `npm`, `make`. No shell wrappers (`bash -c`), no compound
> operators (`&&`, `||`), no pipes or redirects. One command per field.
>
> Examples by language:
> - Go: `"tests": "go test ./..."`, `"command": "go vet ./..."`
> - Python: `"tests": "pytest tests/"`
> - Node: `"tests": "npm test"`
> - Multi-step: `"tests": "make test"` (put compound logic in Makefile)
",
  activeForm="Implementing <issue-id>",
  metadata={
    "files": ["<expected-modified-file-1>", "<expected-modified-file-2>"],
    "validation": {
      "tests": "<test-command>",
      "files_exist": ["<expected-output-file-1>"],
      "content_check": [
        {"file": "<expected-output-file-1>", "pattern": "<required-structure-pattern>"}
      ],
      "command": "<optional-build-or-smoke-command>"
    }
  }
)
```

---

## Docs/Chore/CI IMPL TaskCreate (Test Exemption)

Use for non-spec-eligible issues (`docs`/`chore`/`ci`). `tests` is optional for this category; keep structural or command/lint checks.

```
TaskCreate(
  subject="<issue-id>: <issue-title>",
  description="Implement beads issue <issue-id> (`docs`/`chore`/`ci` path).",
  activeForm="Implementing <issue-id>",
  metadata={
    "files": ["<expected-modified-file-1>"],
    "validation": {
      "files_exist": ["<expected-output-file-1>"],
      "content_check": {
        "file": "<expected-output-file-1>",
        "pattern": "<required-doc-or-config-pattern>"
      },
      "command": "<optional-smoke-command>",
      "lint": "<optional-lint-command>"
    }
  }
)
```

> **Allowlist reminder:** `command` and `lint` fields are also subject to `run_restricted()`.
> Use bare allowlisted binaries only: `go`, `pytest`, `npm`, `make`. Example:
> `"command": "make lint"`, `"lint": "npm run lint"`.

---

## Notes

- **Subject patterns:**
  - SPEC WAVE: `SPEC: <issue-title>` (no issue ID)
  - TEST WAVE: `TEST: <issue-title>` (no issue ID)
  - GREEN/IMPL: `<issue-id>: <issue-title>` (with issue ID)

- **Validation blocks:**
  - Fenced with triple backticks and `validation` language tag
  - Always include for SPEC and TEST waves
  - For `feature`/`bug`/`task` IMPL tasks: required `tests` + structural checks
  - For `docs`/`chore`/`ci` IMPL tasks: `tests` optional (explicit exemption path)
  - Consumed by lead during wave validation
  - **Allowlist constraint:** `tests`, `command`, and `lint` fields execute via `run_restricted()` in `task-validation-gate.sh`. Only bare allowlisted binaries are permitted: `go`, `pytest`, `npm`, `make`. Shell wrappers (`bash -c`), compound operators (`&&`, `||`, `;`), pipes (`|`), and redirects (`>`, `<`) are blocked. Use `make` targets for multi-step validation.

- **activeForm:**
  - Shows in TaskList UI while worker is active
  - Keep concise (3-5 words)
  - Include issue ID for easy tracking

- **Worker context:**
  - SPEC: codebase read access, contract template
  - TEST: contract only, codebase structure (not implementations)
  - GREEN: failing tests (immutable), contract, issue description
  - IMPL: full codebase access, issue description

- **File manifests (`metadata.files`):**
  - **Required** for all TaskCreate entries — list every file the worker will modify
  - Swarm uses manifests for pre-spawn conflict detection (overlapping files = serialize or isolate)
  - Workers receive the manifest in their prompt and must stay within it
  - Derive from issue description, plan, or codebase exploration during planning

- **Category-based skipping:**
  - docs/chore/ci issues bypass SPEC and TEST waves
  - Use standard IMPL TaskCreate for these even if `--test-first` is set
