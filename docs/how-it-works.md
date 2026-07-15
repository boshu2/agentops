# How it works

RPI is a thin one-pass coordinator.

```text
Plan once -> Implement once -> Validate once -> Report -> Stop
```

Plan turns intent into testable acceptance and a bounded write scope. Implement
runs one experiment and packages the observed result. Validate computes exact
content identity, checks that changed-path coverage is complete, checks scope and
acceptance, and asks one distinct fresh context for criterion-level judgment.

Validate is the sole verdict writer. It writes canonical JSON to a temporary
file in the destination directory, flushes it, and atomically renames it to:

```text
.agentops/verdicts/sha256/<artifact-digest>.json
```

An identical existing artifact is success. Different content under the same
digest is an integrity failure and produces `NOT_PROVEN`.

Optional strategies may improve inputs. Optional factory adapters may dispatch
explicit disjoint packets once. Neither can add a core phase, alter a verdict,
or choose continuation.
