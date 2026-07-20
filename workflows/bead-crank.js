export const meta = {
  name: 'bead-crank',
  description: 'DEPRECATED alias of ship-beads — drive a list of beads to confirmed-merged with worktree-isolated implementer subagents + orchestrator-only reconcile. Delegates to ship-beads; prefer that name ("crank" was jargon).',
  whenToUse: 'Legacy entry point. Identical behavior to ship-beads — this is a thin delegate kept only so existing `bead-crank` invocations keep working. Use ship-beads directly. Pass args = {beads:[...ids], mode:"parallel"|"sequential"}.',
  phases: [],
}

// Dedup (workflow-surface cleanup): bead-crank was a 386-line byte-for-byte
// duplicate of ship-beads modulo "crank"→"ship" wording (the diff was 100%
// cosmetic). Duplicated orchestration logic is drift-prone (cf. the --agy flag
// drift), so the body now delegates to the canonical ship-beads workflow.
// `aliases` in the registry is metadata only (no runtime resolver), so this
// shim — not deletion — is what keeps the `bead-crank` name invocable.
return await workflow('ship-beads', args)
