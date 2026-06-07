# Dependabot Configuration

## Standard Configuration

```yaml
# .github/dependabot.yml
version: 2
updates:
  # GitHub Actions
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      actions:
        patterns: ["*"]

  # Rust
  - package-ecosystem: "cargo"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      rust-deps:
        patterns: ["*"]
        exclude-patterns: ["tokio*", "serde*"]

  # Go
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"

  # npm/Bun
  - package-ecosystem: "npm"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      dev-dependencies:
        dependency-type: "development"
      production:
        dependency-type: "production"
```

---

## Grouping Strategies

### Group All (Fewer PRs)

```yaml
groups:
  all-deps:
    patterns: ["*"]
```

### Separate by Type

```yaml
groups:
  production:
    dependency-type: "production"
  development:
    dependency-type: "development"
```

---

## Auto-Merge for Patch Updates

```yaml
# .github/workflows/dependabot-automerge.yml
name: Dependabot Auto-Merge

on: pull_request

permissions:
  contents: write
  pull-requests: write

jobs:
  automerge:
    runs-on: ubuntu-latest
    if: github.actor == 'dependabot[bot]'
    steps:
      - id: metadata
        uses: dependabot/fetch-metadata@v2
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}

      - if: steps.metadata.outputs.update-type == 'version-update:semver-patch'
        run: gh pr merge --auto --squash "$PR_URL"
        env:
          PR_URL: ${{ github.event.pull_request.html_url }}
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Schedule Options

```yaml
schedule:
  interval: "daily"     # Every weekday
  interval: "weekly"    # Monday mornings
  interval: "monthly"   # First of month

  # Custom
  interval: "weekly"
  day: "wednesday"
  time: "09:00"
  timezone: "America/New_York"
```
