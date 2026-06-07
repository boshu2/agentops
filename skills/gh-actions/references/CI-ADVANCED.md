# CI Advanced Patterns

## Table of Contents
- [Matrix Strategies](#matrix-strategies)
- [Caching Patterns](#caching-patterns)
- [Artifact Handling](#artifact-handling)
- [Step Summaries](#step-summaries)
- [Conditional Steps](#conditional-steps)
- [Continue on Error](#continue-on-error)
- [Working Directory](#working-directory)
- [Shell Selection](#shell-selection)

---

## Matrix Strategies

### Cross-Platform

```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
```

### Version Matrix

```yaml
strategy:
  matrix:
    rust: [stable, beta, nightly]
    go: ['1.24', '1.25']
    node: [18, 20, 22]
```

### Include/Exclude

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest]
    include:
      - os: macos-14
        target: aarch64-apple-darwin
    exclude:
      - os: windows-latest
        feature: unix-only
```

---

## Caching Patterns

### Rust

```yaml
- uses: Swatinem/rust-cache@v2
  with:
    key: ${{ matrix.target }}
```

### Go (Automatic)

```yaml
- uses: actions/setup-go@v6
  with:
    go-version: '1.25'
    # Caching enabled by default
```

### Node.js

```yaml
- uses: actions/setup-node@v4
  with:
    node-version: '20'
    cache: 'npm'
```

### Generic Cache

```yaml
- uses: actions/cache@v4
  with:
    path: |
      ~/.cache/my-tool
      .build-cache
    key: ${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('**/lockfile') }}
    restore-keys: |
      ${{ runner.os }}-${{ runner.arch }}-
      ${{ runner.os }}-
```

---

## Artifact Handling

### Upload

```yaml
- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: test-results-${{ matrix.os }}
    path: |
      test-results/
      coverage/
    retention-days: 7
```

### Download in Later Job

```yaml
jobs:
  test:
    steps:
      - uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.json

  report:
    needs: test
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: coverage

      # Or merge all artifacts
      - uses: actions/download-artifact@v4
        with:
          path: artifacts
          merge-multiple: true
```

---

## Step Summaries

```yaml
- name: Generate summary
  run: |
    echo "## Build Results" >> $GITHUB_STEP_SUMMARY
    echo "" >> $GITHUB_STEP_SUMMARY
    echo "| Metric | Value |" >> $GITHUB_STEP_SUMMARY
    echo "|--------|-------|" >> $GITHUB_STEP_SUMMARY
    echo "| Coverage | 85% |" >> $GITHUB_STEP_SUMMARY
```

### Error Annotations

```yaml
- run: |
    echo "::error file=src/main.rs,line=42::Coverage below threshold"
    echo "::warning::Consider adding more tests"
```

---

## Conditional Steps

```yaml
# Only on main
- if: github.ref == 'refs/heads/main'
  run: deploy.sh

# Only on PRs
- if: github.event_name == 'pull_request'
  run: npm run lint:changed

# Only when file exists
- if: hashFiles('Cargo.lock') != ''
  run: cargo audit

# On success/failure/always
- if: success()
  run: notify_success.sh

- if: failure()
  uses: actions/upload-artifact@v4
  with:
    name: failure-logs
    path: logs/

- if: always()
  run: cleanup.sh
```

---

## Continue on Error

```yaml
- run: npm run optional-check
  continue-on-error: true

- id: optional
  run: might_fail.sh
  continue-on-error: true

- if: steps.optional.outcome == 'failure'
  run: echo "Optional step failed, continuing"
```

---

## Working Directory

```yaml
jobs:
  build:
    defaults:
      run:
        working-directory: ./packages/app

    steps:
      - run: npm install  # Runs in ./packages/app
      - run: npm test
        working-directory: ./packages/lib  # Override
```

---

## Shell Selection

```yaml
- run: echo "hello"
  shell: bash

- run: Get-Process
  shell: pwsh

- run: |
    import json
    print(json.dumps({"status": "ok"}))
  shell: python
```
