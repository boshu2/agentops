// practices: [microservices, team-topologies]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	factoryAdmitWorkOrder       string
	factoryAdmitRunID           string
	factoryAdmitLocalPilot      bool
	factoryAdmitRPIHandoff      bool
	factoryAdmitExecutionPacket string
	factoryAdmitEpicID          string
)

var factoryAdmitCmd = &cobra.Command{
	Use:   "admit",
	Short: "Submit a factory work order to agentopsd admission",
	Long: `Submit a typed factory work order to agentopsd as factory.admission
or factory.local-pilot. This only asks the daemon to decide admission; source
mutation remains blocked unless the daemon records an allowed decision and the
executor policy supports the requested RPI handoff.`,
	Args: cobra.NoArgs,
	RunE: runFactoryAdmitCommand,
}

func init() {
	factoryCmd.AddCommand(factoryAdmitCmd)
	factoryAdmitCmd.Flags().StringVar(&factoryAdmitWorkOrder, "work-order", "", "Factory work-order JSON ('@path', '@-', or inline object)")
	factoryAdmitCmd.Flags().StringVar(&factoryAdmitRunID, "run-id", "", "Factory run id (default generated from current time)")
	factoryAdmitCmd.Flags().BoolVar(&factoryAdmitLocalPilot, "local-pilot", false, "Submit as factory.local-pilot instead of factory.admission")
	factoryAdmitCmd.Flags().BoolVar(&factoryAdmitRPIHandoff, "rpi-handoff", false, "Request an admitted rpi.run child job")
	factoryAdmitCmd.Flags().StringVar(&factoryAdmitExecutionPacket, "execution-packet", "", "Execution packet path for --rpi-handoff")
	factoryAdmitCmd.Flags().StringVar(&factoryAdmitEpicID, "epic-id", "", "Optional epic id for --rpi-handoff")
	factoryAdmitCmd.Flags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	factoryAdmitCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	factoryAdmitCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")
}

func runFactoryAdmitCommand(cmd *cobra.Command, args []string) error {
	work, err := readFactoryAdmitWorkOrder(factoryAdmitWorkOrder)
	if err != nil {
		return err
	}
	runID := strings.TrimSpace(factoryAdmitRunID)
	if runID == "" {
		runID = "factory-admit-" + time.Now().UTC().Format("20060102-150405")
	}
	payload, jobType, err := buildFactoryAdmitPayload(runID, work)
	if err != nil {
		return err
	}
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	token, err := resolveAgentOpsDaemonClientMutationToken(cwd, daemonToken, daemonTokenFile)
	if err != nil {
		return err
	}
	response, err := submitFactoryAdmissionJob(cobraContext(cmd), baseURL, token, jobType, payload)
	if err != nil {
		return err
	}
	return renderFactoryAdmitResponse(cmd, response)
}

func readFactoryAdmitWorkOrder(ref string) (daemonpkg.FactoryWorkOrder, error) {
	if strings.TrimSpace(ref) == "" {
		return daemonpkg.FactoryWorkOrder{}, errors.New("--work-order is required")
	}
	payload, err := readSubmitPayload(ref)
	if err != nil {
		return daemonpkg.FactoryWorkOrder{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return daemonpkg.FactoryWorkOrder{}, err
	}
	var work daemonpkg.FactoryWorkOrder
	if err := json.Unmarshal(data, &work); err != nil {
		return daemonpkg.FactoryWorkOrder{}, err
	}
	if err := work.Validate(); err != nil {
		return daemonpkg.FactoryWorkOrder{}, fmt.Errorf("work_order: %w", err)
	}
	return work, nil
}

func buildFactoryAdmitPayload(runID string, work daemonpkg.FactoryWorkOrder) (map[string]any, daemonpkg.JobType, error) {
	mode := daemonpkg.FactoryAdmissionModeAdmissionOnly
	handoff := daemonpkg.FactoryHandoff{Kind: daemonpkg.FactoryHandoffNone}
	if factoryAdmitRPIHandoff {
		mode = daemonpkg.FactoryAdmissionModeRPIHandoff
		handoff = daemonpkg.FactoryHandoff{
			Kind:                daemonpkg.FactoryHandoffRPI,
			ExecutionPacketPath: factoryAdmitExecutionPacket,
			EpicID:              factoryAdmitEpicID,
		}
	}
	if factoryAdmitLocalPilot {
		spec := daemonpkg.NewFactoryLocalPilotJobSpec(runID, work)
		spec.Mode = mode
		spec.Handoff = handoff
		jobSpec, err := spec.ToJobSpec("")
		if err != nil {
			return nil, "", err
		}
		return jobSpec.Payload, jobSpec.Type, nil
	}
	spec := daemonpkg.NewFactoryAdmissionJobSpec(runID, work)
	spec.Mode = mode
	spec.Handoff = handoff
	jobSpec, err := spec.ToJobSpec("")
	if err != nil {
		return nil, "", err
	}
	return jobSpec.Payload, jobSpec.Type, nil
}

func submitFactoryAdmissionJob(ctx context.Context, baseURL, token string, jobType daemonpkg.JobType, payload map[string]any) (submitDaemonJobResponse, error) {
	return submitDaemonJob(ctx, baseURL, token, jobType, payload)
}

func renderFactoryAdmitResponse(cmd *cobra.Command, response submitDaemonJobResponse) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", response.JobID, response.JobType, response.Status)
	return nil
}
