# Go Style Guide

<!-- Canonical source: gitops/docs/standards/golang-style-guide.md -->
<!-- Last synced: 2026-01-19 -->

> **Purpose:** Idiomatic Go coding standards for this repository.

## Scope

This document covers: code formatting, concurrency patterns, interface design, error handling, and testing.

**Related:**
- [Python Style Guide](./python-style-guide.md) - Python coding conventions
- [Shell Script Standards](./shell-script-standards.md) - Bash scripting conventions

---

## Quick Reference

| Standard | Value | Validation |
|----------|-------|------------|
| **Go Version** | 1.24+ | `go version` |
| **Formatter** | `gofmt` / `goimports` | `gofmt -l .` |
| **Linter** | `golangci-lint` | `.golangci.yml` at repo root |
| **Module** | Required | `go.mod` in project root |

---

## golangci-lint Configuration

Create `.golangci.yml` at repo root:

```yaml
# .golangci.yml
version: "2"

run:
  timeout: 5m
  go: "1.24"

linters:
  default: none
  enable:
    - errcheck      # Check error returns
    - govet         # Vet examines Go source
    - staticcheck   # Static analysis
    - ineffassign   # Detect ineffective assignments
    - unused        # Check for unused code
    - misspell      # Find misspellings
    - revive        # Replacement for golint
    - gocritic      # Opinionated linter
    - errname       # Error naming conventions
    - errorlint     # Error wrapping checks
  settings:
    revive:
      rules:
        - name: exported
          arguments: [checkPrivateReceivers]
        - name: var-naming
        - name: blank-imports
    gocritic:
      enabled-tags:
        - diagnostic
        - style
        - performance
    errcheck:
      check-blank: true
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - errcheck
```

**Usage:**
```bash
# Run linter
golangci-lint run ./...

# Auto-fix issues
golangci-lint run --fix ./...

# Check specific package
golangci-lint run ./pkg/...
```

---

## Concurrency Patterns

### Goroutines with WaitGroup

```go
func processItems(items []Item) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(items))

    for _, item := range items {
        wg.Add(1)
        go func(it Item) {
            defer wg.Done()
            if err := process(it); err != nil {
                errCh <- fmt.Errorf("process %s: %w", it.Name, err)
            }
        }(item)
    }

    wg.Wait()
    close(errCh)

    // Collect errors
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }
    return errors.Join(errs...)
}
```

### Channel Select with Context

```go
func worker(ctx context.Context, jobs <-chan Job, results chan<- Result) {
    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-jobs:
            if !ok {
                return
            }
            results <- process(job)
        }
    }
}
```

### Bounded Concurrency (Worker Pool)

```go
func processWithLimit(ctx context.Context, items []Item, limit int) error {
    sem := make(chan struct{}, limit)
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // Capture for goroutine
        g.Go(func() error {
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
            case <-ctx.Done():
                return ctx.Err()
            }
            return process(ctx, item)
        })
    }
    return g.Wait()
}
```

---

## Interface Design

### Accept Interfaces, Return Structs

```go
// Good - Accept interface
func ProcessData(r io.Reader) error {
    // Can accept *os.File, *bytes.Buffer, etc.
}

// Good - Return concrete type
func NewClient(cfg Config) *Client {
    return &Client{cfg: cfg}
}

// Bad - Return interface (hides implementation)
func NewClient(cfg Config) ClientInterface {
    return &Client{cfg: cfg}
}
```

### Small, Focused Interfaces

```go
// Good - Single method interface
type Processor interface {
    Process(ctx context.Context, data []byte) error
}

type Validator interface {
    Validate() error
}

// Composition when needed
type ValidatingProcessor interface {
    Processor
    Validator
}

// Bad - Kitchen sink interface
type Manager interface {
    Create() error
    Update() error
    Delete() error
    List() ([]Item, error)
    Validate() error
    Process() error
    // ... 10 more methods
}
```

---

## Error Handling

### Wrap Errors with Context

```go
import "fmt"

func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config %s: %w", path, err)
    }
    return &cfg, nil
}
```

### Custom Error Types

```go
// Sentinel errors for comparison
var (
    ErrNotFound    = errors.New("not found")
    ErrPermission  = errors.New("permission denied")
)

// Structured error with context
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// Usage with errors.Is/As
func process(id string) error {
    item, err := fetch(id)
    if errors.Is(err, ErrNotFound) {
        return fmt.Errorf("item %s: %w", id, err)
    }

    var valErr *ValidationError
    if errors.As(err, &valErr) {
        log.Printf("validation issue: %s", valErr.Field)
    }
    return err
}
```

### Don't Panic in Libraries

```go
// Good - Return error
func Parse(data []byte) (*Config, error) {
    if len(data) == 0 {
        return nil, errors.New("empty data")
    }
    // ...
}

// Bad - Panic in library code
func Parse(data []byte) *Config {
    if len(data) == 0 {
        panic("empty data")
    }
    // ...
}
```

---

## Table-Driven Tests

```go
func TestValidateNamespace(t *testing.T) {
    tests := []struct {
        name      string
        namespace string
        wantErr   bool
    }{
        {
            name:      "valid namespace",
            namespace: "my-namespace",
            wantErr:   false,
        },
        {
            name:      "empty namespace",
            namespace: "",
            wantErr:   true,
        },
        {
            name:      "invalid characters",
            namespace: "My_Namespace",
            wantErr:   true,
        },
        {
            name:      "too long",
            namespace: strings.Repeat("a", 64),
            wantErr:   true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateNamespace(tt.namespace)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateNamespace(%q) error = %v, wantErr %v",
                    tt.namespace, err, tt.wantErr)
            }
        })
    }
}
```

### Test Helpers

```go
func TestClient(t *testing.T) {
    // t.Helper() marks function as test helper
    // failures report caller's line, not helper's
    client := newTestClient(t)
    // ...
}

func newTestClient(t *testing.T) *Client {
    t.Helper()
    client, err := NewClient(testConfig)
    if err != nil {
        t.Fatalf("failed to create test client: %v", err)
    }
    return client
}
```

---

## Code Template

```go
// Package config provides configuration loading for tools.
package config

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
)

// Errors for this package.
var (
    ErrNotFound = errors.New("config not found")
    ErrInvalid  = errors.New("invalid config")
)

// Config holds application configuration.
type Config struct {
    Namespace string `json:"namespace"`
    Timeout   int    `json:"timeout"`
}

// Loader loads configuration from various sources.
type Loader interface {
    Load(ctx context.Context, path string) (*Config, error)
}

// FileLoader loads config from filesystem.
type FileLoader struct {
    basePath string
}

// NewFileLoader creates a FileLoader with the given base path.
func NewFileLoader(basePath string) *FileLoader {
    return &FileLoader{basePath: basePath}
}

// Load reads and parses config from a file.
func (l *FileLoader) Load(ctx context.Context, path string) (*Config, error) {
    fullPath := l.basePath + "/" + path

    data, err := os.ReadFile(fullPath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
        }
        return nil, fmt.Errorf("read %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("%w: parse %s: %v", ErrInvalid, path, err)
    }

    if err := cfg.validate(); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
    }

    return &cfg, nil
}

func (c *Config) validate() error {
    if c.Namespace == "" {
        return errors.New("namespace required")
    }
    if c.Timeout <= 0 {
        c.Timeout = 30 // Default
    }
    return nil
}
```

---

## Code Complexity

### Complexity Measurement

Use `gocyclo` or `gocognit` to measure function complexity:

```bash
# Install
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# Check changed CLI files with the repo ratchet
bash scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25

# Show all functions sorted by complexity
gocyclo -top 20 ./...
```

The repo gate is a ratchet, not a blanket historical cleanup. CI warns for
changed non-test functions at CC 15+, and fails only when a changed function is
newly at or worsened beyond CC 25. Existing high-complexity functions are
reported when touched but are non-blocking if the change does not make them
worse.

### Complexity Grades

| Grade | CC Range | Action |
|-------|----------|--------|
| A | 1-5 | Ideal |
| B | 6-10 | Acceptable |
| C | 11-14 | Refactor opportunistically |
| D | 15-24 | Warning threshold; refactor or explain when touching |
| F | 25+ | Failure threshold for new or worsened changed functions |

### CLI Hot-File Budget

Large Cobra command files are allowed when they are already established command
surfaces, but they need a stricter edit discipline. For production files under
`cli/cmd/ao/` over 500 lines:

- Keep net production LOC neutral when the change is a refactor or bug fix.
- Add paired direct command tests when behavior changes.
- Move reusable logic into `cli/internal/` instead of adding more package-global
  state or process-wide stdout capture.
- Do not lower the CC failure threshold below 25 without a separate baseline and
  migration plan.

Use this command to identify current hot files:

```bash
find cli/cmd/ao -name '*.go' -not -name '*_test.go' -print0 \
  | xargs -0 wc -l \
  | sort -nr \
  | head
```

### Reducing Complexity in Go

**Pattern 1: Early Returns**
```go
// Bad - nested conditionals (CC=6)
func process(item *Item) error {
    if item != nil {
        if item.Valid {
            if item.Ready {
                return doWork(item)
            }
        }
    }
    return errors.New("invalid")
}

// Good - guard clauses (CC=3)
func process(item *Item) error {
    if item == nil {
        return errors.New("nil item")
    }
    if !item.Valid {
        return errors.New("invalid item")
    }
    if !item.Ready {
        return errors.New("not ready")
    }
    return doWork(item)
}
```

**Pattern 2: Strategy Maps**
```go
// Bad - switch statement (CC grows with cases)
func handle(cmd string) error {
    switch cmd {
    case "create":
        return handleCreate()
    case "update":
        return handleUpdate()
    case "delete":
        return handleDelete()
    // ... more cases
    }
    return errors.New("unknown command")
}

// Good - handler map (CC=2)
var handlers = map[string]func() error{
    "create": handleCreate,
    "update": handleUpdate,
    "delete": handleDelete,
}

func handle(cmd string) error {
    h, ok := handlers[cmd]
    if !ok {
        return errors.New("unknown command")
    }
    return h()
}
```

---

## Common Errors

| Symptom | Cause | Fix |
|---------|-------|-----|
| `undefined: X` | Missing import or typo | Check imports, spelling |
| `cannot use X as Y` | Type mismatch | Check interface compliance |
| `nil pointer dereference` | Uninitialized pointer | Add nil check before use |
| `deadlock` | Goroutine waiting forever | Check channel/mutex usage |
| `race detected` | Data race | Use mutex or channels |
| `context canceled` | Parent context done | Handle `ctx.Err()` |
| `go.mod outdated` | Dependency drift | Run `go mod tidy` |

---

## Anti-Patterns

| Name | Pattern | Why Bad | Instead |
|------|---------|---------|---------|
| Naked Returns | `return` without values | Unclear what's returned | Explicit: `return result, nil` |
| Init Abuse | Heavy logic in `init()` | Hidden side effects, test issues | Explicit initialization |
| Interface Pollution | Interfaces with 10+ methods | Hard to implement/mock | Small interfaces, compose |
| Error Strings | `errors.New("User not found")` | Can't check programmatically | Sentinel errors or types |
| Ignoring Errors | `result, _ := fn()` | Silent failures | Handle or document why ignored |
| Premature Channel | Channels for simple sync | Overhead, complexity | Use mutex for simple cases |

---

## Summary Checklist

| Category | Requirement |
|----------|-------------|
| **Tooling** | Go 1.24+, `gofmt`, `golangci-lint` |
| **Formatting** | All code passes `gofmt -l .` |
| **Linting** | All code passes `golangci-lint run` |
| **Complexity** | Aim for CC ≤ 10; CI warns at 15 and fails new/worsened changed functions at 25 |
| **Errors** | Wrap with context using `fmt.Errorf("...: %w", err)` |
| **Errors** | Use `errors.Is`/`errors.As` for comparison |
| **Errors** | No `panic` in library code |
| **Interfaces** | Accept interfaces, return structs |
| **Interfaces** | Keep interfaces small (1-3 methods) |
| **Concurrency** | Use `context.Context` for cancellation |
| **Concurrency** | Use `errgroup` for bounded concurrency |
| **Tests** | Table-driven tests with subtests |
| **Tests** | Use `t.Helper()` in test helpers |
| **Dependencies** | Prefer standard library |
