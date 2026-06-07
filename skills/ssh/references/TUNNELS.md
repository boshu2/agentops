# SSH Tunnels — Reference

## Table of Contents
- [Local Forward](#local-forward)
- [Remote Forward](#remote-forward)
- [Dynamic SOCKS Proxy](#dynamic-socks-proxy)
- [Jump Hosts](#jump-hosts)
- [Persistent Tunnels](#persistent-tunnels)

---

## Local Forward

**Access remote service via local port** — most common use case.

```bash
# Basic: localhost:8080 → remote's localhost:80
ssh -L 8080:localhost:80 user@host

# Access remote database
ssh -L 5432:localhost:5432 user@dbserver
# Then: psql -h localhost -p 5432

# Access internal service through gateway
ssh -L 5432:db-server:5432 user@jumphost
# Gateway connects to db-server:5432, you access via localhost:5432

# Multiple forwards
ssh -L 8080:localhost:80 -L 5432:localhost:5432 user@host
```

### Syntax

```
-L [bind_address:]local_port:remote_host:remote_port
```

| Part | Description |
|------|-------------|
| bind_address | Local interface (default: localhost) |
| local_port | Port on your machine |
| remote_host | Host to connect to (from server's perspective) |
| remote_port | Port on remote_host |

### Examples

```bash
# Access remote Jupyter notebook
ssh -L 8888:localhost:8888 user@gpu-server

# Access remote Docker daemon
ssh -L 2375:localhost:2375 user@docker-host

# Access remote Redis
ssh -L 6379:localhost:6379 user@redis-server

# Access internal web service through bastion
ssh -L 80:internal-web.corp:80 user@bastion
```

---

## Remote Forward

**Expose local service to remote** — reverse tunnel.

```bash
# Remote's port 9000 → your local port 3000
ssh -R 9000:localhost:3000 user@host

# Anyone on remote can reach your local web server at remote:9000
```

### Syntax

```
-R [bind_address:]remote_port:local_host:local_port
```

### Use Cases

- Share local development server with remote team
- Access local service from cloud server
- Webhook testing (expose local server to internet)

### Example: Expose local web server

```bash
# On your machine (has web server on port 3000)
ssh -R 8080:localhost:3000 user@public-server

# Now public-server:8080 reaches your local:3000
```

**Note:** Server must have `GatewayPorts yes` in sshd_config to bind to 0.0.0.0.

---

## Dynamic SOCKS Proxy

**Route all traffic through remote server** — like a VPN.

```bash
# Create SOCKS5 proxy on localhost:1080
ssh -D 1080 user@host
```

### Usage

Configure browser/application to use SOCKS5 proxy at `localhost:1080`.

```bash
# curl through proxy
curl --socks5-hostname localhost:1080 https://example.com

# Firefox: Settings → Network → SOCKS Host: localhost:1080
```

---

## Jump Hosts

**Connect through intermediate server(s)** — bastion/gateway pattern.

```bash
# Single jump
ssh -J jumphost user@internal-server

# Multiple jumps
ssh -J jump1,jump2,jump3 user@internal-server

# With specific user/port
ssh -J admin@bastion:2222 deploy@internal
```

### In SSH Config

```
Host internal-server
    HostName 10.0.0.50
    User deploy
    ProxyJump bastion

Host bastion
    HostName bastion.example.com
    User admin
    Port 2222
```

Then: `ssh internal-server`

### Legacy (pre-OpenSSH 7.3)

```bash
ssh -o ProxyCommand='ssh -W %h:%p bastion' user@internal
```

---

## Persistent Tunnels

### Keep tunnel alive

```bash
# ServerAliveInterval prevents timeout
ssh -L 8080:localhost:80 -o ServerAliveInterval=60 user@host

# With autossh (reconnects on failure)
autossh -M 0 -o ServerAliveInterval=60 -L 8080:localhost:80 user@host
```

### Background tunnel

```bash
# Fork to background (-f), no shell (-N)
ssh -f -N -L 8080:localhost:80 user@host

# Find and kill later
ps aux | grep 'ssh.*8080'
kill <pid>
```

### SSH Config for persistent tunnels

```
Host tunnel-to-db
    HostName server.example.com
    User tunnel
    LocalForward 5432 localhost:5432
    ServerAliveInterval 60
    ServerAliveCountMax 3
    ExitOnForwardFailure yes
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "channel 0: open failed" | Remote service not listening, check remote port |
| "bind: Address already in use" | Local port taken, use different port |
| Tunnel closes immediately | Add `-N` (no command) or run command |
| Connection drops | Add `ServerAliveInterval 60` |
