# Writing the Premortem Output

> Extracted from premortem/SKILL.md on 2026-04-11.

## Output Location

**Write to:** `.agents/council/YYYY-MM-DD-premortem-<topic>.md`

## Template

```markdown
---
id: premortem-YYYY-MM-DD-<topic-slug>
type: premortem
date: YYYY-MM-DD
source: "[[.agents/plans/YYYY-MM-DD-<plan-slug>]]"
prediction_ids:
  - pm-YYYYMMDD-001
  - pm-YYYYMMDD-002
---

# Premortem: <Topic>

## Council Verdict: PASS / WARN / FAIL

| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-YYYYMMDD-001 | Missing-Requirements | ... | significant | <what will go wrong> |
| pm-YYYYMMDD-002 | Feasibility | ... | significant | <what will go wrong> |
| pm-YYYYMMDD-003 | Scope | ... | moderate | <what will go wrong> |

## Pseudocode Fixes

**Every finding that implies a code change MUST include implementation-ready pseudocode**, not prose-only descriptions. Write the pseudocode in the language of the target file. Workers read issue descriptions, not premortem reports — vague prose leads to workers reimplementing the bug.

Format each code-fix finding as:

    Finding: F1 — <concise description>
    Severity: <severity>
    Fix (pseudocode):
      ```<language>
      // pseudocode in the target file's language
      if tier == "inherit" || tier == "" {
          return "balanced"  // inherit always resolves to balanced
      }
      ```
    Affected files: <path(s)>

Prose-only fix descriptions (e.g., "The inherit tier should fall back to balanced") are insufficient when the fix involves specific logic. If a finding is purely architectural or process-related with no code change, prose is acceptable.

## Shared Findings
- ...

## Known Risks Applied
- `<finding-id>` — `<why it matched this plan>`

## Concerns Raised
- ...

## Recommendation
<council recommendation>

## Decision Gate

[ ] PROCEED - Council passed, ready to implement
[ ] ADDRESS - Fix concerns before implementing
[ ] RETHINK - Fundamental issues, needs redesign
```

Each finding gets a unique prediction ID (`pm-YYYYMMDD-NNN`) for downstream correlation. See [prediction-tracking.md](prediction-tracking.md) for the full tracking lifecycle.

## Step 4.5: Persist Reusable Findings

If the verdict is `WARN` or `FAIL`, persist only the reusable plan/spec failures to `.agents/findings/registry.jsonl`.

Use the finding-registry contract:

- required fields: `dedup_key`, provenance, `pattern`, `detection_question`, `checklist_item`, `applicable_when`, `confidence`
- `applicable_when` must use the controlled vocabulary from the contract
- append or merge by `dedup_key`
- use the contract's temp-file-plus-rename atomic write rule

Do NOT write every comment. Persist only findings that should change future planning or review behavior.

After a catch-ledger update, refresh the canonical recurring-catch advisory sink:

```bash
ao membrane digest
```

Do not run a repository hook from Premortem. Mechanical candidates are compiled
only through `ao membrane derive-checks --detector-evidence <json>` after stored
positives and explicit negative controls exist. They begin as warn-only shadows;
Premortem never activates them.

## Step 4.6: Copy Pseudocode Fixes into Plan Issues

When premortem findings are applied to plan issues (via `TaskUpdate`, `bd update`, or manual edit), **copy the pseudocode block verbatim into the issue body**. Workers read issue descriptions — they do not read premortem reports. If the pseudocode lives only in the premortem report, workers will reimplement the fix from scratch and often get it wrong.

For each finding with a pseudocode fix:

1. Identify which plan issue the finding applies to
2. Append a `## Premortem Fix` section to that issue's description containing the pseudocode block and affected file paths
3. If no matching issue exists, note the gap in the report's Recommendation section
