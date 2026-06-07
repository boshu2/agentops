# Coverage Patterns

## Table of Contents
- [Rust Coverage (llvm-cov)](#rust-coverage-llvm-cov)
- [Go Coverage](#go-coverage)
- [Per-Package Thresholds](#per-package-thresholds)
- [Codecov Integration](#codecov-integration)

---

## Rust Coverage (llvm-cov)

```yaml
name: Coverage

on:
  push:
    branches: [main]
  pull_request:

jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@nightly
        with:
          components: llvm-tools-preview
      - uses: taiki-e/install-action@cargo-llvm-cov

      - name: Generate coverage
        run: |
          cargo llvm-cov --workspace \
            --ignore-filename-regex "(tests/|benches/)" \
            --codecov \
            --output-path codecov.json

      - uses: codecov/codecov-action@v4
        with:
          files: codecov.json
          fail_ci_if_error: false
        env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}

      - name: Check threshold
        run: |
          cargo llvm-cov --json --output-path cov.json
          COVERAGE=$(jq -r '.data[0].totals.lines.percent // 0' cov.json)
          echo "## Coverage: ${COVERAGE}%" >> $GITHUB_STEP_SUMMARY
          if (( $(echo "$COVERAGE < 70" | bc -l) )); then
            echo "::error::Coverage $COVERAGE% is below 70%"
            exit 1
          fi
```

---

## Go Coverage

```yaml
- run: go test -v -covermode=atomic -coverprofile=coverage.out ./...

- name: Enforce threshold
  run: |
    total=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
    echo "Coverage: ${total}%"
    if (( $(echo "$total < 70" | bc -l) )); then
      echo "::error::Coverage below threshold"
      exit 1
    fi

- run: go tool cover -html=coverage.out -o coverage.html

- uses: actions/upload-artifact@v4
  with:
    name: coverage-report
    path: coverage.html
```

---

## Per-Package Thresholds

```yaml
- name: Enforce per-package thresholds
  run: |
    go tool cover -func=coverage.out | awk '
      /^total:/ { next }
      { pkg=$1; pct=$3; gsub(/%/,"",pct); coverage[pkg]=pct }
      END {
        thresholds["pkg/core"]=80
        thresholds["pkg/api"]=75
        thresholds["pkg/utils"]=60
        fail=0
        for (pkg in thresholds) {
          if (coverage[pkg] < thresholds[pkg]) {
            printf "FAIL: %s at %.1f%% (need %.0f%%)\n", pkg, coverage[pkg], thresholds[pkg]
            fail=1
          }
        }
        exit fail
      }
    '
```

---

## Codecov Integration

```yaml
- uses: codecov/codecov-action@v4
  with:
    files: coverage.json,coverage.out
    fail_ci_if_error: false
    verbose: true
  env:
    CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```
