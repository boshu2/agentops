---
id: ranked-packet-2026-04-24-go-cli-quality
type: ranked-packet
date: 2026-04-24
goal: "Analyze the Go CLI for usefulness and quality, compare to best-in-class Go CLIs, and plan gap closure."
---

# Ranked Packet: Go CLI Quality Gap Analysis

## Applied Knowledge

1. `.agents/findings/f-2026-04-14-001.md`
   - Applied as a hard planning rule: every production change under `cli/cmd/ao/` must include paired command tests.
   - Why applicable: the plan will change command behavior, output contracts, completion behavior, and live integration tests.

2. `.agents/learnings/2026-04-14-command-refactors-need-paired-tests.md`
   - Applied as the same command/test-pairing constraint, with explicit issue acceptance criteria for direct Cobra tests.

3. `.agents/findings/f-2026-04-14-002.md`
   - Applied as a closeout constraint: child beads must cite durable artifacts and checked-in paths, not ephemeral seed files.

4. `.agents/learnings/2026-04-12-yagni-bridge-not-clone.md`
   - Applied as an architecture boundary: do not rewrite working external or existing internal pipelines merely to make them "more Go"; bridge, wrap, or standardize thin command surfaces first.

## Retrieved, Not Directly Applied

- `.agents/learnings/2026-04-12-tier1-forge-implementation-wins.md`
  - Useful for prompt/LLM output work, but this plan is primarily CLI contract, tests, and command structure work.

## Active Next-Work Overlap

- `.agents/rpi/next-work.jsonl` contains prior CLI-quality backlog for negative-path CLI error formatting, shared test helpers, `os.Chdir`/project-dir migration, command-output contract drift, and external tool bridge hardening.
- The current baseline reproduced a live external-tool bridge failure: `go test ./...` fails in `cli/cmd/ao.TestGCBridgeVersion_Live` because `/usr/bin/gc version` exits 0 while writing `Can't open version` to stderr and no stdout version.

## Planning Constraints

- Prefer measured contract gaps over broad refactors.
- Keep `cli/cmd/ao` commands thin; move reusable logic into `cli/internal/*` packages where feasible.
- Preserve generated `cli/docs/COMMANDS.md` parity via `scripts/generate-cli-reference.sh --check`.
- Preserve existing JSON contract gates and add coverage only where the audit finds gaps.
- Use L0/L1/L2 tests for command-output contracts, external binary bridges, and CLI docs/completion behavior.
