# Domain-Slice Manifests

This directory holds per-domain `manifest.yaml` files — durable, git-tracked
declarations of DDD bounded context slices used by `ao rpi phased --domain`.

## One directory per domain

```
docs/domains/
  <domain-name>/
    manifest.yaml      # source of truth for this domain slice
```

The `<domain-name>` matches the `domain` field in the manifest and is passed
directly to `--domain`:

```
ao rpi phased --domain goals "Add satisfaction gate to ao goals measure"
ao rpi phased --domain rpi   "Wire domainSliceManifest into phased loader"
```

## What a manifest declares

Each `manifest.yaml` validates against
`schemas/domain-slice-manifest.v1.schema.json` and records:

| Field | Purpose |
|---|---|
| `domain` | Short machine-readable name (matches this directory name) |
| `bounded_context` | One-sentence DDD statement: what this slice owns and does NOT own |
| `directive_ids` | Stable GOALS.md directive IDs (`d-<slug>`) whose acceptance this slice owns |
| `scenario_ids` | Promoted spec scenario IDs from `spec/scenarios/` |
| `context_roots` | Repo-relative implementation paths loaded as agent context |
| `allowed_read_globs` | Read-fence allow list for agents working in this slice |
| `denied_read_globs` | Read-fence deny list (overrides allowed globs when both match) |
| `validation_commands` | Ordered validation steps run after each implementation phase |
| `owner` | Team or person responsible for this slice |

## How it fits with other domain surfaces

The manifest is **not** a replacement for:

- **`skills/domain/SKILL.md`** — the ubiquitous language vocabulary (nouns and
  structural primitives). Load it for terminology; the manifest uses those terms.
- **`docs/contracts/context-map.md`** — the generated architecture view of skill
  relationships by hexagonal role. It shows how skills relate; the manifest
  scopes which files an agent loads.
- **Skill frontmatter** (`hexagonal_role`, `consumes`, `produces`) — per-skill
  classification. The manifest declares the aggregate scope of a slice that may
  span multiple skills.

See [ADR-0004](../adr/ADR-0004-domain-slice-manifest-contract.md) for the full
reconciliation and the rationale for each design decision.

## Adding a new domain slice

1. Create `docs/domains/<name>/manifest.yaml`.
2. Validate it: `python3 -c "import jsonschema, json, yaml; jsonschema.validate(yaml.safe_load(open('docs/domains/<name>/manifest.yaml')), json.load(open('schemas/domain-slice-manifest.v1.schema.json')))"`.
3. Commit the manifest (it is a durable tracked artifact per ADR-0003).
4. Optionally promote the example below as a starting point.

## Example

See [`example/manifest.yaml`](example/manifest.yaml) for a fully populated,
schema-valid example. It uses the `goals` domain as a worked illustration.
