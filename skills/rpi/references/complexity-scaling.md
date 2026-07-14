# Complexity Scaling

Risk classification determines validation depth; inventory size does not create
ceremony by itself.

## Classification Table

| Level | Risk shape | Review depth |
|-------|------------|--------------|
| **routine** | Reversible, one owner, bounded rollback | one quick fresh Premortem + one quick fresh Validate |
| **elevated** | Cross-boundary behavior with concrete named risk | one fresh judge with the affected specialist claims |
| **high** | Irreversible, safety/security critical, or genuinely contested | explicit deep, mixed, or council review |

> `--quick` still means author != judge. Multi-judge consensus scales with
> stakes and disagreement, not file, issue, or wave count.

## Detection

Classify after planning from reversibility, blast radius, authority, safety,
security, and decision contestability. Record the named trigger for any depth
above routine.

## Flag Precedence (explicit always wins)

| Flag | Effect |
|------|--------|
| `--fast-path` | Narrows claims and deterministic scope; does not waive fresh judges |
| `--deep` (passed to /rpi) | Requests high-depth review |
| No flag | Use the named-risk classification above |

Premortem and Validate independence apply at every depth. Learn and the
orchestrator transition do not change with depth.
