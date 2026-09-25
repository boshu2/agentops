package fixture

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestActualSourceBoundary(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r := filepath.Join(base, "fixture")
	c := exec.Command("python3", "setup.py", r)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("setup %v %s", e, out)
	}
	frozen := map[string][]byte{}
	for _, n := range []string{"source.jsonl", "task.json", "access.json", "config.json", "comments.json"} {
		frozen[n], _ = os.ReadFile(filepath.Join(r, n))
	}
	c = exec.Command("bash", "workflow.sh", r)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("workflow %v %s", e, out)
	}
	read := func(n string) []byte {
		b, e := os.ReadFile(filepath.Join(r, n))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	for n, b := range frozen {
		if !bytes.Equal(b, read(n)) {
			t.Fatalf("changed input %s", n)
		}
	}
	raw := read("out/allowed.json")
	var got struct {
		Bytes       string `json:"bytes_base64"`
		Start       int    `json:"start_byte"`
		End         int    `json:"end_byte"`
		Serialized  int    `json:"serialized_bytes"`
		Complete    bool   `json:"complete_reading"`
		Delivery    string `json:"host_delivery"`
		Enforcement string `json:"access_enforcement"`
	}
	if e := json.Unmarshal(raw, &got); e != nil {
		t.Fatal(e)
	}
	decoded, e := base64.StdEncoding.DecodeString(got.Bytes)
	if e != nil || !bytes.Equal(decoded, frozen["source.jsonl"][:16]) || got.Start != 0 || got.End != 16 || got.Serialized != len(raw) || len(raw) > 4096 || got.Complete || got.Delivery != "host-delivery-unverified" || got.Enforcement != "not_attested" {
		t.Fatalf("wrong read facts %s", raw)
	}
	rc, e := strconv.Atoi(strings.TrimSpace(string(read("out/denied.rc"))))
	if e != nil || rc == 0 || len(read("out/denied.json")) != 0 {
		t.Fatal("denial emitted bytes or succeeded")
	}
	if bytes.Contains(read("out/denied.err"), []byte("SYNTHETIC_CASS_CANARY")) {
		t.Fatal("denial leaked source")
	}
	if strings.SplitN(strings.TrimSpace(string(read("out/missing-resource.txt"))), "\n", 2)[0] != "STOP_REQUIRED_RESOURCE" {
		t.Fatal("missing reference not stopped")
	}
	calls := strings.Split(strings.TrimSpace(string(read("ao-calls.log"))), "\n")
	if len(calls) != 2 {
		t.Fatalf("expected authorized and denied calls only: %v", calls)
	}
	var summary map[string]any
	if e = json.Unmarshal(read("out/summary.json"), &summary); e != nil {
		t.Fatal(e)
	}
	if summary["allowed_complete_reading"] != false || summary["allowed_host_delivery"] != "host-delivery-unverified" {
		t.Fatal("false completion or denial summary")
	}
	if summary["denied_destination"] != "DENIED" || summary["missing_resource"] != "STOP_REQUIRED_RESOURCE" {
		t.Fatal("summary does not match declared machine-readable disposition contract")
	}
}
