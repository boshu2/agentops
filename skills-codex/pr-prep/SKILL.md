---
name: pr-prep
description: 'Prepare an external PR draft safely. Triggers: "$pr-prep", "prepare PR", "draft upstream PR".'
---
# $pr-prep — External Contribution Packaging

Match the target repository's conventions, validate the current branch, and
write a PR draft for user review. Preparation is not authorization to publish.

## Critical Constraints

- **Why: prevent duplicate work.** Search open/merged issues and PRs before prep.
- **Why: respect upstream.** Read target instructions, contribution docs,
  templates, CI, and recent accepted PRs before choosing title or body shape.
- **Why: preserve atomicity.** Stop on unrelated files, merged overlap,
  generated noise, debug output, or secrets.
- **Why: keep evidence truthful.** Never check off a test that did not pass.
- **Why: avoid destructive rewrites.** Suggest commit splits; do not rewrite
  history without separate user authorization.
- **Why: publication changes external state.** Show the exact title, body, base,
  remote, and branch; wait for explicit approval before `gh pr create` or push.

## Scope

Use for external contributions, not routine internal commits or AgentOps'
direct-to-main landing path. The folded `pr-research` trigger enters upstream
research first and writes `.agents/research/YYYY-MM-DD-upstream-<slug>.md`.

## Workflow

1. **Prior work.** Resolve upstream/base and search issues/PRs for duplicate or
   competing work. Stop for user direction when overlap exists.
2. **Conventions.** Read `AGENTS.md`, `CONTRIBUTING.md`, templates, CI, release
   policy, recent merged PRs, and commit subjects in the target repo.
3. **Isolation.** Inspect the merge-base range, files, and commits. Every file
   must support one behavior story; report unrelated or sensitive content.
4. **Validation.** Run the target's documented formatter, lint, build, tests,
   and contribution checks. Capture exact commands and outcomes.
5. **Commit advice.** Under 50 lines and four files, prefer one commit. For
   larger diffs, use [commit-split-advisor.md](references/commit-split-advisor.md)
   to propose buildable ordered groups. Do not apply the rewrite.
6. **Draft.** Write `.agents/pr-prep/YYYY-MM-DD-<slug>.md` following the
   upstream template: title, remote/base, issue, why/what summary, changes,
   executed test evidence, unrun checks, compatibility, rollback, and risks.
7. **Checkpoint — user review.** Present draft path and exact submission tuple.
   Ask whether to revise or submit, then wait. Preparation alone is not approval.
8. **Submission.** Only after explicit approval, ensure HEAD and the draft still
   match; then use `gh pr create --title <approved-title> --body-file
   <approved-file> --base <approved-base>` and report the URL.

## User Review Gate

Stop after drafting. Submission is blocked until the user explicitly approves
the exact title, body, base, remote, branch, and disclosed validation state.

## Codex Execution Profile

- Use local shell and file inspection to ground every convention claim.
- Keep upstream research, branch evidence, and the PR draft on disk so another
  Codex session can reproduce the decision.
- Treat the user-review checkpoint as a hard external-side-effect boundary.
- Prefer `gh` machine-readable output for discovery; never parse display prose
  when a structured field is available.

## Guardrails

- Do not create, push, edit, close, or comment on a PR without explicit scope.
- Do not infer submit approval from a request to prepare or draft.
- Do not use force push or rewrite commits unless separately authorized.
- Do not hide failed, skipped, or unavailable validation.
- If HEAD, base, title, or body changes after approval, request approval again.

## PR Body Shape

```markdown
## Summary
<what changed and why>

## Changes
- <reviewable point>

## Test plan
- [x] `<executed command>` — <PASS result>
- [ ] <not run> — <reason>

Fixes #<issue>
```

## Output Specification

- **Artifact directory:** `.agents/pr-prep/` for the review draft and
  `.agents/research/` for optional upstream research; the branch holds code.
- **Filename convention:** `YYYY-MM-DD-<contribution-slug>.md`.
- **Serialization/schema format:** upstream-template Markdown with title,
  remote/base, issue, summary, changes, test evidence, risks, and rollback.
- **Validator command:** run `bash skills-codex/pr-prep/scripts/validate.sh`,
  upstream-prescribed checks, `git diff --check`, and an appropriate secret scan.
- **Downstream handoff:** user review first; approved draft only then flows to
  `gh pr create --body-file` and the upstream maintainer.

## Quality Rubric

- Matches upstream contribution and commit conventions.
- Contains one isolated contribution story with no sensitive or stray files.
- Separates passed, failed, and unrun validation honestly.
- Explains behavior, rationale, compatibility, rollback, and reviewer risks.
- Names reproducible branch range, base, remote, commands, and draft path.
- Preserves the explicit user-approval boundary for every external mutation.

## Examples

**User says:** "Prepare this branch for an upstream PR."

Research upstream, validate the branch, write the draft, and wait for review.

**User says:** "Submit the unchanged draft I approved."

Confirm the approval tuple and HEAD still match, create the PR, and report URL.

## Troubleshooting

| Problem | Response |
|---|---|
| Competing PR exists | Show overlap and wait for user direction |
| Branch mixes concerns | Suggest splits without rewriting history |
| Required check is red | Keep it red and do not imply readiness |
| HEAD changed after approval | Refresh evidence and ask again |

## References

- [commit-split-advisor.md](references/commit-split-advisor.md)
- [case-study-historical-context.md](references/case-study-historical-context.md)
- [lessons-learned.md](references/lessons-learned.md)
- [package-extraction.md](references/package-extraction.md)
