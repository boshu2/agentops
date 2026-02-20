package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	validateSubCmd := &cobra.Command{
		Use:     "validate <step>",
		GroupID: "inspection",
		Short:   "Validate step requirements",
		Long: `Validate that an artifact meets quality requirements.

Checks for required sections, formatting, and tier criteria.

Legacy artifacts without schema_version can use --lenient mode (expires in 90 days by default).
Default mode is STRICT (requires explicit --lenient flag).

Examples:
  ao ratchet validate research --changes .agents/research/topic.md
  ao ratchet validate plan --changes epic:ol-0001
  ao ratchet validate research --changes old.md --lenient
  ao ratchet validate research --changes old.md --lenient --lenient-expiry 180`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetValidate,
	}
	validateSubCmd.Flags().StringSliceVar(&ratchetFiles, "changes", nil, "Files to validate")
	validateSubCmd.Flags().BoolVar(&ratchetLenient, "lenient", false, "Allow legacy artifacts without schema_version (expires in 90 days)")
	validateSubCmd.Flags().IntVar(&ratchetLenientDays, "lenient-expiry", 90, "Days until lenient bypass expires")
	ratchetCmd.AddCommand(validateSubCmd)
}

// runRatchetValidate validates step requirements.
func runRatchetValidate(cmd *cobra.Command, args []string) error {
	stepName := args[0]
	step := ratchet.ParseStep(stepName)
	if step == "" {
		return fmt.Errorf("unknown step: %s", stepName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	validator, err := ratchet.NewValidator(cwd)
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	files := resolveValidationFiles(cwd, step)
	if len(files) == 0 {
		return fmt.Errorf("no files to validate (use --changes or ensure output exists)")
	}

	opts := buildValidateOptions()
	w := cmd.OutOrStdout()

	allValid := true
	for _, file := range files {
		result, err := validator.ValidateWithOptions(step, file, opts)
		if err != nil {
			return fmt.Errorf("validate %s: %w", file, err)
		}

		if GetOutput() == "json" {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result) //nolint:errcheck // CLI JSON output to stdout
		} else {
			formatValidationResult(w, file, result, &allValid)
		}
	}

	if !allValid {
		return fmt.Errorf("validation failed: one or more artifacts are invalid")
	}

	return nil
}

// resolveValidationFiles determines which files to validate.
// Uses explicit --changes files if provided, otherwise locates expected output.
func resolveValidationFiles(cwd string, step ratchet.Step) []string {
	if len(ratchetFiles) > 0 {
		return ratchetFiles
	}

	locator, _ := ratchet.NewLocator(cwd)
	pattern := ratchet.GetExpectedOutput(step)
	if strings.HasPrefix(pattern, "epic:") || strings.HasPrefix(pattern, "issue:") {
		return nil
	}

	if path, _, err := locator.FindFirst(pattern); err == nil {
		return []string{path}
	}
	return nil
}

// buildValidateOptions creates validation options from command flags.
func buildValidateOptions() *ratchet.ValidateOptions {
	opts := &ratchet.ValidateOptions{
		Lenient: ratchetLenient,
	}
	if ratchetLenient && ratchetLenientDays > 0 {
		expiryTime := time.Now().AddDate(0, 0, ratchetLenientDays)
		opts.LenientExpiryDate = &expiryTime
	}
	return opts
}

// formatValidationResult prints a single validation result in text format.
func formatValidationResult(w io.Writer, file string, result *ratchet.ValidationResult, allValid *bool) {
	fmt.Fprintf(w, "Validation: %s\n", file)
	if result.Valid {
		fmt.Fprintln(w, "  Status: VALID ✓")
	} else {
		fmt.Fprintln(w, "  Status: INVALID ✗")
		*allValid = false
	}

	if result.Lenient {
		fmt.Fprintln(w, "  Mode: LENIENT (legacy bypass)")
		if result.LenientExpiryDate != nil {
			fmt.Fprintf(w, "  Expires: %s\n", *result.LenientExpiryDate)
		}
		if result.LenientExpiringSoon {
			fmt.Fprintln(w, "  ⚠️  Expiring soon - migration required")
		}
	}

	if len(result.Issues) > 0 {
		fmt.Fprintln(w, "  Issues:")
		for _, issue := range result.Issues {
			fmt.Fprintf(w, "    - %s\n", issue)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "  Warnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "    - %s\n", warn)
		}
	}

	if result.Tier != nil {
		fmt.Fprintf(w, "  Tier: %d (%s)\n", *result.Tier, result.Tier.String())
	}
}
