# Releasing AgentOps

This is repository release policy for the `ao` binary and plugin bundles. It is
outside the AgentOps semantic loop: release checks never create a Validate
verdict, and a verdict never pushes a tag.

## Preconditions

Choose the exact release commit, then run the ordinary deterministic suite:

```bash
bash scripts/ci-local-release.sh --release-version X.Y.Z
bash tests/docs/validate-doc-release.sh
cd cli && go test ./...
```

Inspect the generated release artifacts, the final diff, and the version in the
Go binary and plugin manifests. Local artifacts under ignored directories are
evidence for the operator; they are not tracked lifecycle state.

If this repository's operator wants semantic review of the release candidate,
invoke the Validate skill once from a fresh context against that exact subject.
Record the verdict separately. Do not make the release workflow produce or
strengthen a semantic verdict.

## Publish

Update `CHANGELOG.md`, commit the release changes through normal Git policy,
then create and push the annotated tag:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

`.github/workflows/release.yml` is the publisher-only workflow. It builds the
declared platforms, produces checksums and attestations, publishes the GitHub
release, and updates configured package distribution. It does not merge
branches, close work, or invoke AgentOps semantic validation.

Use `workflow_dispatch` only to retry an existing tag after inspecting the
previous publisher run. Do not start a second publisher for the same tag while
one is active.

## Verify

After the publisher completes:

1. Confirm the GitHub release points to the intended tag and commit.
2. Verify checksums for at least one downloaded artifact.
3. Install the published binary in a clean temporary environment and run
   `ao version` plus one read-only command.
4. Confirm Claude, Codex, and Gemini bundle manifests contain the generated
   metadata-owned skill inventory.

## Failure and rollback

Before publication, fix the candidate and create a new exact release commit.
After publication, prefer a new patch release. Move or recreate a public tag
only when repository policy explicitly permits it and no consumer can have
observed the old artifact.

Release failures are repository failures, not AgentOps `FAIL` or
`NOT_PROVEN` verdicts. Diagnose them with the release workflow logs and the
deterministic commands above.

## Owned surfaces

| Surface | Responsibility |
|---|---|
| `.goreleaser.yml` | binary archives, checksums, and release notes |
| `.github/workflows/release.yml` | tagged publication |
| `.github/workflows/validate.yml` | optional hosted deterministic checks |
| `scripts/ci-local-release.sh` | local deterministic release rehearsal |
| `scripts/validate-release.sh` | built-binary release checks |
| `tests/docs/validate-doc-release.sh` | documentation and generated-reference checks |
