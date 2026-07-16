package doctor

import (
	"context"
	"testing"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

func TestReadRuntimeMapsApplicationRequest(t *testing.T) {
	options, err := (ReadRuntime{ToolVersion: "3.0.0"}).Options(context.Background(), doctorapp.ReadRequest{
		Only: []string{"one"}, Skip: []string{"two"}, Quick: true, Online: true,
		Severity: "P2", JSON: true, Since: "prior",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.ToolVersion != "3.0.0" || options.Since != "prior" || !options.Quick || !options.Online || !options.JSON {
		t.Fatalf("options = %+v", options)
	}
	if len(options.Only) != 1 || options.Only[0] != "one" || len(options.Skip) != 1 || options.Skip[0] != "two" {
		t.Fatalf("options = %+v", options)
	}
}
