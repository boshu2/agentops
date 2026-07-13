# Agent Dispatch Checklist

Standard references to include in every agent dispatch prompt. Near-zero cost to include; prevents agents from rediscovering known issues.

## Required References

### 1. Framework Footguns
**File:** `docs/agent-footguns.md`
**Include when:** Always (for any agent working in this repo)
**What it covers:** Cobra global state, os.Chdir scope, Go flat package model, stale binary, shell aliases, test patterns, embedded asset sync

### 2. Scope-Escape Template
**File:** `docs/contracts/scope-escape-report.md`
**Include when:** Always (for any agent with a "fix X" or "implement Y" mandate)
**What it covers:** Structured template for when a task exceeds the agent's scope. Agents should produce an audit instead of forcing a bad fix.

### 3. Shared Test Helpers Registry
**File:** `cli/cmd/ao/testutil_test.go`
**Include when:** Agent writes Go tests in cli/cmd/ao/
**What it covers:** Centralized test helpers (captureStdout, chdirTemp, setupAgentsDir, etc.). Check here before declaring new helpers to avoid duplicate symbol errors.

### 4. Known Contracts
**Directory:** `docs/contracts/`
**Include when:** Agent creates documentation or reports
**What it covers:** Scope-escape report template, dispatch checklist (this file), and other standardized formats

## Dispatch Prompt Template

Include this block in worker prompts:

```
KNOWN FOOTGUNS: Read docs/agent-footguns.md before starting.
SCOPE OVERFLOW: If task exceeds your mandate, use docs/contracts/scope-escape-report.md template.
SHARED HELPERS: Before declaring test helpers, check cli/cmd/ao/testutil_test.go.
GIT POLICY: Do NOT run git add, git commit, or git push — the lead commits.
```

## Maintenance

Update this checklist when:
- A new reference document is created that all agents should know about
- A postmortem identifies a knowledge gap that dispatch injection could prevent
- A contract or template is added to docs/contracts/
