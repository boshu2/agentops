# SSH Commands — Reference

## Table of Contents
- [Connection](#connection)
- [Remote Execution](#remote-execution)
- [File Transfer - SCP](#file-transfer---scp)
- [File Transfer - rsync](#file-transfer---rsync)
- [Known Hosts](#known-hosts)
- [Debugging](#debugging)

---

## Connection

```bash
# Basic connection
ssh user@hostname

# Specific port
ssh -p 2222 user@hostname

# Specific identity file
ssh -i ~/.ssh/my_key user@hostname

# Force password auth (bypass key)
ssh -o PreferredAuthentications=password user@hostname

# Disable host key checking (risky, use for known-safe hosts only)
ssh -o StrictHostKeyChecking=no user@hostname
```

---

## Remote Execution

```bash
# Single command
ssh user@host "ls -la /var/log"

# Multiple commands
ssh user@host "cd /app && git pull && pm2 restart all"

# With pseudo-terminal (for interactive commands)
ssh -t user@host "htop"

# Run script from stdin
ssh user@host 'bash -s' < local_script.sh

# Run with environment variable
ssh user@host "export FOO=bar && echo \$FOO"
```

---

## File Transfer - SCP

```bash
# Copy file to remote
scp local.txt user@host:/remote/path/

# Copy file from remote
scp user@host:/remote/file.txt ./local/

# Copy directory recursively
scp -r ./local_dir user@host:/remote/path/

# Preserve timestamps and permissions
scp -p file.txt user@host:/path/

# Use specific port
scp -P 2222 file.txt user@host:/path/

# Use specific key
scp -i ~/.ssh/key file.txt user@host:/path/

# Through jump host
scp -o ProxyJump=bastion file.txt user@internal:/path/
```

---

## File Transfer - rsync

**Preferred over SCP** — faster for large/incremental transfers.

```bash
# Sync directory to remote
rsync -avz ./local/ user@host:/remote/path/

# Sync from remote
rsync -avz user@host:/remote/path/ ./local/

# With progress and compression
rsync -avzP ./local/ user@host:/remote/path/

# Dry run (show what would transfer)
rsync -avzn ./local/ user@host:/remote/path/

# Delete files on remote that don't exist locally
rsync -avz --delete ./local/ user@host:/remote/path/

# Exclude patterns
rsync -avz --exclude='*.log' --exclude='node_modules' ./local/ user@host:/remote/

# Use specific port
rsync -avz -e 'ssh -p 2222' ./local/ user@host:/remote/

# With bandwidth limit (KB/s)
rsync -avz --bwlimit=1000 ./local/ user@host:/remote/
```

### rsync flags

| Flag | Description |
|------|-------------|
| `-a` | Archive mode (preserves permissions, timestamps, etc.) |
| `-v` | Verbose |
| `-z` | Compress during transfer |
| `-P` | Show progress + partial (resume) |
| `-n` | Dry run |
| `--delete` | Remove remote files not in source |
| `--exclude` | Skip matching patterns |

---

## Known Hosts

```bash
# Remove old host key (after server rebuild)
ssh-keygen -R hostname

# Scan and add host key
ssh-keyscan hostname >> ~/.ssh/known_hosts

# Scan specific port
ssh-keyscan -p 2222 hostname >> ~/.ssh/known_hosts

# Get key fingerprint
ssh-keygen -lf ~/.ssh/known_hosts
```

---

## Debugging

```bash
# Verbose output
ssh -v user@host

# Very verbose
ssh -vv user@host

# Maximum verbosity
ssh -vvv user@host
```

### Common Issues

| Issue | Debug Step |
|-------|------------|
| Connection refused | Check port, firewall: `nc -zv host 22` |
| Permission denied | Check key permissions: `chmod 600 ~/.ssh/id_*` |
| Host key changed | Remove old key: `ssh-keygen -R hostname` |
| Timeout | Check network, try `-o ConnectTimeout=10` |
