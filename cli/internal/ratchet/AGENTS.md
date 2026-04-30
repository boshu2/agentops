---
package: cli/internal/ratchet
status: active
owner: agentopsd
contract_source: cli/internal/ratchet/ratchet.go (Step enum, AllSteps)
---

# cli/internal/ratchet

Brownian Ratchet workflow tracking — a tool-agnostic gating mechanism for the RPI workflow that records progress and refuses to slide backward.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Step taxonomy:** seven canonical steps in workflow order — `research`, `pre-mortem`, `plan`, `implement`, `crank`, `vibe`, `post-mortem`. Aliases are tolerated (see `stepAliases`) but the seven canonical names are the contract.
- **Skill surface:** consumed by `skills/ratchet/SKILL.md` (check / record / verify gates). Hooks call into this package for "did the workflow actually advance?" decisions.

## Interfaces

- **Public API:**
  - `ratchet.go` — `Step`, `AllSteps()`, alias resolution.
  - `chain.go` — append-only chain of recorded steps with monotonicity guarantees.
  - `gate.go` — gate evaluation: "is this step allowed given the current chain?".
  - `validate.go` — schema/structural validation of recorded entries.
  - `maturity.go` — maturity scoring derived from chain history.
  - `location.go` — locates chain files on disk.
  - `skill_drafts.go` — skill draft tracking integrated with the chain.
- **File locking:** `filelock_unix.go` and `filelock_windows.go` are platform-gated by build tags; the unix path uses `flock`, Windows uses LockFileEx semantics.
- **Fuzz coverage:** `fuzz_test.go` fuzzes the ratchet entry parser — keep it green when changing entry shapes.

## Non-obvious rules

- **The ratchet only moves forward.** Recording a step that isn't a valid successor (per `gate.go`) must fail loudly. No silent success.
- **Aliases are convenience, not contract.** `stepAliases` accepts `premortem`, `postmortem`, `pre_mortem`, `post_mortem`, plus semantic aliases (`formulate` → `plan`, `autopilot`/`execute` → `crank`, `validate` → `vibe`, `review` → `post-mortem`). New aliases require care — they collapse semantics.
- **File lock is mandatory for writes.** Multiple `ao` invocations can race on the same chain file; the `filelock_*.go` shims must be used for any chain mutation. Don't bypass them in tests either — use `t.TempDir()` for isolation.
- **Maturity is derived, not stored.** `maturity.go` computes scores from the chain on demand. Don't introduce a cached maturity field without invalidation rules.
- **Cross-platform parity matters.** Build-tagged files (`filelock_unix.go`, `filelock_windows.go`) must keep behavioral parity. Windows CI catches drift in the `windows-smoke` job.

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Skill: `skills/ratchet/SKILL.md`.
- Pattern source: olympus per-folder `AGENTS.md` ownership convention.
- Sibling packages: `cli/internal/rpi` (workflow driver consuming ratchet gates), `cli/internal/types` (shared error/result types).
