#!/usr/bin/env bash
set -euo pipefail

die() { printf 'dcg-config: %s\n' "$*" >&2; exit 2; }
usage() { printf '%s\n' 'usage: write-config.sh --root ABS --kind project|allowlist --input ABS [--replace] --approve TOKEN' >&2; }
sha256_file() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

root=''
kind=''
input=''
approval=''
replace=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --root) [[ $# -ge 2 ]] || die '--root needs a value'; root=$2; shift 2 ;;
    --kind) [[ $# -ge 2 ]] || die '--kind needs a value'; kind=$2; shift 2 ;;
    --input) [[ $# -ge 2 ]] || die '--input needs a value'; input=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    --replace) replace=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

[[ "$root" == /* && -d "$root" && ! -L "$root" ]] || die 'root must be an existing absolute non-symlink directory'
root=$(cd "$root" && pwd -P)
[[ "$root" != / ]] || die 'root may not be filesystem root'
[[ -d "$root/.git" || -f "$root/.git" ]] || die 'root must be a Git worktree'
[[ "$input" == /* && -f "$input" && ! -L "$input" && -s "$input" ]] || die 'input must be an absolute nonempty regular non-symlink file'
case "$kind" in
  project) target="$root/.dcg.toml" ;;
  allowlist) target="$root/.dcg/allowlist.toml" ;;
  *) die 'kind must be project or allowlist' ;;
esac
[[ ! -L "$target" ]] || die 'target may not be a symlink'
if [[ -e "$target" ]]; then
  $replace || die 'target exists; --replace and an approval bound to its current digest are required'
  old_digest=$(sha256_file "$target")
else
  old_digest=absent
fi
new_digest=$(sha256_file "$input")
expected="dcg:write:$target:$new_digest:$old_digest"
[[ "$approval" == "$expected" ]] || die "exact approval required: $expected"

dcg_bin=${DCG_BIN:-dcg}
if [[ "$dcg_bin" == */* ]]; then [[ -x "$dcg_bin" ]] || die "dcg binary is not executable: $dcg_bin"; else dcg_bin=$(command -v "$dcg_bin") || die 'dcg unavailable'; fi
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required'
version=$($timeout_bin --kill-after=2s 10 "$dcg_bin" --version)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]] || die "unrecognized dcg version: $version"
help=$($timeout_bin --kill-after=2s 10 "$dcg_bin" --help)
[[ "$help" == *'test'* && "$help" == *'doctor'* ]] || die 'dcg command surface lacks test/doctor'

# Both probes exercise candidate parsing. A safe command must be allowed and a
# core destructive command must remain blocked; otherwise the candidate cannot
# become the active safety config.
$timeout_bin --kill-after=2s 15 "$dcg_bin" test -c "$input" 'git status'
set +e
$timeout_bin --kill-after=2s 15 "$dcg_bin" test -c "$input" 'git reset --hard HEAD'
blocked_rc=$?
set -e
[[ "$blocked_rc" -ne 0 && "$blocked_rc" -ne 124 ]] || die 'candidate config did not block the core destructive probe'

target_dir=$(dirname "$target")
mkdir -p "$target_dir"
[[ ! -L "$target_dir" ]] || die 'target directory became a symlink'
umask 077
tmp="$target_dir/.dcg-config.$$"
trap 'rm -f "$tmp"' EXIT
cp "$input" "$tmp"
[[ "$(sha256_file "$tmp")" == "$new_digest" ]] || die 'staged config digest changed'
mv -f "$tmp" "$target"
trap - EXIT
[[ "$(sha256_file "$target")" == "$new_digest" ]] || die 'installed config digest mismatch'
printf 'dcg-config: target=%s old=%s new=%s atomic=true\n' "$target" "$old_digest" "$new_digest" >&2
