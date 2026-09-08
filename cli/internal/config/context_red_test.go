package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Context routing must be a real config consumer, independent of legacy learning paths.
func TestContextConfigurationIsLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("context:\n  bundle_root: /caller/private/bundle\n  evidence_root: /caller/private/evidence\n  maintenance_work_ref: fixture-anchor\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_CONFIG", path)
	cfg, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	var route map[string]string
	if err = json.Unmarshal(got["context"], &route); err != nil {
		t.Fatalf("caller context routing is not loaded: %v", err)
	}
	if route["bundle_root"] != "/caller/private/bundle" || route["maintenance_work_ref"] != "fixture-anchor" {
		t.Fatalf("context routing lost: %v", route)
	}
}
