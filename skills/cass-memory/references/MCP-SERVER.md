# CM MCP Server Integration

## Table of Contents
- [Starting the Server](#starting-the-server)
- [Tools Exposed](#tools-exposed)
- [Resources Exposed](#resources-exposed)
- [Client Configuration](#client-configuration)
- [Performance](#performance)
- [LLM Cost Estimates](#llm-cost-estimates)

---

## Starting the Server

```bash
# Local-only (recommended)
cm serve --port 3001

# With auth token (for non-loopback)
MCP_HTTP_TOKEN="<random>" cm serve --host 0.0.0.0 --port 3001
```

---

## Tools Exposed

| Tool | Purpose | Parameters |
|------|---------|------------|
| `cm_context` | Get rules + history | `task, limit?, history?, days?, workspace?` |
| `cm_feedback` | Record feedback | `bulletId, helpful?, harmful?, reason?` |
| `cm_outcome` | Record session outcome | `sessionId, outcome, rulesUsed?` |
| `memory_search` | Search playbook/cass | `query, scope?, limit?, days?` |
| `memory_reflect` | Trigger reflection | `days?, maxSessions?, dryRun?` |

---

## Resources Exposed

| URI | Purpose |
|-----|---------|
| `cm://playbook` | Current playbook state |
| `cm://diary` | Recent diary entries |
| `cm://outcomes` | Session outcomes |
| `cm://stats` | Playbook health metrics |

---

## Client Configuration

### Claude Code

Add to `~/.config/claude/mcp.json`:

```json
{
  "mcpServers": {
    "cm": {
      "command": "cm",
      "args": ["serve"]
    }
  }
}
```

### With Custom Port

```json
{
  "mcpServers": {
    "cm": {
      "command": "cm",
      "args": ["serve", "--port", "3001"]
    }
  }
}
```

---

## Performance

| Operation | Typical Latency |
|-----------|-----------------|
| `cm context` (cached) | 50-150ms |
| `cm context` (cold) | 200-500ms |
| `cm context` (no cass) | 30-80ms |
| `cm reflect` (1 session) | 5-15s |
| `cm reflect` (5 sessions) | 20-60s |
| `cm playbook list` | <50ms |
| `cm similar` (keyword) | 20-50ms |
| `cm similar` (semantic) | 100-300ms |

---

## LLM Cost Estimates

| Operation | Typical Cost |
|-----------|--------------|
| Reflect (1 session) | $0.01-0.05 |
| Reflect (7 days) | $0.05-0.20 |
| Validate (1 rule) | $0.005-0.01 |

With default budget ($0.10/day, $2.00/month): ~5-10 sessions/day.

### Monitor Usage

```bash
cm usage                   # LLM budget status
```

---

## Automating Reflection

### Cron Job

```bash
# Daily at 2am
0 2 * * * /usr/local/bin/cm reflect --days 7 >> ~/.cass-memory/reflect.log 2>&1
```

### Claude Code Hook

`.claude/hooks.json`:

```json
{
  "post-session": ["cm reflect --days 1"]
}
```

---

## Privacy & Security

### Local-First Design

- All data stays on your machine
- No cloud sync, no telemetry
- Cross-agent enrichment is opt-in with explicit consent
- Audit log for enrichment events

### Secret Sanitization

Before processing, content is sanitized:
- OpenAI/Anthropic/AWS/Google API keys
- GitHub tokens
- JWTs
- Passwords and secrets in config patterns

### Privacy Controls

```bash
cm privacy status    # Check settings
cm privacy enable    # Enable cross-agent enrichment
cm privacy disable   # Disable enrichment
```
