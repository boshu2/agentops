# Runtime Neutrality Contract

Status: active · Owner: this document (moved from the former `shared` skill
2026-07-29, bead `age-skill-overhaul-reboot-sjv7v.11`; that skill was deleted
2026-09-03)

The rules below govern any shared reference a consuming skill loads — the
same scope the former `shared` skill declared. They are corpus doctrine, not skill
behavior, so they live here — a declared contract owner — instead of inside
a skill that bundles nothing.

## The contract

- Default to the current agent and local shell.
- Use a runtime-native fresh context only when the caller or consuming
  workflow requests it.
- Treat runtime and factory state as adapter evidence; never translate it
  into core Plan, Candidate, RPI, or verdict state.
- Missing optional tools degrade only the optional capability that needs
  them.
- Source skill contracts and executable behavior outrank shared prose.

Shared context is context, not permission: reading a shared reference never
authorizes starting a runtime, tracker, substrate, network call, or external
mutation. Authority comes from the caller or the consuming skill's contract.

Named failure mode — **reference promotion**: shared prose quietly outranking
a source skill contract because it was read more recently. Just-in-time
loading is the counter-discipline: a reference read only when needed cannot
silently become a dependency, while anything loaded by default eventually
gets treated as one.

The core loop has no hard dependency on this document.
