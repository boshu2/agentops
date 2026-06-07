# Scenario Schema Reference

Schema definition for holdout scenario files stored in `.agents/holdout/`.
The canonical JSON Schema is at `schemas/scenario.v1.schema.json`.

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique identifier, format: `s-YYYY-MM-DD-NNN` |
| `version` | integer | yes | Schema version (currently `1`) |
| `date` | string | yes | ISO 8601 date when the scenario was authored |
| `goal` | string | yes | One-line statement of what the scenario validates |
| `narrative` | string | yes | Multi-sentence description of the user journey or system behavior |
| `expected_outcome` | string | yes | What success looks like in concrete, observable terms |
| `acceptance_vectors` | array | yes | List of measurable checks (see below) |
| `satisfaction_threshold` | number | yes | Minimum overall score to pass (0.0-1.0) |
| `scope` | object | no | Files, functions, and behaviors the scenario covers |
| `source` | string | yes | Provenance: `human`, `agent`, or `prod-telemetry` |
| `status` | string | yes | Lifecycle state: `active`, `retired`, `blocked`, or `draft` |

### Acceptance Vector Fields

Each entry in `acceptance_vectors` contains:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `dimension` | string | yes | What aspect is measured (e.g., `correctness`, `performance`, `security`) |
| `threshold` | number | yes | Minimum score for this vector (0.0-1.0) |
| `check` | string | yes | Shell command that exits 0 on success, non-zero on failure |

### Scope Fields

The optional `scope` object contains:

| Field | Type | Description |
|-------|------|-------------|
| `files` | array of strings | File paths relevant to this scenario |
| `functions` | array of strings | Function names exercised by this scenario |
| `behaviors` | array of strings | High-level behavior labels |

## Example Scenario

```json
{
    "id": "s-2026-04-05-001",
    "version": 1,
    "date": "2026-04-05",
    "goal": "User can authenticate with valid credentials",
    "narrative": "A user visits the login page, enters valid credentials, and expects to be redirected to the dashboard with a session cookie set.",
    "expected_outcome": "Dashboard loads within 2 seconds, session cookie is HttpOnly and Secure, user profile data is displayed correctly.",
    "acceptance_vectors": [
        {
            "dimension": "correctness",
            "threshold": 0.9,
            "check": "grep -q 'HttpOnly' response_headers"
        },
        {
            "dimension": "performance",
            "threshold": 0.7,
            "check": "curl -o /dev/null -w '%{time_total}' | awk '{exit ($1 > 2)}'"
        }
    ],
    "satisfaction_threshold": 0.8,
    "scope": {
        "files": ["src/auth/middleware.go", "src/auth/session.go"],
        "functions": ["Authenticate", "CreateSession"],
        "behaviors": ["login flow", "session management"]
    },
    "source": "human",
    "status": "active"
}
```

## Scoring

During validation (STEP 1.8 of `/validate`), each acceptance vector's
`check` command runs against the implementation. The vector scores 1.0 if
the command exits 0, and 0.0 otherwise. The scenario's overall satisfaction
score is the average across all vectors, weighted equally.

A scenario passes if its overall score meets or exceeds `satisfaction_threshold`.

## Schema Location

The canonical JSON Schema lives at `schemas/scenario.v1.schema.json` in this
repository. Use it for programmatic validation:

```bash
ao scenario validate
```

This runs every `.json` file in `.agents/holdout/` against the schema and
reports violations.
