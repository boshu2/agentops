#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <proposal.md> [--expected-output-path <path>] [--model-output-approval <id> --sensitive-output-approval <id> --authorized-path <exact-path> --authorized-audience <audience>]" >&2
  exit 2
}

[[ $# -ge 1 ]] || usage
proposal="$1"
shift
approval_id=""
model_approval_id=""
authorized_path=""
authorized_audience=""
expected_output_path="$proposal"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --model-output-approval) [[ $# -ge 2 ]] || usage; model_approval_id="$2"; shift 2 ;;
    --sensitive-output-approval) [[ $# -ge 2 ]] || usage; approval_id="$2"; shift 2 ;;
    --authorized-path) [[ $# -ge 2 ]] || usage; authorized_path="$2"; shift 2 ;;
    --authorized-audience) [[ $# -ge 2 ]] || usage; authorized_audience="$2"; shift 2 ;;
    --expected-output-path) [[ $# -ge 2 ]] || usage; expected_output_path="$2"; shift 2 ;;
    *) usage ;;
  esac
done
[[ -f "$proposal" && ! -L "$proposal" ]] || usage
if [[ -n "$model_approval_id" || -n "$approval_id" || -n "$authorized_path" || -n "$authorized_audience" ]]; then
  [[ -n "$model_approval_id" && ${#model_approval_id} -le 256 ]] || usage
  [[ -n "$approval_id" && ${#approval_id} -le 256 && -n "$authorized_path" && -n "$authorized_audience" ]] || usage
  [[ "$(cd "$(dirname "$expected_output_path")" && pwd -P)/$(basename "$expected_output_path")" == "$(cd "$(dirname "$authorized_path")" && pwd -P)/$(basename "$authorized_path")" ]] || {
    echo "operationalize output: sensitive approval does not match exact output path" >&2
    exit 1
  }
fi

python3 - "$proposal" "$expected_output_path" "$model_approval_id" "$approval_id" "$authorized_path" "$authorized_audience" <<'PY'
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
expected_output_path = Path(sys.argv[2])
model_approval = sys.argv[3]
write_approval = sys.argv[4]
authorized_path = sys.argv[5]
authorized_audience = sys.argv[6]
raw = path.read_bytes()
if len(raw) > 65536:
    raise SystemExit("operationalize output: artifact exceeds 65536 bytes")
text = raw.decode("utf-8")
required = {
    "Classification": r"^(public|internal|restricted)$",
    "Redaction review": r"^passed$",
    "Model approval": r"^.+$",
    "Sensitive approval": r"^.+$",
    "Audience": r"^.+$",
    "Output path": r"^.+$",
    "Retention until": r"^(none|[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z)$",
}
if not re.search(r"^## Sensitive-output review\s*$", text, re.MULTILINE):
    raise SystemExit("operationalize output: missing Sensitive-output review")
values = {}
for field, pattern in required.items():
    match = re.search(rf"^- {re.escape(field)}:\s*(.+?)\s*$", text, re.MULTILINE)
    if not match or not re.fullmatch(pattern, match.group(1)):
        raise SystemExit(f"operationalize output: invalid {field}")
    values[field] = match.group(1)
if Path(values["Output path"]).resolve() != expected_output_path.resolve():
    raise SystemExit("operationalize output: declared output path does not match artifact")
patterns = {
    "private-key": r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----",
    "aws-access-key": r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b",
    "github-token": r"\bgh[opusr]_[A-Za-z0-9]{20,}\b|\bgithub_pat_[A-Za-z0-9_]{20,}\b",
    "bearer-token": r"(?i)\bAuthorization\s*:\s*Bearer\s+[A-Za-z0-9._~+/=-]{12,}",
    "password-assignment": r"(?i)\b(?:password|passwd|secret|api[_-]?key)\s*[:=]\s*[^\s<\[][^\r\n]{5,}",
}
hits = [name for name, pattern in patterns.items() if re.search(pattern, text)]
if hits and (not model_approval or not write_approval):
    raise SystemExit("operationalize output: sensitive categories require model and write approval: " + ",".join(hits))
if model_approval or write_approval:
    if values["Model approval"] != model_approval or values["Sensitive approval"] != write_approval:
        raise SystemExit("operationalize output: declarations do not match caller approvals")
    if values["Classification"] != "restricted" or values["Retention until"] == "none":
        raise SystemExit("operationalize output: approved sensitive output needs restricted classification and retention")
    if Path(authorized_path).resolve() != expected_output_path.resolve() or values["Audience"] != authorized_audience:
        raise SystemExit("operationalize output: approval path or audience differs from declaration")
elif values["Model approval"] != "none" or values["Sensitive approval"] != "none":
    raise SystemExit("operationalize output: artifact claims an unverified approval")
print(f"valid operationalization proposal: {path} (sensitive_matches={len(hits)})")
PY
