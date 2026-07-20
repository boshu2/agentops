# Workflows

Reusable orchestration conveyors for the Claude Code Workflow tool. Workflows
are a **Claude-only runtime adapter** — the same doctrine as `skills-codex/`
(Codex-only): canonical source lives here, and a runtime link step installs it
where the one runtime that consumes it resolves names.

Three generic conveyor shapes:

| Workflow | Shape | Use when |
|---|---|---|
| `audit-dimensions` | pipeline: finder → skeptic, per dimension | auditing a subject across independent lenses |
| `verify-fixes` | parallel adversarial verifiers, one per group | refuting "it's fixed" claims after a change |
| `implement-wave` | parallel disjoint-scope lanes → one fresh verifier | executing a wave of bead-shaped work items |
| `rpi` | pipeline: plan → implement → fresh validate | one intent through the core loop to a durable verdict |

Four repo-doctrine conveyors also live here: `bdd-foundry` (behavior-first
planning → acceptance-gated beads), `operating-loop` (one capability through
the seven-move loop end to end), `ship-beads` (drive a list of beads to
confirmed-merged), and `bead-crank` (deprecated alias delegating to
`ship-beads`). Each documents itself in its `meta` header.

## Install

From the canonical checkout:

```bash
ao workflows link
```

Links land in the **project-local `.claude/workflows/`** directory, where the
Claude Code harness resolves named workflows. The directory is gitignored;
only the runtime links live there — `workflows/` is the tracked source of
truth. `ao workflows link` mirrors `ao skills link` semantics: idempotent,
refuses to replace foreign links or real files, and `ao workflows unlink`
removes only links pointing back into this checkout.

**Session-snapshot caveat:** Claude Code snapshots the named-workflow registry
at session start. Newly minted links appear in the next session, not the one
already running.

## Doctrine: thin conveyors

These scripts are **thin conveyors**. All task semantics — the subject, the charters, the briefs, the acceptance criteria — arrive via `args`. The script contributes only orchestration shape, guardrail scaffolding (RED-first, disjoint ownership, no-stash, adversarial verification, destructive-command-guard awareness), and result plumbing. If a prompt inside a script ever encodes knowledge about a specific repo, defect, or session, that is a bug in the script. Agents operate in the session working directory; pass `args.root` only if you must point them elsewhere. Malformed args throw immediately with the expected shape — a thrown workflow is better than a silently wrong fleet.

**The bead is the reusable artifact, not the orchestration.** `implement-wave` lanes are deliberately bead-shaped — `{key, scope, brief, acceptance}` — because acceptance is the contract the verifier judges against, exactly how a bead carries acceptance into Validate. Write good beads; the conveyor is interchangeable.

## Durable evidence

Workflow results live in the chat that ran them. When verdicts must outlive the chat, say so in the verify-stage brief (`verify.brief` in `implement-wave`, or the group items' wording in `verify-fixes`) and instruct the verifier to persist through the product's own Validate skill (`verdict.v2`). The workflow itself never owns lifecycle — no retry, no closure, no landing.

## audit-dimensions

Fan one audit subject across caller-defined dimensions. Each dimension gets a read-only finder (findings must cite re-openable evidence), then a skeptic re-opens every citation and returns `CONFIRMED | REFUTED | DOWNGRADED` per finding. Only non-refuted findings survive, with corrected severities.

Args: `{ subject: string, dimensions: [{ key, charter }], bar?: string, root?: string, maxFindingsPerDimension?: number }`
Returns: `{ dimensions: [{ key, summary, findings, refuted }] }` — a dimension whose auditor died comes back with empty `findings` and an `error` field, never silently dropped.

```js
Workflow({ name: 'audit-dimensions', args: {
  subject: 'the v2.1 release candidate on the current branch',
  dimensions: [
    { key: 'docs', charter: 'Check user-facing docs match actual CLI behavior.' },
    { key: 'errors', charter: 'Check error paths fail loudly, never silently swallow.' },
  ],
  bar: 'blocker = ships broken to users; minor = cosmetic',
  maxFindingsPerDimension: 5,
}})
```

## verify-fixes

Adversarial verification of claimed fixes. One verifier per group tries to break every claim with fresh evidence; a claim earns `RESOLVED` only when it survives. `INCOMPLETE` covers partial fixes and anything the verifier could not actually check; `REGRESSED` covers new breakage. Side observations land in `residuals`.

Args: `{ context: string, root?: string, groups: [{ key, items: [string] }] }`
Returns: `{ groups: [{ key, verdicts: [{ item, verdict, evidence }], residuals }] }` — a group whose verifier died comes back with every item `INCOMPLETE` and an `error` field: a dead verifier is unverified work, never silent success.

```js
Workflow({ name: 'verify-fixes', args: {
  context: 'PR #42 on this repo claims to fix flag parsing in cli/parse.go',
  groups: [
    { key: 'parsing', items: [
      '--json and --robot are no longer mutually destructive',
      'unknown flags produce a non-zero exit with a hint',
    ]},
  ],
}})
```

## implement-wave

One wave of parallel implementer lanes with strictly disjoint file ownership, then a single fresh adversarial verifier judging every lane against its acceptance. Lane scaffolding enforces: RED reproduced before editing (pre-fix binaries built first), GREEN proven after, no `git stash` on the shared tree, out-of-scope needs reported in `constraints` instead of edited, destructive-command guards respected. A failed lane is surfaced to the verifier rather than dropped.

Args: `{ context: string, conventions?: string, root?: string, lanes: [{ key, scope: [string], brief, acceptance }], verify?: { brief } }`
Returns: `{ implementers: [{ key, summary, red_repro, green_proof, files_changed, constraints }], verification: { verdicts, residuals } }` — a lane whose agent died comes back with an `error` field and is still handed to the verifier.

```js
Workflow({ name: 'implement-wave', args: {
  context: 'repo at the session working directory, branch fix/wave-1; Go CLI',
  conventions: 'gofmt; table-driven tests; wrap errors with %w',
  lanes: [
    { key: 'ab-101',
      scope: ['cli/internal/parse/**'],
      brief: 'Make flag aliases case-insensitive.',
      acceptance: 'go test ./cli/internal/parse/... passes including a new case-insensitivity test that fails before the change.' },
  ],
  verify: { brief: 'Persist each lane verdict via the Validate skill (verdict.v2).' },
}})
```

## rpi

One caller intent through the core loop, once: Plan shapes one active behavior
and snapshots the exact intent bytes under SHA-256 identity (validate tooling's
`snapshot-intent`), Implement runs one bounded RED→GREEN experiment strictly
inside the write scope, then a separately spawned Validate context re-verifies
the intent digest, computes the subject manifest over the changed paths, judges
every acceptance criterion with fresh evidence, and persists `verdict.v2` via
`store-verdict` with distinct author/validator context ids and a freshness
attestation. The script is the wall: the validator receives only the intent
identity, acceptance, write scope, changed paths, check receipts, and author
context id — never the implementer's narrative. Any dead stage degrades the
result to `NOT_PROVEN` with an `error` naming the stage.

Doctrine: one pass, no retry loop, no revision path, no lifecycle ownership —
the authoring context structurally cannot issue its own binding PASS.

Args: `{ intent: string, root?: string, writeScope?: [string], acceptance?: string }` — `writeScope`/`acceptance` are caller-fixed when given, otherwise Plan derives them.
Returns: `{ verdict: PASS|FAIL|NOT_PROVEN, verdictPath, intentDigest, changedPaths, filesSummary, criteria: [{ criterion, result, evidence }] }` (plus `error` when a stage died; `filesSummary` is the implementer's caller-facing note — it never reaches the validator).

```js
Workflow({ name: 'rpi', args: {
  intent: 'ao doctor should exit non-zero and name the missing hook when the pre-push hook is absent',
  writeScope: ['cli/internal/doctor/**'],
}})
```
