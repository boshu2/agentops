# Corpus learning seam — the field-level public/private boundary

> **SUPERSEDED CLI SURFACE:** The corpus commands cited below were retired; the
> field-boundary discussion remains historical design context.

> **Status:** contract · **Epic:** ag-k7tq9 (corpus private/public separation) · **Slice:** S3 (ag-2srq1)
> **Authority:** the unanimous cross-family council verdict
> (private: `.agents/council/2026-06-15-corpus-private-public-seam-verdict.md`, not tracked in this public repo).
> **Schema:** [`schemas/learning.v1.schema.json`](../../schemas/learning.v1.schema.json).
> **Migration:** `ao corpus classify` (`cli/internal/corpus/classify.go`).

The corpus (`boshu2/agentops-corpus`, mounted at `.agents/learnings/`) is **lossless
and private by default**. Evidence, provenance, and source pointers live there
forever. Sensitivity is a **publish property, not a capture property**: nothing is
decided at mine time. The seam is enforced at **promote time**, **field-level**,
with the source session's sensitivity as a **ceiling**.

This contract defines which fields of a learning record may cross that seam.

## The two promote-gate fields

Every learning record carries two frontmatter fields that gate promotion. Both
default to the most conservative value (`ao corpus classify` backfills them):

| Field | Type | Default | Meaning |
|---|---|---|---|
| `sensitivity` | `unknown` \| `private` \| `public` | `unknown` | The ceiling. `unknown` = un-triaged; `private` = tainted, never publishes; `public` = abstracted, cleared for the wiki. |
| `publishable` | boolean | `false` | The allowlist flag. `true` only after the lesson is abstracted **and** passes the leak scanner. |

**Promote rule (allowlist, fail-closed):** a record may be promoted to `docs/wiki/`
only when `sensitivity == "public"` **AND** `publishable == true`. Default excludes.
Inclusion is earned, never assumed. A single fat-finger cannot publish the corpus.

## The field boundary — what crosses, what never does

| Field(s) | Class | Crosses the seam? |
|---|---|---|
| the abstracted **lesson** body (the markdown after the frontmatter, once generalized) | publishable | **yes** — and only this |
| `source_session` | private provenance | **never** |
| `source_bead` / bead ids | private provenance | **never** |
| evidence paths (`Evidence:` refs, file:line citations into private repos) | private evidence | **never** |
| any fleet / client / peer-agent / private-namespace / mythology / brand reference | private (leak markers) | **never** — hard-fail in the scanner |

Only the **lesson** crosses. `evidence` / `provenance` / `source_session` stay in the
private corpus — which is also what keeps the provenance graph private (dovetails with
mesh ag-5qltf). Mine-time labels (`tier`, `maturity`, `category`) are **triage only**,
**not** a declassification boundary.

## Defense in depth (all fail-closed)

1. **Physical separation** — the corpus is a separate private repo; no
   `.gitignore`-negation single point of failure.
2. **Allowlist, not blocklist** — `ao corpus publish` (S6) emits only records that
   pass the promote rule above. Default = exclude.
3. **Leak scan on rendered output** — `ao corpus scan` / `cli/internal/corpusscan`
   (S4): any fleet/client/peer/PII/brand/landmine marker hit = hard FAIL, **never
   auto-redact**.
4. **Pre-push guard + CI re-scan** (S1/S7) — reject staged raw corpus paths; re-scan
   `wiki/**` on push, fail closed.

## Migration

`ao corpus classify <dir>` backfills the two defaults onto every learning record,
**malformed-tolerant** (operates on the `---` frontmatter fence textually; never parses
the YAML body, so one junk record cannot abort the run) and **idempotent** (an existing
real decision is never overwritten). Meta docs (`CORPUS-POLICY.md`, `README.md`, …) are
skipped. Dry-run by default; `--apply` writes.

```bash
ao corpus classify .agents/learnings            # dry run — report only
ao corpus classify .agents/learnings --apply    # write the safe defaults
```
