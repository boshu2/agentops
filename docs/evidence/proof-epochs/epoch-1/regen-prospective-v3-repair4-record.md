---
record_type: prospective-packet-record
proof_epoch: 1
status: frozen
experiment: p1-regen-prospective-v3-repair4
author_context_id: claude-opus5:session_013fqQZAMZVFswA3oFrCDAFf:p1-regen-prospective-v3-repair4-author
validator_context_id: null
invocation_id: root:regen-repair4:4bd9f73a-8de1-4e8a-8b77-65ee532658e4
base_commit: e6fdd51e51e3d3e449e938289f20d0df72c27705
base_tree: 262dbf3215dba9a3d747ce577b5f958f0593129c
---

# P1 regen prospective v3 — repair4 packet record

## Identity

| Field | Value |
|---|---|
| invocation_id | `root:regen-repair4:4bd9f73a-8de1-4e8a-8b77-65ee532658e4` (caller-supplied, bound verbatim) |
| author_context_id | `claude-opus5:session_013fqQZAMZVFswA3oFrCDAFf:p1-regen-prospective-v3-repair4-author` |
| validator_context_id | `null` — a fresh, author-distinct validator must supply its own |
| judgment_id | none — minting one here would be self-validation |

The invocation ID is used **verbatim and identically** everywhere it appears. No
second invocation identity exists anywhere in the packet.

## Packet location

Every runtime artifact lives under kernel-excluded paths:

```text
.agents/ao/intents/<intent-digest>.intent
.agents/ao/reports/subject-manifest-before.json
.agents/ao/reports/scope-index.json
.agents/ao/reports/subject-manifest-final.json
.agents/ao/reports/check-receipts/*.json
.agents/ao/reports/effect-receipt.json
.agents/ao/reports/invocation.json
.agents/ao/verdicts/                      (empty — none minted)
```

`.agents/ao/intents`, `.agents/ao/reports`, `.agents/ao/verdicts` and `.git` are
exactly `COMPLETE_RUNTIME_EXCLUSIONS`, so the packet is never part of the
observed subject and no tracked packet copy exists.

## Observation policy

Identical on both manifests, and the only policy under which `COMPLETE` coverage
is achievable:

```json
{"observation_roots": [{"id": "repository", "includes": ["."]}],
 "exclusions": [".agents/ao/intents", ".agents/ao/reports",
                ".agents/ao/verdicts", ".git"]}
```

## The repair4 constraint

Every `check-receipt.v1` referenced by the effect receipt binds the **FINAL**
subject manifest digest. Receipts are built only after the final manifest is
frozen, which is only possible after the last tracked mutation. Repair3's
receipts carried the BEFORE digest; a receipt naming a manifest that predates
the change it attests cannot evidence that the check ran against the subject
actually produced.

## Frozen criteria (not self-judged)

R4-MODE-01, R4-DIRECT-01, R4-126-01, R4-CONTENT-01, R4-RECEIPT-FINAL-01,
R4-COMPLETE-01, R4-SCOPE-01 — stated in the intent record and left for a fresh
validator.

## Non-claims

- No `verdict.v3` is minted and no criterion is self-judged.
- No merge, push, projection regeneration, or release.
