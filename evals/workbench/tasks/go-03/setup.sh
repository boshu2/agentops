#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../go-cli" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Stub out the Divide function body (replace real implementation with stub)
cat > "$WORKDIR/internal/calc/calc_divide_patch.py" << 'PYEOF'
import re, sys
path = sys.argv[1]
with open(path) as f:
    src = f.read()
# Replace Divide function body with stub
src = re.sub(
    r'(func Divide\(a, b float64\) \(float64, error\) \{)\n.*?\n}',
    r'\1\n\treturn 0, nil\n}',
    src,
    flags=re.DOTALL
)
with open(path, 'w') as f:
    f.write(src)
PYEOF
python3 "$WORKDIR/internal/calc/calc_divide_patch.py" "$WORKDIR/internal/calc/calc.go"
rm "$WORKDIR/internal/calc/calc_divide_patch.py"

# Remove TestDivide from calc_test.go
awk '
  /^func TestDivide\(/ { skip=1 }
  skip && /^}$/ { skip=0; next }
  !skip { print }
' "$WORKDIR/internal/calc/calc_test.go" > "$WORKDIR/internal/calc/calc_test.go.tmp"
mv "$WORKDIR/internal/calc/calc_test.go.tmp" "$WORKDIR/internal/calc/calc_test.go"

# Remove unused imports that were only used by TestDivide
sed -i '/"errors"/d; /"math"/d' "$WORKDIR/internal/calc/calc_test.go"
