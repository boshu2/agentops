# Python CI Template

Complete CI workflow for Python projects using uv.

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  PYTHON_VERSION: '3.13'

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: astral-sh/setup-uv@v7
        with:
          python-version: ${{ env.PYTHON_VERSION }}
      - run: uv sync --locked
      - run: uv run ruff check src/
      - run: uv run mypy src/ --ignore-missing-imports

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: astral-sh/setup-uv@v7
        with:
          python-version: ${{ env.PYTHON_VERSION }}
          enable-cache: true
          cache-dependency-glob: uv.lock
      - run: uv sync --locked
      - run: uv run pytest tests/ -v --tb=short
```

---

## Key Actions

| Purpose | Action |
|---------|--------|
| Setup | `astral-sh/setup-uv@v7` |
| Lint | `uv run ruff check` |
| Type check | `uv run mypy` |
| Test | `uv run pytest` |

---

## PyPI Publishing

```yaml
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

## Maturin (Rust + Python)

See [PYTHON-WHEELS.md](PYTHON-WHEELS.md) for Rust extension wheels.
