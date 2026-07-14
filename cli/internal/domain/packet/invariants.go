package packet

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const executionPacketSchemaPath = "schemas/execution-packet.schema.json"

//go:embed schemas/execution-packet.schema.json
var executionPacketSchemaBytes []byte

var (
	ErrSchemaViolation = errors.New("packet: execution packet violates rich schema")
	ErrSchemaLoad      = errors.New("packet: load execution packet schema")

	executionPacketSchemaOnce sync.Once
	executionPacketSchema     *jsonschema.Schema
	executionPacketSchemaErr  error
)

// Validate returns the first rich-schema violation, or nil.
func (p ExecutionPacket) Validate() error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal execution packet for schema validation: %w", err)
	}
	return ValidateJSON(data)
}

// ValidateJSON enforces schemas/execution-packet.schema.json against raw
// execution-packet bytes. Raw validation is used by storage loads so unknown
// persisted fields cannot be hidden by struct unmarshalling.
func ValidateJSON(data []byte) error {
	schema, err := schemaForExecutionPacket()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse execution packet: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaViolation, err)
	}
	return nil
}

// DecodeRequirements carries caller-owned presence requirements independently
// of the packet schema-version field-ownership policy.
type DecodeRequirements struct {
	PremortemVerdict bool
}

// DecodeJSON validates and decodes a packet with optional mortem evidence.
func DecodeJSON(data []byte) (ExecutionPacket, error) {
	return DecodeJSONWithRequirements(data, DecodeRequirements{})
}

// DecodeJSONWithRequirements validates, decodes, and then enforces required
// evidence declared by the caller.
func DecodeJSONWithRequirements(data []byte, requirements DecodeRequirements) (ExecutionPacket, error) {
	p, err := decodeJSON(data)
	if err != nil {
		return ExecutionPacket{}, err
	}
	if requirements.PremortemVerdict && p.PremortemVerdict == "" {
		return ExecutionPacket{}, fmt.Errorf("required execution packet field is absent: premortem_verdict")
	}
	return p, nil
}

// decodeJSON also accepts the pre-rich slim persisted shape and migrates it in
// memory so older archives remain readable.
func decodeJSON(data []byte) (ExecutionPacket, error) {
	if err := ValidateJSON(data); err == nil {
		var p ExecutionPacket
		if err := json.Unmarshal(data, &p); err != nil {
			return ExecutionPacket{}, err
		}
		return p, nil
	} else {
		if migrated, ok, migrateErr := migrateLegacySlimJSON(data); ok || migrateErr != nil {
			if migrateErr != nil {
				return ExecutionPacket{}, migrateErr
			}
			// Fail closed: the migrated packet must itself satisfy the rich
			// schema. The legacy path must not become a hole that admits packets
			// ValidateJSON would reject — e.g. a partial slim object (only
			// test_levels) that migrates to a packet missing objective.
			remarshaled, mErr := json.Marshal(migrated)
			if mErr != nil {
				return ExecutionPacket{}, mErr
			}
			if vErr := ValidateJSON(remarshaled); vErr != nil {
				return ExecutionPacket{}, fmt.Errorf("%w: migrated legacy packet: %v", ErrSchemaViolation, vErr)
			}
			return migrated, nil
		}
		return ExecutionPacket{}, err
	}
}

func schemaForExecutionPacket() (*jsonschema.Schema, error) {
	executionPacketSchemaOnce.Do(func() {
		executionPacketSchema, executionPacketSchemaErr = compileExecutionPacketSchema()
	})
	return executionPacketSchema, executionPacketSchemaErr
}

func compileExecutionPacketSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(executionPacketSchemaBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrSchemaLoad, executionPacketSchemaPath, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(executionPacketSchemaPath, doc); err != nil {
		return nil, fmt.Errorf("%w: add resource %s: %v", ErrSchemaLoad, executionPacketSchemaPath, err)
	}
	schema, err := compiler.Compile(executionPacketSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: compile %s: %v", ErrSchemaLoad, executionPacketSchemaPath, err)
	}
	return schema, nil
}

type legacySlimPacket struct {
	PlanPath         string                     `json:"plan_path"`
	EpicID           string                     `json:"epic_id,omitempty"`
	Complexity       Complexity                 `json:"complexity"`
	TestLevels       []TestLevel                `json:"test_levels"`
	RankedPacketPath string                     `json:"ranked_packet_path,omitempty"`
	Provenance       legacySlimPacketProvenance `json:"provenance"`
}

type legacySlimPacketProvenance struct {
	CreatedAt string `json:"created_at"`
	Source    string `json:"source"`
	RunID     string `json:"run_id,omitempty"`
}

func migrateLegacySlimJSON(data []byte) (ExecutionPacket, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ExecutionPacket{}, false, err
	}
	if _, hasSchemaVersion := raw["schema_version"]; hasSchemaVersion {
		return ExecutionPacket{}, false, nil
	}
	if !looksLikeLegacySlimPacket(raw) {
		return ExecutionPacket{}, false, nil
	}
	var legacy legacySlimPacket
	if err := json.Unmarshal(data, &legacy); err != nil {
		return ExecutionPacket{}, true, fmt.Errorf("parse legacy slim execution packet: %w", err)
	}
	p := legacy.toExecutionPacket()
	if err := p.Validate(); err != nil {
		return ExecutionPacket{}, true, fmt.Errorf("migrate legacy slim execution packet: %w", err)
	}
	return p, true, nil
}

func looksLikeLegacySlimPacket(raw map[string]json.RawMessage) bool {
	allowed := map[string]struct{}{
		"plan_path":          {},
		"epic_id":            {},
		"complexity":         {},
		"test_levels":        {},
		"ranked_packet_path": {},
		"provenance":         {},
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			return false
		}
	}
	if _, ok := raw["provenance"]; ok {
		return true
	}
	if levels, ok := raw["test_levels"]; ok && len(bytes.TrimSpace(levels)) > 0 && bytes.TrimSpace(levels)[0] == '[' {
		return true
	}
	return false
}

func (p legacySlimPacket) toExecutionPacket() ExecutionPacket {
	objective := p.EpicID
	if objective == "" {
		objective = p.PlanPath
	}
	out := ExecutionPacket{
		SchemaVersion:    1,
		Objective:        objective,
		RunID:            p.Provenance.RunID,
		EpicID:           p.EpicID,
		PlanPath:         p.PlanPath,
		ContractSurfaces: []string{},
		TrackerMode:      "legacy",
		Complexity:       p.Complexity,
		TestLevels: &ExecutionPacketTestLevels{
			Required:    append([]TestLevel(nil), p.TestLevels...),
			Recommended: []TestLevel{},
			Rationale:   "migrated from legacy slim execution packet",
		},
		RankedPacketPath: p.RankedPacketPath,
		Source:           p.Provenance.Source,
	}
	if isRFC3339(p.Provenance.CreatedAt) {
		out.GeneratedAt = p.Provenance.CreatedAt
	}
	return out
}

func isRFC3339(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}
