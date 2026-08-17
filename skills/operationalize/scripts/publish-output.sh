#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 --source <staged.md> --output-root <.agents/scratch/operationalize> --destination <exact-path> --authorization-id <id> [--model-output-approval <id> --sensitive-output-approval <id> --authorized-audience <audience>]" >&2
  exit 2
}

source_path=""
output_root=""
destination=""
authorization_id=""
model_approval=""
sensitive_approval=""
authorized_audience=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source) [[ $# -ge 2 ]] || usage; source_path="$2"; shift 2 ;;
    --output-root) [[ $# -ge 2 ]] || usage; output_root="$2"; shift 2 ;;
    --destination) [[ $# -ge 2 ]] || usage; destination="$2"; shift 2 ;;
    --authorization-id) [[ $# -ge 2 ]] || usage; authorization_id="$2"; shift 2 ;;
    --model-output-approval) [[ $# -ge 2 ]] || usage; model_approval="$2"; shift 2 ;;
    --sensitive-output-approval) [[ $# -ge 2 ]] || usage; sensitive_approval="$2"; shift 2 ;;
    --authorized-audience) [[ $# -ge 2 ]] || usage; authorized_audience="$2"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "$authorization_id" && ${#authorization_id} -le 256 ]] || { echo 'operationalize publish: authorization ID is required' >&2; exit 2; }
[[ -f "$source_path" && ! -L "$source_path" ]] || { echo 'operationalize publish: source must be a regular staged file' >&2; exit 2; }
[[ -d "$output_root" && ! -L "$output_root" ]] || { echo 'operationalize publish: output root must be a real directory' >&2; exit 2; }
root_real="$(cd "$output_root" && pwd -P)"
case "$root_real" in
  */.agents/scratch/operationalize) ;;
  *) echo 'operationalize publish: output root must end in .agents/scratch/operationalize' >&2; exit 2 ;;
esac
[[ -n "$destination" && -d "$(dirname "$destination")" ]] || { echo 'operationalize publish: destination parent must already exist' >&2; exit 2; }
destination_real="$(cd "$(dirname "$destination")" && pwd -P)/$(basename "$destination")"
case "$destination_real" in
  "$root_real"/*) ;;
  *) echo 'operationalize publish: destination is outside the output root' >&2; exit 2 ;;
esac
[[ ! -e "$destination_real" && ! -L "$destination_real" ]] || { echo 'operationalize publish: destination already exists' >&2; exit 1; }
source_real="$(cd "$(dirname "$source_path")" && pwd -P)/$(basename "$source_path")"
case "$source_real" in
  "$root_real"/*) echo 'operationalize publish: staged source must be outside the durable output root' >&2; exit 2 ;;
esac

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
validator_args=("$source_real" --expected-output-path "$destination_real")
if [[ -n "$model_approval" || -n "$sensitive_approval" || -n "$authorized_audience" ]]; then
  [[ -n "$model_approval" && -n "$sensitive_approval" && -n "$authorized_audience" ]] || usage
  validator_args+=(
    --model-output-approval "$model_approval"
    --sensitive-output-approval "$sensitive_approval"
    --authorized-path "$destination_real"
    --authorized-audience "$authorized_audience"
  )
fi
bash "$script_dir/validate-output.sh" "${validator_args[@]}"

python3 - "$source_real" "$destination_real" "$authorization_id" <<'PY'
import hashlib
import json
import os
from pathlib import Path
import sys

source = Path(sys.argv[1])
destination = Path(sys.argv[2])
authorization_id = sys.argv[3]
payload = source.read_bytes()
fd = None
created = False
try:
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    fd = os.open(destination, flags, 0o600)
    created = True
    view = memoryview(payload)
    while view:
        written = os.write(fd, view)
        if written <= 0:
            raise OSError("short write")
        view = view[written:]
    os.fsync(fd)
    os.close(fd)
    fd = None
    observed = destination.read_bytes()
    if observed != payload:
        raise OSError("post-write digest mismatch")
except Exception:
    if fd is not None:
        os.close(fd)
    if created:
        try:
            destination.unlink()
        except OSError as cleanup_error:
            raise SystemExit(f"operationalize publish: write failed and destination cleanup failed: {cleanup_error.__class__.__name__}")
    raise

print(json.dumps({
    "schema_version": "operationalize-publish-receipt.v1",
    "authorization_id": authorization_id,
    "reads": [str(source)],
    "writes": [str(destination)],
    "bytes_written": len(payload),
    "sha256": hashlib.sha256(payload).hexdigest(),
}))
PY
