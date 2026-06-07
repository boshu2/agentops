# Vault Structure and Internals

## Table of Contents
- [Directory Layout](#directory-layout)
- [Auth File Locations](#auth-file-locations)
- [How Detection Works](#how-detection-works)
- [Tips](#tips)

---

## Directory Layout

```
~/.local/share/caam/
├── vault/                          # Saved auth profiles
│   ├── claude/
│   │   ├── alice@gmail.com/
│   │   │   ├── .claude.json        # Backed up auth
│   │   │   ├── auth.json           # From ~/.config/claude-code/
│   │   │   └── meta.json           # Timestamp, original paths
│   │   └── bob@gmail.com/
│   │       └── ...
│   ├── codex/
│   │   └── work@company.com/
│   │       └── auth.json
│   └── gemini/
│       └── personal@gmail.com/
│           ├── settings.json
│           └── oauth_credentials.json
│
├── profiles/                       # Isolated profiles (advanced)
│   └── codex/
│       └── work@company.com/
│           ├── profile.json        # Profile metadata
│           ├── codex_home/         # Isolated CODEX_HOME
│           │   └── auth.json
│           └── home/               # Pseudo-HOME with symlinks
│               ├── .ssh -> ~/.ssh
│               └── .gitconfig -> ~/.gitconfig
│
├── state/                          # Runtime state
│   ├── cooldowns.json              # Active cooldown tracking
│   ├── health.json                 # Profile health scores
│   └── usage.json                  # Usage analytics
│
└── config.yaml                     # User configuration
```

---

## Auth File Locations

### Claude Code (Claude Max)

| File | Location |
|------|----------|
| Main auth | `~/.claude.json` |
| Claude Code auth | `~/.config/claude-code/auth.json` |
| Settings | `~/.claude/settings.json` |

**Login:** `/login` inside CLI

### Codex CLI (GPT Pro)

| File | Location |
|------|----------|
| Auth | `~/.codex/auth.json` (or `$CODEX_HOME/auth.json`) |
| Config | `~/.codex/config.toml` |

**Login:** `codex login` or `codex login --device-auth`

### Gemini CLI (Gemini Ultra)

| File | Location |
|------|----------|
| Settings | `~/.gemini/settings.json` |
| OAuth | `~/.gemini/oauth_credentials.json` |
| API Key | `~/.gemini/.env` |

**Login:** Start `gemini`, select "Login with Google"

---

## How Detection Works

`caam status` uses **content hashing**:

1. SHA-256 hash current auth files
2. Compare against all vault profiles
3. Match = that's what's active

### Benefits

- Profiles detected even if you switched manually
- No hidden state files that can desync
- Works correctly after reboots
- Self-healing if state gets corrupted

### Example

```bash
$ caam status
claude: alice@gmail.com (active)
  Hash: a7f3b2c1d4...
  Last switch: 2h ago
  Health: 🟢 Healthy

codex: work@company.com (active)
  Hash: e9b1d4f5c6...
  Last switch: 30m ago
  Health: 🟡 Token expiring soon
```

---

## Tips

### Use Email as Profile Name

Self-documenting — you'll never forget which account is which.

```bash
# Good
caam backup claude alice@gmail.com

# Bad
caam backup claude account1
```

### Backup Before Clearing

```bash
caam backup claude current@email.com && caam clear claude
```

Or use the flag:
```bash
caam activate claude new@email.com --backup-current
```

### Don't Sync Vault Across Machines

Auth tokens often contain machine-specific identifiers (device IDs). Backup and restore on each machine separately.

### Check Auth Paths After Tool Updates

```bash
caam paths
```

If locations change, CAAM will be updated. File an issue if you notice discrepancies.

---

## Meta File Format

Each vault profile includes `meta.json`:

```json
{
  "email": "alice@gmail.com",
  "tool": "claude",
  "created_at": "2025-01-15T10:30:00Z",
  "last_activated": "2025-01-18T14:22:00Z",
  "original_paths": {
    ".claude.json": "/home/user/.claude.json",
    "auth.json": "/home/user/.config/claude-code/auth.json"
  },
  "hash": "a7f3b2c1d4e5f6..."
}
```

---

## Config File

`~/.caam/config.yaml`:

```yaml
stealth:
  rotation:
    enabled: true
    algorithm: smart  # smart | round_robin | random
  cooldown:
    enabled: true     # Warn when activating cooldown profiles
    default_minutes: 60

health:
  refresh_interval: 5m
  penalty_decay_rate: 0.2  # 20% reduction every 5 minutes

analytics:
  enabled: true
  retention_days: 30
```
