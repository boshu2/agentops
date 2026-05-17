# JSM CLI Capability Map

This map documents the `jsm` command surfaces useful for clean-room skill-quality extraction. It separates read-only analysis commands from mutating commands that must not run during standards extraction.

## Safe Read-Only Commands

| Phase | Commands | Output To Capture | Notes |
|---|---|---|---|
| Command discovery | `jsm --help`, `jsm <cmd> --help` | command list, flags, JSON/offline support | Start here before using a new subcommand. |
| Inventory | `jsm list --json`, `jsm verify --json` | installed count, pinned/update state, verification status | `verify` is integrity, not marketplace readiness. |
| Metadata | `jsm show <skill> --json`, `jsm info <skill> --json` | version, hash, license, policy, tags | May require network. Do not copy proprietary content fields. |
| Search | `jsm search <query> --json` | candidate skill names and metadata | Online search may hit the subscription service. |
| Relationships | `jsm related <skill> --json` | pairs, alternatives, prerequisites, extensions | Useful for finding adjacent methodology skills. |
| Graph | `jsm graph insights --json`, `jsm graph cycles --json` | node/edge counts, clusters, cycles, keystones | Full graph exports can be large. |
| Validation | `jsm validate <skill-dir> --json` | success flag, errors, warnings | External publishing gate; may differ from AgentOps repo rules. |
| Security | `jsm security status --json`, targeted `jsm security scan` | ACIP status and findings | Prefer exact installed skill names or exact files. |
| Effectiveness | `jsm effectiveness show --skill <slug> --json` | local outcome aggregates | Read-only show is safe; recording outcomes is mutating. |
| CASS | `jsm cass status --json`, `jsm cass search <query> --json`, `jsm cass mine <topic> --dry-run --json` | availability, session matches, dry-run pattern preview | CASS may contain sensitive session content. Summarize only themes. |

## Mutating Commands To Avoid

Do not run these during clean-room analysis:

| Command | Risk |
|---|---|
| `jsm install`, `jsm install-all` | changes local skill installs |
| `jsm sync` | reconciles local skills with cloud state |
| `jsm push` | publishes/uploads local skill content |
| `jsm upgrade`, `jsm rollback` | changes installed skill versions |
| `jsm pin`, `jsm unpin` | changes update policy |
| `jsm effectiveness record` | writes local outcome metrics |
| `jsm cass mark`, `jsm cass unmark` | mutates session-mining state |
| `jsm cass mine` without `--dry-run` | may generate skill drafts from sessions |

## Observed Local State

Inspection on 2026-05-16 showed:

| Command | Result |
|---|---|
| `jsm --version` | `jsm 0.3.6` |
| `jsm verify --json` | 118 verified, 0 failed |
| `jsm graph insights --json` | 118 nodes, 13,806 edges, 1 cluster, 0 cycles |
| `jsm security status --json` | ACIP enabled, bundled baseline |
| `jsm cass status --json` | CASS unavailable locally |

## Workflow Use

### Extraction Discovery

```bash
jsm --help
jsm graph insights --json
jsm graph cycles --json
jsm related operationalizing-expertise --json
```

Use this to find methodology skills, graph structure, and relationship hints.

### Package Quality

```bash
jsm validate /path/to/skill --json
jsm verify --json
```

Use `validate` for package readiness and `verify` for installed corpus integrity.

### Export Readiness

```bash
scripts/check-jsm-export.sh --json skills/<name>
```

This AgentOps wrapper copies the skill to a temporary directory, normalizes exported script modes, and then runs `jsm validate` without touching source files.

### Inventory

```bash
scripts/inventory-jsm-skills.sh --json
scripts/inventory-jsm-skills.sh --backup-root /Users/bo/backups/jsm-skills-20260516-090905 --markdown
```

Use this for repeatable structural counts across live installs or backup archives.

## Caveats

- Offline mode is not a guarantee that every subcommand avoids network lookups.
- Some graph commands resolve by UUID more reliably than by slug.
- Full graph export can be large because the local graph is dense.
- `jsm validate` reports publishing constraints; AgentOps repo-runtime constraints are separate.
- CASS, when installed, can expose sensitive session content. Treat it as opt-in and summarize only non-sensitive patterns.

---

**Source:** Pattern-only inspection of the local `jsm` CLI and installed JSM corpus. No proprietary source text copied.
