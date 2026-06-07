# CM Command Reference

## Table of Contents
- [Context Retrieval](#context-retrieval)
- [Playbook Management](#playbook-management)
- [Learning & Feedback](#learning--feedback)
- [Onboarding](#onboarding)
- [Trauma Guard](#trauma-guard)
- [System Commands](#system-commands)

---

## Context Retrieval

```bash
# THE MAIN COMMAND - run before non-trivial tasks
cm context "implement user authentication" --json

# Limit results for token budget
cm context "fix bug" --json --limit 5 --no-history

# With workspace filter
cm context "refactor" --json --workspace /path/to/project

# Self-documenting explanation
cm quickstart --json

# System health
cm doctor --json
cm doctor --fix  # Auto-fix issues

# Find similar rules
cm similar "error handling best practices"
```

---

## Playbook Management

```bash
cm playbook list                              # All rules
cm playbook get b-8f3a2c                      # Rule details
cm playbook add "Always run tests first"      # Add rule
cm playbook add --file rules.json             # Batch add from file
cm playbook add --file rules.json --session /path/session.jsonl  # Track source
cm playbook remove b-xyz --reason "Outdated"  # Remove
cm playbook export > backup.yaml              # Export
cm playbook import shared.yaml                # Import
cm playbook bootstrap react                   # Apply starter to existing

cm top 10                                     # Top effective rules
cm stale --days 60                            # Rules without recent feedback
cm why b-8f3a2c                               # Rule provenance
cm stats --json                               # Playbook health metrics
```

---

## Learning & Feedback

```bash
# Manual feedback
cm mark b-8f3a2c --helpful
cm mark b-xyz789 --harmful --reason "Caused regression"
cm undo b-xyz789                              # Revert feedback

# Session outcomes (positional: status, rules)
cm outcome success b-8f3a2c,b-def456
cm outcome failure b-x7k9p1 --summary "Auth approach failed"
cm outcome-apply                              # Apply to playbook

# Reflection (usually automated)
cm reflect --days 7 --json
cm reflect --session /path/to/session.jsonl   # Single session
cm reflect --workspace /path/to/project       # Project-specific

# Validation
cm validate "Always check null before dereferencing"

# Audit sessions against rules
cm audit --days 30

# Deprecate permanently
cm forget b-xyz789 --reason "Superseded by better pattern"
```

### Inline Feedback

Leave feedback in code comments (parsed during reflection):

```typescript
// [cass: helpful b-8f3a2c] - this rule saved me from a rabbit hole
// [cass: harmful b-x7k9p1] - this advice was wrong for our use case
```

---

## Onboarding

Zero-cost playbook building using your existing agent:

```bash
cm onboard status                             # Check progress
cm onboard gaps                               # Category gaps
cm onboard sample --fill-gaps                 # Prioritized sessions
cm onboard sample --agent claude --days 14    # Filter by agent/time
cm onboard sample --workspace /path/project   # Filter by workspace
cm onboard sample --include-processed         # Re-analyze sessions
cm onboard read /path/session.jsonl --template  # Rich context
cm onboard mark-done /path/session.jsonl      # Mark processed
cm onboard reset                              # Start fresh
```

---

## Trauma Guard

```bash
cm trauma list                                # Active patterns
cm trauma add "DROP TABLE" --description "Mass deletion" --severity critical
cm trauma heal t-abc --reason "Intentional migration"
cm trauma remove t-abc
cm trauma scan --days 30                      # Scan for traumas
cm trauma import shared-traumas.yaml

cm guard --install                            # Claude Code hook
cm guard --git                                # Git pre-commit hook
cm guard --install --git                      # Both
cm guard --status                             # Check installation
```

---

## System Commands

```bash
cm init                                       # Initialize
cm init --starter typescript                  # With template
cm init --force                               # Reinitialize (creates backup)
cm starters                                   # List templates
cm serve --port 3001                          # MCP server
cm usage                                      # LLM cost stats
cm privacy status                             # Privacy settings
cm privacy enable                             # Enable cross-agent enrichment
cm privacy disable                            # Disable enrichment
cm project --format agents.md                 # Export for AGENTS.md
```

---

## Batch Rule Addition

After analyzing a session, add multiple rules at once:

```bash
# Create JSON file
cat > rules.json << 'EOF'
[
  {"content": "Always run tests before committing", "category": "testing"},
  {"content": "Check token expiry before auth debugging", "category": "debugging"},
  {"content": "AVOID: Mocking entire modules in tests", "category": "testing"}
]
EOF

# Add all rules
cm playbook add --file rules.json

# Track which session they came from
cm playbook add --file rules.json --session /path/to/session.jsonl

# Or pipe from stdin
echo '[{"content": "Rule", "category": "workflow"}]' | cm playbook add --file -
```

---

## Output Format

All commands support `--json` for machine-readable output.

**Design principle:** stdout = JSON only; diagnostics go to stderr.

### Success Response

```json
{
  "success": true,
  "task": "fix the auth timeout bug",
  "relevantBullets": [
    {
      "id": "b-8f3a2c",
      "content": "Always check token expiry before auth debugging",
      "effectiveScore": 8.5,
      "maturity": "proven",
      "relevanceScore": 0.92
    }
  ],
  "antiPatterns": [...],
  "historySnippets": [...],
  "suggestedCassQueries": [...]
}
```

### Error Response

```json
{
  "success": false,
  "code": "PLAYBOOK_NOT_FOUND",
  "error": "Playbook file not found",
  "hint": "Run 'cm init' to create a new playbook",
  "recovery": ["cm init", "cm doctor --fix"]
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 1 | Internal error |
| 2 | User input/usage |
| 3 | Configuration |
| 4 | Filesystem |
| 5 | Network |
| 6 | cass error |
| 7 | LLM/provider error |
