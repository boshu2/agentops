# Comprehensive Technical Architecture Report Template

Copy this template and fill in the sections.

---

# [Project Name] - Technical Architecture Report

## Executive Summary

**[Project]** is a [CLI tool / web service / library] that [main purpose]. Built with [language] [version].

**Key Statistics:**
- ~X,XXX lines of code across Y modules
- Language: [Rust 1.XX / TypeScript 5.X / Python 3.XX]
- Key dependencies: [dep1], [dep2], [dep3], [dep4], [dep5]

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
| CLI main | `src/main.rs:15` | Parses args via clap, dispatches commands |
| HTTP router | `src/routes/mod.rs:1` | Sets up axum/express routes |
| [Add more] | `path:line` | Description |

---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `TypeName` | `src/model.rs:10` | Core domain object representing X |
| `Config` | `src/config.rs:5` | Runtime configuration loaded from file/env |
| `Storage` | `src/storage.rs:1` | Persistence layer abstraction |
| [Add more] | `path:line` | Description |

---

## Data Flow

```
[Input Source]
     │
     ▼
[Entry Point] ─── parses/validates
     │
     ▼
[Handler/Controller] ─── orchestrates
     │
     ▼
[Core Domain Logic] ─── business rules
     │
     ▼
[Storage/External] ─── persists/calls
     │
     ▼
[Output/Response]
```

**Happy Path Description:**
1. User invokes [command/endpoint]
2. [Entry] parses input and creates [Type]
3. [Handler] calls [Core] which processes...
4. Result is [stored/returned/displayed]

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
| SQLite (rusqlite) | Local persistence | Yes |
| reqwest | HTTP client for external APIs | No |
| tokio | Async runtime | Yes |
| serde | Serialization | Yes |
| [Add more] | Purpose | Yes/No |

---

## Configuration

| Source | Location/Example | Priority |
|--------|------------------|----------|
| Environment var | `APP_CONFIG=/path/to/config.toml` | 1 (highest) |
| Config file | `~/.config/app/config.toml` | 2 |
| CLI flag | `--config /path` | 3 |
| Default | Hardcoded in `src/config.rs:50` | 4 (lowest) |

**Key Config Options:**
- `option_name`: Description, default value
- `another_option`: Description, default value

---

## Module Structure

```
src/
├── main.rs          # Entry point, CLI setup
├── config.rs        # Configuration loading
├── model/           # Core domain types
│   ├── mod.rs
│   └── types.rs
├── handlers/        # Request/command handlers
│   └── mod.rs
├── storage/         # Persistence layer
│   ├── mod.rs
│   └── sqlite.rs
└── utils/           # Shared utilities
    └── mod.rs
```

---

## Test Infrastructure

| Type | Location | Count |
|------|----------|-------|
| Unit tests | `src/**/*.rs` (inline) | ~XXX |
| Integration | `tests/integration/` | ~XX |
| E2E | `tests/e2e/` | ~X |

**Running Tests:**
```bash
cargo test              # All tests
cargo test --lib        # Unit only
cargo test --test e2e   # E2E only
```

---

## Error Handling

- Error type: `src/error.rs` - uses thiserror/anyhow
- Propagation: `?` operator, Result<T, Error>
- User-facing: Formatted messages in CLI/API responses

---

## Logging

- Framework: tracing / log / env_logger
- Levels: Configurable via `RUST_LOG` or `--verbose`
- Output: stderr (CLI), structured JSON (service)

---

## Notes & Gotchas

- [Any non-obvious behavior]
- [Known limitations]
- [Areas needing improvement]

---

*Generated: [Date]*
*By: [Agent/Human]*
