# Common Extractable Patterns

## Error Handling

**Signature:** Custom error types with context

```rust
// Instance A (project_a)
#[derive(Debug, thiserror::Error)]
pub enum ProjectAError {
    #[error("Config error: {0}")]
    Config(#[from] ConfigError),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

// Instance B (project_b)
#[derive(Debug, thiserror::Error)]
pub enum ProjectBError {
    #[error("Database error: {0}")]
    Database(#[from] DbError),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

// Extracted: Error macro or trait
```

**Variance:** Error variants per domain
**Invariant:** Structure, From impls, Display format

---

## Retry Logic

**Signature:** Retry with exponential backoff

```typescript
// Common pattern across projects
async function withRetry<T>(
  fn: () => Promise<T>,
  { maxAttempts = 3, initialDelay = 100, backoffFactor = 2 }
): Promise<T>
```

**Variance:** Max attempts, delay, backoff factor
**Invariant:** Retry loop, exponential backoff formula

---

## CLI Structure

**Signature:** Command + subcommands + global flags

```rust
#[derive(Parser)]
struct Cli {
    #[command(subcommand)]
    command: Commands,

    #[arg(long, global = true)]
    verbose: bool,

    #[arg(long, global = true)]
    json: bool,
}
```

**Variance:** Command names, specific flags
**Invariant:** Global --verbose, --json, subcommand pattern

---

## Config Loading

**Signature:** File + env + defaults cascade

```python
# Pattern: config file → env vars → defaults
def load_config():
    config = DEFAULT_CONFIG.copy()
    config.update(load_file("config.yaml"))
    config.update(load_env_vars("APP_"))
    return config
```

**Variance:** File paths, env prefix, defaults
**Invariant:** Cascade order, merge logic

---

## Database Connection Pool

**Signature:** Pool with health checks

```go
type Pool struct {
    conns     chan *Conn
    maxSize   int
    healthCheck func(*Conn) bool
}
```

**Variance:** Pool size, health check logic
**Invariant:** Channel-based pool, acquire/release

---

## HTTP Client Wrapper

**Signature:** Client with timeout, retry, auth

```typescript
class ApiClient {
    constructor(
        private baseUrl: string,
        private timeout: number = 30000,
        private retries: number = 3,
    ) {}

    async get<T>(path: string): Promise<T>
    async post<T>(path: string, body: unknown): Promise<T>
}
```

**Variance:** Base URL, timeout, auth headers
**Invariant:** Timeout handling, retry logic, error mapping

---

## Logging Setup

**Signature:** Structured logging with levels

```rust
fn setup_logging(verbose: bool, json: bool) {
    let filter = if verbose { "debug" } else { "info" };
    let format = if json { json_format() } else { pretty_format() };
    // Initialize logger
}
```

**Variance:** Default level, output format
**Invariant:** Level filtering, format switching

---

## Extraction Signals

When you see these patterns 3+ times, extract:

| Signal | Pattern | Extract As |
|--------|---------|------------|
| Same imports | `use retry::*` everywhere | Shared crate |
| Copy-paste | Identical functions | Library |
| Similar structs | `Config`, `Options` | Shared types |
| Same CLI flags | `--verbose --json` | CLI module |
| Same workflow | "Run X, then Y, then Z" | Skill |
| Same structure | Project layout | Template |
