# Go CLI compatibility baseline

`profiles/*.json` are immutable, normalized `ao capabilities --json` manifests
captured from the execution base in `metadata.json`. Only `tool_version` and the
platform OS/architecture are normalized; timestamps, absolute paths, unstable
ordering, or any other ambient field fail capture.

Before a command family moves, `families/<name>/case.json` and
`ownership.json` are committed without production edits. A following
`lineage.json` binds their digests to that fixture commit. Evidence checks are
executable shell assertions. Every compatibility dimension must cite at least
one check or give a reviewed `not_applicable` reason. Once frozen, case and
ownership files cannot change during the migration.
