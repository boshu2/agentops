package main

import "testing"

// TestVerifyReceiptsCmd_Registered locks the command surface (age-rk3r.12):
// `receipts` is a subcommand of `verify` with NoArgs, so `ao verify receipts`
// routes to the generator front door. The parent `verify` uses DisableFlagParsing,
// which must NOT swallow the subcommand — cobra resolves subcommands before flag
// parsing, so `ao verify receipts` reaches this command while `ao verify <change>`
// still forwards to the review engine.
func TestVerifyReceiptsCmd_Registered(t *testing.T) {
	var found bool
	for _, c := range verifyCmd.Commands() {
		if c.Use == "receipts" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("`receipts` is not registered under `ao verify`")
	}

	if verifyReceiptsCmd.RunE == nil {
		t.Fatal("verify receipts has no RunE")
	}

	// NoArgs: zero args accepted, any positional rejected.
	if verifyReceiptsCmd.Args == nil {
		t.Fatal("verify receipts must set Args (cobra.NoArgs)")
	}
	if err := verifyReceiptsCmd.Args(verifyReceiptsCmd, []string{}); err != nil {
		t.Fatalf("verify receipts should accept zero args, got %v", err)
	}
	if err := verifyReceiptsCmd.Args(verifyReceiptsCmd, []string{"extra"}); err == nil {
		t.Fatal("verify receipts should reject positional args (cobra.NoArgs)")
	}
}
