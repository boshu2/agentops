# Lesson Format Contract

This is the canonical format for entries in `.agents/learnings/` — durable, agent-readable lessons captured at the end of a session.

The contract has three goals:
1. **Searchable** — every lesson has a frontmatter schema an `ao inject` or grep can filter.
2. **Falsifiable** — every rule names what would prove it wrong.
3. **Graduating** — lessons that survive citation pressure move into `PRACTICE-REGISTRY.md` → CI gates → standards.

This contract applies to **new** lessons. Pre-existing files predating this spec are grandfathered.

---

## File layout

| Path | Contents |
|---|---|
| `.agents/learnings/<YYYY-MM-DD>-<id>.md` | **One file per lesson.** Date prefix + the lesson's `id`. Gitignored — local-only. |
| `.agents/learnings/<YYYY-MM-DD>-<session-slug>-index.md` | (Optional) Per-session index linking the day's individual files. Useful for retros. |

Filename rules:
- Date prefix = the date the lesson was first captured.
- `id` = kebab-case slug, identical to the `bd remember --key` value, identical to the lesson's `id:` frontmatter field. **All three must match.**
- One concept per file. Composite lessons split.

---

## Required frontmatter

```yaml
---
id: <kebab-case-slug>          # matches bd remember --key and the filename id
date: YYYY-MM-DD
severity: critical | high | medium | low
trigger: |
  When you'd recognize this rule applies. Concrete signals — patterns
  in commit messages, CI output, bd queries, file states. The future
  agent reads this to decide "does my situation match?"
verifiable: |
  How to test the rule fires correctly. A PR number, a command,
  a commit SHA. Future agents validate the rule still holds.
rule: |
  One-sentence imperative prescription. This is the bd remember body.
falsified_by: |
  What would prove this rule wrong. "None" means "I haven't thought
  hard enough" — rewrite until you find a concrete failure mode.
practice: <slug> | unassigned | proposed | accepted | encoded
related:
  - bd-memory:<key>
  - learning:<other-lesson-id>
  - bead:<bead-id>
  - pr:<number>
---
```

### Field rules

- **`id`** — Kebab-case. 2–6 words. Identical to filename id, identical to `bd remember --key`. Required for cross-referencing.
- **`date`** — ISO date of capture. Not the date of the underlying event (cite that in Context).
- **`severity`**:
  - `critical` — Workflow-breaking. Ignoring it loses work, breaks production, or silently corrupts state.
  - `high` — High-friction recurring pain. Multiple sessions hit it.
  - `medium` — Efficient workaround for a real pattern.
  - `low` — Nice-to-know taste/style.
- **`trigger`** — Mandatory. A rule without a recognition signal is dogma. Be concrete: "When a commit message says 'landed' or 'fast-forwarded'…"
- **`verifiable`** — Mandatory. Cite at least one of: PR #, command output, commit SHA, test name. "Just trust me" is not verifiable.
- **`rule`** — Mandatory. Imperative voice ("Verify push," not "we should verify push"). ≤200 chars when serialized. This is the body of the matching `bd remember` entry.
- **`falsified_by`** — Mandatory. The condition under which the rule would no longer apply. Forces honest thinking about scope.
- **`practice`** — The graduation state (see below).
- **`related`** — Other artifacts that cite or are cited by this lesson. Use the `<type>:<id>` shorthand.

---

## Body sections

After the frontmatter, three required sections:

```markdown
## Context

The story — what happened, dated, with specifics. PR numbers, file paths,
commit SHAs. Future agents read this to ground the rule in a real failure.

## Why this matters

The leverage. What breaks if you don't follow it. Be specific about cost —
"hours of diagnostic time," "lost work," "silent regression."

## How to apply

Concrete steps in the next session. Commands to run, files to grep,
patterns to recognize. Imperative voice.
```

Optional fourth section:

```markdown
## See also

Pointers to longer-form artifacts (DUEL.md verdicts, design docs,
external references).
```

---

## Graduation path

A lesson's `practice:` field tracks its maturity:

| State | Meaning | Trigger to next state |
|---|---|---|
| `unassigned` | Fresh lesson, not yet matched to a registered practice slug. | Lesson cited 3+ times across sessions (via `related:` pointers in newer lessons or skill frontmatter `practices:` lists). |
| `proposed` | Slug proposed for `PRACTICE-REGISTRY.md`. | Operator/reviewer accepts the proposal; entry added to the registry. |
| `accepted` | Slug now exists in `PRACTICE-REGISTRY.md`. Skills and lessons cite it via `practices: [<slug>]`. | A CI gate, hook, or standards-doc encoding lands that enforces the lesson mechanically. |
| `encoded` | The rule is mechanically enforced (CI check, pre-commit hook, server-side rule). The lesson becomes archival evidence. | Terminal state. Lesson may be moved to `.agents/learnings/archive/`. |

A lesson that **never** graduates past `unassigned` after a year is a candidate for archive (low leverage, no traction) or rewrite (the rule is malformed).

---

## Companion `bd remember` entry

Every learning file MUST have a paired `bd remember` entry:

```bash
bd remember "<rule frontmatter field, verbatim>" --key <id>
```

Constraints:
- **One line.** No paragraphs. ≤200 chars including the imperative.
- **Key matches `id`** in the frontmatter, character-for-character.
- **Body is the `rule:` field**, with no rewriting. Two surfaces, one source of truth.

`bd prime` injects these on session start, so every agent has the rule's atomic form. The deep file is the citation; the memory is the recall.

---

## Canonical example

A lesson from the 2026-05-17 cascade session, in full form:

```yaml
---
id: landed-means-pushed
date: 2026-05-17
severity: critical
trigger: |
  Any handoff, commit message, or status report using "landed,"
  "shipped," "fast-forwarded to main," or "merged" without an
  origin/main verification step. Especially in multi-agent
  sessions where another agent claims they shipped something.
verifiable: |
  Run `git log origin/main..local-main`. If output is non-empty,
  local has commits not on origin. Reproduced via PR #293 chain
  (3 prior-session commits stranded on local main for 2 days).
rule: |
  "Landed" means visible on origin/main via gh api or the GitHub
  web UI. Never trust local fast-forward state.
falsified_by: |
  A workflow where local-main edits auto-mirror to origin (no such
  workflow exists in agentops today). Or branch protection blocking
  all main writes (this exists now — the rule's failure mode is dead
  by construction for agentops; rule applies only to repos without
  server-side protection).
practice: unassigned
related:
  - bd-memory:landed-means-pushed
  - bead:soc-dmxn
  - pr:296
  - pr:293
---

## Context

In the 2026-05-17 session, a handoff document said the prior agent
had "landed 3 commits on main via fast-forward." Two days later, a
rebase exposed that those commits (d7f4f675, b09a864b, ce1891eb)
existed ONLY on local main — they had never been pushed. Local main
had diverged from origin/main by 73 commits. The prior agent had
fast-forwarded into local main and then ended the session without
`git push`.

## Why this matters

Cost in this session alone: 30+ minutes of confused rebase attempts,
re-reading the handoff three times, and eventually a worktree-based
cherry-pick to reconstruct the lost-and-found commits onto origin/main.
In the worst case, the original commits would have been silently
overwritten when origin/main moved.

## How to apply

1. Before trusting any "landed" or "merged" claim:
   `git fetch origin && git log origin/main..local-main`
2. If non-empty: the claim is wrong. Investigate before any rebase.
3. Going forward: branch protection on agentops main makes
   direct-main pushes impossible, so this failure mode is dead by
   construction for this repo. For other repos (mt-olympus on free
   plan, external repos), the rule still applies until paid
   protection is enabled.

## See also

- `.agents/council/sdlc-shape-2026-05-17/DUEL.md` — the verdict that
  encoded the PR-only rule into branch protection.
```

The companion `bd remember`:

```bash
bd remember '"Landed" means visible on origin/main via gh api or web UI. Never trust local fast-forward state.' --key landed-means-pushed
```

---

## Anti-patterns (banned by this contract)

| Anti-pattern | Why it fails |
|---|---|
| Multiple lessons in one file | Can't link to a single rule; can't graduate one without the others. |
| Missing `trigger:` field | Future agents can't recognize when the rule applies. Dogma. |
| Missing `falsified_by:` field | Untestable. The rule outlives its truth. |
| `bd remember` body that's a paragraph | Memories scan in seconds; paragraphs don't. |
| `bd remember --key` ≠ filename id ≠ frontmatter id | Cross-referencing breaks. Three names for the same thing. |
| Rule in passive voice ("we should…", "it would be good to…") | Not actionable. Use imperative ("Verify push," "Grep the test corpus"). |
| `practice: unassigned` for >1 year with no citations | Low leverage. Archive or rewrite. |

---

## Linting (future)

A `validate-lesson-format.sh` script (filed as a follow-up bead) will check:

- File path matches `<YYYY-MM-DD>-<id>.md`.
- Frontmatter has all 9 required fields.
- `id:` matches filename `id`.
- `bd remember --key <id>` exists; body matches `rule:` field.
- `severity:` is one of the 4 allowed values.
- `practice:` is one of the 5 allowed values, with consistent state transitions.
- `related:` items resolve (file paths exist, bead IDs are valid, PR numbers exist).

Until that lints, this contract is honor-system.

---

## See also

- `.agents/learnings/` — the lesson corpus this format governs.
- `PRACTICE-REGISTRY.md` — graduation destination for `accepted` lessons.
- `docs/contracts/context-map.md` — the sibling contract for skill structure.
- `docs/contracts/skill-ports-and-adapters.md` — the contract this format mirrors in shape (frontmatter + body sections + graduation).
