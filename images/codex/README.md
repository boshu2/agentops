# Codex compatibility image

Codex release twins are generated under `skills-codex/` from the canonical
`skills/` tree. Metadata declares whether a twin is parity-generated or has a
cataloged Codex-specific override; generated hashes bind every twin to its
source.

Codex users install the AgentOps Codex plugin (see the README Quickstart), which
ships these twins. Contributors working from a checkout can run `ao skills link`
instead. The twins are a generated projection, not a second source of truth.

Verify the generated image and source hashes with:

```bash
bash images/codex/verify.sh
bash scripts/regen-codex-hashes.sh --check
```

The authoritative conversion contract is
`docs/contracts/codex-skill-api.md`; the generated inventory is
`skills-codex/.agentops-manifest.json`.
