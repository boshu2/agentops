# Go CLI G0–G2 semantic validation intent

Date: 2026-07-24

Author context: `codex-root-go-g0-g2-author-20260724`

Candidate commits:

- G0: `2e1f32a2d50bf94733fe0de935704010da92cfba`
- G1: `7d74854d8b7539a2a6c81a9fd1dc7dd432fe32aa`
- G2: `bc711ea6fccc2173acfb8f7c8109c5263a3fd43a`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Intent

Independently judge the already-integrated Go CLI containment, effect/output,
and subprocess-lifecycle program as one sibling release candidate. The
candidate subject is the current exact content of every path changed by the
three commits above; interleaved proof-kernel history is not part of this
candidate.

## Acceptance

### GO-1 — Identifier containment

Every eval-controlled identifier used in filesystem construction is validated
and canonicalized before joining an owned root. Absolute paths, traversal,
separator aliases, dot spellings, platform-specific volume forms, control
characters, and post-normalization escape attempts fail before mutation.
Hostile tests prove that no read, write, rename, cleanup, or artifact path can
escape the owned root.

### GO-2 — Owned temporary state

Automatically created runtime-isolation directories have explicit ownership
and lifecycle semantics. They are reused only when identity matches and are
cleaned on success, error, timeout, and cancellation as declared. Caller-owned
directories are preserved. Tests distinguish both cases and prove no silent
leak in the automatic path.

### GO-3 — Honest command effects

Effectful commands expose one shared operation/effect vocabulary. Global
`--dry-run` either suppresses every declared mutation or is rejected for an
unsupported operation before mutation. The command contract and implementation
agree on filesystem, process, host, credential, and external effects; dry-run
tests exercise the effective root command rather than only leaf helpers.

### GO-4 — Structured output equivalence

For read commands that advertise both spellings, `--json` and `-o json`
produce the same schema and semantic payload. Effectful commands use the same
structured operation-result vocabulary. Human output remains human-readable,
machine output contains no human preamble, and the stale eval help reference
is absent from generated command documentation.

### GO-5 — Bounded subprocess output

Every migrated subprocess path captures or streams stdout and stderr under an
enforced hard bound while the process is running. A child cannot force
unbounded memory growth before truncation. Results disclose truncation, byte
counts, exit state, and relevant cleanup facts.

### GO-6 — Cancellation, timeout, and descendant cleanup

Caller cancellation and deadlines propagate through eval, gates, goals, and
other migrated execution paths. On timeout, cancellation, abnormal exit, or
parent error, the runner terminates the process tree, waits under a bound, and
reports cleanup outcome. POSIX behavioral tests cover descendant cleanup;
Windows-specific code must at least compile and have deterministic unit
coverage where the current host cannot execute it.

### GO-7 — Compatibility and regression safety

Existing successful command behavior and output contracts remain compatible
unless the intent explicitly tightens unsafe behavior. Focused hostile tests,
full `go test ./...`, race tests, `go vet ./...`, the pinned lint wrapper,
generated command-reference parity, and Windows cross-compilation pass.

### GO-8 — Exact independent verdict

A fresh author-distinct validator inspects the exact subject, maps all eight
criteria to replayed evidence, records checked and not-checked surfaces, and
writes one durable binding verdict under the active epoch-1 proof contract.
Platform behavior that was only cross-compiled remains a stated residual risk
and is not described as behaviorally executed.

## Non-goals

- Do not expand into unrelated CLI product changes or the T2 catalog reader.
- Do not change skill semantics, proof-kernel behavior, release policy, Git
  history, installed binaries, or external systems.
- Do not claim Windows behavioral process-tree proof from a macOS run.
- Do not repair a failed semantic result inside validation.

## First useful checks

```bash
cd cli
go test ./internal/evalsubstrate/... ./internal/clicontract/... ./internal/subprocess/...
go test ./...
go test -race ./...
go vet ./...
```
