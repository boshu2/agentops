# Go CI Template

Complete CI workflow for Go projects.

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'
      - run: go build -v ./...
      - run: go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

      - name: Enforce coverage threshold
        run: |
          total=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
          echo "Coverage: ${total}%"
          if (( $(echo "$total < 70" | bc -l) )); then
            echo "::error::Coverage $total% is below 70%"
            exit 1
          fi

      - uses: codecov/codecov-action@v4
        with:
          files: coverage.out
          fail_ci_if_error: false
        env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'
      - uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  e2e:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'
      - run: go test -v ./tests/e2e -count=1
```

---

## Key Actions

| Purpose | Action |
|---------|--------|
| Setup | `actions/setup-go@v6` (auto-caches) |
| Lint | `golangci/golangci-lint-action@v4` |
| Release | `goreleaser/goreleaser-action@v6` |

---

## GoReleaser Integration

See [GORELEASER.md](GORELEASER.md) for release automation.
