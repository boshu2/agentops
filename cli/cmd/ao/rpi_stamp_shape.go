// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

// amRunner runs an `am robot ...` invocation and returns its stdout. It is
// injectable so tests never depend on a live Agent Mail server (go.md: test
// low-level functions directly; don't depend on external CLIs).
type amRunner func(args ...string) ([]byte, error)

// defaultAMRunner shells out to the real `am` CLI with a short timeout so a slow
// or wedged server can never hang the live /discovery path.
func defaultAMRunner(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "am", args...)
	return cmd.Output()
}

// amActiveEnvelope is the slice of `am robot agents --active --json` we read.
type amActiveEnvelope struct {
	Count int `json:"count"`
}

// amReservationsEnvelope is the slice of `am robot reservations --json` we read.
// Each active reservation carries the holding lane (`agent`) and the reserved
// `path`; grouping path-by-agent yields the per-lane write-sets ValidateShape
// checks for overlap. Field names mirror the am ReservationEntry contract
// (crates/mcp-agent-mail-cli/src/robot.rs).
type amReservationsEnvelope struct {
	AllActive []struct {
		Agent string `json:"agent"`
		Path  string `json:"path"`
	} `json:"all_active"`
}

// parseAMActiveCount extracts the live-writer count from `am robot agents
// --active --json`. A parse failure yields 0 (the honest single-agent floor),
// never an error that would block the live path.
func parseAMActiveCount(out []byte) int {
	var env amActiveEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return 0
	}
	if env.Count < 0 {
		return 0
	}
	return env.Count
}

// parseAMReservationWriteSets groups active reservation paths by holding lane,
// producing the per-lane write-sets [][]string ValidateShape's overlap predicate
// consumes. Lanes are returned in a stable (sorted-by-agent) order so the
// decision is deterministic. Entries with an empty agent or path are skipped.
func parseAMReservationWriteSets(out []byte) [][]string {
	var env amReservationsEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil
	}
	byAgent := map[string][]string{}
	for _, r := range env.AllActive {
		agent := strings.TrimSpace(r.Agent)
		path := strings.TrimSpace(r.Path)
		if agent == "" || path == "" {
			continue
		}
		byAgent[agent] = append(byAgent[agent], path)
	}
	if len(byAgent) == 0 {
		return nil
	}
	agents := make([]string, 0, len(byAgent))
	for a := range byAgent {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	sets := make([][]string, 0, len(agents))
	for _, a := range agents {
		sets = append(sets, byAgent[a])
	}
	return sets
}

// gatherShapeInputs builds the observable ValidateShape inputs from the live
// Agent Mail roster + reservations, the unattended/durability signal, and the
// model's proposed shape. am failures degrade silently to the single-agent
// floor — the live path always gets a record, never a hang or an error.
func gatherShapeInputs(project, proposed string, unattended bool, run amRunner) orchestration.ShapeInputs {
	in := orchestration.ShapeInputs{
		Proposed:       proposed,
		UnattendedNeed: unattended,
	}
	if run == nil {
		return in
	}
	if out, err := run("robot", "agents", "--project", project, "--active", "--json"); err == nil {
		in.LiveWriters = parseAMActiveCount(out)
	}
	if out, err := run("robot", "reservations", "--project", project, "--json"); err == nil {
		in.WriteSets = parseAMReservationWriteSets(out)
	}
	return in
}

// stampShapeOnPacket reads the execution packet at path, replaces its
// orchestration_decision block with the validated verdict, and writes it back —
// preserving every other field. The packet is read as a generic object so a
// hand-compiled live packet (which /discovery writes) keeps fields the Go
// executionPacket struct may not enumerate.
func stampShapeOnPacket(path string, verdict orchestration.ShapeVerdict, ts string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read execution packet %s: %w", path, err)
	}
	var packet map[string]any
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse execution packet %s: %w", path, err)
	}
	decision := map[string]any{
		"chosen_shape": verdict.Shape,
		"rationale":    verdict.Rationale,
		"ts":           ts,
	}
	// predicates_fired is always present so the record is honest about whether a
	// predicate fired (empty list = single-agent default, no escalation).
	fired := verdict.PredicatesFired
	if fired == nil {
		fired = []string{}
	}
	decision["predicates_fired"] = fired
	packet["orchestration_decision"] = decision

	out, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal execution packet: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write execution packet %s: %w", path, err)
	}
	return nil
}

// resolveStampRunID determines the run id whose durable per-run archive snapshot
// should also be stamped, in priority order: the RPI_RUN_ID env (the loop sets
// it) then the packet's own run_id field (the live /discovery packet carries it).
// Returns "" when neither resolves — there is then no archive to mirror.
func resolveStampRunID(packetPath string) string {
	if v := strings.TrimSpace(os.Getenv("RPI_RUN_ID")); v != "" {
		return v
	}
	data, err := os.ReadFile(packetPath)
	if err != nil {
		return ""
	}
	var packet struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		return ""
	}
	return strings.TrimSpace(packet.RunID)
}

// runArchivePacketPath returns the per-run archive packet that mirrors the alias
// at packetPath: <root>/.agents/rpi/runs/<run-id>/execution-packet.json, where
// <root> is the repo root holding the alias
// (<root>/.agents/rpi/execution-packet.json). Returns "" when runID is empty.
func runArchivePacketPath(packetPath, runID string) string {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return ""
	}
	// packetPath = <root>/.agents/rpi/<file> → three Dir() hops yield <root>.
	root := filepath.Dir(filepath.Dir(filepath.Dir(packetPath)))
	return filepath.Join(rpiRunRegistryDir(root, runID), executionPacketFile)
}

// stampShapeEverywhere stamps the verdict onto the alias packet and, when a run
// id is resolvable and the durable per-run archive already exists, onto that
// archive too (age-9gc). /discovery STEP 6 writes BOTH the alias and the run
// archive, but only the alias routed through stamp-shape before — leaving the
// durable snapshot without the validated orchestration_decision. The archive is
// mirrored only when it already exists: stamp-shape records decisions on
// existing packets, it does not create run archives. Returns the paths stamped.
func stampShapeEverywhere(packetPath string, verdict orchestration.ShapeVerdict, ts string) ([]string, error) {
	if err := stampShapeOnPacket(packetPath, verdict, ts); err != nil {
		return nil, err
	}
	stamped := []string{packetPath}
	archivePath := runArchivePacketPath(packetPath, resolveStampRunID(packetPath))
	if archivePath == "" || archivePath == packetPath {
		return stamped, nil
	}
	if _, err := os.Stat(archivePath); err != nil {
		return stamped, nil //nolint:nilerr // no archive (or unreadable) → alias-only, not an error
	}
	if err := stampShapeOnPacket(archivePath, verdict, ts); err != nil {
		return stamped, err
	}
	return append(stamped, archivePath), nil
}

// proposedFromPacket reads an already-present orchestration_decision.chosen_shape
// so a shape the model wrote by hand on the live path is treated as the proposal
// ValidateShape validates (and overrides when ground truth disagrees).
func proposedFromPacket(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var packet struct {
		OrchestrationDecision struct {
			ChosenShape string `json:"chosen_shape"`
		} `json:"orchestration_decision"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		return ""
	}
	return strings.TrimSpace(packet.OrchestrationDecision.ChosenShape)
}

func newRPIStampShapeCmd() *cobra.Command {
	var (
		packetFlag   string
		projectFlag  string
		proposedFlag string
		unattended   bool
		noAM         bool
	)
	cmd := &cobra.Command{
		Use:   "stamp-shape",
		Short: "Stamp the orchestration-shape decision onto the execution packet (live /discovery wire)",
		Long: `Compute the orchestration shape from observable ground truth (Agent Mail
live-writer count + per-lane reservation write-sets + the unattended/durability
signal) via orchestration.ValidateShape, and stamp the resulting
orchestration_decision onto .agents/rpi/execution-packet.json.

This is the live wire: /discovery STEP 6 hand-compiles the packet, then invokes
this command so the LIVE path carries a validated orchestration_decision (the
Go seed-writer only runs in the retired rpi_* engine). am failures degrade to
the single-agent floor; the packet always gets a record.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			packetPath := packetFlag
			if strings.TrimSpace(packetPath) == "" {
				packetPath = filepath.Join(cwd, ".agents", "rpi", executionPacketFile)
			}
			project := projectFlag
			if strings.TrimSpace(project) == "" {
				project = cwd
			}
			proposed := proposedFlag
			if strings.TrimSpace(proposed) == "" {
				proposed = proposedFromPacket(packetPath)
			}
			var run amRunner
			if !noAM {
				run = defaultAMRunner
			}
			inputs := gatherShapeInputs(project, proposed, unattended, run)
			verdict := orchestration.ValidateShape(inputs)
			ts := time.Now().UTC().Format(time.RFC3339)
			stamped, err := stampShapeEverywhere(packetPath, verdict, ts)
			if err != nil {
				return err
			}
			fired := "none"
			if len(verdict.PredicatesFired) > 0 {
				fired = strings.Join(verdict.PredicatesFired, ",")
			}
			archiveNote := ""
			if len(stamped) > 1 {
				archiveNote = fmt.Sprintf(" (+%d run-archive snapshot)", len(stamped)-1)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "orchestration_decision stamped: shape=%s predicates_fired=%s%s\n", verdict.Shape, fired, archiveNote)
			return nil
		},
	}
	cmd.Flags().StringVar(&packetFlag, "packet", "", "Path to the execution packet (default .agents/rpi/execution-packet.json)")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Agent Mail project key for live-writer/reservation gathering (default cwd)")
	cmd.Flags().StringVar(&proposedFlag, "proposed", "", "Model-proposed shape to validate/override (default: the packet's existing chosen_shape)")
	cmd.Flags().BoolVar(&unattended, "unattended", false, "Mark the durability axis: the run must outlive the session (→ ATM)")
	cmd.Flags().BoolVar(&noAM, "no-am", false, "Skip Agent Mail gathering (single-agent floor unless --unattended)")
	return cmd
}

func init() {
	addRPISubcommand(newRPIStampShapeCmd())
}
