#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

if ! command -v rg >/dev/null 2>&1; then
    echo "error: rg is required for this audit script"
    exit 1
fi

tmp_calls="$(mktemp)"
tmp_counts="$(mktemp)"
trap 'rm -f "$tmp_calls" "$tmp_counts"' EXIT

rg -n --no-heading 'exec\.Command(Context)?\(|exec\.LookPath\(' cli/cmd/ao cli/internal \
    | rg -v '_test\.go' \
    | perl -ne '
if (/^(.*?):(\d+):(.*)$/) {
  $file=$1; $line=$2; $rest=$3; $bin="<dynamic>";
  if ($rest =~ /exec\.CommandContext\([^,]+,\s*"([^"]+)"/) { $bin=$1; }
  elsif ($rest =~ /exec\.Command\(\s*"([^"]+)"/) { $bin=$1; }
  elsif ($rest =~ /exec\.LookPath\(\s*"([^"]+)"/) { $bin=$1; }
  print "$file\t$bin\t$line\n";
}' > "$tmp_calls"

awk -F '\t' '
{
  key = $1 "\t" $2
  count[key]++
}
END {
  for (k in count) {
    print count[k] "\t" k
  }
}
' "$tmp_calls" | sort -nr > "$tmp_counts"

echo "# AO CLI External Command Dependency Audit"
echo
echo "generated_at_utc: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "repo_root: $REPO_ROOT"
echo

total_calls="$(wc -l < "$tmp_calls" | tr -d '[:space:]')"
unique_files="$(cut -f1 "$tmp_calls" | sort -u | wc -l | tr -d '[:space:]')"
unique_bins="$(cut -f2 "$tmp_calls" | sort -u | wc -l | tr -d '[:space:]')"

echo "## Summary"
echo "- total_calls: $total_calls"
echo "- unique_files: $unique_files"
echo "- unique_binaries: $unique_bins"
echo

echo "## File/Binary Matrix (count file binary)"
cat "$tmp_counts"
echo

echo "## Raw Calls (file binary line)"
cat "$tmp_calls"
