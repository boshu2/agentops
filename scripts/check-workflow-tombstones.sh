#!/usr/bin/env bash
# Retired Claude workflows must fail before dispatch or repository mutation.
# An optional directory allows a disposable fixture to test this gate.
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
candidate_dir="${1:-$REPO_ROOT/workflows}"

if ! command -v node >/dev/null 2>&1; then
  echo "FAIL: node not found; cannot check retired workflows" >&2
  exit 1
fi

for name in bdd-foundry ship-beads bead-crank operating-loop; do
  candidate="$candidate_dir/$name.js"
  if [ ! -f "$candidate" ]; then
    echo "FAIL: retired workflow missing: $candidate" >&2
    exit 1
  fi
  if ! node --check "$candidate"; then
    echo "FAIL: retired workflow has invalid syntax: $candidate" >&2
    exit 1
  fi
  # Check the entire allowed source shape without executing candidate code.
  # A module import would run any side effect placed before its retirement throw.
  if ! node --input-type=module - "$candidate" "$name" <<'NODE'
import { readFileSync } from 'node:fs'

const [, , file, name] = process.argv
const source = readFileSync(file, 'utf8')
function fail(reason) {
  process.stderr.write(`FAIL: ${name} is not an inert retirement tombstone: ${reason}\n`)
  process.exit(1)
}
if (!source.endsWith('\n') || /[\r\u2028\u2029]/u.test(source)) fail('unexpected line separator')
const lines = source.slice(0, -1).split('\n')
const header = [
  'export const meta = {',
  `  name: '${name}',`,
  null,
  null,
  '  phases: [',
  "    { title: 'Migration notice', detail: 'fails immediately with replacement pointers' },",
  '  ],',
  '}',
]
for (let i = 0; i < header.length; i++) {
  if (header[i] !== null && lines[i] !== header[i]) fail(`metadata line ${i + 1}`)
}
if (!/^  description: 'Retired compatibility tombstone for [^'\\]*',$/u.test(lines[2])) fail('description')
if (!/^  whenToUse: 'Never[^'\\]*',$/u.test(lines[3])) fail('selectability')

let line = header.length
if (lines[line++] !== '') fail('missing separator')
while (lines[line] === '' || /^\/\/(?: |$)/u.test(lines[line] ?? '')) line++
if (lines[line++] !== 'throw new Error(') fail('missing immediate retirement throw')

const parts = []
while (line < lines.length && lines[line] !== ')') {
  const match = /^ {2,4}'([^'\\]*)'( \+)?$/u.exec(lines[line++])
  if (!match) fail('nonliteral retirement message')
  parts.push(match)
}
if (parts.length === 0 || lines[line++] !== ')' || line !== lines.length) fail('trailing or missing code')
for (let i = 0; i < parts.length; i++) {
  if (Boolean(parts[i][2]) !== (i < parts.length - 1)) fail('message concatenation')
}
if (!parts.map((part) => part[1]).join('').includes(`workflows/${name}.js is retired`)) {
  fail('missing retirement error')
}
NODE
  then
    exit 1
  fi
done

echo "OK: retired workflows fail before dispatch"
