# Provenance ledger

`docs/provenance/ledger.jsonl` is the committed audit authority for AgentOps
SDLC provenance edges (`agentops-sdlc-provenance.v1`). Each non-blank line is a
single hash-chained edge event written by `ao provenance add`.

The ledger is intentionally append-only:

- validate it with `scripts/validate-provenance-ledger.sh --jsonl docs/provenance/ledger.jsonl`;
- verify the hash chain through `cli/internal/provenancegraph.VerifyChain`;
- rebuild any Dolt/goalstrace projection from this committed JSONL if they ever
  disagree.

The initial file may be empty. A missing or empty ledger means "no committed
edges yet"; the first append creates the genesis record with an empty
`prev_hash`.
