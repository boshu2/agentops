# Testing Patterns

## Table of Contents
- [Go Fuzzing](#go-fuzzing)
- [Rust Fuzzing](#rust-fuzzing)
- [Benchmarks](#benchmarks)
- [Static Analysis](#static-analysis)
- [Checksum Health](#checksum-health)

---

## Go Fuzzing

```yaml
name: Fuzz

on:
  schedule:
    - cron: '0 3 * * *'  # Nightly
  workflow_dispatch:
    inputs:
      fuzz_time:
        description: 'Duration per target'
        default: '10m'

jobs:
  fuzz:
    runs-on: ubuntu-latest
    timeout-minutes: 120
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'

      - run: mkdir -p fuzz-results

      - name: Fuzz ParseInput
        run: |
          go test -fuzz=FuzzParseInput \
            -fuzztime=${{ inputs.fuzz_time || '10m' }} \
            ./pkg/parser/... 2>&1 | tee fuzz-results/parse.log || true

      - name: Check for crashes
        run: |
          if grep -l "panic:" fuzz-results/*.log 2>/dev/null; then
            echo "::error::Fuzz test found panics!"
            exit 1
          fi

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: fuzz-results
          path: fuzz-results/
          retention-days: 30
```

---

## Rust Fuzzing

```yaml
name: Fuzz

on:
  schedule:
    - cron: '0 2 * * 0'  # Weekly Sunday
  workflow_dispatch:

jobs:
  fuzz:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        target: [parse_input, validate_config]
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@nightly
      - run: cargo install cargo-fuzz

      - name: Run fuzzer
        run: cargo fuzz run ${{ matrix.target }} -- -max_total_time=3600
        continue-on-error: true

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: crashes-${{ matrix.target }}
          path: fuzz/artifacts/${{ matrix.target }}/
```

---

## Benchmarks

### Rust

```yaml
- run: cargo bench --bench perf_tests -- --noplot

- name: Compare to main
  if: github.event_name == 'pull_request'
  run: |
    cargo bench --bench perf_tests -- --save-baseline pr
    git checkout main
    cargo bench --bench perf_tests -- --save-baseline main
    cargo bench --bench perf_tests -- --load-baseline main --baseline pr
```

### Go

```yaml
- run: go test -bench=. -benchmem -count=5 ./... | tee bench.txt

- name: Check for regressions
  run: |
    go test -bench=BenchmarkCriticalPath -benchmem ./pkg/core/... | \
      awk '/BenchmarkCriticalPath/ { if ($3 > 1000000) { print "SLOW"; exit 1 } }'
```

---

## Static Analysis

### UBS Integration

```yaml
- name: Prepare changed files
  id: files
  run: |
    BASE="${{ github.event.pull_request.base.sha || github.event.before }}"
    mapfile -t files < <(git diff --name-only "$BASE" HEAD | grep -E '\.(ts|js|rs|py)$')
    if [[ ${#files[@]} -gt 0 ]]; then
      echo "count=${#files[@]}" >> $GITHUB_OUTPUT
      printf '%s\n' "${files[@]}" > changed_files.txt
    else
      echo "count=0" >> $GITHUB_OUTPUT
    fi

- name: Run UBS
  if: steps.files.outputs.count != '0'
  run: xargs ubs --ci < changed_files.txt
```

### Clippy

```yaml
- uses: actions-rs/clippy-check@v1
  with:
    token: ${{ secrets.GITHUB_TOKEN }}
    args: --all-targets --all-features -- -D warnings
```

---

## Checksum Health

```yaml
name: Checksum Health

on:
  schedule:
    - cron: '0 6,18 * * *'
  workflow_dispatch:

permissions:
  contents: read
  issues: write

jobs:
  check:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v4

      - name: Verify checksums
        id: verify
        run: |
          ACTUAL=$(sha256sum install.sh | awk '{print $1}')
          EXPECTED=$(grep 'install.sh$' SHA256SUMS | awk '{print $1}')
          if [[ "$ACTUAL" != "$EXPECTED" ]]; then
            echo "healthy=false" >> $GITHUB_OUTPUT
          else
            echo "healthy=true" >> $GITHUB_OUTPUT
          fi

      - name: Create issue if unhealthy
        if: steps.verify.outputs.healthy == 'false'
        run: |
          gh issue create \
            --title "URGENT: Checksum drift detected" \
            --label "checksum-drift" \
            --body "SHA256SUMS out of sync. Run ./scripts/update_checksums.sh"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
