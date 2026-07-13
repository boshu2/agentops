# Homebrew Tap for AgentOps — pointer only

**This directory does not contain the live Homebrew formula.**

The real tap lives at **[github.com/boshu2/homebrew-agentops](https://github.com/boshu2/homebrew-agentops)**
and its `Formula/agentops.rb` is generated automatically by **GoReleaser** on every
release tag (see the `brews:` block in [`.goreleaser.yml`](../.goreleaser.yml) and the
[Release Publisher workflow](../.github/workflows/release.yml)). The generated formula
carries the correct version, `license "Apache-2.0"`, and per-artifact `sha256` sums.

Do **not** re-add a hand-written `Formula/agentops.rb` here. A checked-in formula
drifts silently from the release — it once shipped `version "2.31.0"` +
`license "MIT"` while the repo was 3.2.0 / Apache-2.0, so anyone browsing the repo
read the wrong license. `scripts/check-release-parity.sh` guards against a
reintroduced in-repo formula that disagrees with `LICENSE` or the release tag.

## Install

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
```

Or directly:

```bash
brew install boshu2/agentops/agentops
```

## Update

```bash
brew update && brew upgrade agentops
ao version
```
