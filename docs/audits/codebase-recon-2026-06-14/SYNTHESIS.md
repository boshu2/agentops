# Codebase Recon Synthesis — 2026-06-14

> **Read this first.** Executive summary across all six recon reports for the
> release window `v3.1.0..HEAD` (`c98836977` 2026-06-10 → `ab6039808` 2026-06-14).
> **Scope is the diff only** — every report is constrained to what changed in this
> window; unchanged code was not re-audited. Scale: **147 commits · 1,602 files ·
> +37,404 / −109,471** (large net deletion).
>
> Source reports: [audit](codebase-audit.md) · [report](codebase-audit-report.md) ·
> [risk](codebase-audit-risk.md) · [archaeology](codebase-audit-archaeology.md) ·
> [patterns](codebase-audit-patterns.md) · [briefing](codebase-audit-briefing.md).

## Verdict

**Healthy, disciplined consolidation-and-hardening release — ship-quality, no
blocking findings introduced.** All six reports independently converge on the same
picture: this window is *subtraction plus assurance*, not features. A ~104-skill
corpus prune (the entire −109k story) was executed through a deterministic tool
(`ao skills retire`) with a ledger-backed, validator-checked audit trail rather
than a hand-swarm; against it, ~+6.7k Go LOC of genuinely new, fail-closed,
heavily-tested security/verification machinery landed (provenance ledger,
context-quorum rewrite, bounded `ao converge` loop, schema-validated codex
dispatch). Build/vet are clean at HEAD and every new Go surface ships a test twin
(often larger than the impl). The release **removes more risk than it introduces**.
Residual risk is concentrated in one deliberate doctrine change (quorum default
weakened from family-floor → context-floor), some carried-forward migration debt,
and one unwired drift gate.

## Converging findings (multiple reports independently flagged — the signal)

1. **"Prove the gate bites" is the assurance signature of the window** — flagged as
   the single strongest new pattern by [patterns](codebase-audit-patterns.md)
   (Meta-pattern 0), [report](codebase-audit-report.md), [risk](codebase-audit-risk.md),
   and [archaeology](codebase-audit-archaeology.md). A gate is treated as a lie until
   it rejects a planted known-bad AND accepts a planted known-good: the two-sided
   canary in `converge_canary.go`, in-place tamper-verify in
   `provenancegraph/verify.go`, and codex receipt-validates-its-own-contract.
2. **Fail-closed is the house default** — all reports note every new decision path
   (quorum empty-context, converge zero-verdict round, ledger missing-vs-tampered,
   dispatch auth/stdin refusals) defaults to deny/strict; no fail-open paths
   introduced.
3. **The skill prune is the −109k story, and it was tooled not swarmed** — every
   report. `ao skills retire <slug> --into <target>` ripples ~15 hand-maintained
   surfaces in fixed order and flips the dispositions ledger non-lossily (active →
   historical row, never deleted). Corpus ~166 → **73 live skill dirs** (77 declared
   incl. historical rows).
4. **Context-quorum doctrine reversal: family-floor → fresh-context floor** — the
   most architecturally significant code change, flagged by all six. Old rule: ≥2
   distinct model families. New rule: ≥2 distinct non-author contexts; cross-family
   demoted to opt-in `RequireCrossFamily` (default OFF). `CanonicalizeContextID`
   closes cosmetic-forgery of "distinct" contexts. Universally flagged as the top
   judgment item / cross-surface reconciliation risk.
5. **Heavy rebase-reconciliation tax on the hot shared checkout** — every report
   names the `ag-xwjlc` seams epic: ~7–11 of 16 commits are post-rebase splice
   repair / twin re-registration / regen. The dominant *process* smell (not a code
   defect): landing changes through this twin-mirrored, generated-registry repo is
   expensive under concurrent `main`.
6. **The prune left dangling doc/skill references** — [audit](codebase-audit.md) (W),
   [risk](codebase-audit-risk.md) (M-4), [archaeology](codebase-audit-archaeology.md).
   `check-doc-skill-refs.sh` was *created* this window and flags exactly the 7 stale
   refs in the doctrine spine (`docs/architecture/operating-loop.md`) — but it is
   **not wired into CI**, so the drift is live and unguarded.
7. **Prior 2026-06-11 recon P1s tracked to addressed-status** — [report](codebase-audit-report.md)
   and [briefing](codebase-audit-briefing.md): F6 provenance-ledger-missing **RESOLVED**;
   F1 bd/Dolt SPOF **deprecated-not-removed** (tracker now `br`, legacy `.beads/`
   preserved); F3 2,210-line bash pre-push gate **NOT addressed, grew to ~2,252**.

## Top risks (deduplicated across audit + risk reports, with severity)

| Sev | Risk | Source |
|---|---|---|
| **High** | **Quorum default weakened** — two fresh contexts of the *same* model now satisfy the significant-action floor (delete, merge-main). The assertion "context, not model = independence" is in godoc, not validated; contradicts standing `cost-law: quorum at gates` memory. Must confirm every *binding* caller sets `RequireCrossFamily: true`, or the family floor silently vanished at one-way doors. | [risk](codebase-audit-risk.md) H-1, [report](codebase-audit-report.md), [briefing](codebase-audit-briefing.md) §3.1 |
| **Med** | **Doc-spine drift unguarded** — 7 dangling slash-refs in `operating-loop.md` (the declared *primary navigation*); the detector exists but isn't a CI gate. | [audit](codebase-audit.md), [risk](codebase-audit-risk.md) M-4 |
| **Med** | **`cli/cmd/ao` surface concentration worsened** — converge, provenance_verify, skills_retire, codex_schema + codex.go (+1,296) all land in the single `main` package the prior recon flagged (620-file/9.2MB coupling that blocks deletions). Change-risk pools in one namespace. | [report](codebase-audit-report.md) Design-warn |
| **Med** | **codex dispatch runs packet commands via `sh -c`** — acceptable while packets are operator-trusted local artifacts; becomes arbitrary command execution if packet provenance ever widens (network handoff, PR, `.agents/mto-handoff`). Document the trust boundary at the call site. | [risk](codebase-audit-risk.md) M-1 |
| **Med** | **Provenance chain is tamper-*evident*, not tamper-*proof*** (unkeyed SHA-256). A writer with file access can recompute a clean chain. Git history is the real anchor — do not market as tamper-proof. | [risk](codebase-audit-risk.md) M-2 |
| **Med** | **`ao skills retire` mutates ~6 correlated surfaces; ordering is load-bearing** — a regen failure mid-run can leave the ledger flipped but inventories stale. Confirm transactionality / clear partial-failure reporting + post-retire `regen-all --check`. | [risk](codebase-audit-risk.md) M-3 |
| **Med→P2** | **Carried-forward migration debt** — 16 `bd`-named scripts still invoke retired `bd` verbs; legacy `.beads/` Dolt config preserved byte-for-byte (SPOF deprecated, physically present). | [audit](codebase-audit.md), [report](codebase-audit-report.md) |
| **Low** | Stray `wt-ag-*` worktrees in repo root (clutter + wrong-root edit risk); 5,669-line generated `registry.json` diff is review-opaque (trust-the-generator); env-sourced context IDs feed the quorum axis (fail-closed if unset); possible reference loss in folds without a named rescue. | [archaeology](codebase-audit-archaeology.md) P2/P3, [risk](codebase-audit-risk.md) L-1..L-4 |
| **Watch** | Unreleased state — substantial breaking + doctrinal change on `main` with **no CHANGELOG entry / no 3.2 tag**; family→context flip not yet reconciled in cross-repo consumers (olympusd, fleet memory). | [briefing](codebase-audit-briefing.md) §7, [report](codebase-audit-report.md) |

## Strongest reusable patterns + architecture facts worth remembering

- **Prove-the-gate-bites** (two-sided canary): plant known-bad → assert reject, plant
  known-good → assert accept, before trusting any PASS. Now load-bearing Go, not
  advisory. [patterns](codebase-audit-patterns.md) Meta-0.
- **Fail-closed validation ordered before any side effect** — validate raw input
  against an explicit contract, reject before any irreversible action, never let
  input relax its own guard (`OPENAI_API_KEY` always forbidden regardless of packet
  self-guards). [patterns](codebase-audit-patterns.md) P1.
- **Schema-embedded-in-binary + on-disk parity twin** — validator ships inside the Go
  binary (`cli/embedded/schemas/`) with a parity test against the human-editable SOT
  in `schemas/`. Emerging convention (only the codex pair so far). [patterns](codebase-audit-patterns.md) P2.
- **Teardown via single-writer disposition ledger, not direct `rm`** — destructive
  corpus change expressed as keep/fold/cut data rows, executed by one rippling tool.
  [patterns](codebase-audit-patterns.md) P3.
- **Convention → Gate → bats, in the same arc** — no convention lands without a
  mechanical gate and a bats proof the gate behaves. [patterns](codebase-audit-patterns.md) P4.
- **Test-first Go** — 11 new `_test.go`, test-to-impl ratio scales with blast radius
  (provenance/skills_retire tests ≥ their impls). [patterns](codebase-audit-patterns.md) P5.
- **Architecture facts:** corpus = the product surface (cull = count); 6 Bounded
  Contexts (BC6 Orchestration added this window); workflows are now first-class
  drift-gated registry entries (Claude-only, no Codex twin); tracker = `br` at
  `_beads/` (private nested repo); provenance ledger at `docs/provenance/ledger.jsonl`
  now real + tamper-gated; north star reframed from *scope* ("control plane for
  everything") to *destination* ("navigator/GPS for stochastic agentic work");
  AgentOps standalone, Mount Olympus an optional one-way extension.

## Prioritized actions (each cites its source report)

1. **Grep every binding significant-action caller and confirm `RequireCrossFamily: true`** (or consciously accept same-model fresh-context quorum). The gate code is correct; the *policy default* is the exposure. — [risk](codebase-audit-risk.md) H-1.
2. **Wire `check-doc-skill-refs.sh` into the blocking gate set** and repoint/retire the 7 dangling refs in `operating-loop.md` (e.g. `/vibe`→`/validate`). The detector exists; make it bite. — [audit](codebase-audit.md), [risk](codebase-audit-risk.md) M-4.
3. **Reconcile the family→context flip in cross-repo surfaces** (olympusd, other-repo briefs, fleet memory still say "≥2 families") so no orchestrator mis-states the gate. — [report](codebase-audit-report.md), [briefing](codebase-audit-briefing.md) §3.1.
4. **Document the `sh -c` trust boundary at `codex.go:1674`**; switch to argv exec / allowlist if packet provenance ever widens beyond operator-trusted local artifacts. — [risk](codebase-audit-risk.md) M-1.
5. **Confirm `ao skills retire` is transactional / reports partial-failure clearly, and that `regen-all --check` runs post-retire in the gate.** — [risk](codebase-audit-risk.md) M-3.
6. **Finish the bd→br migration**: rewrite/retire the 16 `bd`-named scripts and physically retire legacy `.beads/` Dolt config so the deprecated SPOF isn't carried in-tree. — [audit](codebase-audit.md), [report](codebase-audit-report.md) F1.
7. **Peel cohesive new lanes (converge, provenance, codex-dispatch) out of `cli/cmd/ao` `main`** into their own (sub)packages as they stabilize, to stop change-risk pooling. — [report](codebase-audit-report.md) Design-warn.
8. **Prune merged `wt-ag-*` worktrees** (`git worktree prune` + remove stale dirs) and confirm `regen-all.sh --check` is blocking so no generated file is ever hand-touched. — [archaeology](codebase-audit-archaeology.md) P2.
9. **Add a CHANGELOG/version entry (tag 3.2) before anyone installs from `main`** — substantial breaking + doctrinal change is currently unversioned. — [briefing](codebase-audit-briefing.md) §7.
10. **Spot-check 2–3 high-value folded skills** (e.g. `codebase-risk-audit` → `review`) to confirm method survived, not just the trigger phrase; and consider making twin-generation + registry regen a single idempotent pre-push command so rebase never leaves spliced markers. — [archaeology](codebase-audit-archaeology.md) P1/P3.

## Missing reports note

**None missing — all six expected reports are present and substantial** (8–15k each,
fully written, not stub-thin): `codebase-audit.md`, `codebase-audit-archaeology.md`,
`codebase-audit-briefing.md`, `codebase-audit-patterns.md`, `codebase-audit-report.md`,
`codebase-audit-risk.md`. (Note: an earlier SYNTHESIS revision recorded an empty
directory from a premature synth pass before the workers landed; this revision
supersedes it — the reports did land.)
