# Isolated Profiles — Running Multiple Accounts Simultaneously

## Table of Contents
- [When to Use](#when-to-use)
- [Setup](#setup)
- [How It Works](#how-it-works)
- [Workflow Examples](#workflow-examples)

---

## When to Use

| Mode | Use Case |
|------|----------|
| **Vault Profiles** | Switch between accounts sequentially (most common) |
| **Isolated Profiles** | Run multiple accounts **at the same time** |

### Isolated Profile Use Cases

- Running work and personal accounts in parallel
- Multi-agent swarm with different account pools
- Testing across accounts simultaneously
- NTM panes with different accounts per pane

---

## Setup

### 1. Create Isolated Profiles

```bash
caam profile add codex work@company.com
caam profile add codex personal@gmail.com
```

### 2. Login to Each (One-Time)

```bash
caam login codex work@company.com      # Opens browser
caam login codex personal@gmail.com    # Opens browser
```

### 3. Run Simultaneously

```bash
# Terminal 1
caam exec codex work@company.com -- "implement feature X"

# Terminal 2
caam exec codex personal@gmail.com -- "review PR #123"
```

---

## How It Works

Each isolated profile gets its own `$HOME` and tool-specific home:

```
~/.local/share/caam/profiles/
└── codex/
    └── work@company.com/
        ├── profile.json        # Profile metadata
        ├── codex_home/         # Isolated CODEX_HOME
        │   └── auth.json
        └── home/               # Pseudo-HOME with symlinks
            ├── .ssh -> ~/.ssh
            └── .gitconfig -> ~/.gitconfig
```

When you run `caam exec`:
1. Sets `$HOME` to the isolated home directory
2. Sets tool-specific env (e.g., `$CODEX_HOME`)
3. Symlinks your real `.ssh`, `.gitconfig`, etc.
4. Auth files are isolated, everything else is shared

---

## Workflow Examples

### Parallel Development

```bash
# Work project in one terminal
caam exec codex work@company.com -- "implement auth system"

# Personal project in another
caam exec codex personal@gmail.com -- "review my side project"
```

### NTM Integration

```bash
# Spawn agents with different accounts
ntm spawn myproject --cc=2

# Pane 1: Use work account
caam exec claude work@company.com -- "$(cat prompt.txt)"

# Pane 2: Use personal account
caam exec claude personal@gmail.com -- "$(cat prompt.txt)"
```

### Multi-Account Swarm

```bash
# Create profiles for all accounts
caam profile add claude alice@gmail.com
caam profile add claude bob@gmail.com
caam profile add claude carol@gmail.com

# Login to each
caam login claude alice@gmail.com
caam login claude bob@gmail.com
caam login claude carol@gmail.com

# Run in parallel across multiple terminals
for email in alice bob carol; do
  caam exec claude ${email}@gmail.com -- "$(cat swarm_prompt.txt)" &
done
```

---

## Commands

| Command | Description |
|---------|-------------|
| `caam profile add <tool> <email>` | Create isolated profile |
| `caam profile ls [tool]` | List isolated profiles |
| `caam profile delete <tool> <email>` | Delete profile |
| `caam profile status <tool> <email>` | Show profile status |
| `caam login <tool> <email>` | Run login for isolated profile |
| `caam exec <tool> <email> [-- args]` | Run CLI with profile |

---

## Vault vs Isolated

| Feature | Vault | Isolated |
|---------|-------|----------|
| Accounts active | 1 at a time | Multiple simultaneously |
| Setup complexity | Lower | Higher |
| Switching speed | ~50ms | N/A (parallel) |
| Directory isolation | No | Yes |
| Use case | Sequential switching | Parallel execution |

### When to Use Each

**Use Vault Profiles when:**
- You switch accounts throughout the day
- Only one session active at a time
- Want simplest setup

**Use Isolated Profiles when:**
- You need accounts running in parallel
- Multi-agent swarm with account diversity
- Different accounts for different terminals
