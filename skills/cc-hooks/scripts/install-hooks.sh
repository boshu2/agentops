#!/usr/bin/env bash
# Transactionally install the package-owned dispatcher into one explicit scope.
set -euo pipefail
umask 077

usage() {
  echo "usage: $0 --user | --project-root /absolute/non-symlink/root" >&2
  exit 2
}

mode=""; scope_root=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --user) [[ -z "$mode" ]] || usage; mode=user; shift ;;
    --project-root) [[ -z "$mode" && $# -ge 2 ]] || usage; mode=project; scope_root="$2"; shift 2 ;;
    --project-root=*) [[ -z "$mode" ]] || usage; mode=project; scope_root="${1#*=}"; shift ;;
    *) usage ;;
  esac
done
[[ -n "$mode" ]] || usage
if [[ "$mode" == user ]]; then scope_root="${HOME:?HOME is required for --user}"; fi
[[ "$scope_root" == /* && -d "$scope_root" && ! -L "$scope_root" ]] \
  || { echo "ERROR: install scope must be an existing absolute non-symlink directory" >&2; exit 2; }
scope_root="$(cd "$scope_root" && pwd -P)"

script_dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_dir="$(dirname "$script_dir")"
src_dispatch="$skill_dir/hooks/policy-dispatch.sh"
src_policies="$skill_dir/policies/policies.json"
lint="$script_dir/lint-policies.sh"
[[ -f "$src_dispatch" && ! -L "$src_dispatch" ]] || { echo "ERROR: dispatcher missing or symlinked" >&2; exit 1; }
[[ -f "$src_policies" && ! -L "$src_policies" ]] || { echo "ERROR: registry missing or symlinked" >&2; exit 1; }
command -v jq >/dev/null || { echo "ERROR: jq required" >&2; exit 1; }
command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1 \
  || { echo "ERROR: timeout or gtimeout required" >&2; exit 1; }
bash "$lint" "$src_policies"

settings="$scope_root/.claude/settings.json"
hooks_dir="$scope_root/.claude/hooks/aop"
claude_dir="$scope_root/.claude"
[[ ! -L "$claude_dir" && ! -L "$(dirname "$hooks_dir")" && ! -L "$settings" && ! -L "$hooks_dir" ]] \
  || { echo "ERROR: install target contains a symlink" >&2; exit 2; }

if [[ -e "$settings" ]]; then
  [[ -f "$settings" ]] || { echo "ERROR: settings target is not a regular file" >&2; exit 2; }
  settings_size="$(wc -c < "$settings" | tr -d ' ')"
  [[ "$settings_size" -le 1048576 ]] || { echo "ERROR: settings exceed 1048576 bytes" >&2; exit 2; }
  jq empty "$settings" || { echo "ERROR: settings are not valid JSON" >&2; exit 2; }
fi

mkdir -p "$claude_dir/hooks"

transaction="$(mktemp -d "$claude_dir/.aop-install.XXXXXX")"
cleanup() { rm -rf -- "$transaction"; }
trap cleanup EXIT
stage_hooks="$transaction/hooks"
stage_settings="$transaction/settings.json"
mkdir "$stage_hooks"
install -m 0755 "$src_dispatch" "$stage_hooks/policy-dispatch.sh"
install -m 0644 "$src_policies" "$stage_hooks/policies.json"
if [[ -e "$settings" ]]; then cp -p "$settings" "$transaction/original-settings.json"; else printf '{}\n' > "$transaction/original-settings.json"; fi

dst="$hooks_dir/policy-dispatch.sh"
jq --arg cmd "$dst" '
  .hooks //= {} |
  .hooks.PreToolUse //= [] |
  reduce ("Bash", "Edit|Write") as $m (.;
    if any(.hooks.PreToolUse[]?; .matcher == $m and any((.hooks // [])[]?; .command == $cmd))
    then .
    else .hooks.PreToolUse += [{"matcher":$m,"hooks":[{"type":"command","command":$cmd}]}]
    end
  )
' "$transaction/original-settings.json" > "$stage_settings"
jq empty "$stage_settings"

backup_hooks="$transaction/old-hooks"
backup_settings="$transaction/old-settings.json"
had_hooks=0; had_settings=0
if [[ -e "$hooks_dir" ]]; then mv "$hooks_dir" "$backup_hooks"; had_hooks=1; fi
if [[ -e "$settings" ]]; then mv "$settings" "$backup_settings"; had_settings=1; fi

rollback() {
  rm -rf -- "$hooks_dir"
  rm -f -- "$settings"
  [[ "$had_hooks" -eq 0 ]] || mv "$backup_hooks" "$hooks_dir"
  [[ "$had_settings" -eq 0 ]] || mv "$backup_settings" "$settings"
}

if ! mv "$stage_hooks" "$hooks_dir"; then rollback; echo "ERROR: hook install failed" >&2; exit 1; fi
if ! mv "$stage_settings" "$settings"; then rollback; echo "ERROR: settings install failed" >&2; exit 1; fi
chmod 600 "$settings"

grep -qF "$dst" "$settings" || { rollback; echo "ERROR: installed settings verification failed" >&2; exit 1; }
[[ -x "$hooks_dir/policy-dispatch.sh" && -f "$hooks_dir/policies.json" ]] \
  || { rollback; echo "ERROR: installed hook verification failed" >&2; exit 1; }

echo "installed bounded policy dispatcher in $mode scope"
echo "settings: .claude/settings.json"
echo "hooks: .claude/hooks/aop"
