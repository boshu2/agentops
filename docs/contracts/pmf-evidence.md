# Contract: PMF Evidence Gate

> **Status:** active
> **Bead:** soc-m6v5.8
> **Ships in:** v3.0.0 (per `.agents/plans/2026-05-20-ultimate-dogfood-3.0.md` D5)
> **Catalogued from:** `docs/documentation-index.md`

## The rule

Public docs (PRODUCT.md, README.md, `docs/launch/*.md`) MUST NOT back a measurable claim — PMF, productivity, time-to-first-validated-flow, council win rates, ablation deltas — by citing **only** a `.agents/` path. `.agents/` is gitignored. A reader of the public doc has no way to reach the evidence.

To cite a `.agents/` artifact from a public doc:

1. Promote it once via `scripts/export-evidence.sh <bead-id> <source-path> [<dest-name>]`.
2. The promoter copies the artifact to `docs/evidence/<bead-id>/<dest-name>` and appends a provenance footer:
   ```
   <!--
   provenance:
     source-path: .agents/research/foo.md
     source-sha256: <64-char hex>
     promoted-at: <UTC ISO-8601>
     bead-id: soc-xxx
     promoter: scripts/export-evidence.sh
   -->
   ```
3. Cite the **tracked** `docs/evidence/...` path in your public doc. You can mention the `.agents/` path too (for transparency), but the tracked path is the load-bearing reference.

## Why we don't just commit `.agents/`

`.agents/` exists to capture session-scratch state, drafts, agent ledgers, intermediate research, and per-cycle telemetry that would balloon the repo if all committed. The promotion step is the explicit "this artifact is durable enough to ship publicly" decision, and the provenance footer makes "is this still the same evidence?" mechanically checkable.

## The gate

`scripts/check-pmf-evidence.sh` walks the canonical public-doc set (PRODUCT.md, README.md, `docs/launch/*.md`) and flags any line that names a `.agents/` path **without** the same file also referencing a `docs/evidence/` promotion. Operator can opt-out per-line by wrapping with `<!-- internal -->` (e.g. a footnote that's documenting the rule itself).

Exit codes: `0` clean, `1` violations found, `2` usage, `3` target unreadable. Designed for CI advisory use initially; can be hardened to T0-required after one week of clean runs.

## Drift detection

If `export-evidence.sh` is re-run with the same `bead-id`/`dest-name` but the source SHA-256 has changed since the original promotion, it refuses to overwrite and exits 1 with a `DRIFT` message. The operator must delete the existing `docs/evidence/...` file and re-promote intentionally. This prevents silent evidence swaps.

## Round-trip example

```bash
# 1. Research writes evidence to .agents/
$ /research "ablation experiment" > .agents/research/2026-06-01-ablation.md

# 2. Decide it's release-worthy → promote
$ scripts/export-evidence.sh soc-vuu6.33 .agents/research/2026-06-01-ablation.md
export-evidence: wrote docs/evidence/soc-vuu6.33/2026-06-01-ablation.md
  source: .agents/research/2026-06-01-ablation.md
  bead: soc-vuu6.33
  sha256: <hash>

# 3. PRODUCT.md cites the tracked path
$ grep -n 'evidence' PRODUCT.md
...
PMF wedge: see docs/evidence/soc-vuu6.33/2026-06-01-ablation.md (also .agents/research/2026-06-01-ablation.md for raw notes)

# 4. The gate is happy
$ scripts/check-pmf-evidence.sh PRODUCT.md
check-pmf-evidence: OK — PRODUCT.md clean
```

## Acceptance criteria

```yaml
acceptance_criteria:
  - id: ac-m6v5.8.1
    description: "PMF evidence gate refuses public claims citing ONLY .agents/ paths that have not been promoted"
    check_type: test_pass
    check_command: "bats tests/scripts/check-pmf-evidence.bats"
    evidence_path: "tests/scripts/check-pmf-evidence.bats"
    evidence_required: true
    weight: 0.6
  - id: ac-m6v5.8.2
    description: "export-evidence.sh promotes a .agents/ artifact to docs/evidence/<bead-id>/ with a provenance footer"
    check_type: test_pass
    check_command: "bats tests/scripts/export-evidence.bats"
    evidence_path: "tests/scripts/export-evidence.bats"
    evidence_required: true
    weight: 0.4
```

## Non-goals

- Does NOT validate the truth of the claim itself (that's `/council` work).
- Does NOT scan internal `.agents/`-prefixed docs — they are allowed to cite each other freely.
- Does NOT enforce that every measurable claim has evidence — just that *if* a claim cites `.agents/`, the same file must also reference a tracked promotion.

## See also

- `.agents/plans/2026-05-20-ultimate-dogfood-3.0.md` — the 3.0 release plan that prescribed this gate
- `.agents/council/2026-05-20-pre-mortem-ultimate-dogfood-3.0.md` — pre-mortem HIGH-1 surfaced the `.agents/`-gitignored contradiction this gate resolves
- `scripts/export-evidence.sh` — the promoter
- `scripts/check-pmf-evidence.sh` — the gate
