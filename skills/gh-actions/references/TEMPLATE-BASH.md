# Bash CI Template

Complete CI workflow for Bash script projects.

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  shellcheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ludeeus/action-shellcheck@master
        with:
          scandir: '.'
          severity: warning
          additional_files: 'myscript install.sh'

  syntax:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Check script syntax
        run: |
          for script in *.sh scripts/*.sh; do
            [ -f "$script" ] && bash -n "$script"
          done

  tests:
    needs: [shellcheck, syntax]
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - name: Install bash 5 (macOS)
        if: runner.os == 'macOS'
        run: |
          brew install bash
          echo "$(brew --prefix)/bin" >> $GITHUB_PATH

      - name: Run tests
        run: ./scripts/run_tests.sh

  install-test:
    needs: [shellcheck, syntax]
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - name: Test installer
        run: |
          chmod +x install.sh
          DEST=/tmp/test-install ./install.sh
          test -x /tmp/test-install/myscript
```

---

## Key Actions

| Purpose | Action |
|---------|--------|
| Lint | `ludeeus/action-shellcheck@master` |
| Syntax | `bash -n script.sh` |

---

## ShellCheck Severity Levels

```yaml
with:
  severity: error    # Only errors
  severity: warning  # Errors + warnings (recommended)
  severity: info     # All issues
  severity: style    # Everything including style
```
