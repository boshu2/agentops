# Example Architecture Reports

## Example 1: beads_rust (CLI Tool)

Real report from a local-first issue tracker:

```markdown
# beads_rust - Technical Architecture Report

## Executive Summary

**beads_rust** is a local-first issue tracker CLI optimized for AI coding agents. Built with Rust 1.85, Edition 2024.

**Key Statistics:**
- ~3,500 lines of code across 12 modules
- Language: Rust 1.85 (Edition 2024)
- Key dependencies: clap, rusqlite, serde, chrono, anyhow

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `src/main.rs:1` | Parses args via clap, dispatches to commands |
| Commands | `src/commands/*.rs` | Individual command implementations |

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `Issue` | `src/model.rs:15` | Core domain object - the issue/bead |
| `Storage` | `src/storage.rs:1` | SQLite persistence layer |
| `Cli` | `src/main.rs:20` | clap-derived CLI structure |
| `Config` | `src/config.rs:1` | Runtime configuration |

---

## Data Flow

```
CLI Input (br create "title")
     │
     ▼
Clap Parser ─── validates args
     │
     ▼
Command Handler ─── orchestrates
     │
     ▼
Storage Layer ─── SQLite + JSONL sync
     │
     ▼
Output (JSON/table/confirmation)
```

**Happy Path:** User runs `br create "Fix bug"` → clap parses → CreateCommand runs → Storage inserts to SQLite → JSONL sync triggered → ID printed.

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| rusqlite (bundled SQLite) | Local persistence | Yes |
| serde/serde_json | Serialization | Yes |
| clap | CLI parsing | Yes |
| chrono | Timestamps | Yes |
| rich_rust | Terminal formatting | No |

---

## Configuration

| Source | Example | Priority |
|--------|---------|----------|
| Env var | `BR_DB_PATH=/path/to/db` | Highest |
| Config file | `.beads/config.yaml` | Medium |
| Default | `.beads/beads.db` | Lowest |

---

## Test Infrastructure

| Type | Location | Count |
|------|----------|-------|
| Unit tests | `src/*.rs` (inline) | ~40 |
| Integration | `tests/` | ~15 |
| Benchmarks | `benches/storage_perf.rs` | 1 suite |

**Running Tests:**
```bash
cargo test              # All tests
cargo test --lib        # Unit only
cargo bench             # Performance benchmarks
```

---

## Notes & Gotchas

- JSONL sync is one-way (SQLite → JSONL) for git compatibility
- Issue IDs are base36 encoded for compactness
- `--robot` flag outputs JSON for agent consumption
```

---

## Example 2: Web Service (Express/TypeScript)

```markdown
# api-gateway - Technical Architecture Report

## Executive Summary

**api-gateway** is an Express.js API gateway handling auth, rate limiting, and request routing. Built with TypeScript 5.3.

**Key Statistics:**
- ~2,100 lines across 8 modules
- Language: TypeScript 5.3
- Key dependencies: express, passport, redis, zod, pino

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| Server boot | `src/index.ts:1` | Express app initialization |
| Router setup | `src/routes/index.ts:1` | Route registration |
| Middleware chain | `src/middleware/index.ts:1` | Auth, rate limit, logging |

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `User` | `src/types/user.ts:5` | Authenticated user shape |
| `ApiRequest` | `src/types/request.ts:1` | Extended Express Request |
| `RateLimitConfig` | `src/config/limits.ts:10` | Per-route rate limits |

---

## Data Flow

```
HTTP Request
     │
     ▼
Express Router ─── path matching
     │
     ▼
Middleware Stack ─── auth, rate limit, validation
     │
     ▼
Route Handler ─── business logic
     │
     ▼
Upstream Service ─── proxy to microservices
     │
     ▼
Response Transform ─── standardize format
     │
     ▼
HTTP Response
```

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| Redis | Rate limiting, sessions | Yes |
| PostgreSQL | User data | Yes |
| Upstream APIs | Backend services | Yes |
| Sentry | Error tracking | No |

---

## Configuration

| Source | Example | Priority |
|--------|---------|----------|
| Env var | `DATABASE_URL`, `REDIS_URL` | Highest |
| Config file | `config/production.json` | Medium |
| Default | `config/default.json` | Lowest |

Uses `node-config` for layered configuration.
```

---

## Quick vs Deep Reports

| Report Type | Time | Depth | Use When |
|-------------|------|-------|----------|
| **Quick Scan** | 10 min | Entry points + key types | Orientation, PR review |
| **Standard** | 30 min | Full template | Onboarding, documentation |
| **Deep Dive** | 1+ hr | + sequence diagrams, all flows | Architecture review, audits |

### Quick Scan Prompt

```
Give me a quick architecture overview of this codebase:
- What is it?
- Entry points (main, routes, handlers)
- 3 key types
- Main data flow

Keep it under 200 lines.
```

### Deep Dive Additions

For deep reports, also include:
- Sequence diagrams for critical flows
- All error handling paths
- Performance characteristics
- Security considerations
- Technical debt inventory
