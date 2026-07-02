// practices: [design-by-contract]
package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

func TestProvenanceReaderVersion_PrintsBareInteger(t *testing.T) {
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)
	if err := provenanceReaderVersionCmd.RunE(c, nil); err != nil {
		t.Fatalf("ledger-reader-version: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	// Must be a bare integer the shell hook can compare with `-lt`.
	n, err := strconv.Atoi(got)
	if err != nil {
		t.Fatalf("output %q is not a bare integer: %v", got, err)
	}
	if n != provenancegraph.LedgerReaderVersion {
		t.Fatalf("printed %d, want LedgerReaderVersion=%d", n, provenancegraph.LedgerReaderVersion)
	}
	if provenancegraph.LedgerReaderVersion < 1 {
		t.Fatalf("LedgerReaderVersion must be >= 1 (the age-rk3r.3 v1.1 floor)")
	}
}
