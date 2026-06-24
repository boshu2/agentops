# E3.CC — Concurrency/crash contract for the hash-chained provenance ledger
#
# age-membrane-memory-arch-tz2s.4.5. The provenance ledger (docs/provenance/
# ledger.jsonl) is the membrane's verdict audit authority — "no verdict = not
# done" reads from it, and the #trivial / pawl-pre-push gates check it. Every
# edge SEALS onto the current chain tip (prev_hash = last record's hash), so the
# read-seal-write in Store.Append is a critical section: two concurrent appenders
# that each read the same tip would both seal onto it and FORK the chain. ml8's
# standing pawl-service made this live — concurrent routes emit verdict edges
# concurrently. The contract: appends are serialized by a cross-process advisory
# lock (flock on a sidecar .lock file), so the chain never forks and no append is
# lost. The executable proof is store_test.go:TestStore_ConcurrentAppendDoesNotForkChain.

Feature: Provenance ledger append is concurrency-safe and never forks the chain

  Background:
    Given a provenance ledger Store bound to a JSONL path

  Scenario: Concurrent appends of distinct edges keep one intact chain
    Given 32 distinct edges to append from 32 concurrent writers
    When all writers call Store.Append at the same time
    Then every edge is present in the ledger exactly once
    And VerifyChain reports no break (each prev_hash links to the prior hash)
    And no two records share a prev_hash

  Scenario: A second process appending concurrently is serialized, not interleaved
    Given one writer holds the append lock on the ledger
    When a second writer attempts to append
    Then the second writer blocks until the first releases the lock
    And the second edge seals onto the first edge's hash, not a stale tip

  Scenario: Idempotent skip still takes the lock
    Given an edge whose identity already exists in the ledger
    When Store.Append is called for that identity under concurrency
    Then the append is an idempotent no-op (Skipped=true)
    And no duplicate record and no fork is produced

  Scenario: The lock sidecar never pollutes the committed ledger
    Given an append creates the ledger and its .lock sidecar
    Then the .lock sidecar is gitignored (docs/provenance/*.lock)
    And only ledger.jsonl is the committed audit record
