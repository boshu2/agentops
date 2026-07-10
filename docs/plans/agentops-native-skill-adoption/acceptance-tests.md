# Acceptance-test receipt

The 21 frozen Given/When/Then scenarios in `behaviors.md` map one-for-one to the
first 21 tests in `tests/scripts/agentops-native-skills.bats`. Test 22 is the
clean-room/provenance gate that applies to the whole adoption arc.

## First red run

Command:

```bash
bats tests/scripts/agentops-native-skills.bats
```

Observed 2026-07-09 after the second independent seam review: exit `1`, `1..22`, all
22 tests failed for the intended unsatisfied semantics. The artifact validators,
real NTM worker adapter, Agent Mail port/adapter, supervisor, review-lane port/adapter, skill-mesh gates,
dependency graph, clean-room gate, and migrated skill sources do not exist yet.
The GC test now runs eight named `CANONICAL` cases against the real pack
finalizer; it fails on the known schema-invalid fields, raw lane verdicts,
uncontained/nonexistent evidence, and fabricated DEGRADED pawl verdicts.

No command-not-found result is treated as an application refutation: shell
acceptance tests assert required executables before invoking them, while Go
acceptance tests use `go -C <repo>/cli test ./internal/...` and fail at the
intended compile/contract resolution inside the real module. The prior
root-level `go test ./cli/...` mistake and vacuous GC filter (zero matching
tests) were removed before this receipt was accepted.

## Scenario mapping

| Scenario | Executable acceptance |
|---|---|
| B1.1 | `idea-genie produces an evidence-grounded idea-portfolio artifact` |
| B1.2 | `idea-genie can return no-new-work without manufacturing candidates` |
| B2.1 | `dueling-idea-genies emits a sealed challenge packet for plan-pawl` |
| B2.2 | `dueling-idea-genies routes reversible choices without NTM ceremony` |
| B3.1 | `codebase-recon validates evidence-bounded fact inference unknown claims` |
| B3.2 | `codebase-recon requires a verified delta when a prior pack exists` |
| B4.1 | `pattern-mining promotes only a three-exemplar holdout-proven pattern` |
| B4.2 | `pattern-mining keeps weak evidence as a hypothesis` |
| B5.1 | `NTM AgentWorker executes the real robot spawn send observe lifecycle` |
| B5.2 | `agent lifecycle uses suspect then bounded nudge then replacement` |
| B6.1 | `pawl-review returns a fresh read-only nonce-bound NTM lane result` |
| B6.2 | `review transport loss cannot become semantic REFUTED or CONFIRMED` |
| B7.1 | `using-gc exposes optional worker and review-lane composition` |
| B7.2 | `GC real finalizer emits canonical verdicts with contained nonempty evidence` |
| B7.3 | `GC and NTM remain independently selectable adapters` |
| B8.1 | `ATM-era callers migrate to agent-native and pawl-review` |
| B8.2 | `canonical skills keep NTM and Agent Mail as external adapters` |
| B9.1 | `every admitted new capability is reachable from an existing entry point` |
| B9.2 | `entry points delegate without copying leaf workflows` |
| B10.1 | `existing catalog context-map and ao graph regenerate every live skill` |
| B10.2 | `graph topology rejects duplicates dangling cycles and unreachable non-roots` |
| Arc | `clean-room gate rejects planted copied text and validates captured manifests` |

## Green proof required

The same 22-test command must pass unchanged after implementation, followed by
the complete pack finalizer suite, focused Go tests for orchestration and graph
code, graph/catalog/context-map regeneration and check mode, skill/schema/parity
audits, and the normal fast gate.
