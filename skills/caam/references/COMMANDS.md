# CAAM Complete Command Reference

## Table of Contents
- [Auth File Swapping](#auth-file-swapping)
- [Smart Profile Management](#smart-profile-management)
- [Profile Isolation](#profile-isolation)
- [Configuration](#configuration)

---

## Auth File Swapping

### backup

Save current auth files to vault.

```bash
caam backup <tool> <email>
```

**Example:**
```bash
caam backup claude alice@gmail.com
```

### activate

Restore auth files from vault (instant switch).

```bash
caam activate <tool> <email> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--auto` | Use rotation algorithm to pick best |
| `--backup-current` | Backup current auth before switching |
| `--force` | Activate even if in cooldown |

**Aliases:** `switch`, `use`

**Examples:**
```bash
caam activate claude bob@gmail.com
caam activate claude --auto
caam switch codex work@company.com --backup-current
```

### status

Show which profile is currently active.

```bash
caam status [tool]
```

Uses content hashing to detect active profile, even if switched manually.

### ls

List all saved profiles in vault.

```bash
caam ls [tool]
```

### clear

Remove auth files (logout state).

```bash
caam clear <tool>
```

### delete

Remove a saved profile from vault.

```bash
caam delete <tool> <email>
```

### paths

Show auth file locations for each tool.

```bash
caam paths [tool]
```

Useful if tool updates change file locations.

### uninstall

Restore originals and remove caam data/config.

```bash
caam uninstall [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be restored/removed |
| `--keep-backups` | Keep vault after restoring originals |
| `--force` | Skip confirmation prompt |

---

## Smart Profile Management

### cooldown set

Mark profile as rate-limited.

```bash
caam cooldown set <provider/profile> [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--minutes` | 60 | Cooldown duration |

**Examples:**
```bash
caam cooldown set claude                    # Current profile, 60min
caam cooldown set claude/work@co.com --minutes 120
```

### cooldown list

List active cooldowns with remaining time.

```bash
caam cooldown list
```

### cooldown clear

Clear cooldown for a profile.

```bash
caam cooldown clear <provider/profile>
caam cooldown clear --all
```

### next

Preview which profile rotation would select.

```bash
caam next <tool>
```

**Output:**
```
Recommended: bob@gmail.com
  + Healthy token (expires in 4h 32m)
  + Not used recently (2h ago)

Alternatives:
  alice@gmail.com - Used recently (15m ago)

In cooldown:
  carol@gmail.com - In cooldown (45m remaining)
```

### run

Wrap CLI with automatic failover on rate limits.

```bash
caam run <tool> [flags] [-- args]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--max-retries` | 1 | Maximum retry attempts |
| `--cooldown` | 60m | Cooldown after rate limit |
| `--algorithm` | smart | Rotation algorithm |
| `--quiet` | off | Suppress notifications |

**Example:**
```bash
caam run claude -- "explain this code"
```

### project set

Associate current directory with a profile.

```bash
caam project set <tool> <profile>
```

**Example:**
```bash
cd ~/projects/work-app
caam project set claude work@company.com
```

### project get

Show project associations for current directory.

```bash
caam project get [tool]
```

### tui

Interactive TUI dashboard.

```bash
caam tui
```

---

## Profile Isolation

For running multiple accounts simultaneously.

### profile add

Create isolated profile directory.

```bash
caam profile add <tool> <email>
```

### profile ls

List isolated profiles.

```bash
caam profile ls [tool]
```

### profile delete

Delete isolated profile.

```bash
caam profile delete <tool> <email>
```

### profile status

Show isolated profile status.

```bash
caam profile status <tool> <email>
```

### login

Run login flow for isolated profile.

```bash
caam login <tool> <email>
```

### exec

Run CLI with isolated profile.

```bash
caam exec <tool> <email> [-- args]
```

**Example:**
```bash
caam exec codex work@company.com -- "implement feature X"
```

---

## Configuration

Config file: `~/.caam/config.yaml`

```yaml
stealth:
  rotation:
    enabled: true
    algorithm: smart  # smart | round_robin | random
  cooldown:
    enabled: true     # Warn when activating cooldown profiles
```

### config commands

```bash
caam config --list          # Show all config
caam config --get key       # Get specific value
caam config --set key=value # Set value
caam config --edit          # Open in editor
```
