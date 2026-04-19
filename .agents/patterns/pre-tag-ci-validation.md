---
maturity: established
utility: 0.80
---

# Pre-Tag CI Validation

## Pattern

Run both the local release gate and the remote CI pipeline to green on HEAD
before creating a release tag. Neither gate alone is sufficient — local and
remote exercise different surfaces (secret scan, schema, doc-release-gate,
skill-lint line limits, cli-docs-parity, JSON permissions), and failures
surface incrementally if you only check one.

## Why It Works

Release tags are expensive to rewrite. Each retag costs a force-push plus
downstream artifact re-builds (GoReleaser, Homebrew tap, release notes), and
every rewrite breaks consumers who already pinned the tag. Running both gates
before tagging converts an incremental, multi-hour failure cascade into a
single pre-flight check measured in minutes.

## How To Apply

1. Run `scripts/ci-local-release.sh` locally and resolve every failure.
2. Push HEAD to the release branch and watch the remote CI pipeline turn
   green — do not tag off a yellow or red run.
3. Keep `docs/CHANGELOG.md` and root `CHANGELOG.md` in sync before tagging.
   The doc-release-gate inspects `docs/CHANGELOG.md` specifically.
4. Only then run `git tag vX.Y.Z && git push origin vX.Y.Z`.
5. If a post-tag failure slips through, prefer `scripts/retag-release.sh` and
   capture the root cause in a new pre-flight gate rather than re-retagging.

## Retrieval Cues

Release tag validation, pre-tag CI, tag rewrite, force-push release, retag,
GoReleaser pipeline, ci-local-release gate, doc-release-gate, release
readiness, skill-lint line limit, cli-docs-parity, secret-scan, release
cascade failure.
