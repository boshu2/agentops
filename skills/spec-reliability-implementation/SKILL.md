---
name: spec-reliability-implementation
description: |-
  Use when implementing a written spec into a reliable service with acceptance examples and observability.
  Triggers:
skill_api_version: 1
user-invocable: false
hexagonal_role: driving-adapter
practices:
- behavior-driven-development
- design-by-contract
- defensive-programming
- evidence-driven
consumes:
- specification
- repo-context
produces:
- acceptance-examples.feature
- running-service
- conformance-report.md
context_rel:
- kind: customer-of
  with: validate
context:
  window: fork
  intent:
    mode: task
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  dependencies:
  - standards
  - validate
output_contract: "file: conformance-report.md (spec\u2192behavior verification verdict, with acceptance results)"
---

# spec-reliability-implementation — a spec becomes a service that provably behaves like the spec

## ⚠️ Critical Constraints

- **The spec is the source of truth, not the code.** Every accepted behavior must
  trace to a spec clause. **Why:** code that "works" but diverges from the spec is
  a silent breach — the bug ships because nothing checks it against intent.

- **Acceptance examples come BEFORE implementation.** Write the executable
  examples (Given/When/Then) from the spec first, watch them fail, then make them
  pass. **Why:** examples written after the code only re-encode the code's
  behavior, so they can never catch a spec mismatch.
  ```
  WRONG: implement endpoint → write a test that asserts whatever it returns
  CORRECT: write Then "POST /orders twice with same key yields ONE order" → run (fails) → implement idempotency → run (passes)
  ```

- **Retries require idempotency; idempotency requires a key.** Never add a retry
  to a non-idempotent write. **Why:** retrying a non-idempotent call (double
  charge, duplicate order) turns a transient blip into a data-corruption bug.
  ```
  WRONG: client retries POST /charge on timeout → two charges
  CORRECT: client sends Idempotency-Key; server dedupes → one charge, retry-safe
  ```

- **An error path the spec names but the service does not handle is a FAIL,**
  not a TODO. **Why:** unhandled spec'd errors (bad input, downstream down, auth
  failure) are exactly the cases that page on-call.

## Why This Exists

Specs and services drift the moment code starts. The default failure mode is a
service that demos green but breaks the spec in the unhappy paths — the timeout,
the duplicate request, the malformed payload, the dependency outage. This skill
forces a closed loop: spec → executable acceptance examples → gated
implementation → reliability primitives (errors/retries/idempotency/observability)
→ a verification step that re-checks behavior against the spec before ship. The
output is not "it runs" — it's "it provably does what the spec says, including
when things go wrong."

## Quick Start

1. Read the spec. Extract a **clause inventory** — every testable assertion, each
   with an id (S1, S2, …). Mark each as happy-path, error-path, or
   reliability-property (retry/idempotency/observability).
2. Write `acceptance-examples.feature` — one scenario per clause, in Given/When/Then.
3. Run the examples; confirm they FAIL (red). No red, no proof.
4. Implement the smallest code to turn each scenario green, one clause at a time.
5. Add reliability primitives where the inventory demands them (see Methodology §4).
6. Run `bash {baseDir}/scripts/validate.sh` and the acceptance suite.
7. Produce `conformance-report.md`: clause → scenario → PASS/FAIL with evidence.

## Methodology

### 1. Spec → clause inventory (verification checkpoint: every testable line has an id)
Parse the spec into atomic, testable clauses. For each: id, quote, category
(happy / error / reliability / non-functional), and the observable that proves it.
A clause with no observable is not yet testable — push back on the spec, don't guess.

### 2. Clauses → acceptance examples (checkpoint: every clause maps to ≥1 scenario)
Write `acceptance-examples.feature` (Gherkin or your runner's equivalent). Each
scenario names its clause id in a tag or comment. Cover happy AND error paths.
Run the suite; it MUST be red before any implementation. Save the red run as evidence.

### 3. Implement against gates (checkpoint: one clause goes green per change, suite stays sorted)
Implement the thinnest slice to flip one scenario green. After each change, run
the suite + lint + type-check. Never batch — green one clause at a time so a
regression is attributable. Keep already-green scenarios green.

### 4. Reliability primitives (checkpoint: each property has a test that fails without it)
Add only what the inventory demands, each with a scenario that fails when removed:
- **Error handling:** map every spec'd error to a typed response/status. No bare
  500s for cases the spec anticipated. Validate input at the boundary.
- **Retries:** apply to idempotent operations only; bounded attempts, exponential
  backoff + jitter, a deadline/budget, and a circuit breaker for a failing
  dependency. Distinguish retryable (timeout, 503) from terminal (400, 409).
- **Idempotency:** dedupe writes by an idempotency key (client-supplied or derived);
  persist the key→result so a replay returns the original result, not a new effect.
- **Observability:** structured logs with a correlation/request id; metrics for
  rate / errors / latency (RED); a `/healthz` liveness and `/readyz` readiness
  probe; trace spans across the request and its downstream calls.

### 5. Verify behavior matches spec (checkpoint: conformance report is all-green or ship is blocked)
Run the full acceptance suite against the running service (not mocks of itself).
Exercise error paths and reliability properties deliberately (inject a timeout,
replay a request, kill a dependency). Build the conformance report. A single
unexplained FAIL blocks ship.

## Output Specification

- **`acceptance-examples.feature`** — executable Given/When/Then, one scenario per
  spec clause, tagged with the clause id.
- **`conformance-report.md`** — a table `clause id | spec quote | scenario |
  PASS/FAIL | evidence`, a reliability-properties section
  (errors/retries/idempotency/observability each PASS/FAIL), and a one-line
  ship/no-ship verdict.

## Quality Rubric

- Every spec clause in the inventory has at least one acceptance scenario, and
  every scenario names its clause id (no orphan scenarios, no untested clauses).
- The acceptance suite was demonstrably red before implementation and is green
  after, against the actually-running service (evidence captured both times).
- Each reliability property (error handling, retries, idempotency, observability)
  has a scenario that FAILS when the property is removed — proving the test bites.

## Examples

**Idempotent order creation (clause S4).**
Spec S4: "Creating an order is idempotent over the client's idempotency key."
Scenario: `Given a new idempotency key K / When I POST /orders with K twice /
Then exactly one order exists And both responses return the same order id`.
Run red (server ignores K) → implement key→result store → run green. The retry
scenario (S7: "client retries on 503") now composes safely on top of S4.

**Downstream outage (clause S9).**
Spec S9: "If the payment provider is unavailable, return 503 and do not create a
pending charge." Scenario injects a provider outage, asserts 503 + no charge row
and a logged correlation id. Without the circuit breaker + boundary check, the
scenario fails (leaks a pending charge) — proving the property is real.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Acceptance suite green on first run (never red) | Tests written after/around the code | Rewrite the scenario from the spec clause; confirm it fails on current code first |
| Retry causes duplicate side effects | Retrying a non-idempotent write | Add an idempotency key + key→result store before enabling retries |
| Service "passes" but breaks in prod on bad input | Error-path clauses never made into scenarios | Re-scan the spec for error clauses; add a scenario + typed handler per clause |
| Can't tell which change broke a scenario | Batched multiple clauses per change | Implement one clause per change; run the suite after each |
| Conformance report has unexplained FAIL | Behavior diverges from spec | Treat as ship-blocker; trace clause → code, fix the code OR escalate the spec ambiguity |

## See Also

| I need to… | Use | Reference |
|---|---|---|
| Run the structure self-check | `bash {baseDir}/scripts/validate.sh` (Execute) | — |
| Run this skill under Codex | Read [codex-parity.md](references/codex-parity.md) | references/codex-parity.md |
