# Python Wheel Building (Maturin)

## Rust + Python Extension

```yaml
name: Python Wheels

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  build-wheels:
    name: Build ${{ matrix.target }}
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-latest
            target: x86_64
          - os: ubuntu-24.04-arm
            target: aarch64
          - os: macos-14
            target: aarch64
          - os: macos-15-intel
            target: x86_64
          - os: windows-latest
            target: x64

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'

      - name: Build wheels
        uses: PyO3/maturin-action@v1
        with:
          target: ${{ matrix.target }}
          args: --release --out dist
          manylinux: auto

      - uses: actions/upload-artifact@v4
        with:
          name: wheels-${{ matrix.os }}-${{ matrix.target }}
          path: dist/*.whl

  sdist:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: PyO3/maturin-action@v1
        with:
          command: sdist
          args: --out dist
      - uses: actions/upload-artifact@v4
        with:
          name: sdist
          path: dist/*.tar.gz

  release:
    needs: [build-wheels, sdist]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - uses: pypa/gh-action-pypi-publish@release/v1
        with:
          password: ${{ secrets.PYPI_API_TOKEN }}
```

---

## Pure Python (uv)

```yaml
name: Release

on:
  push:
    tags: ['v*']

jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      id-token: write  # Trusted publishing
    steps:
      - uses: actions/checkout@v4
      - uses: astral-sh/setup-uv@v7
      - run: uv build
      - uses: pypa/gh-action-pypi-publish@release/v1
```

---

## Trusted Publishing (OIDC)

No API token needed - configure on PyPI:
1. Go to PyPI project settings
2. Add trusted publisher
3. Enter: owner, repo, workflow file, environment (optional)

```yaml
permissions:
  id-token: write

- uses: pypa/gh-action-pypi-publish@release/v1
  # No password needed!
```

---

## Test PyPI First

```yaml
- uses: pypa/gh-action-pypi-publish@release/v1
  with:
    repository-url: https://test.pypi.org/legacy/
```

---

## Multiple Python Versions

```yaml
- uses: PyO3/maturin-action@v1
  with:
    args: --release --out dist --find-interpreter
```
