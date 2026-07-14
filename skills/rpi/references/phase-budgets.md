# Bounded Tranches Without Phase Controllers

Historical RPI implementations assigned independent time boxes, attempt
allowances, review meters, and wave caps to individual phases. Those controls
are retired. They duplicated state machines, multiplied retries, and turned a
soft flow boundary into permission to work.

The live contract is deliberately smaller:

- one leaf is active per writer;
- a routine tranche pulls one to three low-risk waves;
- each intermediate wave runs only targeted deterministic checks;
- three waves or 90 minutes is a soft return boundary with exact resume state;
- the frozen tranche receives one independent Validate and one Learn pass; and
- the orchestrator records `NOTE`, `REPAIR`, `REPLAN`, `HOLD`, or `ANDON` from
  evidence using the [run disposition contract](pull-flow-governor.md).

These defaults control WIP and feedback cadence. They do not authorize
execution, accumulate cost state, or create a retry allowance. A runtime or
operator may impose a real time, cost, or quota ceiling; RPI treats that ceiling
as external evidence rather than implementing another meter.

Complexity may scale the depth of Discovery, Premortem, and Validate. It does
not add controllers. A timeout or repeated result is evidence for `REPAIR`,
`REPLAN`, or `HOLD`. Only an evidence-backed human-only decision or genuinely
spent hard external ceiling becomes `ANDON`.
