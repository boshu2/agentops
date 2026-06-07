# Signing and Provenance for GitHub Actions

## Table of Contents
- [Sigstore Keyless Signing](#sigstore-keyless-signing)
- [Minisign Signing](#minisign-signing)
- [SLSA Build Provenance](#slsa-build-provenance)
- [SBOM Generation](#sbom-generation)
- [Security Scanning](#security-scanning)

---

## Sigstore Keyless Signing

No keys to manage - uses OIDC identity from GitHub Actions.

### Sign Container Images

```yaml
permissions:
  id-token: write
  packages: write

steps:
  - uses: sigstore/cosign-installer@v3
    with:
      cosign-release: v2.2.4

  - name: Build and push image
    id: build
    uses: docker/build-push-action@v5
    with:
      push: true
      tags: ghcr.io/owner/app:${{ github.sha }}

  - name: Sign image (keyless)
    env:
      COSIGN_EXPERIMENTAL: "1"
    run: |
      cosign sign --yes ghcr.io/owner/app@${{ steps.build.outputs.digest }}
```

### Sign Binary Artifacts

```yaml
- name: Sign with cosign
  env:
    COSIGN_EXPERIMENTAL: "1"
  run: |
    cosign sign-blob --yes dist/myapp-linux-amd64 \
      --output-signature dist/myapp-linux-amd64.sig \
      --output-certificate dist/myapp-linux-amd64.cert
```

---

## Minisign Signing

Self-managed key for deterministic, offline-verifiable signatures.

```yaml
- name: Sign with minisign
  env:
    MINISIGN_SECRET_KEY: ${{ secrets.MINISIGN_SECRET_KEY }}
  run: |
    printf "%s" "$MINISIGN_SECRET_KEY" > minisign.key
    chmod 600 minisign.key
    minisign -S -s minisign.key -m dist/SHA256SUMS -x dist/SHA256SUMS.minisig
    rm minisign.key
```

### Generate minisign keypair (one-time setup)

```bash
minisign -G -p minisign.pub -s minisign.key
# Store minisign.key content in MINISIGN_SECRET_KEY secret
# Publish minisign.pub in your repo
```

---

## SLSA Build Provenance

### Using GitHub's Official Action (Level 3)

```yaml
permissions:
  contents: write
  id-token: write
  attestations: write

steps:
  - name: Build artifacts
    run: make build

  - uses: actions/attest-build-provenance@v2
    with:
      subject-path: dist/*
```

### Using slsa-github-generator

```yaml
jobs:
  build:
    outputs:
      hashes: ${{ steps.hash.outputs.hashes }}
    steps:
      - run: make build
      - id: hash
        run: |
          cd dist
          HASHES=$(sha256sum * | base64 -w0)
          echo "hashes=$HASHES" >> $GITHUB_OUTPUT
      - uses: actions/upload-artifact@v4
        with:
          name: artifacts
          path: dist/

  provenance:
    needs: build
    permissions:
      actions: read
      id-token: write
      contents: write
    uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.0.0
    with:
      base64-subjects: ${{ needs.build.outputs.hashes }}
```

---

## SBOM Generation

### Using Syft

```yaml
- uses: anchore/sbom-action/download-syft@v0
  with:
    syft-version: v1.4.1

- name: Generate SBOM (SPDX)
  run: syft dir:. -o spdx-json > sbom.spdx.json

- name: Generate SBOM (CycloneDX)
  run: syft dir:. -o cyclonedx-json > sbom.cdx.json
```

### For Container Images

```yaml
- name: Generate image SBOM
  run: syft ghcr.io/owner/app@${{ steps.build.outputs.digest }} -o spdx-json > sbom.spdx.json

- name: Attach SBOM attestation
  env:
    COSIGN_EXPERIMENTAL: "1"
  run: cosign attest --yes --predicate sbom.spdx.json --type spdx $IMAGE_DIGEST
```

---

## Security Scanning

### Dependency Audit (Rust)

```yaml
- run: cargo install cargo-audit
- run: cargo audit
```

### Dependency Audit (npm)

```yaml
- run: npm audit --audit-level=high
```

### Code Scanning (CodeQL)

```yaml
- uses: github/codeql-action/init@v3
  with:
    languages: javascript, python
- uses: github/codeql-action/autobuild@v3
- uses: github/codeql-action/analyze@v3
```
