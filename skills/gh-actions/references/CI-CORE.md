# CI Core Patterns

## Table of Contents
- [Standard Structure](#standard-structure)
- [Trigger Configuration](#trigger-configuration)
- [Path-Based Triggering](#path-based-triggering)
- [Concurrency](#concurrency)
- [Job Dependencies](#job-dependencies)
- [Environment Variables](#environment-variables)
- [Timeout Protection](#timeout-protection)
- [Checkout Options](#checkout-options)

---

## Standard Structure

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  workflow_dispatch:

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  lint:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps: [...]

  test:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps: [...]

  build:
    needs: [lint, test]
    runs-on: ubuntu-latest
    steps: [...]
```

---

## Trigger Configuration

```yaml
# Standard CI triggers
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

# Release triggers
on:
  push:
    tags: ['v*']

# Scheduled triggers
on:
  schedule:
    - cron: '0 3 * * *'  # 3 AM UTC daily
```

---

## Path-Based Triggering

```yaml
# Only when code changes
on:
  push:
    paths:
      - 'src/**'
      - 'Cargo.toml'
      - '.github/workflows/ci.yml'

# Ignore docs-only changes
on:
  push:
    paths-ignore:
      - '**.md'
      - 'docs/**'
```

---

## Concurrency

```yaml
# Cancel stale runs (CI)
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

# Don't cancel (releases)
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
```

---

## Job Dependencies

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest

  test:
    needs: lint
    runs-on: ubuntu-latest

  build:
    needs: [lint, test]
    runs-on: ubuntu-latest

  deploy:
    needs: build
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
```

### Pass Data Between Jobs

```yaml
jobs:
  compute:
    outputs:
      version: ${{ steps.ver.outputs.version }}
    steps:
      - id: ver
        run: echo "version=1.2.3" >> $GITHUB_OUTPUT

  use:
    needs: compute
    steps:
      - run: echo "${{ needs.compute.outputs.version }}"
```

---

## Environment Variables

```yaml
# Workflow-level
env:
  CARGO_TERM_COLOR: always
  RUST_BACKTRACE: 1

jobs:
  test:
    # Job-level
    env:
      CI: true
      TEST_DB: sqlite
    steps:
      # Step-level
      - run: deploy.sh
        env:
          DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}
```

---

## Timeout Protection

```yaml
jobs:
  test:
    timeout-minutes: 30  # Job-level

    steps:
      - run: npm test
        timeout-minutes: 15  # Step-level
```

**Never use default 6h timeout.**

---

## Checkout Options

```yaml
# Shallow (default, faster)
- uses: actions/checkout@v4

# Full history (changelog, tags)
- uses: actions/checkout@v4
  with:
    fetch-depth: 0

# PR head for comparison
- uses: actions/checkout@v4
  with:
    ref: ${{ github.event.pull_request.head.sha }}

# With submodules
- uses: actions/checkout@v4
  with:
    submodules: recursive
```
