Feature: Phase 1 — read-only memory-layer audit
  As an operator consolidating fragmented agent memory onto bd
  I want a read-only inventory + keep/drop manifest + reversibility plan
  So that I can approve the migration at Gate A before anything is written

  Background:
    Given a target home with one or more memory layers
    And the audit runs strictly read-only

  @covered-by:skills/bd-first-memory-migration/tests/test_phase1_audit.py
  Scenario: Inventory every present layer without mutation
    When I run the layer scanner against the home
    Then each present layer reports file count, bytes, and dup-rate
    And absent layers are reported with a reason, never fatal
    And no file in the home is created, modified, or deleted

  @covered-by:skills/bd-first-memory-migration/tests/test_phase1_audit.py
  Scenario: Flag a stalled ao flywheel
    Given an ao store where over 95 percent of nodes are provisional
    When I run the scanner
    Then the ao layer is marked STALLED with the provisional percentage

  @covered-by:skills/bd-first-memory-migration/tests/test_phase1_audit.py
  Scenario: Flag a bloated MEMORY.md index
    Given a Claude MEMORY.md index longer than 200 lines
    When I run the scanner
    Then that index is flagged as likely bloated

  @covered-by:skills/bd-first-memory-migration/tests/test_phase1_audit.py
  Scenario: Classify keepers and junk into a dry-run manifest
    Given authored knowledge files and a pile of duplicated pending extracts
    When I run the classifier
    Then authored knowledge is verdict keep
    And pending-extract and duplicate files are verdict drop
    And the manifest reports estimated disk reclaimed and memories-after
    And the manifest is marked dry_run true

  @covered-by:skills/bd-first-memory-migration/tests/test_phase1_audit.py
  Scenario: Produce the Gate A audit report with a reversibility plan
    When I generate the audit report
    Then it contains the current-state table and the keep/drop summary
    And it names every later destructive action with its rollback
    And it states that Gate A must be approved before Phase 2 writes
