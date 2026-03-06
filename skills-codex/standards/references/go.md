# Go Standards (Tier 1)

## Target Version

Detect from `go.mod`. Use all features up to and including that version. Never use features from newer versions. Current project target: **Go 1.26**.

## Required

- `gofmt` (automatic)
- `golangci-lint run` passes
- All exported symbols documented

## Error Handling

- Always check errors: `if err != nil`
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Never `_ = err` without `// nolint:errcheck` comment
- Use `errors.Is(err, target)` instead of `err == target` -- works with wrapped errors (1.13+)
- Use `errors.Join(err1, err2)` to aggregate errors from parallel operations or multi-step cleanup (1.20+)
- Use `context.WithCancelCause` / `context.Cause` to attach error reasons to cancellations (1.20+)

## Common Issues

| Pattern | Problem | Fix |
|---------|---------|-----|
| `%v` for errors | Breaks error chain | Use `%w` |
| `panic()` in library | Crashes caller | Return error |
| Naked goroutine | No error handling | errgroup or channels |
| `interface{}` | Type safety loss | Use `any` (1.18+), generics, or specific types |
| `err == target` | Misses wrapped errors | `errors.Is(err, target)` (1.13+) |
| `atomic.StoreInt32` | Type-unsafe | `atomic.Bool` / `atomic.Int64` / `atomic.Pointer[T]` (1.19+) |
| `for i := 0; i < n; i++` | Verbose | `for i := range n` (1.22+) |
| Manual loop for contains/sort | Error-prone, verbose | `slices.Contains`, `slices.SortFunc` (1.21+) |
| `sync.Once` + closure wrapper | Verbose, easy to misuse | `sync.OnceFunc` / `sync.OnceValue` (1.21+) |

## Interfaces

- Accept interfaces, return structs
- Keep interfaces small (1-3 methods)
- Define interfaces where used, not implemented

## Documentation

- All exported symbols must have godoc comments starting with the symbol name
- Package-level doc in `doc.go` for non-trivial packages
- Include runnable `Example_*` functions in `_test.go` files
- Run `go doc ./...` to verify documentation

## Concurrency

- Always pass `context.Context` as first param
- Use `sync.Mutex` for shared state; use type-safe atomics (`atomic.Bool`, `atomic.Int64`, `atomic.Pointer[T]`) for simple flags/counters (1.19+)
- Prefer channels for communication
- Use `sync.OnceFunc(fn)` instead of `sync.Once` + wrapper; `sync.OnceValue(fn)` when returning a value (1.21+)
- Use `context.AfterFunc(ctx, cleanup)` to register cleanup on cancellation (1.21+)
- Loop variables are safe to capture in goroutines since 1.22 (each iteration gets its own copy)

## Modern Standard Library

### slices package (1.21+)

Prefer `slices` over hand-written loops:

| Function | Replaces |
|----------|----------|
| `slices.Contains(items, x)` | Manual search loop |
| `slices.Index(items, x)` | Manual search loop returning index |
| `slices.IndexFunc(items, fn)` | Manual search loop with predicate |
| `slices.Sort(items)` | `sort.Slice` / `sort.Strings` |
| `slices.SortFunc(items, cmp)` | `sort.Slice` with less function |
| `slices.Max(items)` / `slices.Min(items)` | Manual loop tracking max/min |
| `slices.Reverse(items)` | Manual swap loop |
| `slices.Compact(items)` | Manual dedup of consecutive elements |
| `slices.Clip(s)` | `s[:len(s):len(s)]` to remove excess capacity |
| `slices.Clone(s)` | `append([]T(nil), s...)` |

Iterator consumption (1.23+):

| Function | Usage |
|----------|-------|
| `slices.Collect(iter)` | Build slice from iterator |
| `slices.Sorted(iter)` | Collect and sort in one step |

### maps package (1.21+; Keys/Values return iterators as of 1.23)

| Function | Replaces |
|----------|----------|
| `maps.Clone(m)` | Manual map copy loop |
| `maps.Copy(dst, src)` | Manual map merge loop |
| `maps.DeleteFunc(m, fn)` | Manual delete loop with predicate |
| `maps.Keys(m)` | Manual key collection loop (returns iterator, 1.23+) |
| `maps.Values(m)` | Manual value collection loop (returns iterator, 1.23+) |

### cmp package (1.22+)

- `cmp.Or(a, b, c)` -- returns first non-zero value. Replaces `if x == "" { x = default }` chains:
  ```go
  name := cmp.Or(os.Getenv("NAME"), config.Name, "default")
  ```

### strings / bytes improvements

| Function | Version | Replaces |
|----------|---------|----------|
| `strings.Cut(s, sep)` / `bytes.Cut(b, sep)` | 1.18+ | `Index` + slice arithmetic |
| `strings.CutPrefix(s, prefix)` / `strings.CutSuffix(s, suffix)` | 1.20+ | `HasPrefix` + `TrimPrefix` |
| `strings.Clone(s)` / `bytes.Clone(b)` | 1.20+ | Manual copy (prevents memory leaks from substring references) |

### net/http improvements (1.22+)

Enhanced `ServeMux` with method and path parameters:

```go
mux.HandleFunc("GET /api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    // ...
})
```

May eliminate the need for third-party routers for simple APIs.

### Other stdlib

| Function | Version | Replaces |
|----------|---------|----------|
| `fmt.Appendf(buf, fmt, args...)` | 1.19+ | `[]byte(fmt.Sprintf(...))` -- avoids allocation |
| `time.Since(start)` | 1.0+ | `time.Now().Sub(start)` |
| `time.Until(deadline)` | 1.8+ | `deadline.Sub(time.Now())` |
| `errors.Join(err1, err2)` | 1.20+ | Discarding all but the first error (see Error Handling) |
| `reflect.TypeFor[T]()` | 1.22+ | `reflect.TypeOf((*T)(nil)).Elem()` |
| `min(a, b)` / `max(a, b)` | 1.21+ | `if a > b` patterns or custom helpers |
| `clear(m)` / `clear(s)` | 1.21+ | Manual map deletion loop / manual slice zeroing |

## Testing

### CI-Safe Test Pattern

When testing functions that shell out to external CLIs (`bd`, `ao`, `gh`, etc.), **test the low-level function directly** instead of the wrapper that invokes the CLI. This ensures tests pass in CI where the CLI may not be installed.

```go
// BAD: calls processDiscoveryPhase() which requires bd CLI
func TestGateDiscoveryVerdictC2Event(t *testing.T) {
    processDiscoveryPhase(ctx, root, opts) // fails in CI — bd not available
}

// GOOD: test event shape directly via the underlying function
func TestGateDiscoveryVerdictC2Event(t *testing.T) {
    ev, err := appendRPIC2Event(root, rpiC2EventInput{
        RunID: runID, Phase: 1, Type: "gate.discovery.verdict",
        Message: "Pre-mortem verdict: PASS",
        Details: map[string]any{"verdict": "PASS", "report": "report.md"},
    })
    require.NoError(t, err)
    assert.Equal(t, "gate.discovery.verdict", ev.Type)
}
```

**Rule:** If a function's only untestable part is the external CLI call, extract the testable logic (event emission, state mutation, file I/O) into a separate function and test that.

### Table-Driven Tests

Prefer table-driven tests for functions with multiple input/output cases:

```go
func TestClassifyServeArg(t *testing.T) {
    tests := []struct {
        name      string
        flagRunID string
        args      []string
        wantGoal  string
        wantRunID string
    }{
        {"empty", "", nil, "", ""},
        {"flag run-id", "rpi-abc12345", nil, "", "rpi-abc12345"},
        {"arg goal", "", []string{"fix the bug"}, "fix the bug", ""},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            goal, runID := classifyServeArg(tt.flagRunID, tt.args)
            assert.Equal(t, tt.wantGoal, goal)
            assert.Equal(t, tt.wantRunID, runID)
        })
    }
}
```

## HTTP Handler Security

Go HTTP handlers in this codebase are localhost-only but should still follow defense-in-depth:

| Pattern | Risk | Fix |
|---------|------|-----|
| `innerHTML = userInput` in embedded HTML | XSS | Use DOM construction (`createElement` + `textContent`) |
| `r.URL.Query().Get("param")` used in file paths | Path traversal | Reject `..`, `/`, `\` before use |
| `fmt.Fprintf(w, userInput)` in HTML handler | XSS | Use `html/template` or `text/template` with escaping |
| `filepath.Join(root, userInput)` | Path traversal | Validate input against allowlist pattern (e.g., `regexp`) |
| `Access-Control-Allow-Origin: *` | CORS bypass | Acceptable for localhost-only; restrict for public APIs |

**Query parameter validation pattern:**

```go
param := strings.TrimSpace(r.URL.Query().Get("id"))
if param != "" && (strings.Contains(param, "..") || strings.Contains(param, "/") || strings.Contains(param, "\\")) {
    http.Error(w, "invalid parameter", http.StatusBadRequest)
    return
}
```

**DOM construction instead of innerHTML:**

```javascript
// BAD: innerHTML with user-controlled data
el.innerHTML = '<span>' + userInput + '</span>';

// GOOD: DOM construction
const span = document.createElement('span');
span.textContent = userInput;
el.appendChild(span);
```

## Future Features (Go 1.24+)

This section tracks features by first-supported Go version and can be used to plan future target upgrades.

| Feature | Version | What It Replaces |
|---------|---------|------------------|
| `t.Context()` | 1.24+ | `context.WithCancel(context.Background())` in tests |
| `b.Loop()` | 1.24+ | `for i := 0; i < b.N; i++` in benchmarks |
| `omitzero` JSON tag | 1.24+ | `omitempty` (which fails for `time.Duration`, structs, slices, maps) |
| `strings.SplitSeq` / `FieldsSeq` | 1.24+ | `strings.Split` when iterating (avoids intermediate slice) |
| `wg.Go(fn)` | 1.25+ | `wg.Add(1)` + `go func() { defer wg.Done(); ... }()` |
| `new(val)` | 1.26+ | `x := val; &x` for pointer creation |
| `errors.AsType[T](err)` | 1.26+ | `var target T; errors.As(err, &target)` |
