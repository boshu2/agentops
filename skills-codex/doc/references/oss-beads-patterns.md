# Documentation Patterns from Beads

> Extracted from analysis of beads (chronicle) repository.
> Beads demonstrates exemplary OSS documentation practices.

## Overview

Beads is a Git-backed issue tracker for AI-supervised coding workflows.
As of v0.48.0, it has ~90 markdown files with comprehensive documentation.

**Why study beads?**
- Actively maintained OSS project
- Targets AI-assisted development (similar audience)
- Extensive documentation coverage
- Clear writing style

---

## README.md Pattern

### Structure

```
1. Project name + one-liner
2. Quick install (single command)
3. Quick start (3-5 commands)
4. Key features (bullet list)
5. Documentation links
6. Community/contributing
7. License
```

### Key Elements

**Title + Tagline:**
```markdown
# beads (bd)

> Git-backed issue tracker for AI-supervised coding workflows.
```

**Quick Install:**
```markdown
## Installation

```bash
brew tap steveyegge/beads && brew install bd
```
```

**Quick Start:**
```markdown
## Quick Start

```bash
bd init                  # Initialize in project
bd create "Fix bug" -p 1 # Create issue
bd ready                 # Find unblocked work
bd vc status             # Optional Dolt status check; JSONL auto-sync is automatic
```
```

**Features as Bullets (not walls of text):**
```markdown
## Features

- **Zero setup** - `bd init` creates project-local database
- **Dependency tracking** - Four dependency types
- **Ready work detection** - Find issues with no blockers
- **Agent-friendly** - `--json` flags for programmatic use
```

---

## AGENTS.md Pattern

### Structure

```
1. Quick reference commands
2. Session close protocol
3. Workflow overview
4. Common operations
```

### Key Elements

**Command Quick Reference:**
```markdown
## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd vc status          # Inspect Dolt state if needed (JSONL auto-sync is automatic)
```
```

**Session Close Protocol (Critical):**
```markdown
## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below.
Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work**
2. **Run quality gates** (if code changed)
3. **Update issue status**
4. **PUSH TO REMOTE** - This is MANDATORY
5. **Verify** - All changes committed AND pushed
```

**Emphasis on Critical Rules:**
```markdown
**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing
- NEVER say "ready to push when you are" - YOU must push
```

---

## CLI_REFERENCE.md Pattern

### Structure

```
1. Overview table of all commands
2. Global flags section
3. Each command with:
   - Synopsis
   - Description
   - Flags table
   - Examples
```

### Key Elements

**Command Synopsis:**
```markdown
## bd create

Create a new issue.

### Synopsis

```
bd create <title> [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--type` | `-t` | `task` | Issue type |
| `--priority` | `-p` | `2` | Priority (0-4) |
| `--description` | `-d` | | Issue description |
| `--json` | | `false` | Output as JSON |

### Examples

```bash
# Create a bug with high priority
bd create "Login fails on Safari" -t bug -p 1

# Create with description
bd create "Add dark mode" -d "Support system preference"
```
```

---

## TROUBLESHOOTING.md Pattern

### Structure

```
1. Quick fixes section (most common issues)
2. Categorized issues
3. Each issue with:
   - Symptoms
   - Cause
   - Solution
4. Recovery procedures
```

### Key Elements

**Issue Format:**
```markdown
### Issue: Database is locked

**Symptoms:**
```
bd: database is locked (SQLITE_BUSY)
```

**Cause:** Another process has the database open.

**Solutions:**

1. **Stop daemon and retry:**
   ```bash
   bd daemon stop
   bd <your-command>
   ```

2. **Check for hung processes:**
   ```bash
   ps aux | grep bd
   kill <pid>
   ```
```

**Quick Fixes Section:**
```markdown
## Quick Fixes

| Problem | Solution |
|---------|----------|
| "database is locked" | `bd daemon stop && bd daemon start` |
| "JSONL conflict markers" | `git checkout --theirs .beads/issues.jsonl` |
| "circular dependency" | `bd doctor` (diagnose only) |
```

---

## CONFIG.md Pattern

### Structure

```
1. Configuration overview
2. Configuration levels (project, user, env)
3. Settings table with all options
4. Examples for common scenarios
```

### Key Elements

**Configuration Levels:**
```markdown
## Configuration Precedence

1. **Environment variables** (highest) - `BEADS_*`
2. **CLI flags** - `--flag`
3. **Project config** - `.beads/config.yaml`
4. **User config** - `~/.config/beads/config.yaml`
5. **Defaults** (lowest)
```

**Settings Table:**
```markdown
## Settings Reference

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `sync.auto_commit` | bool | `true` | Auto-commit on sync |
| `sync.auto_push` | bool | `false` | Auto-push on sync |
| `sync.branch` | string | | Separate sync branch |
| `daemon.port` | int | `0` | Daemon port (0=auto) |
```

---

## CHANGELOG.md Pattern

### Format

Based on [Keep a Changelog](https://keepachangelog.com/):

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.48.0] - 2026-01-17

### Added
- **VersionedStorage interface** - Abstract storage layer
- **`bd types` command** - List valid issue types (#1102)

### Fixed
- **Doctor sync branch health check** - Removed destructive --fix (GH#1062)
- **Duplicate merge target selection** - Use combined weight (GH#1022)

### Changed
- **Daemon CLI refactor** - Consolidated subcommands

### Documentation
- Add lazybeads TUI to community tools (#951)
```

### Key Practices

1. **Link issue numbers** - `(#123)` or `(GH#123)`
2. **Bold feature names** - `**Feature name**`
3. **Categorize changes** - Added, Fixed, Changed, etc.
4. **Include dates** - `[0.48.0] - 2026-01-17`
5. **Link comparison URLs** at bottom

---

## Documentation Organization

### Directory Structure

```
beads/
├── README.md                 # Overview + quick start
├── AGENTS.md                 # AI assistant guide
├── CONTRIBUTING.md           # Contributor guide
├── CHANGELOG.md              # Version history
├── SECURITY.md               # Vulnerability reporting
│
├── docs/
│   ├── QUICKSTART.md         # Detailed getting started
│   ├── CLI_REFERENCE.md      # Complete command reference
│   ├── ARCHITECTURE.md       # System design
│   ├── CONFIG.md             # Configuration options
│   ├── TROUBLESHOOTING.md    # Common issues
│   ├── FAQ.md                # Frequently asked questions
│   ├── GIT_INTEGRATION.md    # Git workflows
│   ├── WORKTREES.md          # Git worktree support
│   ├── MULTI_REPO_*.md       # Multi-repo patterns
│   └── <FEATURE>.md          # Feature-specific docs
│
├── examples/
│   ├── README.md             # Examples index
│   ├── python-agent/         # Python integration
│   ├── bash-agent/           # Shell scripts
│   └── <pattern>/            # Usage patterns
│
└── integrations/
    ├── beads-mcp/            # MCP server
    └── claude-code/          # Claude Code plugin
```

### Navigation Principles

1. **README links to docs/** - Don't duplicate, link
2. **Each doc is self-contained** - Can be read standalone
3. **Cross-references** - "See also" sections
4. **Index pages** - examples/README.md lists all examples

---

## Style Guidelines

### From Beads' Writing Style

1. **Direct language** - "Run this command" not "You may want to run"
2. **Active voice** - "The daemon exports" not "Issues are exported by"
3. **Tables for structured data** - Commands, flags, options
4. **Code blocks for examples** - Always with language hint
5. **Warnings are prominent** - Use blockquotes or boxes
6. **No jargon without definition** - Explain terms on first use

### Warning Format

```markdown
**⚠️ WARNING:** Daemon mode does NOT work correctly with git worktrees.
```

Or:

```markdown
> **Note:** For environments with shell access, CLI is recommended over MCP.
```

### Example Quality

Bad:
```markdown
Run the create command to create an issue.
```

Good:
```markdown
```bash
bd create "Fix authentication bug" -t bug -p 1 --json
```
```

---

## Metrics from Beads

| Metric | Value |
|--------|-------|
| Total .md files | ~90 |
| README.md length | ~200 lines |
| CLI_REFERENCE.md | ~800 lines |
| TROUBLESHOOTING.md | ~845 lines |
| CONFIG.md | ~615 lines |
| CHANGELOG entries | 48+ versions |
| Integration guides | 4+ (MCP, Claude Code, Aider, etc.) |

### Coverage Analysis

- **Tier 1:** 4/4 (LICENSE, README, CONTRIBUTING, CODE_OF_CONDUCT)
- **Tier 2:** 5/5 (SECURITY, CHANGELOG, AGENTS, templates)
- **Tier 3:** 6+/6 (QUICKSTART, ARCHITECTURE, CLI_REFERENCE, CONFIG, TROUBLESHOOTING, examples)

**Score: 100% coverage across all tiers**

---

## Applying to Your Project

1. **Start with README.md** - Use beads' structure as template
2. **Add AGENTS.md early** - AI assistants need context
3. **Document commands** - CLI_REFERENCE.md for any CLI
4. **Anticipate problems** - TROUBLESHOOTING.md saves support time
5. **Keep CHANGELOG** - Start from v0.1.0, update every release
