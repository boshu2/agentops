package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var registryTypeFilter string

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Query the unified registry",
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registry entries",
	Args:  cobra.NoArgs,
	RunE:  runRegistryListCommand,
}

func init() {
	rootCmd.AddCommand(registryCmd)
	registryCmd.AddCommand(registryListCmd)
	registryListCmd.Flags().StringVar(&registryTypeFilter, "type", "", "Filter by surface type (skills, hooks, stores, jobs, evals, cli, cadence)")
}

var validRegistryTypes = []string{"skills", "hooks", "stores", "jobs", "evals", "cli", "cadence"}

type registryFile struct {
	SchemaVersion int                    `json:"schema_version"`
	GeneratedAt   string                 `json:"generated_at"`
	Summary       map[string]interface{} `json:"summary"`
	Surfaces      registrySurfaces       `json:"surfaces"`
	Cadence       []registryCadence      `json:"cadence_recommendations"`
}

type registrySurfaces struct {
	Skills   []registrySkill   `json:"skills"`
	Hooks    []registryHook    `json:"hooks"`
	Stores   []registryStore   `json:"knowledge_stores"`
	JobTypes []registryJobType `json:"job_types"`
	Evals    []registryEval    `json:"evals"`
	CLI      []registryCLI     `json:"cli_commands"`
}

type registrySkill struct {
	Name string `json:"name"`
	Tier string `json:"tier"`
	Path string `json:"path"`
}

type registryHook struct {
	Name      string `json:"name"`
	Lifecycle string `json:"lifecycle"`
	Path      string `json:"path"`
}

type registryStore struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type registryJobType struct {
	JobType string `json:"job_type"`
	Domain  string `json:"domain"`
	Action  string `json:"action"`
}

type registryEval struct {
	Suite     string `json:"suite"`
	EvalCount int    `json:"eval_count"`
	Path      string `json:"path"`
}

type registryCLI struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type registryCadence struct {
	Name        string `json:"name"`
	Cadence     string `json:"cadence"`
	Cron        string `json:"cron"`
	JobType     string `json:"job_type"`
	Description string `json:"description"`
}

func runRegistryListCommand(cmd *cobra.Command, _ []string) error {
	if registryTypeFilter != "" && !isValidRegistryType(registryTypeFilter) {
		return fmt.Errorf("invalid type %q; valid types: %s", registryTypeFilter, strings.Join(validRegistryTypes, ", "))
	}

	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	regPath := filepath.Join(cwd, "registry.json")

	data, err := os.ReadFile(regPath)
	if err != nil {
		return fmt.Errorf("registry.json not found — run: bash scripts/generate-registry.sh")
	}

	var reg registryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("parse registry.json: %w", err)
	}

	if jsonFlag {
		return printRegistryJSON(cmd, &reg)
	}
	return printRegistryTable(cmd, &reg)
}

func isValidRegistryType(t string) bool {
	for _, v := range validRegistryTypes {
		if v == t {
			return true
		}
	}
	return false
}

func printRegistryJSON(cmd *cobra.Command, reg *registryFile) error {
	var output interface{}
	switch registryTypeFilter {
	case "skills":
		output = reg.Surfaces.Skills
	case "hooks":
		output = reg.Surfaces.Hooks
	case "stores":
		output = reg.Surfaces.Stores
	case "jobs":
		output = reg.Surfaces.JobTypes
	case "evals":
		output = reg.Surfaces.Evals
	case "cli":
		output = reg.Surfaces.CLI
	case "cadence":
		output = reg.Cadence
	default:
		output = reg
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func printRegistryTable(cmd *cobra.Command, reg *registryFile) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	show := func(t string) bool {
		return registryTypeFilter == "" || registryTypeFilter == t
	}

	if show("skills") {
		fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		for _, s := range reg.Surfaces.Skills {
			fmt.Fprintf(w, "skill\t%s\t%s\n", s.Name, s.Tier)
		}
	}
	if show("hooks") && len(reg.Surfaces.Hooks) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, h := range reg.Surfaces.Hooks {
			fmt.Fprintf(w, "hook\t%s\t%s\n", h.Name, h.Lifecycle)
		}
	}
	if show("stores") && len(reg.Surfaces.Stores) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, s := range reg.Surfaces.Stores {
			fmt.Fprintf(w, "store\t%s\t%s\n", s.Name, s.Purpose)
		}
	}
	if show("jobs") && len(reg.Surfaces.JobTypes) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, j := range reg.Surfaces.JobTypes {
			fmt.Fprintf(w, "job\t%s\t%s.%s\n", j.JobType, j.Domain, j.Action)
		}
	}
	if show("evals") && len(reg.Surfaces.Evals) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, e := range reg.Surfaces.Evals {
			fmt.Fprintf(w, "eval\t%s\t%d files\n", e.Suite, e.EvalCount)
		}
	}
	if show("cli") && len(reg.Surfaces.CLI) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, c := range reg.Surfaces.CLI {
			fmt.Fprintf(w, "cli\t%s\t%s\n", c.Name, c.Path)
		}
	}
	if show("cadence") && len(reg.Cadence) > 0 {
		if registryTypeFilter == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "TYPE\tNAME\tDETAIL\n")
		}
		for _, c := range reg.Cadence {
			fmt.Fprintf(w, "cadence\t%s\t%s\n", c.Name, c.Cadence)
		}
	}

	return w.Flush()
}
