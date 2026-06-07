# Release Extras

## Table of Contents
- [Signed Release with SLSA](#signed-release-with-slsa)
- [Version Validation](#version-validation)
- [Install Script Inclusion](#install-script-inclusion)

---

## Signed Release with SLSA

Complete workflow combining signing, SBOM, and provenance.

```yaml
name: Secure Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write
  packages: write
  id-token: write

env:
  COSIGN_VERSION: v2.2.4
  SYFT_VERSION: v1.4.1

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      # ... build steps ...

      - name: Generate checksums
        run: sha256sum dist/* > dist/SHA256SUMS

      - name: Sign with minisign
        env:
          MINISIGN_SECRET_KEY: ${{ secrets.MINISIGN_SECRET_KEY }}
        run: |
          echo "$MINISIGN_SECRET_KEY" > key.sec
          minisign -S -s key.sec -m dist/SHA256SUMS
          rm key.sec

      - uses: anchore/sbom-action/download-syft@v0
        with:
          syft-version: ${{ env.SYFT_VERSION }}

      - name: Generate SBOM
        run: syft dir:. -o spdx-json > dist/sbom.spdx.json

      - uses: actions/attest-build-provenance@v2
        with:
          subject-path: dist/*

      - uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/*
            dist/SHA256SUMS.minisig
            dist/sbom.spdx.json
```

---

## Version Validation

Ensure tag matches version in source files.

```yaml
- name: Extract version from tag
  id: version
  run: echo "version=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

- name: Verify VERSION file matches tag
  run: |
    file_version=$(cat VERSION | tr -d '\r\n')
    if [[ "$file_version" != "${{ steps.version.outputs.version }}" ]]; then
      echo "::error::VERSION file ($file_version) != tag (${{ steps.version.outputs.version }})"
      exit 1
    fi

- name: Verify script version matches tag
  run: |
    script_version=$(grep -m1 'VERSION=' myscript | cut -d'"' -f2)
    if [[ "$script_version" != "${{ steps.version.outputs.version }}" ]]; then
      echo "::error::Script VERSION != tag"
      exit 1
    fi
```

---

## Install Script Inclusion

Bundle install scripts with release.

```yaml
- name: Copy install scripts
  run: |
    cp install.sh dist/
    cp install.ps1 dist/ 2>/dev/null || true
    cd dist
    sha256sum install.sh > install.sh.sha256
```
