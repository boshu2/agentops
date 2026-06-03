# Task: advisory-tier guard gate

AgentOps CI doctrine: a job is either **required** (blocks merge — a PR check) or
**informational** (runs but is not a PR check). There is no "advisory" middle tier:
a job that is `continue-on-error: true` must NOT also surface as a required PR check.

Write an executable script `check-no-advisory-tier.sh` in the current directory.
Contract: `check-no-advisory-tier.sh <validate.yml>`
- exit **1** if any job marked `continue-on-error: true` ALSO appears in the
  `summary.needs:` list (i.e. a continue-on-error job is surfaced as a PR check),
- exit **0** otherwise.

Just write the script. Do not explain.
