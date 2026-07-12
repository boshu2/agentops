# Composable Security Suite Runbook

Use this reference for authorized binary assurance, baseline comparison, policy enforcement, and offline repo-surface redteam. The main `security` skill owns authorization, pawl recovery, and release decisions.

## Commands

Capture an owned binary:

```bash
python3 skills/security/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite/ao-current
```

Compare with a known-good baseline by adding `--baseline-dir .tmp/security-suite/ao-baseline --fail-on-removed`. Enforce policy by adding `--policy-file skills/security/references/policy-example.json --fail-on-policy-fail`.

Run offline redteam:

```bash
python3 skills/security/scripts/prompt_redteam.py scan \
  --repo-root . \
  --pack-file skills/security/references/agentops-redteam-pack.json \
  --out-dir .tmp/security-suite-redteam
```

## Artifact Inventory

Binary suites write `static/static-analysis.json`, `dynamic/dynamic-analysis.json`, `contract/contract.json`, optional compare/policy verdicts, and `suite-summary.json` beneath `--out-dir`. Redteam writes `redteam/redteam-results.json` and `redteam/redteam-results.md`.

Preserve command exit codes with the artifacts. A missing optional compare/policy artifact is valid only when that phase was not requested.

## Triage

- Do not relax policy or refresh a baseline merely because a candidate fails; classify the delta and require explicit judgment for intentional contract changes.
- Empty dynamic evidence means the owned binary or safe command needs inspection.
- Zero captured commands means the expected help interface was not observed.
- Redteam failure means either the control regressed or the attack-pack matcher needs an intentional revision.
