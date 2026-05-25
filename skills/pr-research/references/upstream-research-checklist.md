# Upstream Research Checklist

## Pre-Research Checklist

Before any code exploration, gather governance and contribution context:

| Item | Where to Look | Why It Matters |
|------|--------------|----------------|
| CONTRIBUTING.md | Root, `.github/`, `docs/` | Required process, commit format, testing expectations |
| PR template | `.github/PULL_REQUEST_TEMPLATE.md` or `.github/PULL_REQUEST_TEMPLATE/` | Required sections, checklist items |
| Issue templates | `.github/ISSUE_TEMPLATE/` | Expected issue format, labels |
| CODE_OF_CONDUCT.md | Root, `.github/` | Community norms |
| CI config | `.github/workflows/`, `.circleci/`, `Jenkinsfile` | What checks must pass before merge |
| License | `LICENSE`, `COPYING` | CLA/DCO requirements (see below) |
| CODEOWNERS | `.github/CODEOWNERS` | Who reviews what directories |
| Recent releases | `gh release list --limit 5` | Release cadence, stability expectations |

## Maintainer Signal Analysis

Assess project health and contribution friendliness:

| Signal | How to Measure | Healthy | Concerning |
|--------|---------------|---------|------------|
| PR review time | `gh pr list --state merged --json mergedAt,createdAt --limit 20` | <7 days median | >30 days |
| Open PR count | `gh pr list --state open \| wc -l` | <50 | >200 |
| Issue response time | First comment on recent issues | <48h | >2 weeks |
| Contributor diversity | `git shortlog -sn --since="6 months ago" \| wc -l` | >5 active | 1-2 only |
| Bot activity | Look for dependabot, renovate, CI bots | Automated maintenance | None |
| Stale PR handling | Search for "stale" label or bot | Clear policy | PRs rot silently |

## License Compliance Matrix

| Requirement | How to Detect | What You Must Do |
|-------------|--------------|-----------------|
| **CLA (Contributor License Agreement)** | `grep -ri "CLA" CONTRIBUTING.md`, CLA bot in PR checks | Sign the CLA before or with your first PR |
| **DCO (Developer Certificate of Origin)** | `grep -ri "DCO\|sign-off\|Signed-off-by" CONTRIBUTING.md` | Add `Signed-off-by:` line to every commit (`git commit -s`) |
| **Neither** | No mention of CLA or DCO | Standard license terms apply |
| **Corporate CLA** | "Corporate" or "employer" in CLA text | May need employer signature |

## PR Archaeology Patterns

When analyzing merged PRs, look for:

```bash
# Commit message style
git log --oneline -30 | head -10
# Conventional commits?
git log --oneline -30 | grep -cE "^[a-f0-9]+ (feat|fix|docs|refactor|test|chore)"

# Average PR size
gh pr list --state merged --limit 20 --json additions,deletions \
  | jq '[.[] | .additions + .deletions] | add / length'

# Review patterns
gh pr list --state merged --limit 10 --json reviewDecision,reviews \
  | jq '.[].reviewDecision'

# Branch naming
gh pr list --state merged --limit 20 --json headRefName \
  | jq -r '.[].headRefName' | sed 's|/.*||' | sort | uniq -c | sort -rn
```

## Output Format

Research findings should follow this structure:

```markdown
# PR Research: {repo-name}

## Executive Summary
{2-3 sentences: project health score, contribution friendliness, recommended approach}

## Governance
| Document | Status | Key Requirements |
|----------|--------|-----------------|
| CONTRIBUTING.md | Present/Missing | {summary of requirements} |
| PR Template | Present/Missing | {required sections} |
| CLA/DCO | Required/Not Required | {type and process} |

## Conventions
- **Commit style**: {conventional/imperative/freeform}
- **Branch naming**: {pattern, e.g., feat/*, fix/*}
- **PR size norm**: {small/medium/large, with numbers}
- **Review process**: {approvals required, CODEOWNERS enforced}

## Health Signals
- **Review latency**: ~{N} days median
- **Active contributors**: {N} in last 6 months
- **Open PR backlog**: {N} PRs
- **Assessment**: {healthy/moderate/concerning}

## Contribution Opportunities
| Issue | Type | Difficulty | Notes |
|-------|------|------------|-------|
| #{N} | {type} | {easy/medium/hard} | {why this is a good target} |

## Risks and Mitigations
| Risk | Mitigation |
|------|-----------|
| {risk} | {how to handle} |

## Next Steps
-> `/plan .agents/research/YYYY-MM-DD-pr-{repo}.md`
```
