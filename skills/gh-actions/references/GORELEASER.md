# GoReleaser Patterns

## Basic .goreleaser.yaml

```yaml
version: 2
project_name: myapp

builds:
  - main: ./cmd/myapp
    binary: myapp
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}
      - -X main.Commit={{.ShortCommit}}
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude: ["^docs:", "^test:", "^chore:"]
```

---

## GitHub Actions Integration

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

---

## Homebrew Tap Publishing

```yaml
brews:
  - name: myapp
    repository:
      owner: myorg
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: https://github.com/myorg/myapp
    description: "My CLI tool"
    license: MIT
    install: bin.install "myapp"
    test: system "#{bin}/myapp", "--version"
```

---

## Scoop Bucket Publishing

```yaml
scoops:
  - name: myapp
    repository:
      owner: myorg
      name: scoop-bucket
    homepage: https://github.com/myorg/myapp
    description: "My CLI tool"
    license: MIT
```

---

## Signing with Cosign

```yaml
signs:
  - cmd: cosign
    env: [COSIGN_EXPERIMENTAL=1]
    certificate: "${artifact}.cert"
    args: [sign-blob, --output-certificate=${certificate}, --output-signature=${signature}, "${artifact}", --yes]
    artifacts: checksum

docker_signs:
  - cmd: cosign
    env: [COSIGN_EXPERIMENTAL=1]
    artifacts: manifests
    args: [sign, "${artifact}", --yes]
```

---

## SBOM Generation

```yaml
sboms:
  - artifacts: archive
    cmd: syft
    args: ["${artifact}", --output, "spdx-json=${document}"]
```
