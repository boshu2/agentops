# Perspectives

## Default: Independent Judges (No Perspectives)

When no `--preset` or `--perspectives` flag is provided, all judges get the **same prompt** with no perspective label. Diversity comes from independent sampling, not personality labels.

| Judge | Prompt | Assigned To |
|-------|--------|-------------|
| **Judge 1** | Independent judge — same prompt as all others | Agent 1 |
| **Judge 2** | Independent judge — same prompt as all others | Agent 2 |
| **Judge 3** | Independent judge — same prompt as all others | Agent 3 (--deep/--mixed) |

The default judge prompt (no perspective labels):

```
You are Council Judge {N}. You are one of {TOTAL} independent judges evaluating the same target.

{JSON_PACKET}

Instructions:
1. Analyze the target thoroughly
2. Write your analysis to: .agents/council/{OUTPUT_FILENAME}
   - Start with a JSON code block matching the output_schema
   - Follow with Markdown explanation
3. Send verdict to team lead

Your job is to find problems. A PASS with caveats is less valuable than a specific FAIL.
```

When `--preset` or `--perspectives` is used, judges receive the perspective-labeled prompt instead (see Agent Prompts section).

## Custom Perspectives

Simple name-based:
```bash
/council --perspectives="security,performance,ux" validate the API
```

## Built-in Presets

Use `--preset=<name>` for common persona configurations:

| Preset | Perspectives | Best For |
|--------|-------------|----------|
| `default` | (none — independent judges) | General validation |
| `security-audit` | attacker, defender, compliance, web-security | Security review |
| `architecture` | scalability, maintainability, simplicity | System design |
| `research` | breadth, depth, contrarian | Deep investigation |
| `ops` | reliability, observability, incident-response | Operations review |
| `code-review` | error-paths, api-surface, spec-compliance | Code validation (used by /vibe) |
| `plan-review` | missing-requirements, feasibility, scope, spec-completeness | Plan validation (used by /pre-mortem) |
| `doc-review` | clarity-editor, accuracy-verifier, completeness-auditor, audience-advocate | Documentation quality review |
| `retrospective` | plan-compliance, tech-debt, learnings | Post-implementation review (used by /post-mortem) |
| `product` | user-value, adoption-barriers, competitive-position | Product-market fit review (used by /pre-mortem when PRODUCT.md exists) |
| `developer-experience` | api-clarity, error-experience, discoverability | Developer UX review (used by /vibe when PRODUCT.md exists) |

```bash
/council --preset=security-audit validate the auth system
/council --preset=research --explorers=3 research upgrade automation
/council --preset=architecture research microservices boundaries
```

**Preset definitions** are built-in perspective configurations.

**Persona name mappings:**

| Preset | Name | Perspective |
|--------|------|-------------|
| security-audit | **Red** | attacker |
| security-audit | **Blue** | defender |
| security-audit | **Auditor** | compliance |
| security-audit | **WebSec** | web-security |
| architecture | **Scale** | scalability |
| architecture | **Craft** | maintainability |
| architecture | **Razor** | simplicity |
| code-review | **Pathfinder** | error-paths |
| code-review | **Surface** | api-surface |
| code-review | **Spec** | spec-compliance |
| plan-review | **Gaps** | missing-requirements |
| plan-review | **Reality** | feasibility |
| plan-review | **Scope** | scope |
| plan-review | **Completeness** | spec-completeness |
| retrospective | **Compass** | plan-compliance |
| retrospective | **Debt** | tech-debt |
| retrospective | **Harvest** | learnings |
| product | **User** | user-value |
| product | **Friction** | adoption-barriers |
| product | **Edge** | competitive-position |
| developer-experience | **Signal** | api-clarity |
| developer-experience | **SOS** | error-experience |
| developer-experience | **Beacon** | discoverability |
| doc-review | **Clarity** | clarity-editor |
| doc-review | **Accuracy** | accuracy-verifier |
| doc-review | **Coverage** | completeness-auditor |
| doc-review | **Audience** | audience-advocate |
| research | **Wide** | breadth |
| research | **Deep** | depth |
| research | **Contrarian** | contrarian |
| ops | **Uptime** | reliability |
| ops | **Lens** | observability |
| ops | **Oncall** | incident-response |

**Preset perspective details:**

```
security-audit:
  attacker:   {name: Red}       "How would I exploit this? What's the weakest link?"
  defender:   {name: Blue}      "How do we detect and prevent attacks? What's our blast radius?"
  compliance:   {name: Auditor}   "Does this meet regulatory requirements? What's our audit trail?"
  web-security: {name: WebSec}    "What OWASP Top 10 risks are present? Check for injection, XSS, path traversal, CORS misconfig, auth bypass, CSRF, response splitting, SSRF (outbound URL validation), missing security headers (HSTS, X-Frame-Options, CSP), rate limiting on auth/API/upload endpoints, and credential/token exposure in logs."

architecture:
  scalability:     {name: Scale}  "Will this handle 10x load? Where are the bottlenecks?"
  maintainability: {name: Craft}  "Can a new engineer understand this in a week? Where's the complexity?"
  simplicity:      {name: Razor}  "What can we remove? Is this the simplest solution?"

research:
  breadth:     {name: Wide}       "What's the full landscape? What options exist? What's adjacent?"
  depth:       {name: Deep}       "What are the deep technical details? What's under the surface?"
  contrarian:  {name: Contrarian} "What's the conventional wisdom wrong about? What's overlooked?"

ops:
  reliability:       {name: Uptime}  "What fails first? What's our recovery time? Where are SPOFs?"
  observability:     {name: Lens}    "Can we see what's happening? What metrics/logs/traces do we need?"
  incident-response: {name: Oncall}  "When this breaks at 3am, what do we need? What's our runbook?"

code-review:
  error-paths:      {name: Pathfinder}  "Trace every error handling path. What's uncaught? What fails silently?"
  api-surface:      {name: Surface}     "Review every public interface. Is the contract clear? Breaking changes?"
  spec-compliance:  {name: Spec}        "Compare implementation against the spec/bead. What's missing? What diverges?"
  # Note: spec-compliance gracefully degrades to general correctness review when no spec
  # is present in context.spec. The judge reviews code on its own merits.

plan-review:
  missing-requirements: {name: Gaps}         "What's not in the spec that should be? What questions haven't been asked?"
  feasibility:          {name: Reality}       "What's technically hard or impossible here? What will take 3x longer than estimated?"
  scope:                {name: Scope}         "What's unnecessary? What's missing? Where will scope creep?"
  spec-completeness:    {name: Completeness}  "Are boundaries defined (Always/Ask First/Never)? Do conformance checks cover all acceptance criteria? Can every acceptance criterion be mechanically verified? Are schema enum values and field names domain-neutral (meaningful in ANY codebase, not just this repo)? Also enforce lifecycle contract completeness: canonical mutation+ack sequence, crash-safe consume protocol with atomic boundary + restart recovery, field-level precedence truth table with anomaly codes, and boundary failpoint conformance tests. Missing/contradictory checklist items are WARN minimum; critical non-mechanically-verifiable invariants are FAIL."

retrospective:
  plan-compliance: {name: Compass}  "What was planned vs what was delivered? What's missing? What was added?"
  tech-debt:       {name: Debt}     "What shortcuts were taken? What will bite us later? What needs cleanup?"
  learnings:       {name: Harvest}  "What patterns emerged? What should be extracted as reusable knowledge?"

product:
  user-value:            {name: User}      "What user problem does this solve? Who benefits and how?"
  adoption-barriers:     {name: Friction}  "What makes this hard to discover, learn, or use? What's the friction?"
  competitive-position:  {name: Edge}      "How does this compare to alternatives? What's our differentiation?"

doc-review:
  clarity-editor:       {name: Clarity}   "Is every sentence unambiguous? Can a reader understand without re-reading? Where's the jargon?"
  accuracy-verifier:    {name: Accuracy}  "Do code examples match the actual API? Are version numbers current? Do links resolve?"
  completeness-auditor: {name: Coverage}  "What's documented but not explained? What's missing entirely? Are edge cases covered?"
  audience-advocate:    {name: Audience}  "Who is the reader? Is the assumed knowledge level consistent? Would a newcomer get lost?"

developer-experience:
  api-clarity:     {name: Signal}  "Is every public interface self-documenting? Can a user predict behavior from names alone?"
  error-experience: {name: SOS}    "When something goes wrong, does the user know what happened, why, and what to do next?"
  discoverability: {name: Beacon}  "Can a new user find this feature without reading docs? Is the happy path obvious?"
```
