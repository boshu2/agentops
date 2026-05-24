# Contract Template

> One contract per issue. Spec workers fill this out before implementation begins.

---

```yaml
# --- Contract Frontmatter ---
issue:      # e.g., ag-abc.3
framework:  # go | python | typescript | rust | shell
category:   # feature | bugfix | refactor | docs | chore | ci
```

---

## Problem

<!-- 1-2 sentences. What is broken, missing, or suboptimal? -->

## Inputs

<!-- Bullet list: name, type, description -->

- `inputName` (type) — description

## Outputs

<!-- Bullet list: name, type, description -->

- `outputName` (type) — description

## Invariants

<!-- Numbered list. Minimum 3. These are properties that must ALWAYS hold. -->

1. ...
2. ...
3. ...

## Failure Modes

<!-- Numbered list: what could go wrong → expected behavior -->

1. **Condition** → expected behavior
2. **Condition** → expected behavior

## Out of Scope

<!-- Explicitly excluded items — prevents scope creep -->

- ...

## Test Cases

<!-- Map each test case to an invariant. Cover: boundaries, errors, success path. -->

| # | Input | Expected | Validates Invariant |
|---|-------|----------|---------------------|
| 1 | ... | ... | #1 |
| 2 | ... | ... | #2 |
| 3 | ... | ... | #3 |

## Contract Granularity

- **1 contract per issue.** Do not combine multiple issues into one contract.
- **Test boundaries, errors, and success paths.** Every contract must have at least one test case for each category.
- **Acceptance criteria are user-facing.** Invariants describe system properties; acceptance criteria describe what the user sees. Keep them separate — invariants go here, acceptance criteria go in the issue.

---

# EXAMPLE: Add Rate Limiting Middleware

```yaml
issue:      ag-xyz.5
framework:  go
category:   feature
```

## Problem

API endpoints accept unlimited requests per client, enabling abuse and risking resource exhaustion under load.

## Inputs

- `request` (*http.Request) — incoming HTTP request with client IP in RemoteAddr
- `config.RateLimit` (int) — max requests per window per client (default: 100)
- `config.RateWindow` (time.Duration) — sliding window duration (default: 1 minute)

## Outputs

- **Pass-through** — request forwarded to next handler with `X-RateLimit-Remaining` header
- **429 response** — JSON error body `{"error": "rate limit exceeded", "retry_after": <seconds>}` with `Retry-After` header

## Invariants

1. A client sending ≤ `RateLimit` requests within `RateWindow` is never rejected.
2. A client exceeding `RateLimit` within `RateWindow` receives HTTP 429 for every subsequent request until the window expires.
3. Rate limit state for one client never affects another client's quota.
4. The middleware adds < 1ms p99 latency to the request path.
5. If the rate limit store is unavailable, requests pass through (fail-open) and an error is logged.

## Failure Modes

1. **Rate store unreachable** → fail-open, log error, increment `ratelimit_store_errors_total` metric.
2. **Malformed RemoteAddr** → treat as unknown client, apply default limit, log warning.
3. **Clock skew between instances** → accept up to 2× burst during window overlap (documented trade-off).

## Out of Scope

- Distributed rate limiting across multiple instances (future work).
- Per-endpoint rate limits (all endpoints share the same limit).
- Authentication-aware rate limiting (keyed by IP only).

## Test Cases

| # | Input | Expected | Validates Invariant |
|---|-------|----------|---------------------|
| 1 | 100 requests from same IP in 60s | All 100 return 200 | #1 — at-limit success |
| 2 | 101st request from same IP in 60s | Returns 429 with `Retry-After` header | #2 — over-limit rejection |
| 3 | 100 requests from IP-A, 100 from IP-B | All 200 return 200 | #3 — client isolation |
| 4 | Request when rate store returns error | Returns 200, error logged | #5 — fail-open behavior |
| 5 | Wait for window expiry after 429 | Next request returns 200 | #2 — window reset |
| 6 | Benchmark 10k requests | p99 < 1ms overhead | #4 — latency bound |
