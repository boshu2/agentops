# AGENTS-CODEX.md — Codex skill parity, CLI skill-map maintenance, audit scripts

> Sibling of [`AGENTS.md`](AGENTS.md), [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CI.md`](AGENTS-CI.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md). Split out of the monolithic AGENTS.md per soc-vuu6.3.

### CLI Skill-Map Refresh

After changing `ao` command usage in any of these locations, refresh [`docs/cli-skills-map.md`](docs/cli-skills-map.md):

- `skills/*/SKILL.md`
- `skills-codex/*/SKILL.md`

Process:
1. Update this map from current sources.
2. Run `bash tests/docs/validate-doc-release.sh` and `bash tests/docs/validate-skill-count.sh` before pushing.

### Codex Skill Maintenance

Codex is a first-class runtime in this repo.

- `skills/<name>/SKILL.md` is the canonical behavior contract.
- `skills-codex-overrides/<name>/` is the Codex-specific tailoring layer.
- `skills-codex-overrides/catalog.json` is the machine-readable treatment map for the full catalog.
- `skills-codex/<name>/` is the checked-in Codex runtime artifact. It is manually maintained, while the legacy manifest/marker files remain part of the validation contract.

> **Editing an EXISTING parity skill regenerates its Codex twin — not just hashes.**
> `make regen-all` / `scripts/codex-sync.sh` refresh parity-only twins from
> `skills/<name>/` whenever their generated body, prompt, or mirrored references
> drift. `scripts/regen-codex-hashes.sh` remains the bookkeeping step after
> content is current. Manual edits under `skills-codex/<name>/` are reserved for
> bespoke skills or deliberate Codex-only divergence recorded in
> `skills-codex-overrides/catalog.json`; otherwise fix the source skill or the
> codex-sync transform/template and regenerate.

> **Bespoke twins are HAND-MAINTAINED in full — body AND references (age-0js4).**
> A `treatment: bespoke` twin (catalog.json — `council`, `crank`, `evolve`,
> `plan`, `premortem`, `research`, `rpi`, … 19 total) is skipped ENTIRELY by
> codex-sync, **including `--force`**. Its `SKILL.md` body and everything under
> `references/`/`scripts/` are authored by hand: many bespoke references are
> deliberate Codex-condensed rewrites of source (e.g. `research/references/
> data-flow-from-entry-points.md` is a substantial hand-rewrite — 85 source lines
> deleted, 56 added), so a source edit does **NOT** auto-propagate, and
> `codex-sync --force --only <bespoke>` reporting "nothing to generate" is
> CORRECT, not a bug. Refreshing a bespoke twin after a source change is a
> deliberate human edit of the twin. **Do not** auto-mirror source over a bespoke
> twin — it would clobber the hand-authored Codex copy. (Auto-refresh was
> evaluated under age-0js4 and rejected: dozens of the bespoke reference files are
> genuine hand-rewrites; only an explicit per-file tracked/bespoke manifest could
> refresh safely, which is disproportionate to the low-frequency cost. *Accidental*
> drift — a twin that should have tracked source but didn't — is the divergence
> gate's job, tracked under age-odv to add an explicit bespoke exemption, not
> codex-sync's.)

> **Pointer twins are exempt from the mirror requirement (`parity_policy: pointer`).**
> Distinct from bespoke: some twins are deliberately THIN POINTERS — they carry no
> mirrored prose, just "the source skill is the source of truth — read it first"
> plus a short Codex Runtime Contract (e.g. `pawl-review`, `agent-mail`,
> `ntm`; ~16 of them). For these there is nothing to mirror, so a source-only prose
> edit must NOT demand twin churn. Declare it once in the twin's frontmatter:
> ```yaml
> parity_policy: pointer   # twin defers to the source body; exempt from source-divergence
> ```
> `validate-codex-generated-artifacts.sh` (`twin_is_pointer`) then skips the
> SKILL.md-body and references divergence gates for that twin. Use this ONLY for a
> genuine pointer — a twin that duplicates source prose must stay a full mirror and
> keep the marker off, so its divergence gate still fires. The twin's own content
> (incl. its Codex Runtime Contract) is still validated by the source→codex
> existence check and the manifest/hash audit. Marking the existing ~16 pointer
> twins is tracked under age-backfill-pointer-twin-markers-uco.

When a skill change affects Codex behavior, phrasing, orchestration, or UX:

1. Update the source skill under `skills/` when the shared contract changes.
2. For parity-only skills, update source or the codex-sync transform/template and regenerate. Update `skills-codex/<name>/SKILL.md` directly only when the Codex runtime copy is bespoke, or update `skills-codex-overrides/<name>/` when the Codex experience should differ from Claude.
   - Prompt/operator-layer changes belong in `skills-codex-overrides/<name>/prompt.md`.
   - Durable Codex-only body rewrites belong in `skills-codex-overrides/<name>/SKILL.md`.
3. Run the semantic audit if the checked-in Codex body looks suspicious:
   ```bash
   bash scripts/audit-codex-parity.sh
   # or target one skill
   bash scripts/audit-codex-parity.sh --skill <name>
   ```
4. Validate the checked-in Codex artifacts:
   ```bash
   bash scripts/audit-codex-parity.sh
   bash scripts/validate-codex-override-coverage.sh
   bash scripts/validate-codex-generated-artifacts.sh --scope worktree
   bash scripts/validate-codex-backbone-prompts.sh
   bash scripts/validate-codex-rpi-contract.sh
   bash scripts/validate-codex-lifecycle-guards.sh
   bash scripts/validate-headless-runtime-skills.sh
   ```

Think of `skills/` as the shared contract, `skills-codex-overrides/` as the durable Codex-only tailoring layer, and `skills-codex/` as the checked-in Codex artifact shipped to users.
