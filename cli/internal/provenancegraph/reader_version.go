package provenancegraph

// payloadHashSkewHint is shared by verification paths. A mismatch may be real
// corruption or a reader whose historical field set differs from the writer.
// The ledger is evidence only; this diagnostic never authorizes continuation.
const payloadHashSkewHint = "payload_hash mismatch — record content was tampered with, or this reader does not understand the historical record shape; rebuild ao from source and inspect the exact record"
