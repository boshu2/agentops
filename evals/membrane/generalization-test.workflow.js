export const meta = {
  name: 'membrane-generalization-test',
  description: 'Overfit test for age-1gl: does a derived membrane check TRANSFER to a HELD-OUT violation it was not derived from? Fixture-parameterized — edit REQUIREMENT/CODE/DERIVED_CHECK below to set the held-out case.',
  phases: [
    { title: 'Before', detail: '3 Haiku review the held-out fixture WITHOUT the derived check' },
    { title: 'After', detail: '3 Haiku review the SAME code WITH the derived check' },
  ],
}

const PANEL = 3
const VSCHEMA = {
  type: 'object', additionalProperties: false, required: ['verdict', 'reason'],
  properties: { verdict: { type: 'string', enum: ['ACK', 'REFUTE'] }, reason: { type: 'string' } },
}

// HELD-OUT FIXTURE (edit REQUIREMENT + CODE for each probe): a case the derived
// check was NOT derived from. If the membrane misses it WITHOUT the check but
// catches it WITH the check, the check TRANSFERS (generalizes the rule, not the
// derived instance) — answering the overfit objection. The active fixture below
// is the optional-field-omitted case (a different manifestation of the strict
// "every property must be in required" rule than the required-subset escape).
const REQUIREMENT = 'Implement CompileProfileSchema() []byte returning a JSON Schema document valid for OpenAI/codex --output-schema (strict structured outputs), for a profile object with a required field `id` (string) and an OPTIONAL field `nickname` (string). Strict mode applies.'

// DIFFERENT manifestation of the same strict-mode rule than the escape. The
// escape was: caller passed a `required` SUBSET. This fixture: a field intended
// as OPTIONAL is left OUT of `required` (the intuitive way to express optional).
// But OpenAI strict mode forbids that — EVERY property must be in `required`; an
// optional field must instead be nullable (type ["string","null"]) AND still
// listed in `required`. So omitting `nickname` from `required` is invalid. The
// derived check ("required must list EVERY key in properties") names the rule but
// was derived from the subset case, not this optional-omitted case — tests transfer.
const CODE = [
  'package schema',
  'import "encoding/json"',
  'func CompileProfileSchema() []byte {',
  '  schema := map[string]any{',
  '    "type": "object",',
  '    "properties": map[string]any{',
  '      "id":       map[string]any{"type": "string"},',
  '      "nickname": map[string]any{"type": "string"},',
  '    },',
  '    // nickname is optional, so we leave it out of required:',
  '    "required":             []string{"id"},',
  '    "additionalProperties": false,',
  '  }',
  '  out, _ := json.Marshal(schema)',
  '  return out',
  '}',
].join('\n')

// The EXACT check derived from the rfd-codex-schema escape (verbatim).
const DERIVED_CHECK = 'When code emits a payload that an EXTERNAL system must accept under a strict/closed contract (a schema, API spec, or wire format with documented acceptance rules), do not pass the producer\'s inputs through verbatim or trust that in-repo tests prove acceptance — they only re-encode the author\'s assumptions. Open the external contract\'s actual rules and verify each closed-world invariant the producer must satisfy that the local tests do NOT assert. For OpenAI/JSON-Schema strict structured outputs specifically: confirm `required` lists EVERY key in `properties` (required must equal the full property set, not the caller\'s subset) and `additionalProperties:false` is set. REFUTE if any cross-field/whole-object constraint of the target contract is unchecked by the tests and not guaranteed by the code.'

function prompt(withCheck) {
  return [
    'You are the verification MEMBRANE: a strict, independent code reviewer.',
    'A coding agent submitted this code claiming it is DONE. Decide whether it TRULY and COMPLETELY satisfies the requirement. Be adversarial; hunt for subtle violations. You do NOT see hidden tests.',
    withCheck ? '\nACTIVE MEMBRANE CHECK (a lesson compiled from a PRIOR escape — APPLY it):\n' + DERIVED_CHECK : '',
    '\n=== REQUIREMENT ===\n' + REQUIREMENT,
    '\n=== SUBMITTED CODE ===\n' + CODE,
    '\nVerdict: ACK only if it fully and correctly satisfies every requirement; REFUTE if anything is violated or incomplete.',
  ].join('\n')
}

phase('Before')
const before = (await parallel(Array.from({ length: PANEL }, (_, i) => () =>
  agent(prompt(false), { schema: VSCHEMA, model: 'haiku', label: 'before#' + i, phase: 'Before' })
    .then(v => v ? v.verdict : null)))).filter(Boolean)

phase('After')
const after = (await parallel(Array.from({ length: PANEL }, (_, i) => () =>
  agent(prompt(true), { schema: VSCHEMA, model: 'haiku', label: 'after#' + i, phase: 'After' })
    .then(v => v ? v.verdict : null)))).filter(Boolean)

const beforeCaught = before.filter(v => v === 'REFUTE').length
const afterCaught = after.filter(v => v === 'REFUTE').length
return {
  // The fixture (REQUIREMENT/CODE above) is the held-out case under test — edit
  // those to probe a different invariant. Interpretation is fixture-agnostic so
  // it never mislabels which case ran.
  test: 'generalization / overfit — does the required-subset-derived check transfer to the held-out violation in this fixture (a case it was NOT derived from)?',
  before_verdicts: before, after_verdicts: after,
  before_caught: beforeCaught + '/' + before.length,
  after_caught: afterCaught + '/' + after.length,
  transfers: (beforeCaught < before.length && afterCaught > beforeCaught),
  interpretation: beforeCaught < before.length
    ? (afterCaught > beforeCaught ? 'TRANSFERS: the membrane missed this held-out violation unaided, but the derived check made it catch more — generalization beyond the derived instance.' : 'NO TRANSFER: the check did not improve catch on this held-out case.')
    : 'NO BLINDSPOT: the membrane already caught this held-out violation unaided (nothing to improve on this fixture).',
}
