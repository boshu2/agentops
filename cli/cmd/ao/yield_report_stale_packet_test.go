// Tests for the stale goal-design-packet sweep in `ao yield report`'s andon
// queue (evidence-bound-goal-closeout scenario S3, slice B2).
//
// Executed-red TDD. Fixture fidelity (.claude/rules/go.md): packet fixtures
// carry the REAL persisted frontmatter shape (modeled key-for-key on
// .agents/goal-design/evidence-bound-goal-closeout), and the fixture
// provenance ledger copies the field layout of a real verdict edge from
// docs/provenance/ledger.jsonl — never a minimal stub production could not
// emit. Everything runs through runYieldReport (L2), rooted in a t.TempDir()
// via setYieldReportState, so the sweep is exercised end-to-end and hermetic.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stalePacketIntentTemplate is a realistic goal-design intent.md, modeled on
// .agents/goal-design/evidence-bound-goal-closeout/intent.md (same key set and
// nesting the production goal-design skill emits).
const stalePacketIntentTemplate = `---
schema_version: 1
kind: goal-design.intent
id: gd-intent-{{SLUG}}
slug: {{SLUG}}
created_at: '2026-07-08T09:00:00Z'
status: {{STATUS}}
objective: Fixture packet for the stale goal-design sweep - one behavior with
  a mechanical evidence trail.
why_it_matters: A packet left in draft while its work shipped splits the knowledge
  layer from tracker/repo ground truth.
domain_terms:
- term: goal-design packet
  definition: The two-artifact intent and driver contract that turns a goal into
    loop-ready work.
  source: docs/contracts/goal-design-artifacts.md
bdd:
  feature: Fixture behavior
  scenarios:
  - id: S1
    name: Fixture scenario
    given:
    - A fixture precondition
    when:
    - The fixture action runs
    then:
    - The fixture outcome is observable
boundaries:
  bounded_context: bc-loop
  in_scope:
  - one fixture slice
  non_goals:
  - anything beyond the fixture
  rollback_or_containment: Revert the fixture commit.
evidence_for_done:
  first_failing_proof: fixture test goes red then green
  validation_command: scripts/check-goal-design-packet.sh .agents/goal-design/{{SLUG}}
  evidence_path: .agents/goal-design/{{SLUG}}
  independent_gate: validate
inputs_to_recheck:
  repo_paths:
  - cli/cmd/ao/
hard_rules:
- Keep behavior slices small.
---
# Goal Design Intent: {{SLUG}}

## Objective

Fixture packet for the stale goal-design sweep.
`

// stalePacketDriverTemplate is a realistic goal-design driver.md, modeled on
// .agents/goal-design/evidence-bound-goal-closeout/driver.md. Note the nested
// "- id:" under candidate_beads and the top-level "id:" — the sweep must read
// the candidate ids, not the artifact id.
const stalePacketDriverTemplate = `---
schema_version: 1
kind: goal-design.driver
id: gd-driver-{{SLUG}}
slug: {{SLUG}}
created_at: '2026-07-08T09:00:00Z'
status: {{STATUS}}
intent_ref:
  path: .agents/goal-design/{{SLUG}}/intent.md
  sha256: 16e60dc9ab33e87ce8fddba9705fc5d0ec21f3f8a085d5cc7b84a19ad6bef3a5
  schema_version: 1
loop_routing:
  delivery: Land the single candidate as one slice.
  rpi: One inner tick for the candidate.
  promotion: Contract doc updates ride the slice.
  knowledge: Record why the packet went stale.
candidate_beads:
- id: {{BEAD}}
  behavior: 'S1: fixture behavior ships with mechanical evidence'
  bounded_context: bc-loop
  first_failing_proof: fixture test red then green
  write_scope:
  - cli/cmd/ao/
  close_signal: Fixture test green, independent PASS verdict
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
  - The change starts mixing unrelated behavior.
route_back_rules:
  validation_fails: AUTO-REDO the slice against the fixture proof.
  bead_closes_with_new_signal: Use the close verdict to choose the next candidate.
  candidate_stale: Re-read the named inputs and revalidate.
  promotion_contradicts_intent: Stop for /council before proceeding.
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: Only when durability is required.
    workflow: Only for deterministic structured DAG needs.
artifact_validation:
  checker_command: scripts/check-goal-design-packet.sh .agents/goal-design/{{SLUG}}
  independent_validator: validate
  required_verdict: PASS
---
# Goal Design Driver: {{SLUG}}

## Source Intent

- Intent artifact: ` + "`.agents/goal-design/{{SLUG}}/intent.md`" + `

## Candidate Beads

| Candidate | Behavior |
| --- | --- |
| {{BEAD}} | S1: fixture behavior |
`

// stalePacketCloseStamp is the driver-body close stamp a CLOSED packet carries,
// byte-matching the production close tool's appended shape: a blank line, then
// bare stamp bullets — no heading (scripts/goal-design-packet.py stamp_lines;
// real example: .agents/goal-design/evidence-bound-post-mortem-closeout).
const stalePacketCloseStamp = `
- Closed: 2026-07-09T10:00:00Z (prior status: validated)
- Disposition {{BEAD}}: closed - fixture bead landed with verdict
`

// writeStalePacketFixture writes a real-shaped goal-design packet
// (intent.md + driver.md) under root/.agents/goal-design/<slug>/ with the
// given status in BOTH files, and returns the packet dir.
func writeStalePacketFixture(t *testing.T, root, slug, status, candidateBeadID string) string {
	t.Helper()
	dir := filepath.Join(root, ".agents", "goal-design", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir packet dir: %v", err)
	}
	r := strings.NewReplacer("{{SLUG}}", slug, "{{STATUS}}", status, "{{BEAD}}", candidateBeadID)
	driver := r.Replace(stalePacketDriverTemplate)
	if status == "closed" {
		driver += r.Replace(stalePacketCloseStamp)
	}
	for name, content := range map[string]string{
		"intent.md": r.Replace(stalePacketIntentTemplate),
		"driver.md": driver,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// writeStaleSweepLedger writes root/docs/provenance/ledger.jsonl from raw lines.
func writeStaleSweepLedger(t *testing.T, root string, lines ...string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "provenance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir provenance dir: %v", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
}

// confirmedVerdictEdgeLine is a CONFIRMED pawl-verdict edge naming slug —
// field-for-field the shape of a real docs/provenance/ledger.jsonl verdict
// line (the sweep only parses it; hashes need not verify).
func confirmedVerdictEdgeLine(slug, ts string) string {
	return fmt.Sprintf(`{"schema_version":"agentops-sdlc-provenance.v1","from_id":"%s@8e3ce11","from_type":"verdict","to_id":"8e3ce11668003ab9557310e3a0989397067f32e1","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict %s disposition=CONFIRMED","bead_id":"%s","trust_tier":"inferred","ts":"%s","reviewer_family":"claude+gpt","rounds":1,"duration_s":216,"tokens_est":3286,"evidence_path":"/tmp/pawl-evidence/%s-opus.txt","prev_hash":"cfc53da9e8cefc53d0fb453bed2f8682d2255f934cd20ed941c242d926b1988f","payload_hash":"ce6c22c79a4cab2c66c647a31077013b59a2854ed7bf9a1c18ffd520415a50a3","hash":"b5e5da272bcb31a4f74e70bc6a67e7454a3bf4b5ab6ecd9fdf6c06366c589983"}`,
		slug, slug, slug, ts, slug)
}

// generatedByEdgeLine is a NON-verdict bead→commit creation edge naming slug —
// the self-triggering shape the predicate must ignore (the packet-creation
// commit always names the slug).
func generatedByEdgeLine(slug, ts string) string {
	return fmt.Sprintf(`{"schema_version":"agentops-sdlc-provenance.v1","from_id":"%s","from_type":"bead","to_id":"feat/%s-fixture","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"_beads/issues.jsonl#%s","trust_tier":"authored","ts":"%s","prev_hash":"","payload_hash":"52b578a2673e9b35c934ede99964c2737d7bf23e8d297a761f2e1a78a25f2527","hash":"ae78526fed7a820d27cdaf7edc219e8e72b913ac0bbcdb1829504ebc1cc4ec7c"}`,
		slug, slug, slug, ts)
}

// stalePacketRows filters the andon queue to the stale-packet rows.
func stalePacketRows(doc yieldReportDoc) []yieldReportAndonRow {
	rows := []yieldReportAndonRow{}
	for _, r := range doc.AndonQueue {
		if r.Kind == "stale-packet" {
			rows = append(rows, r)
		}
	}
	return rows
}

// TestRunYieldReport_StalePacketFlaggedByLedgerEvidence is fixture (i): a
// packet still in draft whose slug is named by a CONFIRMED verdict edge in the
// provenance ledger is flagged as ONE stale-packet andon row whose why names
// the packet path AND the ledger evidence. The edge is 30h old — the sweep is
// a queue, not a --since window. A junk line and a non-verdict edge in the
// ledger must not break or trigger the sweep.
func TestRunYieldReport_StalePacketFlaggedByLedgerEvidence(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)
	slug := "stale-sweep-fixture"
	writeStalePacketFixture(t, root, slug, "draft", "B1")

	edgeTS := reportTestNow.Add(-30 * time.Hour).UTC().Format(time.RFC3339)
	writeStaleSweepLedger(t, root,
		generatedByEdgeLine(slug, "2026-07-08T05:00:00Z"),
		"not-json {{",
		confirmedVerdictEdgeLine(slug, edgeTS),
	)

	doc := decodeReport(t)
	rows := stalePacketRows(doc)
	if len(rows) != 1 {
		t.Fatalf("stale-packet rows = %d, want exactly 1; queue: %+v", len(rows), doc.AndonQueue)
	}
	row := rows[0]
	if row.ID != slug {
		t.Errorf("row.ID = %q, want %q", row.ID, slug)
	}
	if row.Kind != "stale-packet" {
		t.Errorf("row.Kind = %q, want %q", row.Kind, "stale-packet")
	}
	wantPath := ".agents/goal-design/" + slug
	if !strings.Contains(row.Why, wantPath) {
		t.Errorf("row.Why = %q, must name the packet path %q", row.Why, wantPath)
	}
	if !strings.Contains(row.Why, "docs/provenance/ledger.jsonl") {
		t.Errorf("row.Why = %q, must name the ledger evidence source", row.Why)
	}
	if row.Since != edgeTS {
		t.Errorf("row.Since = %q, want the verdict-edge ts %q", row.Since, edgeTS)
	}
	if row.Age != "30h" {
		t.Errorf("row.Age = %q, want %q", row.Age, "30h")
	}
	if len(doc.AndonQueue) != 1 {
		t.Errorf("andon queue = %d rows, want only the stale-packet row: %+v", len(doc.AndonQueue), doc.AndonQueue)
	}
}

// TestRunYieldReport_ClosedOrSupersededPacketNeverFlagged is fixture (ii): a
// packet with status closed in BOTH files — with the SAME CONFIRMED ledger
// evidence — is never flagged; nor is a superseded packet.
func TestRunYieldReport_ClosedOrSupersededPacketNeverFlagged(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)
	writeStalePacketFixture(t, root, "closed-fixture", "closed", "B1")
	writeStalePacketFixture(t, root, "superseded-fixture", "superseded", "B1")
	writeStaleSweepLedger(t, root,
		confirmedVerdictEdgeLine("closed-fixture", "2026-07-08T06:00:00Z"),
		confirmedVerdictEdgeLine("superseded-fixture", "2026-07-08T06:00:00Z"),
	)

	doc := decodeReport(t)
	if rows := stalePacketRows(doc); len(rows) != 0 {
		t.Errorf("closed/superseded packets flagged: %+v", rows)
	}
	if len(doc.AndonQueue) != 0 {
		t.Errorf("andon queue = %+v, want empty", doc.AndonQueue)
	}
}

// TestRunYieldReport_DraftPacketWithoutEvidenceNotFlagged is fixture (iii): a
// draft packet with NO closing evidence is never flagged — neither an
// unrelated CONFIRMED edge nor the packet's own non-verdict creation edge
// counts as evidence, and an unresolvable tracker (empty stub) is skipped
// silently.
func TestRunYieldReport_DraftPacketWithoutEvidenceNotFlagged(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)
	slug := "no-evidence-fixture"
	writeStalePacketFixture(t, root, slug, "draft", "B1")
	writeStaleSweepLedger(t, root,
		confirmedVerdictEdgeLine("some-other-goal", "2026-07-08T06:00:00Z"),
		generatedByEdgeLine(slug, "2026-07-08T05:00:00Z"),
	)

	doc := decodeReport(t)
	if rows := stalePacketRows(doc); len(rows) != 0 {
		t.Errorf("evidence-less draft packet flagged: %+v", rows)
	}
	if len(doc.AndonQueue) != 0 {
		t.Errorf("andon queue = %+v, want empty", doc.AndonQueue)
	}
}

// TestRunYieldReport_StalePacketFlaggedByLandedCommit is scenario S2 of
// verification-surface-honesty (evidence arm c): a draft packet whose ONLY
// shipped evidence is a landed trunk commit whose SUBJECT cites the slug
// together with a driver candidate id is flagged, with that commit named —
// no provenance ledger and no tracker rows at all.
func TestRunYieldReport_StalePacketFlaggedByLandedCommit(t *testing.T) {
	root := gitInitRepoT(t)
	setYieldReportState(t, root)
	slug := "landed-arm-fixture"
	writeStalePacketFixture(t, root, slug, "draft", "B1")
	sha := commitFileT(t, root, "src.txt", "the slice", "feat(loop): fixture slice lands (landed-arm-fixture B1)")

	doc := decodeReport(t)
	rows := stalePacketRows(doc)
	if len(rows) != 1 {
		t.Fatalf("stale-packet rows = %d, want exactly 1; queue: %+v", len(rows), doc.AndonQueue)
	}
	row := rows[0]
	if row.ID != slug {
		t.Errorf("row.ID = %q, want %q", row.ID, slug)
	}
	wantPath := ".agents/goal-design/" + slug
	if !strings.Contains(row.Why, wantPath) {
		t.Errorf("row.Why = %q, must name the packet path %q", row.Why, wantPath)
	}
	if !strings.Contains(row.Why, sha[:12]) {
		t.Errorf("row.Why = %q, must name the landed commit %s", row.Why, sha[:12])
	}
	if !strings.Contains(row.Why, "B1") {
		t.Errorf("row.Why = %q, must name the cited candidate B1", row.Why)
	}
	wantSince := gitCommitTimeUTC(t, root, sha)
	if row.Since != wantSince {
		t.Errorf("row.Since = %q, want the landed commit's committer date %q", row.Since, wantSince)
	}
}

// TestRunYieldReport_PacketCreationCommitNeverSelfTriggers is the S2 negative
// arm: a draft packet whose ONLY slug mention anywhere is its own
// packet-creation commit (slug in the subject, NO candidate id) is never
// flagged — the landed-commit arm must not degenerate into bare git-log slug
// matching (the rejected self-triggering design).
func TestRunYieldReport_PacketCreationCommitNeverSelfTriggers(t *testing.T) {
	root := gitInitRepoT(t)
	setYieldReportState(t, root)
	slug := "creation-only-fixture"
	writeStalePacketFixture(t, root, slug, "draft", "B1")
	commitFileT(t, root, "notes.txt", "packet", "docs(goal-design): add creation-only-fixture packet")

	doc := decodeReport(t)
	if rows := stalePacketRows(doc); len(rows) != 0 {
		t.Errorf("creation-commit-only packet flagged (self-triggering arm): %+v", rows)
	}
	if len(doc.AndonQueue) != 0 {
		t.Errorf("andon queue = %+v, want empty", doc.AndonQueue)
	}
}

// gitCommitTimeUTC returns sha's committer date as UTC RFC3339 — the shape
// rfc3339OrEmpty renders into row.Since.
func gitCommitTimeUTC(t *testing.T, repo, sha string) string {
	t.Helper()
	raw := runGitT(t, repo, "show", "-s", "--format=%cI", sha)
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse committer date %q: %v", raw, err)
	}
	return ts.UTC().Format(time.RFC3339)
}

// TestRunYieldReport_StalePacketFlaggedFromSubdirCwd is scenario S1 of
// verification-surface-honesty: the sweep must flag a stale packet identically
// when the command runs from a repo SUBDIRECTORY. It drives the command-level
// root-resolution seam (testProjectDir simulates the cwd runYieldReport
// resolves), not buildAndonQueue with a pre-resolved root — running `ao yield
// report` from cli/ silently reported no stale packets on 2026-07-10.
func TestRunYieldReport_StalePacketFlaggedFromSubdirCwd(t *testing.T) {
	root := gitInitRepoT(t)
	setYieldReportState(t, root)
	slug := "subdir-cwd-fixture"
	writeStalePacketFixture(t, root, slug, "draft", "B1")
	edgeTS := reportTestNow.Add(-30 * time.Hour).UTC().Format(time.RFC3339)
	writeStaleSweepLedger(t, root, confirmedVerdictEdgeLine(slug, edgeTS))

	subdir := filepath.Join(root, "cli")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	// setYieldReportState pointed the command at root and registered cleanup;
	// re-point it at the subdirectory to simulate a subdir invocation.
	testProjectDir = subdir

	doc := decodeReport(t)
	rows := stalePacketRows(doc)
	if len(rows) != 1 {
		t.Fatalf("stale-packet rows from subdir cwd = %d, want exactly 1 (sweep went cwd-blind); queue: %+v", len(rows), doc.AndonQueue)
	}
	row := rows[0]
	if row.ID != slug {
		t.Errorf("row.ID = %q, want %q", row.ID, slug)
	}
	wantPath := ".agents/goal-design/" + slug
	if !strings.Contains(row.Why, wantPath) {
		t.Errorf("row.Why = %q, must name the packet path %q", row.Why, wantPath)
	}
	if row.Since != edgeTS {
		t.Errorf("row.Since = %q, want the verdict-edge ts %q", row.Since, edgeTS)
	}
}

// TestRunYieldReport_StalePacketFlaggedByClosedCandidateBead covers the second
// evidence arm: a validated packet whose driver candidate bead id resolves to
// a CLOSED bead in the tracker is flagged, with why naming the packet path and
// the closed bead — no provenance ledger present at all.
func TestRunYieldReport_StalePacketFlaggedByClosedCandidateBead(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)
	slug := "closed-bead-fixture"
	writeStalePacketFixture(t, root, slug, "validated", "age-b2work")

	closedAt := reportTestNow.Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	stubReportBeads(t, map[string][]reportBead{
		"closed": {{ID: "age-b2work", Title: "fixture slice landed", Status: "closed", ClosedAt: closedAt}},
	})

	doc := decodeReport(t)
	rows := stalePacketRows(doc)
	if len(rows) != 1 {
		t.Fatalf("stale-packet rows = %d, want exactly 1; queue: %+v", len(rows), doc.AndonQueue)
	}
	row := rows[0]
	if row.ID != slug {
		t.Errorf("row.ID = %q, want %q", row.ID, slug)
	}
	wantPath := ".agents/goal-design/" + slug
	if !strings.Contains(row.Why, wantPath) {
		t.Errorf("row.Why = %q, must name the packet path %q", row.Why, wantPath)
	}
	if !strings.Contains(row.Why, "age-b2work") {
		t.Errorf("row.Why = %q, must name the closed candidate bead", row.Why)
	}
	if row.Since != closedAt {
		t.Errorf("row.Since = %q, want the bead closed_at %q", row.Since, closedAt)
	}
	if row.Age != "3h" {
		t.Errorf("row.Age = %q, want %q", row.Age, "3h")
	}
}
