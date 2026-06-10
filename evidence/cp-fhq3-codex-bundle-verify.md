# cp-fhq3 — Codex bundle (3.1) verification evidence

Track: wf31-codex_bundle (worker for cp-fhq3 Phase C PACKAGE).
Base: origin/main @ 3cc6d1cdc (includes 2026-06-10 4-skill lesson encoding d286a5fc2 + mirror refresh 3cc6d1cdc).

## Finding

The IMAGE-CORE Codex recipe is ALREADY materialized as an installable bundle at version 3.1.0.
No source changes required. All artifacts present and in sync. Evidence below is MEASURED
(verbatim command output captured this run).

## Artifacts (present)

- `.codex-plugin/plugin.json` — name `agentops`, version `3.1.0`, skills `./skills-codex`
- `images/codex/{manifest.json, README.md, verify.sh}`
- `scripts/{install-codex.sh, install-codex-plugin.sh, install-codex-native-skills.sh, install-codex.ps1}`
- `scripts/regen-codex-hashes.sh` — the hash-drift integrity gate

## Documented install one-liner

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
```

(documented at `images/codex/README.md` §Verify; install-codex.sh parses
`.codex-plugin/plugin.json` version and records install metadata to
`~/.codex/.agentops-codex-install.json`.)

## Measured verification (verbatim)

### Hash-drift gate
```
$ bash scripts/regen-codex-hashes.sh --check
All hashes up to date.
EXIT=0
```

### Image verify (presence + drift)
```
$ bash images/codex/verify.sh
Codex image bundle verify - CORE twins in skills-codex/
  expected  : 61 CORE slugs
Checked 61 CORE slugs.
OK: all 61 CORE twins present (SKILL.md + prompt.md + .agentops-generated.json).
Running drift gate: scripts/regen-codex-hashes.sh --check
All hashes up to date.
OK: codex hashes in sync (no drift).
PASS: Codex image bundle verified (61 CORE twins present + hashes in sync).
EXIT=0
```

### Install scripts syntax
```
$ bash -n scripts/install-codex.sh                  -> OK
$ bash -n scripts/install-codex-plugin.sh           -> OK
$ bash -n scripts/install-codex-native-skills.sh    -> OK
```

### Bundle validators
```
$ bash scripts/validate-codex-install-bundle.sh
Codex install bundle validation OK for working tree (166 skill package(s)).
EXIT=0

$ bash tests/scripts/test-codex-install-bundle.sh
Results: 2 PASS, 0 FAIL
EXIT=0
```

### Manifest counts
```
core_skills: 61   codex_operator_skills: 4
```

## CHANGELOG

3.1.0 "Added" already covers the Codex native plugin curl installer and Day-2 ops
(CHANGELOG.md lines 14-16).

## Conclusion

Codex bundle track for cp-fhq3 is structurally and functionally complete at 3.1.0
on origin/main. The post-3e0f8f4e8 corpus changes (the 2026-06-10 4-skill lesson
encoding) did NOT introduce drift — the hash gate is green at HEAD 3cc6d1cdc.
No code modification was needed; this commit records the measured verification only.
