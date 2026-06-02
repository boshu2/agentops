Feature: Phase 2 — migrate salvaged memory onto bd
  As an operator consolidating fragmented agent memory
  I want keepers imported to bd with typed provenance and decay-ranked recall
  So that bd becomes the single source of truth with a generated cache

  Background:
    Given a Phase-1 manifest of keepers approved at Gate A
    And bd is the canonical memory store

  Scenario: Round-trip a typed memory through the fenced header
    Given a memory with type, source, utility, and access_count
    When I render it and parse it back
    Then every structured field survives unchanged
    And a memory with no header parses as a provisional fact

  Scenario: Rank recall by decay, access, and type
    Given a recent high-access memory and an old untouched memory
    When I recall within a token budget
    Then the recent high-access memory ranks first
    And superseded memories are excluded
    And episodic memories decay faster than feedback memories

  Scenario: Recall bumps the access signal
    When I recall a memory with bumping enabled
    Then its access_count increases by one

  Scenario: Generate the thin MEMORY.md cache from bd
    Given memories of several types in bd
    When I regenerate the MEMORY.md index
    Then it is grouped by type, one line per memory, under 200 lines
    And it is marked generated and not to be hand-edited

  Scenario: Import keepers idempotently with provenance
    Given a manifest with one authored keeper
    When I run the importer twice
    Then exactly one bd memory exists with a file: provenance source
    And re-running produces no duplicates

  Scenario: Dry-run import writes nothing
    When I run the importer with dry-run
    Then the ledger reports the keepers but bd is untouched

  Scenario: Unified write path stamps provenance
    When any harness writes through the remember entrypoint
    Then the memory lands in bd with its type, source, and maturity
