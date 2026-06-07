# OCI/Docker Build Patterns

## Multi-Platform Build

```yaml
name: OCI

on:
  push:
    tags: ['v*']

permissions:
  contents: read
  packages: write
  id-token: write

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3

      - uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha

      - uses: docker/build-push-action@v6
        id: build
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

---

## Cosign Signing

```yaml
- uses: sigstore/cosign-installer@v3

- name: Sign image
  env:
    COSIGN_EXPERIMENTAL: "1"
  run: |
    cosign sign --yes \
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }}
```

---

## SBOM Attestation

```yaml
- name: Generate SBOM
  run: |
    syft ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }} \
      -o spdx-json > sbom.spdx.json

- name: Attest SBOM
  env:
    COSIGN_EXPERIMENTAL: "1"
  run: |
    cosign attest --yes \
      --predicate sbom.spdx.json \
      --type spdx \
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }}
```

---

## Build Args

```yaml
- uses: docker/build-push-action@v6
  with:
    build-args: |
      VERSION=${{ github.ref_name }}
      COMMIT=${{ github.sha }}
      BUILD_DATE=${{ github.event.head_commit.timestamp }}
```

---

## Dockerfile Best Practices

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/myapp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/myapp /myapp
ENTRYPOINT ["/myapp"]
```

---

## Matrix for Multiple Images

```yaml
strategy:
  matrix:
    include:
      - context: ./api
        image: myorg/api
      - context: ./worker
        image: myorg/worker

steps:
  - uses: docker/build-push-action@v6
    with:
      context: ${{ matrix.context }}
      tags: ghcr.io/${{ matrix.image }}:${{ github.ref_name }}
```
