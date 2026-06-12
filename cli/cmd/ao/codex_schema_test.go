package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/embedded"
)

// TestEmbeddedCodexSchemasMatchRepoSchemas is the drift gate for the embedded
// runtime copies: the canonical schemas live at the repo root schemas/ and the
// embedded copies must stay byte-identical.
func TestEmbeddedCodexSchemasMatchRepoSchemas(t *testing.T) {
	for _, name := range []string{"codex-task-packet.schema.json", "codex-run-receipt.schema.json"} {
		t.Run(name, func(t *testing.T) {
			embeddedData, err := embedded.SchemasFS.ReadFile("schemas/" + name)
			if err != nil {
				t.Fatalf("read embedded schema: %v", err)
			}
			repoData, err := os.ReadFile(findRepoFileForTest(t, "schemas", name))
			if err != nil {
				t.Fatalf("read repo schema: %v", err)
			}
			if string(embeddedData) != string(repoData) {
				t.Fatalf("embedded cli/embedded/schemas/%s drifted from repo schemas/%s; re-copy the canonical schema", name, name)
			}
		})
	}
}

func loadCodexExamplePacketMap(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(findRepoFileForTest(t, "docs", "contracts", "examples", "codex", "task-packet.json"))
	if err != nil {
		t.Fatalf("read example task packet: %v", err)
	}
	var packet map[string]any
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal example task packet: %v", err)
	}
	return packet
}

func marshalForTest(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestValidateCodexTaskPacketJSONAcceptsExamplePacket(t *testing.T) {
	packet := loadCodexExamplePacketMap(t)
	if err := validateCodexTaskPacketJSON(marshalForTest(t, packet)); err != nil {
		t.Fatalf("example task packet failed schema validation: %v", err)
	}
}

// TestValidateCodexTaskPacketJSONRejectsSchemaViolations covers packets that
// the old hand-written partial validation accepted: weakened auth constants,
// unknown properties, bad enums, and missing required fields.
func TestValidateCodexTaskPacketJSONRejectsSchemaViolations(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(packet map[string]any)
		wantErr string
	}{
		{
			name: "forbid_api_key false rejected by const",
			mutate: func(packet map[string]any) {
				packet["auth"].(map[string]any)["forbid_api_key"] = false
			},
			wantErr: "forbid_api_key",
		},
		{
			name: "required_mode api-key rejected by enum",
			mutate: func(packet map[string]any) {
				packet["auth"].(map[string]any)["required_mode"] = "api-key"
			},
			wantErr: "required_mode",
		},
		{
			name: "empty reject_env rejected by minItems",
			mutate: func(packet map[string]any) {
				packet["auth"].(map[string]any)["reject_env"] = []any{}
			},
			wantErr: "reject_env",
		},
		{
			name: "weakened login_status_must_contain rejected by const",
			mutate: func(packet map[string]any) {
				packet["auth"].(map[string]any)["login_status_must_contain"] = "Logged in"
			},
			wantErr: "login_status_must_contain",
		},
		{
			name: "unknown top-level property rejected",
			mutate: func(packet map[string]any) {
				packet["escape_hatch"] = true
			},
			wantErr: "additional",
		},
		{
			name: "unknown auth property rejected",
			mutate: func(packet map[string]any) {
				packet["auth"].(map[string]any)["allow_api_key_fallback"] = true
			},
			wantErr: "additional",
		},
		{
			name: "missing created_at rejected",
			mutate: func(packet map[string]any) {
				delete(packet, "created_at")
			},
			wantErr: "created_at",
		},
		{
			name: "invalid capture_mode rejected by enum",
			mutate: func(packet map[string]any) {
				packet["output"].(map[string]any)["capture_mode"] = "stream-everything"
			},
			wantErr: "capture_mode",
		},
		{
			name: "invalid sandbox rejected by enum",
			mutate: func(packet map[string]any) {
				packet["sandbox"] = "no-sandbox"
			},
			wantErr: "sandbox",
		},
		{
			name: "invalid stdin mode rejected by enum",
			mutate: func(packet map[string]any) {
				packet["execution"].(map[string]any)["stdin"].(map[string]any)["mode"] = "open-forever"
			},
			wantErr: "mode",
		},
		{
			name: "invalid resume policy rejected by enum",
			mutate: func(packet map[string]any) {
				packet["resume"] = map[string]any{"policy": "whatever-was-last", "allow_resume": false}
			},
			wantErr: "policy",
		},
		{
			name: "non-string execution environment value rejected",
			mutate: func(packet map[string]any) {
				packet["execution"].(map[string]any)["environment"] = map[string]any{"DEBUG": 1}
			},
			wantErr: "environment",
		},
		{
			name: "schema_version 2 rejected by const",
			mutate: func(packet map[string]any) {
				packet["schema_version"] = 2
			},
			wantErr: "schema_version",
		},
		{
			name: "dispatch mutates_repo true rejected by const",
			mutate: func(packet map[string]any) {
				packet["dispatch"].(map[string]any)["mutates_repo"] = true
			},
			wantErr: "mutates_repo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := loadCodexExamplePacketMap(t)
			tt.mutate(packet)
			err := validateCodexTaskPacketJSON(marshalForTest(t, packet))
			if err == nil {
				t.Fatalf("validateCodexTaskPacketJSON accepted a packet that violates the schema")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("schema error = %q, want to mention %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadCodexTaskPacketRejectsDispatchCommandArgvMismatch(t *testing.T) {
	packet := loadCodexExamplePacketMap(t)
	packet["dispatch"].(map[string]any)["command"] = []any{"codex", "exec", "--sandbox", "workspace-write"}
	path := filepath.Join(t.TempDir(), "packet.json")
	if err := os.WriteFile(path, marshalForTest(t, packet), 0o600); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	_, err := loadCodexTaskPacket(path)
	if err == nil {
		t.Fatalf("loadCodexTaskPacket accepted dispatch.command != execution.argv")
	}
	if !strings.Contains(err.Error(), "dispatch.command") || !strings.Contains(err.Error(), "execution.argv") {
		t.Fatalf("error = %q, want dispatch.command/execution.argv mismatch", err.Error())
	}
}

func TestCodexDispatchRejectsUnknownPacketPropertyBeforeExecution(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	marker := filepath.Join(repo, "worker-ran")
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_MARKER", marker)
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{})
	raw, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	var packet map[string]any
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatalf("unmarshal packet: %v", err)
	}
	packet["unvalidated_extension"] = map[string]any{"anything": "goes"}
	if err := os.WriteFile(packetPath, marshalForTest(t, packet), 0o600); err != nil {
		t.Fatalf("rewrite packet: %v", err)
	}

	_, err = executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err == nil {
		t.Fatalf("codex dispatch succeeded, want schema violation refusal")
	}
	if !strings.Contains(err.Error(), "violates") {
		t.Fatalf("dispatch error = %q, want schema violation", err.Error())
	}
	assertPathAbsent(t, marker)
	assertPathAbsent(t, receiptPath)
}

func TestValidateCodexRunReceiptSchema(t *testing.T) {
	t.Run("dispatch-shaped receipt passes", func(t *testing.T) {
		if err := validateCodexRunReceiptSchema(validCodexReceiptForTest()); err != nil {
			t.Fatalf("valid receipt failed schema validation: %v", err)
		}
	})
	t.Run("example receipt fixture passes", func(t *testing.T) {
		data, err := os.ReadFile(findRepoFileForTest(t, "docs", "contracts", "examples", "codex", "run-receipt.json"))
		if err != nil {
			t.Fatalf("read example receipt: %v", err)
		}
		if err := validateCodexRunReceiptJSON(data); err != nil {
			t.Fatalf("example receipt failed schema validation: %v", err)
		}
	})
	t.Run("invalid verdict status rejected", func(t *testing.T) {
		receipt := validCodexReceiptForTest()
		receipt.Verdict.Status = "MAYBE"
		err := validateCodexRunReceiptSchema(receipt)
		if err == nil || !strings.Contains(err.Error(), "status") {
			t.Fatalf("schema error = %v, want verdict status enum violation", err)
		}
	})
	t.Run("nil changed_files rejected as null", func(t *testing.T) {
		receipt := validCodexReceiptForTest()
		receipt.ChangedFiles = nil
		err := validateCodexRunReceiptSchema(receipt)
		if err == nil || !strings.Contains(err.Error(), "changed_files") {
			t.Fatalf("schema error = %v, want changed_files type violation", err)
		}
	})
	t.Run("invalid auth_mode rejected", func(t *testing.T) {
		receipt := validCodexReceiptForTest()
		receipt.AuthMode = "password"
		err := validateCodexRunReceiptSchema(receipt)
		if err == nil || !strings.Contains(err.Error(), "auth_mode") {
			t.Fatalf("schema error = %v, want auth_mode enum violation", err)
		}
	})
}
