// Package provenance owns Cobra presentation for the `ao provenance` command
// family. The module builds its command tree with host-provided seams and
// delegates every filesystem and clock effect to the host (ledger-path
// resolution, the clock) or to internal/provenanceapp (session mining), so this
// package performs no direct effect.
package provenance

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/provenanceapp"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// Module owns Cobra presentation for the provenance command family.
type Module struct {
	host clicontract.HostOptions

	// add
	addFromType  string
	addToType    string
	addRelation  string
	addTrustTier string
	addEvidence  string
	addTS        string
	addJSON      bool

	// list
	listJSON     bool
	listFromID   string
	listRelation string

	// export
	exportJSON   bool
	exportVerify bool

	// show
	showJSON bool

	// position
	positionJSON bool

	// trace
	traceOrphans bool
	traceStrict  bool
	traceJSON    bool
	traceGraph   string

	// verify
	verifyJSON bool

	// mine-session
	mineFile  string
	mineState string
	mineJSON  bool
}

// NewModule constructs the provenance command module from its host seams.
func NewModule(host clicontract.HostOptions) *Module {
	return &Module{host: host}
}

// Contract declares provenance's real behavior for the family architecture
// gate. The provenance family did not attach a capabilities contract before the
// carve-out, so the composition does not attach this one either; it exists to
// document the family's effect and profile shape.
func (*Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.provenance",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

func (m *Module) ledgerStore() *provenancegraph.Store {
	return provenancegraph.NewStore(m.host.LedgerPath())
}

// Command builds the `ao provenance` command tree.
func (m *Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:     "provenance",
		Short:   "Write and read optional evidence relationships",
		GroupID: "comms",
		Long: `Append and inspect generic, evidence-backed relationships between
artifacts, decisions, and observations. Records use a hash-chained JSONL file
at docs/provenance/ledger.jsonl when that repository path exists.

Provenance is optional audit evidence. Its availability or contents never
change RPI sequencing, candidate identity, or a Validate verdict.`,
	}
	root.AddCommand(m.addCommand())
	root.AddCommand(m.listCommand())
	root.AddCommand(m.exportCommand())
	root.AddCommand(m.showCommand())
	root.AddCommand(m.positionCommand())
	root.AddCommand(m.traceCommand())
	root.AddCommand(m.verifyCommand())
	root.AddCommand(m.mineSessionCommand())
	return root
}

func (m *Module) addCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <from-id> <to-id>",
		Short: "Append a provenance edge to the ledger",
		Long: `Append one schema-valid, hash-chained provenance edge linking a source
node (<from-id>) to a target node (<to-id>). The edge is sealed onto the
current chain tip (prev_hash = the last record's hash) and validated against
schemas/agentops-sdlc-provenance.v1.schema.json before any write.

The command is idempotent: re-running with the same endpoints, relation,
evidence, and trust tier is a no-op (no duplicate row).

Examples:
  ao provenance add decision-42 cli/cmd/ao/provenance_add.go \
    --relation wasGeneratedBy --to-type artifact
  ao provenance add observation-7 decision-42 \
    --relation wasInfluencedBy --from-type observation --to-type decision \
    --trust-tier authored --evidence docs/decision-42.md`,
		Args: cobra.ExactArgs(2),
		RunE: m.runAdd,
	}
	cmd.Flags().StringVar(&m.addRelation, "relation", "", "Typed PROV-O relation (required), e.g. wasGeneratedBy")
	cmd.Flags().StringVar(&m.addFromType, "from-type", "decision", "Source node type (for example decision, artifact, or observation)")
	cmd.Flags().StringVar(&m.addToType, "to-type", "artifact", "Target node type (for example decision, artifact, or observation)")
	cmd.Flags().StringVar(&m.addTrustTier, "trust-tier", "authored", "Trust tier (authored|inferred|mined)")
	cmd.Flags().StringVar(&m.addEvidence, "evidence", "", "Optional evidence pointer (path, commit, CI run URL, event id)")
	cmd.Flags().StringVar(&m.addTS, "ts", "", "Override the UTC RFC3339 timestamp (defaults to now)")
	cmd.Flags().BoolVar(&m.addJSON, "json", false, "Emit the sealed edge as JSON")
	return cmd
}

func (m *Module) runAdd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ts := m.addTS
	if strings.TrimSpace(ts) == "" {
		ts = m.host.Now().UTC().Format(time.RFC3339)
	}

	edge := provenancegraph.Edge{
		FromID:      args[0],
		FromType:    m.addFromType,
		ToID:        args[1],
		ToType:      m.addToType,
		Relation:    m.addRelation,
		EvidenceRef: m.addEvidence,
		TrustTier:   m.addTrustTier,
		TS:          ts,
	}

	res, err := m.ledgerStore().Append(edge)
	if err != nil {
		return fmt.Errorf("append provenance edge: %w", err)
	}

	out := cmd.OutOrStdout()
	if m.addJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Edge)
	}
	if res.Skipped {
		fmt.Fprintf(out, "edge already present (idempotent no-op): %s --%s--> %s\n",
			res.Edge.FromID, res.Edge.Relation, res.Edge.ToID)
		return nil
	}
	fmt.Fprintf(out, "appended edge %s --%s--> %s (hash %s)\n",
		res.Edge.FromID, res.Edge.Relation, res.Edge.ToID, shortHash(res.Edge.Hash))
	return nil
}

func (m *Module) listCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Read provenance edges back from the ledger",
		Long: `Read the provenance edges recorded in docs/provenance/ledger.jsonl, in
ledger (chain) order. Optionally filter by source node id or relation.

Examples:
  ao provenance list
  ao provenance list --json
  ao provenance list --from-id decision-42
  ao provenance list --relation wasGeneratedBy`,
		Args: cobra.NoArgs,
		RunE: m.runList,
	}
	cmd.Flags().BoolVar(&m.listJSON, "json", false, "Emit machine-readable JSON")
	cmd.Flags().StringVar(&m.listFromID, "from-id", "", "Filter to edges whose from_id matches")
	cmd.Flags().StringVar(&m.listRelation, "relation", "", "Filter to edges with this relation")
	return cmd
}

func (m *Module) runList(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	edges, err := m.ledgerStore().Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	filtered := make([]provenancegraph.Edge, 0, len(edges))
	for _, e := range edges {
		if m.listFromID != "" && e.FromID != m.listFromID {
			continue
		}
		if m.listRelation != "" && e.Relation != m.listRelation {
			continue
		}
		filtered = append(filtered, e)
	}

	out := cmd.OutOrStdout()
	if m.listJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}
	if len(filtered) == 0 {
		fmt.Fprintln(out, "no provenance edges")
		return nil
	}
	for _, e := range filtered {
		fmt.Fprintf(out, "%s --%s--> %s [%s] %s\n",
			e.FromID, e.Relation, e.ToID, e.TrustTier, shortHash(e.Hash))
	}
	return nil
}

func (m *Module) exportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Emit a deterministic, hash-chained rendering of the provenance ledger",
		Long: `Read docs/provenance/ledger.jsonl, canonically sort its edges by
(ts, from_id, to_id, relation), re-seal them into a fresh per-record hash chain,
and emit the result. The canonical sort makes the export byte-identical on
re-run regardless of the ledger's physical append order, so the bytes are
reproducible and the chain is independently verifiable.

The exported chain is verified with NO Dolt server: the committed JSONL is the
audit authority and re-chaining uses only the in-process hashing in
cli/internal/provenancegraph (the same prev_hash discipline as the rpi ledger).

Output:
  default        one compact JSON edge per line (JSONL), trailing newline.
  --json         a single indented JSON array of the sealed edges.
  --verify       re-chain and verify only; print a one-line OK summary and emit
                 nothing on stdout that would vary across runs.

Examples:
  ao provenance export
  ao provenance export --json
  ao provenance export --verify`,
		Args: cobra.NoArgs,
		RunE: m.runExport,
	}
	cmd.Flags().BoolVar(&m.exportJSON, "json", false, "Emit a single indented JSON array instead of JSONL")
	cmd.Flags().BoolVar(&m.exportVerify, "verify", false, "Verify the re-chained export and print only a one-line summary")
	return cmd
}

func (m *Module) runExport(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	edges, err := m.ledgerStore().Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	// Canonical sort + fresh hash chain. Deterministic for a given edge set.
	chain, err := provenancegraph.ReChain(edges)
	if err != nil {
		return fmt.Errorf("re-chain provenance ledger: %w", err)
	}

	// The re-chained export must itself be an intact chain.
	if idx, verr := provenancegraph.VerifyChain(chain); verr != nil {
		return fmt.Errorf("exported chain failed verification at record %d: %w", idx, verr)
	}

	out := cmd.OutOrStdout()

	if m.exportVerify {
		fmt.Fprintf(out, "OK: %d edge(s) re-chained and verified (no Dolt server)\n", len(chain))
		return nil
	}

	if m.exportJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		// Always emit a concrete (possibly empty) array, never null.
		if chain == nil {
			chain = []provenancegraph.Edge{}
		}
		return enc.Encode(chain)
	}

	// Default: deterministic JSONL — one compact line per edge, fixed field
	// order (Edge struct tag order), single trailing newline per record.
	for _, e := range chain {
		line, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal edge: %w", err)
		}
		if _, err := fmt.Fprintf(out, "%s\n", line); err != nil {
			return fmt.Errorf("write edge: %w", err)
		}
	}
	return nil
}

func (m *Module) showCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <node-id>",
		Short: "Show generic provenance relationships for one exact node",
		Long: `Read the provenance ledger and show every edge whose from_id or to_id
exactly matches the supplied node. This command reports evidence; it does not
infer completion, landing, validation, or a next action.`,
		Args: cobra.ExactArgs(1),
		RunE: m.runShow,
	}
	cmd.Flags().BoolVar(&m.showJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

type showEdge struct {
	Record      int    `json:"record"`
	Direction   string `json:"direction"`
	Counterpart string `json:"counterpart"`
	Type        string `json:"counterpart_type"`
	Relation    string `json:"relation"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	TrustTier   string `json:"trust_tier"`
	Timestamp   string `json:"ts"`
	Hash        string `json:"hash"`
}

type showReport struct {
	NodeID        string     `json:"node_id"`
	Relationships []showEdge `json:"relationships"`
	TotalRecords  int        `json:"total_records"`
}

func buildShowReport(edges []provenancegraph.Edge, nodeID string) (showReport, error) {
	report := showReport{NodeID: nodeID, Relationships: []showEdge{}, TotalRecords: len(edges)}
	for i, edge := range edges {
		view := showEdge{Record: i + 1, Relation: edge.Relation, EvidenceRef: edge.EvidenceRef, TrustTier: edge.TrustTier, Timestamp: edge.TS, Hash: edge.Hash}
		switch {
		case edge.FromID == nodeID:
			view.Direction, view.Counterpart, view.Type = "outbound", edge.ToID, edge.ToType
		case edge.ToID == nodeID:
			view.Direction, view.Counterpart, view.Type = "inbound", edge.FromID, edge.FromType
		default:
			continue
		}
		report.Relationships = append(report.Relationships, view)
	}
	if len(report.Relationships) == 0 {
		return showReport{}, fmt.Errorf("node %q is not present in the provenance ledger", nodeID)
	}
	return report, nil
}

func renderShowReport(out io.Writer, report showReport) {
	fmt.Fprintf(out, "node %s (%d relationship(s))\n", report.NodeID, len(report.Relationships))
	for _, edge := range report.Relationships {
		fmt.Fprintf(out, "  %s --%s--> %s [%s] record %d/%d\n", edge.Direction, edge.Relation, edge.Counterpart, edge.TrustTier, edge.Record, report.TotalRecords)
		if edge.EvidenceRef != "" {
			fmt.Fprintf(out, "    evidence: %s\n", edge.EvidenceRef)
		}
	}
}

func (m *Module) runShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	edges, err := m.ledgerStore().Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}
	report, err := buildShowReport(edges, args[0])
	if err != nil {
		return err
	}
	if m.showJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	renderShowReport(cmd.OutOrStdout(), report)
	return nil
}

func (m *Module) positionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "Report the read-only provenance ledger tip",
		Long:  "Report the ledger record count and latest hash without inferring lifecycle state.",
		Args:  cobra.NoArgs,
		RunE:  m.runPosition,
	}
	cmd.Flags().BoolVar(&m.positionJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

type positionEdge struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
	Hash     string `json:"hash"`
}

type positionReport struct {
	TotalEdges int           `json:"total_edges"`
	TipHash    string        `json:"tip_hash"`
	Latest     *positionEdge `json:"latest,omitempty"`
}

func buildPositionReport(edges []provenancegraph.Edge) positionReport {
	report := positionReport{TotalEdges: len(edges)}
	if len(edges) == 0 {
		return report
	}
	edge := edges[len(edges)-1]
	report.TipHash = edge.Hash
	report.Latest = &positionEdge{FromID: edge.FromID, ToID: edge.ToID, Relation: edge.Relation, Hash: edge.Hash}
	return report
}

func (m *Module) runPosition(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	edges, err := m.ledgerStore().Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}
	report := buildPositionReport(edges)
	if m.positionJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	if report.Latest == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "provenance ledger is empty")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "provenance ledger: %d edge(s), tip %s\n", report.TotalEdges, shortHash(report.TipHash))
	fmt.Fprintf(cmd.OutOrStdout(), "latest: %s --%s--> %s\n", report.Latest.FromID, report.Latest.Relation, report.Latest.ToID)
	return nil
}

func (m *Module) traceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Audit the provenance graph for orphan artifacts (no inbound edge)",
		Long: `Audit a provenance trace-graph for orphans: engineered artifact nodes
that have NO inbound authored/inferred provenance edge. This generalizes
'ao goals trace --orphans' (the directive→scenario→bead→artifact→learning
chain-gap audit) onto the provenance graph, where the orphan condition is "an
artifact node that nothing produces".

The graph is a JSONL file using the goalstrace Node/Edge JSON contract
(cli/internal/goalstrace/graph.go) with a leading "record" discriminator. Read
the committed audit authority (docs/provenance/ledger.jsonl is the ledger; a
trace-graph is the node/edge projection) or a fixture passed with --graph.

  --orphans   audit for artifact nodes with no inbound edge (required mode).
  --strict    exit non-zero when any orphan exists (the CI gate uses this).
  --json      emit one finding object per line (line-delimited JSON).
  --graph     path to the JSONL trace-graph to audit (required).

Without --strict the audit reports orphans but exits 0 (advisory). With
--strict any orphan fails the command, which is how the no-orphan provenance
gate in .github/workflows/validate.yml blocks merges.

Examples:
  ao provenance trace --orphans --graph tests/fixtures/provenance/orphan-stale-65-jobs.jsonl
  ao provenance trace --orphans --strict --graph docs/provenance/graph.jsonl
  ao provenance trace --orphans --json --graph <file>`,
		Args: cobra.NoArgs,
		RunE: m.runTrace,
	}
	cmd.Flags().BoolVar(&m.traceOrphans, "orphans", false, "Audit for artifact nodes with no inbound provenance edge")
	cmd.Flags().BoolVar(&m.traceStrict, "strict", false, "Exit non-zero when any orphan exists")
	cmd.Flags().BoolVar(&m.traceJSON, "json", false, "Emit each finding as one JSON object per line")
	cmd.Flags().StringVar(&m.traceGraph, "graph", "", "Path to the JSONL trace-graph to audit (required)")
	return cmd
}

func (m *Module) runTrace(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	if !m.traceOrphans {
		return fmt.Errorf("ao provenance trace requires --orphans (the only supported mode)")
	}
	if m.traceGraph == "" {
		return fmt.Errorf("ao provenance trace --orphans requires --graph <path>")
	}

	recs, err := provenancegraph.ReadGraphRecords(m.traceGraph)
	if err != nil {
		return fmt.Errorf("read trace-graph: %w", err)
	}
	findings := provenancegraph.FindOrphans(recs)

	out := cmd.OutOrStdout()
	if m.traceJSON {
		enc := json.NewEncoder(out)
		for _, f := range findings {
			if err := enc.Encode(f); err != nil {
				return fmt.Errorf("encode orphan finding: %w", err)
			}
		}
	} else if len(findings) == 0 {
		fmt.Fprintln(out, "No provenance orphans found.")
	} else {
		for _, f := range findings {
			fmt.Fprintf(out, "ERROR  %-26s %s\n", f.Code, f.Message)
		}
		fmt.Fprintf(out, "\n%d orphan(s)\n", len(findings))
		if !m.traceStrict {
			fmt.Fprintln(out, "(orphans do not fail the command; pass --strict to escalate)")
		}
	}

	if len(findings) > 0 && m.traceStrict {
		return fmt.Errorf("provenance graph has %d orphan(s) (--strict)", len(findings))
	}
	return nil
}

func (m *Module) verifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the committed provenance ledger's hash chain IN PLACE",
		Long: `Read docs/provenance/ledger.jsonl exactly as committed and verify its
per-record hash chain without re-sorting or re-chaining. Each non-blank line
must parse as a schema-valid edge AND its prev_hash must link to the prior
record's hash with payload_hash/hash recomputing — so a tampered field, a
forged hash, or a reordered row is CAUGHT and the offending file line is named.

This is the tamper-detection surface for the audit authority: unlike
'ao provenance export --verify' (which canonically re-sorts and re-chains the
edge set), 'verify' checks the committed bytes in place, so an altered ledger
fails loudly instead of being silently re-sealed.

Exit status:
  0   the committed chain is intact (or the ledger is absent/empty)
  1   a broken/tampered/non-conforming record was found (the line is named)

Examples:
  ao provenance verify
  ao provenance verify --json`,
		Args: cobra.NoArgs,
		RunE: m.runVerify,
	}
	cmd.Flags().BoolVar(&m.verifyJSON, "json", false, "Emit the machine-readable verify result as JSON")
	return cmd
}

func (m *Module) runVerify(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	res, err := m.ledgerStore().VerifyFile()
	if err != nil {
		return fmt.Errorf("verify provenance ledger: %w", err)
	}

	out := cmd.OutOrStdout()
	if m.verifyJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(res); encErr != nil {
			return encErr
		}
	} else if res.Pass {
		fmt.Fprintf(out, "OK: provenance ledger chain intact (%d record(s))\n", res.RecordCount)
	} else {
		fmt.Fprintf(out, "BROKEN: provenance ledger chain breaks at line %d: %s\n",
			res.FirstBrokenLine, res.Message)
	}

	if !res.Pass {
		// Non-zero exit on a broken/tampered ledger, with the line already named.
		return fmt.Errorf("provenance ledger verification failed at line %d: %s",
			res.FirstBrokenLine, res.Message)
	}
	return nil
}

func (m *Module) mineSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mine-session --file <session.jsonl>",
		Short: "Mine deterministic per-inference provenance events from a session transcript",
		Long: `Parse a Claude Code or Codex session transcript and emit the per-inference
provenance events it DETERMINISTICALLY evidences (E6, ADR-0010: build-native, own
the PROV-O graph). Today that is one tool_call event per tool use, with a stable
idempotent id; the Kind enum is extensible to context_entered/context_missed once
their deterministic evidence is defined (never inferred speculatively).

Incremental (--state): re-running mines only NEW lines (skip-consumed), keyed by
a watermark + a prefix checksum. If the transcript's already-mined prefix changed
(truncated/rewritten), the watermark is invalid and the whole file is re-mined
(rollback) — borrowed from cass's incremental-index discipline (stale-is-usable,
recover loudly, never rebuild expensive state unnecessarily).

Output (--json, default): one JSON event per line on stdout. The events feed the
PROV-O graph via a downstream step (e.g. wired as an ASSAY --mine-cmd); this
command does not itself write the committed ledger.`,
		RunE: m.runMineSession,
	}
	cmd.Flags().StringVar(&m.mineFile, "file", "", "Path to the session transcript (.jsonl) to mine (required)")
	cmd.Flags().StringVar(&m.mineState, "state", "", "Path to the incremental watermark state JSON (created/updated; omit for a full one-shot mine)")
	cmd.Flags().BoolVar(&m.mineJSON, "json", true, "Emit events as JSONL on stdout")
	return cmd
}

func (m *Module) runMineSession(cmd *cobra.Command, _ []string) error {
	return provenanceapp.MineSession(provenanceapp.MineOptions{
		File:  m.mineFile,
		State: m.mineState,
		JSON:  m.mineJSON,
	}, cmd.OutOrStdout())
}

// shortHash returns the first 12 chars of a hash for human-readable output.
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
