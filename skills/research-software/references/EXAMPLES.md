# Research Examples

Real sessions showing the workflow.

---

## CLI Tool: Wrangler

```bash
# 1. Clone
git clone --depth 1 https://github.com/cloudflare/workers-sdk.git /tmp/workers-sdk-research
cd /tmp/workers-sdk-research && git fetch --tags && git checkout $(git describe --tags --abbrev=0)

# 2. Explore agent prompt:
# "Investigate /tmp/workers-sdk-research/packages/wrangler: CLI commands, config schema, hidden flags, env vars"

# 3. GitHub
gh pr list -R cloudflare/workers-sdk --state merged --limit 30 --json title,mergedAt
gh issue list -R cloudflare/workers-sdk --label "question" --limit 20

# 4. Web search: "wrangler 2025", "cloudflare workers tutorial 2026"

# 5. Cleanup
rm -rf /tmp/workers-sdk-research
```

**Key findings location:** `packages/wrangler/src/` — commands in `src/`, config schema in types.

---

## Framework: Next.js

```bash
# 1. Clone + stable tag
git clone --depth 1 https://github.com/vercel/next.js.git /tmp/nextjs-research
cd /tmp/nextjs-research && git fetch --tags && git checkout $(git describe --tags --abbrev=0)

# 2. Explore agent prompt:
# "Investigate /tmp/nextjs-research/packages/next/src: exported APIs, experimental flags, config options"

# 3. Quick searches
rg "experimental" /tmp/nextjs-research/packages/next/src/server/config-shared.ts
rg "deprecated" /tmp/nextjs-research/packages/next/src --type ts | head -20

# 4. Web search: "next.js 15 2025", "next.js app router 2026"

# 5. Cleanup
rm -rf /tmp/nextjs-research
```

**Key findings location:** `packages/next/src/server/config-shared.ts` for all config options.

---

## Runtime: Bun

```bash
# 1. Clone
git clone --depth 1 https://github.com/oven-sh/bun.git /tmp/bun-research
cd /tmp/bun-research && git fetch --tags && git checkout $(git describe --tags --abbrev=0)

# 2. Explore agent prompt:
# "Investigate /tmp/bun-research/src: CLI flags, built-in APIs (Bun.*), env vars"

# 3. Quick searches
rg "process\.env\." /tmp/bun-research/src --type ts | head -30
rg "Bun\." /tmp/bun-research/packages/bun-types/bun.d.ts | head -50

# 4. Web search: "bun runtime 2025", "bun vs node 2026"

# 5. Cleanup
rm -rf /tmp/bun-research
```

**Key findings location:** `packages/bun-types/` for all Bun.* APIs.

---

## Typical Output

After Wrangler research:

```markdown
## Wrangler v4.59.2 (2026-01-15)

**Repo:** github.com/cloudflare/workers-sdk @ abc123

### Commands
| Task | Command |
|------|---------|
| Dev | `wrangler dev` |
| Deploy | `wrangler deploy` |
| Tail logs | `wrangler tail` |
| Types | `wrangler types` |

### Config
| Option | Default | Notes |
|--------|---------|-------|
| `name` | required | Worker name |
| `main` | required | Entry point |
| `compatibility_date` | required | Runtime version |

### Gotchas
- **wrangler.toml vs wrangler.jsonc**: jsonc now recommended. Source: PR #1234
- **Auto-provisioning**: KV/R2/D1 auto-created if id omitted. Source: v4.50 release

### Sources
- Code: packages/wrangler/src/config/config.ts:45
- PRs: #5678, #5679
- Posts: blog.cloudflare.com/wrangler-4 (2025-09)
```
