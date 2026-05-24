# rg-optimized — Building ripgrep from source with PCRE2

A short operator note for when the system `rg` is not enough.

## Why build from source

Most distro packages ship `ripgrep` with the default Rust regex engine only. That engine is fast and Unicode-aware, but it deliberately omits a few features that PCRE2 supports:

- Lookahead and lookbehind (`(?=...)`, `(?<=...)`)
- Atomic groups (`(?>...)`)
- Backreferences (`\1`, `\2`, ...)
- Some Unicode property escapes used by editor-style regex

When `rg -P 'pattern'` returns `PCRE2 is not available in this build of ripgrep`, the binary on PATH was compiled without the `pcre2` feature. Building from upstream `BurntSushi/ripgrep` with `--features pcre2` enables the `-P` flag and links the system `libpcre2`. A release build at the same time gives you a binary tuned to the local toolchain and CPU.

## When to run

Use this script when any of the following is true:

- `rg -P` fails with a "PCRE2 is not available" error.
- The packaged `rg --version` is older than 14.x and you need recent fixes.
- You want a single, reproducible local rg across machines without trusting whatever the distro pinned.
- You need maximum throughput on large repos and the packaged build was compiled with conservative defaults.

If `rg --version` already prints `PCRE2 ... is available`, you can skip this entirely.

## Install

From the repo root:

```bash
bash scripts/build-rg.sh
```

This clones `BurntSushi/ripgrep` at the pinned tag (default `14.1.1`), runs `cargo build --release --features 'pcre2'`, and installs the resulting binary to `~/.local/bin/rg`. Re-running the script is safe; it updates the source tree, rebuilds against the requested tag, and replaces the installed binary atomically.

Common overrides:

| Variable | Default | Effect |
|----------|---------|--------|
| `RG_VERSION` | `14.1.1` | Upstream git tag to check out |
| `RG_INSTALL_DIR` | `~/.local/bin` | Where the `rg` binary is written |
| `RG_SRC_DIR` | `~/.cache/agentops/ripgrep-src` | Where the source clone lives |
| `ASSUME_YES` | `0` | Set to `1` to skip the rustup install confirmation |

The script aborts on Windows with a pointer to `winget` / `choco`. If `cargo` is missing, it asks before running rustup.

## Verify

```bash
which rg
rg --version
rg -P '(?<=foo)bar' /dev/null   # should not error; PCRE2 lookbehind
```

A working build prints a `features:` line that mentions `+pcre2` and a `PCRE2 ... is available` line at the bottom of `rg --version`. The script fails with a non-zero exit code if either is missing, so a clean script run is itself the verification.

If `~/.local/bin` is not on your PATH, the script prints a reminder. Add it to your shell rc (`export PATH="$HOME/.local/bin:$PATH"`) so the new `rg` shadows the system copy.

## Troubleshooting

- **`cargo not found`** — let the script invoke rustup, or install Rust first via your usual channel.
- **Linker error about `pcre2`** — on Linux install `libpcre2-dev` (Debian/Ubuntu) or `pcre2-devel` (Fedora/RHEL); on macOS install `pcre2` via Homebrew. The `pcre2` cargo feature still vendors the library when system headers are missing, but a system install gives a faster, cleaner build.
- **`Text file busy` during install** — close any process using `rg` (file watchers, editors) and re-run.

## Related

- Upstream README: `https://github.com/BurntSushi/ripgrep`
- The `rg-optimized` external skill — pattern source for this doc and the build script.

---
> Pattern adopted from `rg-optimized` (ACFS skill corpus). Methodology only — no verbatim text.
