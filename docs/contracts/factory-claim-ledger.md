# Factory Claim Ledger Contract

> **Status:** Draft
> **Decision:** Public factory claims must map to explicit evidence posture,
> not prose confidence.
> **Consumers:** product docs, release/profile governance, evidence export,
> comparison freshness, factory-yield proof, operator review

The claim ledger is the proof boundary for AgentOps' software-factory framing.
Every claim-bearing paragraph in the configured source set carries a stable
marker:

```markdown
<!-- agentops:claim:AOP-CLAIM-README-FACTORY-CONTEXT -->
```

The marker links the public source paragraph to a ledger row. The row records
what is true now, what is missing, which issue owns closure, and what wording is
safe until stronger evidence exists.

## Source Set

The strict checker scans at least:

- `README.md`
- `PRODUCT.md`
- `GOALS.md`
- `docs/index.md`
- `docs/agentops-brief.md`
- `docs/assurance-profile.md`
- `docs/software-factory.md`
- `docs/comparisons/**`
- `docs/contracts/factory-*.md`

## Machine-Readable Files

- Schema: [`factory-claim-ledger.schema.json`](factory-claim-ledger.schema.json)
- Current ledger fixture: [`factory-claim-ledger.example.json`](factory-claim-ledger.example.json)
- Validator: `scripts/check-factory-claim-ledger.sh`

## Closed Enums

| Field | Values |
|---|---|
| `validation_level` | `L0`, `L1`, `L2`, `L3` |
| `release_posture` | `roadmap`, `contracted_l0`, `locally_checked_l1`, `integrated_l2`, `pilot_observed_l3`, `advisory_gate`, `blocking_gate`, `release_gate` |
| `evidence_status` | `none`, `planned`, `partial`, `present`, `stale`, `blocked` |
| `authority_state` | `agentops_owned`, `operator_owned`, `external_authority_required`, `not_claimed` |
| `promotion_state` | `not_promoted`, `eligible`, `promoted`, `demoted`, `blocked` |

Unknown enum values fail strict validation. This is intentional: new posture
states require an explicit contract change.

## Ledger Rows

Each row must include:

| Field | Purpose |
|---|---|
| `claim_id` | Stable marker ID found in a source paragraph. |
| `claim_text` | Short restatement of the public claim. |
| `source.file` / `source.marker` | Where the claim appears. |
| `current_evidence` | Evidence that exists today. |
| `missing_proof` | What must close before the claim promotes. |
| `owner_issue` | `bd` issue that owns closure. |
| `validation_level` | L0-L3 evidence level. |
| `closure_gate` | Command or gate that closes the row. |
| `release_posture` | Release/gate posture. |
| `evidence_status` | Whether evidence is absent, partial, present, stale, or blocked. |
| `authority_state` | Who owns authority for the claim. |
| `promotion_state` | Whether the claim can promote. |
| `anti_overclaim_wording` | Safe wording while evidence is incomplete. |
| `evidence_artifacts` | Artifact refs required for L2/L3 rows. |

## Strict Failure Modes

`scripts/check-factory-claim-ledger.sh --strict` fails when:

- a high-claim paragraph contains `validated`, `factory-grade`, `improves`,
  `throughput`, `high-assurance`, or `autonomous` without a nearby claim marker;
- a source marker is missing from the ledger;
- a ledger row points to a marker missing from source;
- any closed enum contains an unknown value;
- an L2/L3 claim lacks an evidence artifact;
- the configured source set is missing a required file; or
- fixture regressions stop failing as expected.

## Anti-Overclaim Policy

- L0/L1 claims stay described as contracted, scaffolded, local, or roadmap.
- Only L2/L3 rows with evidence artifacts may use "validated" as a proof claim.
- No certification, accreditation, classified-network, safety-critical,
  autonomous merge, or generalized throughput claim may promote from this
  ledger alone.
- `.agents/` runtime artifacts are local evidence, not durable public proof,
  until exported through a reviewed evidence bundle.
