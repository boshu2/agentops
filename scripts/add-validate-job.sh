#!/usr/bin/env bash
# shellcheck disable=SC2089,SC2090
# scripts/add-validate-job.sh
#
# RETIRED (Wave 2 cut-plan): the purpose-job scaffolder that also patched
# scripts/pre-push-gate.sh + tests/scripts/pre-push-gate.bats. Those bash-gate
# surfaces are gone; CI authority is go-gate-shadow (`ao gate check --full`)
# and new checks belong in the Go gate registry
# (cli/internal/gates/checks/seed.go), then docs/contracts/ci-jobs.yaml only
# if a new validate.yml purpose job is intentionally added.
#
# Encodes soc-3oij history; do not revive the bash-gate touch-points.

set -euo pipefail

cat >&2 <<'EOF'
add-validate-job.sh: retired with the legacy bash gate (Wave 2).

To add a blocking check:
  1. Implement scripts/check-*.sh (or a native Go check)
  2. Register it in cli/internal/gates/checks/seed.go
  3. Prefer go-gate-shadow coverage over a new validate.yml purpose job
  4. If you intentionally add a purpose job, update
     .github/workflows/validate.yml + docs/contracts/ci-jobs.yaml, then
     run: scripts/generate-ci-jobs-table.sh --write

EOF
exit 2
