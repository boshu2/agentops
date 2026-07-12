---
name: pr-prep
description: 'Prepare PR commits and body. Triggers: "pr-prep", "pr prep", "prepare pr commits and body.".'
practices:
- continuous-integration
- continuous-delivery
- gitops
hexagonal_role: driving-adapter
consumes:
- domain
produces:
- git-changes
context_rel:
- kind: customer-of
  with: domain
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
license: MIT
compatibility: Requires git, gh CLI
metadata:
  author: AI Platform Team
  version: 1.4.0
  tier: contribute
  internal: false
allowed-tools: Read, Write, Bash, Grep, Glob
output_contract: PR body (markdown), git branch
---
# PR Preparation Skill

Prepare an external contribution by matching the target repository's actual
conventions, proving the branch, and drafting a reviewable PR body. Execute the
workflow; do not submit anything until the user explicitly approves the exact
title, body, base, and remote.

## Critical Constraints

- **Why: prevent duplicate work.** Check open and merged issues/PRs plus the
  current default branch before packaging a contribution.
- **Why: respect the target.** Read the target repository's `AGENTS.md`,
  `CONTRIBUTING.md`, PR template, release notes, and recent accepted PRs; local
  AgentOps conventions never override upstream contribution rules.
- **Why: preserve atomic scope.** Stop when the branch mixes unrelated themes,
  already-merged changes, generated noise, or secrets.
- **Why: keep evidence truthful.** Run the repository-prescribed build, lint,
  and test commands; never mark a check complete when it did not pass.
- **Why: avoid destructive history edits.** Commit splitting is suggestion-only
  unless the user separately authorizes a rewrite.
- **Why: external publication changes state.** Draft first, show the exact
  artifact, and require explicit approval before `gh pr create` or any push.

## Boundary and Modes

Use for external repositories and open-source contribution preparation. Do not
use for routine internal commits or the AgentOps direct-to-main release path.

The folded `pr-research` trigger enters upstream-research mode: perform prior
work discovery, contribution-guideline analysis, merged-PR archaeology, and
maintainer-pattern research before prep. Persist that research at
`.agents/research/YYYY-MM-DD-upstream-<slug>.md`.

Inputs are a target repository, current contribution branch, optional issue,
and optional requested base/remote. Preparation may write local artifacts; it
does not grant authority to publish, rewrite history, or message maintainers.

## Workflow

1. **Check prior work.** Resolve the upstream default branch and remote. Search
   open/closed issues and PRs for the behavior, symbols, and issue ID. Stop on a
   competing contribution until the user chooses how to proceed.
2. **Discover conventions.** Read repository instructions, contribution docs,
   PR templates, CI config, changelog policy, recent merged PRs, and commit
   subjects. Record the exact title/body/test conventions used by maintainers.
3. **Prove isolation.** Compare the merge-base range, list changed files and
   commits, and group by behavior. Remove or report unrelated files, secrets,
   generated artifacts, debug output, and changes already on the base branch.
4. **Validate the branch.** Run the target repo's documented formatter, lint,
   build, tests, and any contribution-specific check. Capture command, exit
   status, and relevant result. A red check remains red in the draft.
5. **Analyze commits.** Recommend a single commit for a diff under 50 lines and
   four files; otherwise follow
   [commit-split-advisor.md](references/commit-split-advisor.md). Suggestions
   name ordered file groups and keep every commit buildable. Do not rewrite.
6. **Draft the PR.** Write `.agents/pr-prep/YYYY-MM-DD-<slug>.md` with proposed
   title, upstream/base, linked issue, concise why/what summary, change list,
   test plan containing only executed evidence, compatibility/rollback notes,
   and reviewer-relevant risks. Follow the upstream template exactly.
7. **Checkpoint — mandatory user review.** Show the title, body, base, remote,
   branch, validation results, and any red/untested checks. Ask for approval or
   revision and wait. Silence, a request to "prepare", or prior push authority
   is not approval to create this PR.
8. **Submit only after approval.** Recheck the diff and validation if HEAD moved,
   then run `gh pr create --title <approved-title> --body-file <approved-file>
   --base <approved-base>`. Report the resulting URL and do not mutate the PR
   further unless asked.

## PR Body Shape

```markdown
## Summary
<what changed and why; 1-3 sentences>

## Changes
- <reviewable behavior or implementation point>

## Test plan
- [x] `<command>` — <result>
- [ ] <not run> — <reason>

Fixes #<issue>
```

Use checked boxes only for executed PASS evidence. Keep failed or skipped checks
visible. Put closing keywords at the end when upstream policy permits them.

## Output Specification

- **Artifact directory:** `.agents/pr-prep/` for the local review draft and
  `.agents/research/` for optional upstream research; the current git branch is
  the code artifact.
- **Filename convention:** `YYYY-MM-DD-<contribution-slug>.md`; never overwrite
  an unrelated draft.
- **Serialization/schema format:** Markdown following the upstream PR template,
  with title, remote/base, summary, changes, test plan, risks, and issue linkage.
- **Validator command:** run `bash skills/pr-prep/scripts/validate.sh`, the
  target repository's documented checks, `git diff --check`, and a secret scan
  appropriate to the repo before requesting approval.
- **Downstream handoff:** consumed first by the user's review and, only after
  explicit approval, by `gh pr create --body-file` and the upstream maintainer.

## Quality Rubric

- **Convention-matched:** title, template, commit shape, and checks follow upstream.
- **Isolated:** every changed file serves one contribution story.
- **Evidence-bound:** the test plan distinguishes passed, failed, and unrun checks.
- **Reviewable:** rationale, behavior, risks, and compatibility are concise and clear.
- **Safe:** no secrets, unrelated changes, destructive rewrite, or implicit submit.
- **Reproducible:** the exact branch range, commands, base, remote, and draft path are named.

## Examples

**User says:** "Prepare this branch for an upstream PR."

Inspect upstream rules and prior work, validate the branch, write the local PR
draft, and stop at the review checkpoint.

**User says:** "Submit the PR using the draft I just approved."

Verify the draft and HEAD still match the approved state, then create exactly
that PR and report its URL.

## Troubleshooting

| Problem | Response |
|---|---|
| Competing PR exists | Show overlap and stop for user direction |
| Branch mixes themes | Suggest split groups; do not rewrite without authorization |
| Required check fails | Keep it red in the draft and do not imply readiness |
| Upstream template is absent | Infer from recent merged PRs and label the assumption |
| HEAD changed after approval | Regenerate evidence and request approval again |

## References

- [pr-prep.feature](references/pr-prep.feature) — executable behavior contract
- [commit-split-advisor.md](references/commit-split-advisor.md) — buildable commit grouping
- [case-study-historical-context.md](references/case-study-historical-context.md) — historical contribution context
- [lessons-learned.md](references/lessons-learned.md) — reusable preparation lessons
- [package-extraction.md](references/package-extraction.md) — package extraction guidance
