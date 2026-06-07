# CM Onboarding Workflow

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Gap-First Approach](#gap-first-approach)
- [Template Output](#template-output)
- [Integration with CASS](#integration-with-cass)

---

## Overview

Build your playbook from existing agent sessions with zero manual effort. The onboarding workflow prioritizes sessions that fill gaps in your playbook.

---

## Quick Start

```bash
# Check current progress
cm onboard status

# Find categories that need more rules
cm onboard gaps

# Get prioritized sessions to analyze
cm onboard sample --fill-gaps

# Analyze a session (rich context for rule extraction)
cm onboard read /path/to/session.jsonl --template --json

# Mark session as processed
cm onboard mark-done /path/to/session.jsonl
```

---

## Commands

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

## Gap-First Approach

The onboarding system identifies which categories lack rules:

| Status | Rule Count | Priority |
|--------|------------|----------|
| `critical` | 0 rules | High |
| `underrepresented` | 1-2 rules | Medium |
| `adequate` | 3-10 rules | Low |
| `well-covered` | 11+ rules | None |

When you run `cm onboard sample --fill-gaps`, sessions are prioritized by which gaps they can fill.

---

## Template Output

`--template` provides rich context for rule extraction:

```bash
cm onboard read /path/to/session.jsonl --template --json
```

Returns:
- **metadata**: path, workspace, message count, topic hints
- **context**: related rules, playbook gaps, suggested focus
- **extractionFormat**: schema, categories, examples
- **sessionContent**: actual session data

### Example Output

```json
{
  "metadata": {
    "path": "$HOME/.claude/sessions/session-001.jsonl",
    "workspace": "/data/projects/myproject",
    "messageCount": 42,
    "topicHints": ["authentication", "JWT", "token refresh"]
  },
  "context": {
    "relatedRules": ["b-8f3a2c: Check token expiry..."],
    "playbook_gaps": ["security", "error-handling"],
    "suggestedFocus": "Look for authentication patterns"
  },
  "extractionFormat": {
    "schema": {"content": "string", "category": "string"},
    "categories": ["debugging", "testing", "security", "..."],
    "examples": [...]
  },
  "sessionContent": "..."
}
```

---

## Integration with CASS

CASS provides **episodic memory** (raw sessions).
CM extracts **procedural memory** (rules and playbooks).

```bash
# CASS: Search raw sessions
cass search "authentication timeout" --robot --limit 5

# CM: Get distilled rules for a task
cm context "authentication timeout" --json
```

### Filtering History by Source

`historySnippets[].origin.kind` is `"local"` or `"remote"`. Remote hits include `origin.host`:

```json
{
  "historySnippets": [
    {
      "source_path": "~/.claude/sessions/session-001.jsonl",
      "origin": { "kind": "local" }
    },
    {
      "source_path": "$HOME/.codex/sessions/session.jsonl",
      "origin": { "kind": "remote", "host": "workstation" }
    }
  ]
}
```

---

## Workflow Pattern

### Initial Bootstrap

1. Initialize with starter playbook:
   ```bash
   cm init --starter typescript
   ```

2. Check gaps:
   ```bash
   cm onboard gaps
   ```

3. Sample sessions to fill gaps:
   ```bash
   cm onboard sample --fill-gaps
   ```

4. Analyze each session:
   ```bash
   cm onboard read /path/session.jsonl --template --json
   # Extract rules, then:
   cm playbook add --file rules.json --session /path/session.jsonl
   cm onboard mark-done /path/session.jsonl
   ```

### Ongoing Learning

After initial bootstrap, learning is automatic:

1. Before tasks: `cm context "<task>" --json`
2. During work: Leave inline feedback comments
3. After sessions: Automation runs reflection
4. Rules evolve based on feedback and decay

---

## Integration with Flywheel

| Tool | Integration |
|------|-------------|
| **CASS** | CM reads from cass episodic memory, writes procedural memory |
| **NTM** | Robot mode integrates with cm for context before agent work |
| **Agent Mail** | Rules can reference mail threads as provenance |
| **BV** | Task context enriched with relevant playbook rules |
