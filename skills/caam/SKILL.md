---
name: caam
user-invocable: false
skill_api_version: 1
hexagonal_role: supporting
metadata:
  tier: execution
description: "Use when switching AI coding CLI accounts quickly to recover from subscription rate limits or OAuth friction."
---
<!-- TOC: Quick Start | THE EXACT PROMPT | Smart Rotation | TUI | Isolated Profiles | References -->

# CAAM — Coding Agent Account Manager

> **Core Problem:** You hit rate limits on your $200/mo Claude Max subscription. Browser OAuth takes 30-60 seconds. CAAM swaps accounts in ~50ms.

## Quick Start

```bash
# Install
curl -fsSL "https://raw.githubusercontent.com/Dicklesworthstone/coding_agent_account_manager/main/install.sh?$(date +%s)" | bash

# Backup current account
caam backup claude alice@gmail.com

# Clear, login with another account, backup that too
caam clear claude
claude  # /login with bob@gmail.com
caam backup claude bob@gmail.com

# Switch instantly forever
caam activate claude alice@gmail.com   # ~50ms
caam activate claude bob@gmail.com     # ~50ms
```

---

## THE EXACT PROMPT — Daily Commands

### Check Status

```bash
caam status
```

Shows active profile for each tool based on content hash matching.

### Instant Switch

```bash
caam activate claude bob@gmail.com
```

Restores auth files from vault. ~50ms, no browser.

### Smart Auto-Switch

```bash
caam activate claude --auto
```

Rotation algorithm picks best profile based on health, recency, cooldowns.

### Mark Rate Limited

```bash
caam cooldown set claude
```

Marks current profile for 60min. Rotation skips it.

### Zero-Friction Aliases

```bash
# Add to .bashrc/.zshrc
alias claude='caam run claude --'
alias codex='caam run codex --'
alias gemini='caam run gemini --'

# Now these auto-failover on rate limits
claude "explain this code"
```

---

## How It Works

Each AI CLI stores OAuth tokens in plain files. CAAM backs them up and restores them:

```
~/.claude.json ←→ ~/.local/share/caam/vault/claude/alice@gmail.com/
~/.codex/auth.json ←→ ~/.local/share/caam/vault/codex/work@company.com/
```

**That's it.** No daemons, no databases, no network calls. Just `cp` with extra steps.

---

## Supported Tools

| Tool | Subscription | Auth Files |
|------|--------------|------------|
| **Claude Code** | Claude Max ($200/mo) | `~/.claude.json`, `~/.config/claude-code/auth.json` |
| **Codex CLI** | GPT Pro ($200/mo) | `~/.codex/auth.json` |
| **Gemini CLI** | Gemini Ultra (~$275/mo) | `~/.gemini/settings.json`, `~/.gemini/oauth_credentials.json` |

---

## Smart Rotation

When you have multiple accounts, rotation picks the best one:

```bash
# Preview what rotation would select
caam next claude

# Let it pick automatically
caam activate claude --auto
```

### Health Indicators

| Icon | Status | Meaning |
|------|--------|---------|
| 🟢 | Healthy | Token valid >1hr, no recent errors |
| 🟡 | Warning | Token expiring <1hr, minor issues |
| 🔴 | Critical | Token expired, repeated errors |
| ⚪ | Unknown | No health data yet |

### Rotation Algorithms

| Algorithm | Description |
|-----------|-------------|
| `smart` (default) | Cooldown + health + recency + jitter |
| `round_robin` | Sequential, skipping cooldowns |
| `random` | Random among non-cooldown |

---

## Cooldown Tracking

```bash
# Mark current profile (60min default)
caam cooldown set claude

# Specify profile and duration
caam cooldown set claude/work@company.com --minutes 120

# View active cooldowns
caam cooldown list

# Clear early
caam cooldown clear claude/work@company.com
```

---

## TUI Dashboard

```bash
caam tui
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate profiles |
| `Enter` | Activate selected |
| `p` | Set project association |
| `c` | Mark/clear cooldown |
| `r` | Refresh |
| `q` | Quit |

---

## Project Associations

Link profiles to directories:

```bash
cd ~/projects/work-app
caam project set claude work@company.com

# Now in this directory:
caam activate claude  # Auto-uses work@company.com
```

---

## Command Reference

| Command | Description |
|---------|-------------|
| `caam backup <tool> <email>` | Save current auth to vault |
| `caam activate <tool> <email>` | Restore auth from vault |
| `caam activate <tool> --auto` | Auto-select best profile |
| `caam status [tool]` | Show active profiles |
| `caam ls [tool]` | List saved profiles |
| `caam clear <tool>` | Remove auth files (logout) |
| `caam run <tool> [-- args]` | Wrap with auto-failover |
| `caam cooldown set <profile>` | Mark as rate limited |
| `caam cooldown list` | Show active cooldowns |
| `caam next <tool>` | Preview rotation selection |
| `caam tui` | Interactive dashboard |

---

## Flywheel Integration

| Tool | Integration |
|------|-------------|
| **NTM** | Each tmux pane uses different account via isolated profiles |
| **Agent Mail** | Agents coordinate account switching across sessions |
| **CASS** | Search sessions by account for usage patterns |

---

## References

| Topic | Reference |
|-------|-----------|
| Complete commands | [COMMANDS.md](references/COMMANDS.md) |
| Isolated profiles | [ISOLATED-PROFILES.md](references/ISOLATED-PROFILES.md) |
| Vault structure | [VAULT.md](references/VAULT.md) |

---

## Validation

```bash
# Verify installation
caam --version

# Check all profiles
caam status

# Test switch
caam activate claude --auto && caam status
```
