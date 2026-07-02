//go:build legacy

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/boshu2/agentops/cli/embedded"
)

// Embedded copies of the canonical repo-root schemas; the parity test in
// codex_schema_test.go asserts they stay byte-identical with schemas/.
const (
	codexTaskPacketSchemaPath = "schemas/codex-task-packet.schema.json"
	codexRunReceiptSchemaPath = "schemas/codex-run-receipt.schema.json"
)

var (
	codexSchemaOnce    sync.Once
	codexPacketSchema  *jsonschema.Schema
	codexReceiptSchema *jsonschema.Schema
	codexSchemaErr     error
)

func compileCodexSchemas() {
	codexPacketSchema, codexSchemaErr = compileEmbeddedCodexSchema(codexTaskPacketSchemaPath)
	if codexSchemaErr != nil {
		return
	}
	codexReceiptSchema, codexSchemaErr = compileEmbeddedCodexSchema(codexRunReceiptSchemaPath)
}

func compileEmbeddedCodexSchema(path string) (*jsonschema.Schema, error) {
	data, err := embedded.SchemasFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read embedded schema %s: %w", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema %s: %w", path, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(path, doc); err != nil {
		return nil, fmt.Errorf("add schema resource %s: %w", path, err)
	}
	schema, err := compiler.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", path, err)
	}
	return schema, nil
}

// validateCodexTaskPacketJSON enforces the full codex-task-packet JSON Schema
// (additionalProperties, auth constants, enums, required fields) against the
// raw packet bytes before any unmarshal-based partial checks run.
func validateCodexTaskPacketJSON(data []byte) error {
	schema, err := codexSchemaFor(codexTaskPacketSchemaPath)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse codex task packet: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		return fmt.Errorf("codex task packet violates %s: %w", codexTaskPacketSchemaPath, err)
	}
	return nil
}

// validateCodexRunReceiptJSON enforces the full codex-run-receipt JSON Schema
// against raw receipt bytes.
func validateCodexRunReceiptJSON(data []byte) error {
	schema, err := codexSchemaFor(codexRunReceiptSchemaPath)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse codex run receipt: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		return fmt.Errorf("codex run receipt violates %s: %w", codexRunReceiptSchemaPath, err)
	}
	return nil
}

// validateCodexRunReceiptSchema marshals a receipt the same way the dispatch
// writer does and validates the result against the receipt schema, so a
// dispatch cannot persist a receipt that violates its own contract without
// recording the violation.
func validateCodexRunReceiptSchema(receipt codexRunReceipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal codex run receipt for schema validation: %w", err)
	}
	return validateCodexRunReceiptJSON(data)
}

func codexSchemaFor(path string) (*jsonschema.Schema, error) {
	codexSchemaOnce.Do(compileCodexSchemas)
	if codexSchemaErr != nil {
		return nil, codexSchemaErr
	}
	switch path {
	case codexTaskPacketSchemaPath:
		return codexPacketSchema, nil
	case codexRunReceiptSchemaPath:
		return codexReceiptSchema, nil
	default:
		return nil, fmt.Errorf("unknown embedded codex schema %s", path)
	}
}
