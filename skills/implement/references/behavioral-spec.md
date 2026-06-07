# Generate Behavioral Spec (Optional)

**Skip if:** `--no-spec` flag, or issue type is `docs`/`chore`/`ci`.

After verification passes, produce a behavioral spec documenting what the implementation
does. This feeds Stage 4 behavioral validation (STEP 1.8 in /validate).

```bash
mkdir -p .agents/specs
cat > .agents/specs/<issue-id>.json <<'SPEC'
{
    "id": "auto-<issue-id>",
    "version": 1,
    "date": "<YYYY-MM-DD>",
    "goal": "<one-line: what user outcome this implementation serves>",
    "narrative": "<2-3 sentences: what the implementation does and how a user interacts with it>",
    "expected_outcome": "<what a satisfied user observes when this works correctly>",
    "acceptance_vectors": [
        {
            "dimension": "<name: correctness|performance|usability|security|...>",
            "threshold": <0.0-1.0>,
            "check": "<optional: mechanical check command>"
        }
    ],
    "satisfaction_threshold": 0.7,
    "scope": {
        "files": ["<list of modified files>"],
        "functions": ["<key functions added/modified>"],
        "behaviors": ["<behavioral descriptions>"]
    },
    "source": "agent",
    "status": "active"
}
SPEC
```

**Guidelines:**
- `acceptance_vectors` should capture the BEHAVIORAL contract, not test assertions.
  Example: `{"dimension": "isolation", "threshold": 1.0, "check": "echo ... | bash hook; test $? -eq 2"}`
- Include at least 2 acceptance vectors (correctness + one other dimension).
- `scope.files` must match the files you actually modified (not planned files).
- The spec is validated by the evaluator council during STEP 1.8 — it is NOT
  visible to YOU during implementation (holdout isolation applies to agent-built
  specs the same way it applies to human-written scenarios).

**If skipped:** Log "Behavioral spec skipped (reason: <flag|issue-type>)" and proceed.
