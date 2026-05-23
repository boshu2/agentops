# Executable spec for the /recover skill — session-context recovery (driving-adapter).
# /recover reconstructs where a session left off: the rpi lifecycle phase from
# .agents/rpi/phased-state.json, the claimed/ready work from bd, and recent git state — rendered
# as a recovery dashboard (or JSON). Hexagon: driving-adapter; consumes bd + rpi; produces
# .agents/rpi/*.md. (soc-qk4b)

Feature: Recover reconstructs in-progress session context
  As an agent resuming after a break or compaction
  I want the prior session's lifecycle phase and claimed work surfaced
  So that I continue where I left off instead of starting cold

  Scenario: the rpi lifecycle phase is detected
    When /recover runs
    Then it reads .agents/rpi/phased-state.json (when present) and reports the current phase

  Scenario: claimed and ready work is surfaced from bd
    When /recover runs
    Then it reports in-progress and ready issues from bd

  Scenario: recent git state is included
    When /recover runs
    Then it shows the current branch, recent commits, and uncommitted changes

  Scenario: machine-readable output on demand
    When /recover --json runs
    Then it emits the recovery state as structured JSON
