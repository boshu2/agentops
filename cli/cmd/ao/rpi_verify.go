package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var errRPILedgerVerificationFailed = errors.New("RPI ledger verification failed")

func init() {
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify RPI ledger integrity",
		Long: `Verify integrity of the RPI ledger.

Checks the ledger chain for corruption and reports a concise PASS/FAIL summary.

Examples:
  ao rpi verify
  ao rpi verify --latest
  ao rpi verify --json`,
		RunE: runRPIVerify,
	}
	verifyCmd.Flags().Bool("latest", false, "Verify the latest RPI ledger state (compatibility alias; current workspace ledger is latest)")
	rpiCmd.AddCommand(verifyCmd)
}

type rpiVerifyOutput struct {
	Status string `json:"status"`
	rpiLedgerVerifyResult
}

func runRPIVerify(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := verifyRPILedger(cwd)
	if err != nil {
		return fmt.Errorf("verify RPI ledger: %w", err)
	}

	status := "FAIL"
	if result.Pass {
		status = "PASS"
	}

	if GetOutput() == "json" {
		payload := rpiVerifyOutput{Status: status, rpiLedgerVerifyResult: result}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fmt.Errorf("encode JSON output: %w", err)
		}
	} else {
		if result.Pass {
			fmt.Printf("PASS records=%d\n", result.RecordCount)
		} else {
			msg := cmp.Or(result.Message, "unknown")
			fmt.Printf("FAIL records=%d first_broken_index=%d message=%s\n", result.RecordCount, result.FirstBrokenIndex, msg)
		}
	}

	if !result.Pass {
		return errRPILedgerVerificationFailed
	}
	return nil
}
