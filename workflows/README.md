# Workflows

Reusable orchestration conveyors for the Claude Code Workflow tool. Workflows
are a **Claude-only runtime adapter** — the same doctrine as `skills-codex/`
(Codex-only): canonical source lives here, and a runtime link step installs it
where the one runtime that consumes it resolves names.

Five generic conveyor shapes:

| Workflow | Shape | Use when |
|---|---|---|
| `audit-dimensions` | pipeline: finder → skeptic, per dimension | auditing a subject across independent lenses |
| `verify-fixes` | parallel adversarial verifiers, one per group | refuting "it's fixed" claims after a change |
| `implement-wave` | parallel disjoint-scope lanes → one fresh verifier | executing a wave of bead-shaped work items |
| `bulk-read` | parallel cheap readers, one per file → line-referenced bullets | answering a question about big files without their bytes entering the caller's context |
| `code-write` | metadata probe for batches → sequential cheap writers, one per item (spec + reference → target) → receipts | writing patterned or boilerplate files the caller should not read back |

Two repository-delivery conveyors also live here, outside the AgentOps
semantic core: `bdd-foundry` (behavior-first planning → acceptance-gated
beads) and `ship-beads` (repository delivery orchestration: drive a list of
beads to confirmed-merged; `bead-crank` is its deprecated alias). The former
seven-move `operating-loop` workflow is a retired tombstone that fails with
replacement pointers — one experiment belongs to the `rpi` skill, multi-bead
delivery to `ship-beads` or a caller-selected factory. Each workflow documents
itself in its `meta` header.

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

## bulk-read

Delegate large or many files to cheap readers. One reader per file is instructed to read the whole file in bounded slices (`Read` with `offset` + `limit ≤ budgetLines`) and answer one question with line-referenced summaries — `{ ref: 'path:line' | 'path:start-end', text }`, most relevant first, at most `maxBullets`. The workflow requires refs to name the requested file and a positive line or ascending range within `lines_covered`, one-line text of at most 200 characters, and a one-line optional `note` of at most 300 characters. It validates nonnegative integer coverage and caps the bullet count, logging dropped bullets. Invalid results become explicit errors without echoing their content.

Readers are instructed to be read-only, summarize without copying source, and report `lines_covered` / `complete` truthfully. A missing, binary or unreadable file comes back with zero bullets and a `note`. The wrapper verifies return structure and bounds, not whether the worker actually read the file or whether a short summary is accurate. Bash read-only behavior and content-free summaries remain agent instructions; neither tool confinement nor live child-to-parent context isolation is established by the stub harness.

Args: `{ question: string, files: [string], root?: string, model?: string (default 'haiku'), maxBullets?: positive safe integer (default 40), budgetLines?: positive safe integer (default 350) }`
Returns: `{ question, files: [{ file, bullets: [{ ref, text }], lines_covered, complete, note? }], bullets_total }` — a dead reader or invalid result produces empty `bullets`, `lines_covered: null`, `complete: false` and an `error` field. A missing result means coverage is unknown, even if the worker read some lines before dying.

```js
Workflow({ name: 'bulk-read', args: {
  question: 'Where are exit codes decided, and which paths return non-zero?',
  files: ['cli/internal/gates/runner.go', 'scripts/check-go-lint.sh'],
  maxBullets: 20,
}})
```

## code-write

Delegate patterned file writes to cheap writers. One writer per item is instructed to read the required `reference` file in bounded slices to learn its patterns (naming, imports, error handling, test shape), write ONLY its `target` to satisfy `spec`, optionally run `check` once, and return a bounded receipt. `reference` is required: no reference, no worker.

Before a batch, one additional cheap agent runs an exact Node command through Bash to resolve target paths with `realpath` and obtain existing files' device/inode identities with `stat`. It reads no file contents. The workflow rejects aliases (including symlinks and hardlinks), missing or invalid metadata, and failed probes before any writer starts. Missing targets resolve through the nearest existing ancestor; dangling symlinks and non-file targets fail the probe. Node must be available to that agent. Writers then run sequentially, one per item, in the shared working tree. A single-item call needs no cross-item identity check.

Absent targets in a batch must have ASCII canonical paths, including all existing ancestors. Without inode identities, JavaScript Unicode normalization and case conversion cannot prove that names are distinct on APFS. Non-ASCII missing paths therefore reject before any writer with a request to use separate calls. ASCII case variants such as `New.js` and `new.js` are also conservatively rejected on every host when either file is absent. Existing Unicode targets remain supported through their native device/inode identities.

The preflight is child-reported metadata at one instant, not a filesystem lock or sandbox. Use targets nobody else is editing; another process could change paths after preflight. Target-only writes remain an agent instruction. A receipt is a child report, not validation: judge the written files with a fresh, author-distinct Validate as usual.

Args: `{ context?: string, root?: string, model?: string (default 'haiku'), budgetLines?: positive safe integer (default 350), items: [{ key, spec, reference, target, check? }] }` — duplicate target strings or filesystem identities throw before writing. The selected model also applies to the metadata probe.
Returns: `{ items: [{ key, target, written, lines, check_ran, check_ok, summary }] }`. The workflow validates key/target identity, booleans, nonnegative integer line count, and a one-line summary of at most 300 characters. Raw check output is excluded because diagnostics can contain source code. Dead writers and invalid receipts return `written: null`, `lines: null`, `check_ran: null`, `check_ok: null`, an empty summary, and an `error`: file and check state are unknown, since a worker can write before it dies. Short-summary semantics remain an agent instruction.

```js
Workflow({ name: 'code-write', args: {
  context: 'Go CLI; tests are table-driven and live next to the source',
  items: [
    { key: 'parse-tests',
      spec: 'Table-driven tests for ParseFlags covering aliases, unknown flags and the --json/--robot pair.',
      reference: 'cli/internal/gates/runner_test.go',
      target: 'cli/internal/parse/parse_test.go',
      check: 'cd cli && go test ./internal/parse/...' },
  ],
}})
```

## Context budget

`bulk-read` and `code-write` are the delegation half of the context-budget pattern; the enforcement half is the opt-in read-budget guard shipped inert in the `cc-hooks` skill (`scripts/install-read-budget-guard.sh` wires it as an opt-in PreToolUse hook; nothing installs it automatically). Once installed, that opt-in hook blocks an unbounded `Read`, `cat`, `head` or `tail` of a file over the line budget (`AOP_READ_BUDGET_LINES`, default 350) and its message names both correct moves: slice the file, or delegate it to `bulk-read` / the `bulk-reader` subagent. Readers and writers are instructed to use slices with `limit ≤ budgetLines`; a compliant slice passes the hook. Model choice belongs to the caller (`model`, default `haiku`); a receipt or a bullet list is a child report, not validation. Nothing here owns a budget account, retry or scheduler. The full pattern lives in `skills/agent-native/references/context-budget-delegation.md`.
