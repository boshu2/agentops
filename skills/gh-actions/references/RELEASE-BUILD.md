# Release Build Workflows

## Table of Contents
- [Rust Release](#rust-release-cross-platform-binaries)
- [Go Release (GoReleaser)](#go-release-goreleaser)
- [TypeScript/Bun Release](#typescriptbun-release)

---

## Rust Release (Cross-Platform Binaries)

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

env:
  CARGO_TERM_COLOR: always

jobs:
  build:
    name: Build ${{ matrix.target }}
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-latest
            target: x86_64-unknown-linux-gnu
            artifact: myapp
            asset: myapp-linux-amd64
          - os: ubuntu-24.04-arm
            target: aarch64-unknown-linux-gnu
            artifact: myapp
            asset: myapp-linux-arm64
          - os: macos-15-intel
            target: x86_64-apple-darwin
            artifact: myapp
            asset: myapp-darwin-amd64
          - os: macos-14
            target: aarch64-apple-darwin
            artifact: myapp
            asset: myapp-darwin-arm64
          - os: windows-latest
            target: x86_64-pc-windows-msvc
            artifact: myapp.exe
            asset: myapp-windows-amd64.exe
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
        with:
          targets: ${{ matrix.target }}
      - uses: Swatinem/rust-cache@v2
        with:
          key: release-${{ matrix.target }}

      - name: Build release binary
        run: cargo build --release --target ${{ matrix.target }}

      - name: Create tarball (Unix)
        if: runner.os != 'Windows'
        run: |
          mkdir -p dist
          cd target/${{ matrix.target }}/release
          tar -cJf ../../../dist/${{ matrix.asset }}.tar.xz ${{ matrix.artifact }}
          cd ../../../dist
          shasum -a 256 "${{ matrix.asset }}.tar.xz" > "${{ matrix.asset }}.tar.xz.sha256"

      - name: Create zip (Windows)
        if: runner.os == 'Windows'
        shell: pwsh
        run: |
          New-Item -ItemType Directory -Force -Path dist
          Compress-Archive -Path "target/${{ matrix.target }}/release/${{ matrix.artifact }}" -DestinationPath "dist/${{ matrix.asset }}.zip"
          cd dist
          $hash = (Get-FileHash -Algorithm SHA256 "${{ matrix.asset }}.zip").Hash.ToLower()
          "$hash  ${{ matrix.asset }}.zip" | Out-File -Encoding ASCII "${{ matrix.asset }}.zip.sha256"

      - uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.asset }}
          path: dist/*

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - name: Extract version
        id: version
        run: echo "version=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

      - uses: softprops/action-gh-release@v2
        with:
          name: v${{ steps.version.outputs.version }}
          generate_release_notes: true
          files: dist/*

    outputs:
      version: ${{ steps.version.outputs.version }}

  notify-homebrew:
    needs: release
    runs-on: ubuntu-latest
    steps:
      - uses: peter-evans/repository-dispatch@v3
        with:
          token: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          repository: owner/homebrew-tap
          event-type: formula-update
          client-payload: '{"tool": "myapp", "version": "${{ needs.release.outputs.version }}"}'
```

---

## Go Release (GoReleaser)

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
          fetch-depth: 0  # Required for changelog
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'
      - uses: goreleaser/goreleaser-action@v5
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

See [GORELEASER.md](GORELEASER.md) for `.goreleaser.yaml` configuration.

---

## TypeScript/Bun Release

```yaml
name: Release

on:
  push:
    tags: ['v*']
  workflow_dispatch:

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: oven-sh/setup-bun@v2
        with:
          bun-version: latest
      - run: bun install --frozen-lockfile

      - name: Build all platforms
        env:
          VERSION: ${{ github.ref_name }}
          GIT_SHA: ${{ github.sha }}
        run: |
          set -euo pipefail
          mkdir -p dist
          BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

          build() {
            local target="$1" outfile="$2"
            echo "Building $outfile..."
            MY_VERSION="$VERSION" MY_SHA="$GIT_SHA" MY_DATE="$BUILD_DATE" \
              bun build --compile --minify --env=MY_* \
              --target="$target" --outfile="dist/$outfile" ./cli.ts
          }

          build bun-linux-x64-baseline cli-linux-x64
          build bun-linux-arm64 cli-linux-arm64
          build bun-darwin-arm64 cli-darwin-arm64
          build bun-darwin-x64 cli-darwin-x64
          build bun-windows-x64-baseline cli-win-x64.exe

          cd dist
          for f in cli-*; do
            sha256sum "$f" > "$f.sha256"
          done

      - uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
          files: dist/*
```
