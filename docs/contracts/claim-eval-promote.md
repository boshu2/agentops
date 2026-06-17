# Claim-Eval-Promote (CEP) Policy Overlay

> Policy for managing claims in AgentOps public-facing documentation.
> Composes with `ClaimEvidencePort` (`cli/internal/ports/claim_evidence.go`),
> `ao claim bind` (`cli/cmd/ao/claim_cmd.go`), and the gate registry
> (`cli/internal/gates/checks/claim_registry.go`).
>
> Duel consensus: `.agents/duel/DUELING_WIZARDS_CEP_REPORT.md`

## Claim Lifecycle

A **claim** is a testable assertion in a public-facing doc (`PRODUCT.md`,
`README.md`, `GOALS.md`, `docs/**`) marked with an HTML comment:

```html
<!-- agentops:claim:AOP-CLAIM-<SCOPE>-<NAME> -->
```

Every claim has a **tier** that governs where it may be cited.

## Tier Model

| Tier | Meaning | Allowed Surfaces |
|------|---------|-----------------|
| **UNPROVEN** | Claim exists but has no evidence on main. | `docs/comparisons/**`, `docs/wiki-for-agents.md` |
| **PILOT** | At least one tracked evidence artifact on main. | `docs/**`, `GOALS.md` |
| **NULL** | Tested; null hypothesis not rejected. | `docs/evals/**` |
| **PROVEN** | Passing eval binding + tracked evidence. | All surfaces |

Tiers are defined in `docs/contracts/claim-registry.yaml` (schema:
`schemas/claim-registry.v1.schema.json`).

## Registry

`docs/contracts/claim-registry.yaml` is the single source of truth for claim
policy. It records each claim's tier, surfaces, owner, evidence paths, and
eval binding.

### Additive Regen

`scripts/regen-claim-registry.sh` scans `agentops:claim:*` markers in tracked
files and creates UNPROVEN stubs for any claim ID not yet in the registry. It
**never** overwrites curated fields (tier, evidence, owner, eval_binding,
notes).

### Drift Gate

`ao gate check` runs `claim.registry-drift` (native Go, Fast + Full tier):
every marker must have a registry entry, and every registry entry must have at
least one marker in source. Drift fails the gate.

### PMF Evidence Gate (WARN-only)

`ao gate check` runs `claim.pmf-evidence` (shell-backed, Fast + Full tier,
**non-blocking**): flags public docs that cite `.agents/`-only paths as
evidence. Fix: promote via `scripts/export-evidence.sh`, then cite the tracked
`docs/evidence/<bead>/` path.

## Promotion Path

1. **Marker → Registry stub:** regen creates an UNPROVEN entry.
2. **Evidence binding:** `ao claim bind --claim <ID> --path <evidence> --level PG2`
   records the binding in `.agents/findings/evidence-bindings.jsonl`.
3. **Tier promotion:** when evidence is tracked on main and (optionally) an eval
   passes, update the tier in `claim-registry.yaml` from UNPROVEN → PILOT →
   PROVEN. Only curators (or `ao claim promote` when wired) change tiers.
4. **Citation ceiling:** the tier-citation ceiling gate (future slice) will block
   when a public surface cites a claim above its allowed tier.

## Composition

This policy overlay sits above the existing `ClaimEvidencePort` and
`ao claim bind` surface. The port handles evidence binding mechanics; this
policy governs **which claims exist, at what tier, and where they may be cited**.

```
markers (source) ──regen──▶ claim-registry.yaml (policy)
                                    │
ao claim bind ─────────────▶ evidence-bindings.jsonl (runtime)
                                    │
ao gate check ◀────────────── drift + PMF evidence checks
```

## Gates Registered

| Gate ID | Type | Tier | Blocking | Artifact |
|---------|------|------|----------|----------|
| `claim.registry-drift` | Native Go | Fast + Full | Yes | `cli/internal/gates/checks/claim_registry.go` |
| `claim.pmf-evidence` | Shell | Fast + Full | **No** (WARN) | `scripts/check-pmf-evidence.sh` |
