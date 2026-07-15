# Codex compatibility image

Codex release twins are generated under `skills-codex/` from the canonical
`skills/` tree. Metadata declares whether a twin is parity-generated or has a
cataloged Codex-specific override; generated hashes bind every twin to its
source.

New installations should use one checkout plus source links:

```bash
cd ~/.local/share/agentops
ao skills link
```

That links the canonical skills into `~/.agents/skills` and
`~/.codex/skills`. Codex release twins and the 3.x native plugin package remain
distribution/migration compatibility artifacts for this release, not a second
source of truth.

Verify the generated image and source hashes with:

```bash
bash images/codex/verify.sh
bash scripts/regen-codex-hashes.sh --check
```

The authoritative conversion contract is
`docs/contracts/codex-skill-api.md`; the generated inventory is
`skills-codex/.agentops-manifest.json`.
