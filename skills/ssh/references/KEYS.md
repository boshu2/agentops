# SSH Key Management — Reference

## Table of Contents
- [Generate Keys](#generate-keys)
- [Deploy Keys](#deploy-keys)
- [SSH Agent](#ssh-agent)
- [Key Security](#key-security)
- [Key Restrictions](#key-restrictions)

---

## Generate Keys

### Ed25519 (Recommended)

```bash
# Default location (~/.ssh/id_ed25519)
ssh-keygen -t ed25519 -C "you@example.com"

# Custom filename
ssh-keygen -t ed25519 -f ~/.ssh/myproject_key -C "project-specific"

# Without passphrase (less secure, for automation)
ssh-keygen -t ed25519 -f ~/.ssh/automation_key -N ""
```

### RSA (Legacy compatibility)

```bash
# 4096 bits minimum
ssh-keygen -t rsa -b 4096 -C "you@example.com"
```

### Key Types Comparison

| Type | Security | Speed | Compatibility |
|------|----------|-------|---------------|
| Ed25519 | Excellent | Fast | Modern systems |
| RSA 4096 | Good | Slower | Universal |
| ECDSA | Good | Fast | Most systems |

**Always use Ed25519 unless you need legacy compatibility.**

---

## Deploy Keys

### ssh-copy-id (Easiest)

```bash
# Default key
ssh-copy-id user@host

# Specific key
ssh-copy-id -i ~/.ssh/mykey.pub user@host

# Specific port
ssh-copy-id -p 2222 user@host
```

### Manual method

```bash
# Copy public key content
cat ~/.ssh/id_ed25519.pub

# On remote server, append to:
echo "public-key-content" >> ~/.ssh/authorized_keys

# Set correct permissions
chmod 700 ~/.ssh
chmod 600 ~/.ssh/authorized_keys
```

### From local machine (one-liner)

```bash
cat ~/.ssh/id_ed25519.pub | ssh user@host "mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
```

---

## SSH Agent

### Start and Add Keys

```bash
# Start agent (bash)
eval "$(ssh-agent -s)"

# Start agent (fish)
eval (ssh-agent -c)

# Add default key
ssh-add

# Add specific key
ssh-add ~/.ssh/mykey

# Add with macOS keychain persistence
ssh-add --apple-use-keychain ~/.ssh/id_ed25519

# Add with timeout (seconds)
ssh-add -t 3600 ~/.ssh/id_ed25519
```

### Manage Agent

```bash
# List loaded keys
ssh-add -l

# List with fingerprints
ssh-add -L

# Remove specific key
ssh-add -d ~/.ssh/mykey

# Remove all keys
ssh-add -D
```

### Agent Forwarding

```bash
# Forward agent to remote (allows using local keys on remote)
ssh -A user@host

# In config
Host myserver
    ForwardAgent yes
```

**Security Warning:** Only forward agent to trusted servers. Malicious admins can use your forwarded agent.

### Persistent Agent (Linux)

Add to `~/.bashrc`:

```bash
if [ -z "$SSH_AUTH_SOCK" ]; then
    eval "$(ssh-agent -s)"
    ssh-add ~/.ssh/id_ed25519
fi
```

### macOS Keychain Integration

In `~/.ssh/config`:

```
Host *
    AddKeysToAgent yes
    UseKeychain yes
    IdentityFile ~/.ssh/id_ed25519
```

---

## Key Security

### File Permissions

```bash
# Directory
chmod 700 ~/.ssh

# Private keys
chmod 600 ~/.ssh/id_*

# Public keys
chmod 644 ~/.ssh/*.pub

# authorized_keys
chmod 600 ~/.ssh/authorized_keys

# Config
chmod 600 ~/.ssh/config
```

### Change Passphrase

```bash
ssh-keygen -p -f ~/.ssh/id_ed25519
```

### View Key Fingerprint

```bash
ssh-keygen -lf ~/.ssh/id_ed25519.pub
```

---

## Key Restrictions

### Restrict in authorized_keys

```
# Force specific command only
command="/usr/bin/rsync --server" ssh-ed25519 AAAA...

# Restrict source IP
from="192.168.1.0/24" ssh-ed25519 AAAA...

# No port forwarding
no-port-forwarding ssh-ed25519 AAAA...

# No agent forwarding
no-agent-forwarding ssh-ed25519 AAAA...

# Combined restrictions
command="/opt/backup.sh",no-port-forwarding,no-agent-forwarding,from="10.0.0.5" ssh-ed25519 AAAA...
```

### Available Restrictions

| Option | Effect |
|--------|--------|
| `command="cmd"` | Only allow this command |
| `from="pattern"` | Restrict source IPs |
| `no-port-forwarding` | Disable all port forwarding |
| `no-agent-forwarding` | Disable agent forwarding |
| `no-X11-forwarding` | Disable X11 forwarding |
| `no-pty` | No interactive shell |
| `environment="VAR=value"` | Set environment variable |

---

## GitHub/GitLab Keys

### Add key to GitHub

```bash
# Copy public key
cat ~/.ssh/id_ed25519.pub | pbcopy  # macOS
cat ~/.ssh/id_ed25519.pub | xclip   # Linux

# Add at: GitHub → Settings → SSH Keys
```

### Test connection

```bash
ssh -T git@github.com
ssh -T git@gitlab.com
```

### Use specific key for Git hosts

In `~/.ssh/config`:

```
Host github.com
    IdentityFile ~/.ssh/github_key
    IdentitiesOnly yes

Host gitlab.com
    IdentityFile ~/.ssh/gitlab_key
    IdentitiesOnly yes
```
