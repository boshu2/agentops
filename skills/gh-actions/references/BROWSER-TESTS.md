# Browser Testing Patterns (Playwright)

## Basic E2E Workflow

```yaml
e2e:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: oven-sh/setup-bun@v2
    - run: bun install --frozen-lockfile
    - run: bunx playwright install --with-deps chromium

    - run: bunx playwright test --project=desktop-chrome
      env:
        BASE_URL: ${{ vars.STAGING_URL }}

    - uses: actions/upload-artifact@v4
      if: always()
      with:
        name: playwright-report
        path: playwright-report/
        retention-days: 7
```

---

## Multi-Browser Matrix

```yaml
strategy:
  fail-fast: false
  matrix:
    browser: [chromium, firefox, webkit]

steps:
  - run: bunx playwright install --with-deps ${{ matrix.browser }}
  - run: bunx playwright test --project=${{ matrix.browser }}
```

---

## With Build Artifact

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: oven-sh/setup-bun@v2
      - run: bun install --frozen-lockfile
      - run: bun run build
      - uses: actions/upload-artifact@v4
        with:
          name: build
          path: dist/

  e2e:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: build
          path: dist/
      - uses: oven-sh/setup-bun@v2
      - run: bun install --frozen-lockfile
      - run: bunx playwright install --with-deps chromium
      - run: bunx playwright test
```

---

## Sharding for Speed

```yaml
strategy:
  matrix:
    shard: [1, 2, 3, 4]

steps:
  - run: bunx playwright test --shard=${{ matrix.shard }}/${{ strategy.job-total }}
```

---

## Trace on Failure

```yaml
- run: bunx playwright test --trace on-first-retry

- uses: actions/upload-artifact@v4
  if: failure()
  with:
    name: traces
    path: test-results/
```
