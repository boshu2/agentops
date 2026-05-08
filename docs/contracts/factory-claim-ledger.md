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
- `docs/trust-factory.md`
- `docs/wiki-for-agents.md`
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

## Operator Workflow

This section describes how a human operator (or a Tier-2 agent acting on the
operator's behalf) interacts with the ledger when authoring docs, updating
evidence, or fixing strict-mode failures.

### Adding a public claim

1. Write the paragraph with the load-bearing claim in a source-set file.
2. Insert a marker line immediately above the paragraph:

   ```markdown
   <!-- agentops:claim:AOP-CLAIM-<DOC-TOKEN>-<TOPIC-TOKEN> -->
   ```

   The `claim_id` is the part after `agentops:claim:`. Keep it stable; the
   marker is the join key between source and ledger.
3. Add a row to `docs/contracts/factory-claim-ledger.example.json` populating
   every field listed in the Ledger Rows table above. `source.marker` must
   match the marker exactly; `source.file` must be a path that the source set
   includes.
4. Pick enum values for the four state-machine fields (see "Choosing enum
   values" below) plus `validation_level`.
5. Run `bash scripts/check-factory-claim-ledger.sh`. Exit 0 means the row is
   wired. Exit non-zero prints which check failed.

### Choosing enum values

Pick the value that describes current state, not aspirational state.

| Field | Value | When it applies |
|---|---|---|
| `validation_level` | `L0` | Claim is contracted but unverified. |
| | `L1` | Verified locally (script passes on operator's box). |
| | `L2` | Verified in CI / integration on shared infrastructure. |
| | `L3` | Verified in pilot/production with a real user signal. |
| `release_posture` | `roadmap` | Not yet built. |
| | `contracted_l0` | Schema/marker exists, no enforcement. |
| | `locally_checked_l1` | Script runs locally; not in CI. |
| | `integrated_l2` | Wired into CI. |
| | `pilot_observed_l3` | Observed in real use. |
| | `advisory_gate` | CI runs the check, failures do not block merge. |
| | `blocking_gate` | CI fails the build on regression. |
| | `release_gate` | Failures block release tagging. |
| `evidence_status` | `none` | No artifact yet. |
| | `planned` | Artifact is in flight (issue exists). |
| | `partial` | Some structural evidence; missing measurement. |
| | `present` | Artifact exists and validates. |
| | `stale` | Was present, now older than its freshness window. |
| | `blocked` | Cannot collect evidence due to dependency. |
| `authority_state` | `agentops_owned` | AgentOps decides truth for this claim. |
| | `operator_owned` | Each operator's own runtime decides. |
| | `external_authority_required` | Needs auditor / vendor / regulator. |
| | `not_claimed` | Documented as out of scope. |
| `promotion_state` | `not_promoted` | Lives in roadmap docs only. |
| | `eligible` | Ready to promote when evidence arrives. |
| | `promoted` | Already in the public surface as a confident claim. |
| | `demoted` | Was promoted, evidence regressed, wording softened. |
| | `blocked` | Cannot promote until upstream gate clears. |

### Updating evidence

When a claim's evidence advances:

- Walk `evidence_status` forward: `planned` → `partial` → `present`.
- If a higher validation level is now reachable (e.g., CI gate added), raise
  `validation_level` (`L1` → `L2`) and update `release_posture` to match
  (`locally_checked_l1` → `integrated_l2` or `advisory_gate`).
- Add artifact references under `evidence_artifacts` with hashes/links per the
  schema. L2/L3 rows must have at least one artifact; the validator enforces
  this.
- If evidence has gone stale (artifact older than its freshness window), flip
  `evidence_status` to `stale` and file a follow-up issue under `owner_issue`.

### What `--strict` failure means

Each strict failure mode maps to a specific operator action:

| Failure | What happened | Action |
|---|---|---|
| high-claim paragraph lacks marker | A trigger term (`validated`, `factory-grade`, `improves`, `throughput`, `high-assurance`, `autonomous`) appears in prose without a marker. | Add a marker + ledger row, OR rephrase to drop the trigger term, OR move the term into a structural element (table/list/header/code-fence) which is exempt. |
| ledger row points to a marker missing from source | A source rewrite removed the marker. | Re-add the marker to a paragraph that still expresses the claim, OR retire the row, OR update `claim_text` and re-anchor. |
| marker missing from ledger | Orphan marker in source. | Add a ledger row, OR remove the marker. |
| closed enum has unknown value | Typo or new posture state. | Fix the typo. New posture states require a contract change (update this file + schema), not a one-off addition. |
| L2/L3 claim lacks evidence artifact | Validation level promoted without backing artifact. | Drop `validation_level` back to L1, OR attach the artifact. |
| configured source set missing a required file | A file in the source set was deleted/moved. | Update `collect_repo_sources()` in `scripts/check-factory-claim-ledger.sh` and the Source Set list above. |
| fixture regressions stop failing | Negative fixtures pass under strict mode; the validator regressed. | Treat as a validator bug; do not silence the fixture. |

### Retiring a claim row

When a public claim is removed (paragraph deleted or rewritten away from the
trigger terms):

1. Delete the matching ledger row.
2. Run `bash scripts/check-factory-claim-ledger.sh` and confirm exit 0.
3. Note the retirement in the commit message so reviewers can confirm the
   public surface no longer carries the claim.

### Worked example

Claim row: `AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE` (added in Wave 1B
follow-up).

**Source paragraph** (`docs/trust-factory.md`, around line 21):

```markdown
<!-- agentops:claim:AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE -->

## The five-step primitive

Every artifact promotion needs five things, in order:

1. **Identity** — what changed?
2. **Reproducibility** — can we replay it?
3. **Evaluation** — what did it pass?
...
```

**Why this `claim_id`:** doc token is `TRUST-FACTORY` (the source file),
topic token is `FIVE-STEP-PRIMITIVE` (the section heading). Stable across
prose edits inside the section.

**Enum values, with one-line justification:**

- `validation_level: L1` — the structural mapping (identity → `runs/`,
  reproducibility → RPI + worktrees, evaluation → councils, evidence →
  citations + ratchets, recovery → rollback + learnings) is verifiable
  locally, but no measured outcome backs it yet.
- `release_posture: advisory_gate` — `scripts/check-factory-claim-ledger.sh`
  runs in CI as an advisory job (Wave 1C), not blocking.
- `evidence_status: partial` — the framing is in the doc; the per-step
  measured outcomes are not.
- `authority_state: agentops_owned` — AgentOps decides what its own factory
  steps are.
- `promotion_state: eligible` — would promote to a confident claim once the
  per-step evidence ledger lands.

**Validator command and expected output:**

```bash
bash scripts/check-factory-claim-ledger.sh
# factory claim ledger: PASS
# exit 0
```

**Anti-overclaim wording in the row:** "The five-step primitive is a framing
borrowed from established trust-factory disciplines; do not imply AgentOps
invented it or has uniquely measured implementation." This is the wording
operators should use in adjacent prose until the L2 evidence lands.
