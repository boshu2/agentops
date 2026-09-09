package session

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestReadSourceMissingContextAndBounds(t *testing.T) {
	for _, args := range [][]string{{"read-source"}, {"read-source", "--file", "/denied", "--start-byte", "0"}, {"read-source", "--file", "/denied", "--start-byte", "0", "--max-bytes", "1", "--json"}, {"read-source", "--file", "/denied", "--start-byte", "-1", "--max-bytes", "1"}, {"read-source", "unexpected"}} {
		root := NewModule(clicontract.HostOptions{}).Command()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Fatalf("invalid input succeeded: %v", args)
		}
		if strings.Contains(out.String(), "bytes_base64") {
			t.Fatal("denied source emitted content")
		}
	}
}
func TestReadSourceRejectsUnmeasuredYAML(t *testing.T) {
	root := NewModule(clicontract.HostOptions{OutputMode: func() string { return "yaml" }}).Command()
	root.SetArgs([]string{"read-source", "--start-byte", "0", "--max-bytes", "1"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "JSON only") {
		t.Fatalf("yaml result: %v", err)
	}
}
