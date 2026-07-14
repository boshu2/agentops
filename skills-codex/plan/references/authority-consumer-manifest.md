# Authority/Consumer Manifest

Use this contract for every migration, rename, deletion, or ownership transfer.
It makes the full propagation surface a checked planning fact before slices are
admitted or parallelized.

## Manifest shape

```json
{
  "schema_version": 1,
  "migration_id": "stable-id",
  "authorities": [
    {"path": "path/to/owner", "symbols": ["OwnedSymbol"]}
  ],
  "inventory": {
    "command": "rg -l 'OwnedSymbol' .",
    "observed_paths": ["path/to/owner", "path/to/consumer"],
    "complete": true
  },
  "consumers": [
    {
      "path": "path/to/consumer",
      "authority_path": "path/to/owner",
      "kind": "runtime"
    }
  ],
  "slices": [
    {"id": "S1", "write_scope": ["path/to/owner", "path/to/consumer"]}
  ]
}
```

`inventory.command` is audit prose recording the safe argv actually run; the
checker never executes it. Capture that command's output independently as
newline-delimited repository-relative paths. `observed_paths` is the normalized,
deduplicated result after false positives are dispositioned.
Authorities name real symbols. Every observed path is classified exactly once
as an authority or consumer and assigned to at least one slice. Consumer kinds
are `runtime`, `test`, `docs`, `generated`, `schema`, or `fixture`.

## Classification

Run:

```bash
(cd "$REPO_ROOT" && rg -l --fixed-strings -- "$SYMBOL" . | LC_ALL=C sort) \
  >"$INVENTORY_OUTPUT"
python3 skills/plan/scripts/check-authority-consumer-manifest.py \
  --repo "$REPO_ROOT" \
  --inventory-output "$INVENTORY_OUTPUT" \
  path/to/manifest.json
```

The caller owns safe execution and quoting. The checker reads the captured
output only as data and requires its normalized path set to equal
`inventory.observed_paths`; extra live output or an invented observed path
fails closed. Never pass `inventory.command` to `sh`, `eval`, or a subprocess.

The checker returns JSON with one of three classifications:

| Status | Scope classification | Meaning |
|---|---|---|
| `complete` | `disjoint` | Every observed path is classified, exists, has an owner, and slice write scopes do not overlap. Parallel execution may be proposed to Premortem. |
| `complete` | `shared` | The inventory is complete, but at least one path has multiple slice owners. Serialize or merge those slices. |
| `incomplete` | `incomplete` | The inventory was not checked, paths are missing/unclassified/unowned, or the shape is invalid. Fail closed and do not dispatch. |

`parallel_safe:true` means only that the declared write scopes are complete and
disjoint. Premortem still checks shared schemas, migrations, public CLI
surfaces, ordering, ownership, and discard paths before a wave is admitted.

## Update boundary

Re-run the inventory at consumption time. If its observed path set changes,
invalidate the manifest and return to Plan. Do not patch a worker prompt with a
new consumer after dispatch; that silently changes the slice boundary.
