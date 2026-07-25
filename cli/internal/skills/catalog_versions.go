package skills

import (
	"fmt"
	"time"
)

type catalogV1Wire struct {
	SchemaVersion *string               `json:"schema_version"`
	GeneratedAt   *string               `json:"generated_at"`
	SkillCount    *int                  `json:"skill_count"`
	Skills        *[]catalogV1EntryWire `json:"skills"`
}

type catalogV1EntryWire struct {
	Name                 *string           `json:"name"`
	Description          *string           `json:"description"`
	HexagonalRole        *string           `json:"hexagonal_role"`
	Consumes             *[]string         `json:"consumes"`
	Produces             *[]string         `json:"produces"`
	ContextRel           *[]contextRelWire `json:"context_rel"`
	Practices            []string          `json:"practices"`
	UserInvocable        *bool             `json:"user_invocable"`
	CodexOverridePresent *bool             `json:"codex_override_present"`
	ReferencesCount      *int              `json:"references_count"`
}

type catalogV2Wire struct {
	SchemaVersion *string               `json:"schema_version"`
	GeneratedAt   *string               `json:"generated_at"`
	SkillCount    *int                  `json:"skill_count"`
	Skills        *[]catalogV2EntryWire `json:"skills"`
}

type catalogV2EntryWire struct {
	catalogV1EntryWire
	Dependencies *[]string `json:"dependencies"`
	GraphRoot    *bool     `json:"graph_root"`
}

type catalogV3Wire struct {
	SchemaVersion *string               `json:"schema_version"`
	SkillCount    *int                  `json:"skill_count"`
	Skills        *[]catalogV3EntryWire `json:"skills"`
}

type catalogV3EntryWire struct {
	catalogV2EntryWire
	Capabilities    *[]string `json:"capabilities"`
	Effects         *[]string `json:"effects"`
	CanonicalStatus *string   `json:"canonical_status"`
	Disposition     *string   `json:"disposition"`
	Tier            *string   `json:"tier"`
}

type catalogV4Wire struct {
	SchemaVersion *string               `json:"schema_version"`
	SkillCount    *int                  `json:"skill_count"`
	Skills        *[]catalogV4EntryWire `json:"skills"`
}

type catalogV4EntryWire struct {
	catalogV3EntryWire
	ContractV3 *contractV3Wire `json:"contract_v3"`
}

type contextRelWire struct {
	Kind *string `json:"kind"`
	With *string `json:"with"`
}

func decodeCatalogV1(data []byte) (*Catalog, error) {
	var wire catalogV1Wire
	if err := decodeStrict(data, &wire); err != nil {
		return nil, err
	}
	if err := validateEnvelope(wire.SchemaVersion, "1", wire.GeneratedAt, wire.SkillCount, wire.Skills); err != nil {
		return nil, err
	}
	entries := make([]CatalogEntry, 0, len(*wire.Skills))
	for index := range *wire.Skills {
		entry, err := normalizeV1Entry(&(*wire.Skills)[index], index)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return &Catalog{
		SchemaVersion: *wire.SchemaVersion,
		GeneratedAt:   *wire.GeneratedAt,
		SkillCount:    *wire.SkillCount,
		Skills:        entries,
	}, nil
}

func decodeCatalogV2(data []byte) (*Catalog, error) {
	var wire catalogV2Wire
	if err := decodeStrict(data, &wire); err != nil {
		return nil, err
	}
	if err := validateEnvelope(wire.SchemaVersion, "2", wire.GeneratedAt, wire.SkillCount, wire.Skills); err != nil {
		return nil, err
	}
	entries := make([]CatalogEntry, 0, len(*wire.Skills))
	for index := range *wire.Skills {
		entry, err := normalizeV2Entry(&(*wire.Skills)[index], index)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return &Catalog{
		SchemaVersion: *wire.SchemaVersion,
		GeneratedAt:   *wire.GeneratedAt,
		SkillCount:    *wire.SkillCount,
		Skills:        entries,
	}, nil
}

func decodeCatalogV3(data []byte) (*Catalog, error) {
	var wire catalogV3Wire
	if err := decodeStrict(data, &wire); err != nil {
		return nil, err
	}
	if err := validateEnvelope(wire.SchemaVersion, "3", nil, wire.SkillCount, wire.Skills); err != nil {
		return nil, err
	}
	entries := make([]CatalogEntry, 0, len(*wire.Skills))
	for index := range *wire.Skills {
		entry, err := normalizeV3Entry(&(*wire.Skills)[index], index)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return &Catalog{SchemaVersion: *wire.SchemaVersion, SkillCount: *wire.SkillCount, Skills: entries}, nil
}

func decodeCatalogV4(data []byte) (*Catalog, error) {
	var wire catalogV4Wire
	if err := decodeStrict(data, &wire); err != nil {
		return nil, err
	}
	if err := validateEnvelope(wire.SchemaVersion, "4", nil, wire.SkillCount, wire.Skills); err != nil {
		return nil, err
	}
	entries := make([]CatalogEntry, 0, len(*wire.Skills))
	for index := range *wire.Skills {
		entry, err := normalizeV4Entry(&(*wire.Skills)[index], index)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return &Catalog{SchemaVersion: *wire.SchemaVersion, SkillCount: *wire.SkillCount, Skills: entries}, nil
}

func validateEnvelope[T any](
	version *string,
	wantVersion string,
	generatedAt *string,
	skillCount *int,
	skills *[]T,
) error {
	if version == nil {
		return fmt.Errorf("schema_version is required for catalog v%s", wantVersion)
	}
	if *version != wantVersion {
		return fmt.Errorf("schema_version %q does not match catalog v%s", *version, wantVersion)
	}
	if wantVersion == "1" || wantVersion == "2" {
		if generatedAt == nil {
			return fmt.Errorf("generated_at is required for catalog v%s", wantVersion)
		}
		if _, err := time.Parse(time.RFC3339, *generatedAt); err != nil {
			return fmt.Errorf("generated_at must be RFC3339: %w", err)
		}
	}
	if skillCount == nil {
		return fmt.Errorf("skill_count is required for catalog v%s", wantVersion)
	}
	if skills == nil {
		return fmt.Errorf("skills is required for catalog v%s", wantVersion)
	}
	return nil
}

func normalizeV1Entry(wire *catalogV1EntryWire, index int) (CatalogEntry, error) {
	path := fmt.Sprintf("skills[%d]", index)
	if err := requireEntryV1(wire, path); err != nil {
		return CatalogEntry{}, err
	}
	if err := validateHexagonalRole(*wire.HexagonalRole, path+".hexagonal_role"); err != nil {
		return CatalogEntry{}, err
	}
	if *wire.ReferencesCount < 0 {
		return CatalogEntry{}, fmt.Errorf("%s.references_count must be non-negative", path)
	}
	return CatalogEntry{
		Name:                 *wire.Name,
		Description:          *wire.Description,
		HexagonalRole:        *wire.HexagonalRole,
		Consumes:             *wire.Consumes,
		Produces:             *wire.Produces,
		ContextRel:           normalizeContextRel(*wire.ContextRel),
		Practices:            wire.Practices,
		UserInvocable:        *wire.UserInvocable,
		CodexOverridePresent: *wire.CodexOverridePresent,
		ReferencesCount:      *wire.ReferencesCount,
	}, nil
}

func normalizeV2Entry(wire *catalogV2EntryWire, index int) (CatalogEntry, error) {
	entry, err := normalizeV1Entry(&wire.catalogV1EntryWire, index)
	if err != nil {
		return CatalogEntry{}, err
	}
	path := fmt.Sprintf("skills[%d]", index)
	if wire.Dependencies == nil {
		return CatalogEntry{}, fmt.Errorf("%s.dependencies is required", path)
	}
	if wire.GraphRoot == nil {
		return CatalogEntry{}, fmt.Errorf("%s.graph_root is required", path)
	}
	if err := validateUniqueStrings(*wire.Dependencies, path+".dependencies"); err != nil {
		return CatalogEntry{}, err
	}
	entry.Dependencies = *wire.Dependencies
	entry.GraphRoot = *wire.GraphRoot
	return entry, nil
}

func normalizeV3Entry(wire *catalogV3EntryWire, index int) (CatalogEntry, error) {
	entry, err := normalizeV2Entry(&wire.catalogV2EntryWire, index)
	if err != nil {
		return CatalogEntry{}, err
	}
	path := fmt.Sprintf("skills[%d]", index)
	switch {
	case wire.Capabilities == nil:
		return CatalogEntry{}, fmt.Errorf("%s.capabilities is required", path)
	case wire.Effects == nil:
		return CatalogEntry{}, fmt.Errorf("%s.effects is required", path)
	case wire.CanonicalStatus == nil:
		return CatalogEntry{}, fmt.Errorf("%s.canonical_status is required", path)
	case wire.Disposition == nil:
		return CatalogEntry{}, fmt.Errorf("%s.disposition is required", path)
	case wire.Tier == nil:
		return CatalogEntry{}, fmt.Errorf("%s.tier is required", path)
	}
	if err := validateUniqueStrings(*wire.Capabilities, path+".capabilities"); err != nil {
		return CatalogEntry{}, err
	}
	if err := validateUniqueStrings(*wire.Effects, path+".effects"); err != nil {
		return CatalogEntry{}, err
	}
	if *wire.CanonicalStatus != "canonical" {
		return CatalogEntry{}, fmt.Errorf("%s.canonical_status must be %q", path, "canonical")
	}
	if !validDisposition(*wire.Disposition) {
		return CatalogEntry{}, fmt.Errorf("%s.disposition has unsupported value %q", path, *wire.Disposition)
	}
	if *wire.Tier == "" {
		return CatalogEntry{}, fmt.Errorf("%s.tier must not be empty", path)
	}
	entry.Capabilities = *wire.Capabilities
	entry.Effects = *wire.Effects
	entry.CanonicalStatus = *wire.CanonicalStatus
	entry.Disposition = *wire.Disposition
	entry.Tier = *wire.Tier
	return entry, nil
}

func normalizeV4Entry(wire *catalogV4EntryWire, index int) (CatalogEntry, error) {
	entry, err := normalizeV3Entry(&wire.catalogV3EntryWire, index)
	if err != nil {
		return CatalogEntry{}, err
	}
	path := fmt.Sprintf("skills[%d].contract_v3", index)
	if wire.ContractV3 == nil {
		return CatalogEntry{}, fmt.Errorf("%s is required", path)
	}
	contract, err := normalizeContractV3(
		wire.ContractV3,
		path,
		entry.Name,
		entry.Dependencies,
	)
	if err != nil {
		return CatalogEntry{}, err
	}
	entry.ContractV3 = &contract
	return entry, nil
}

func requireEntryV1(wire *catalogV1EntryWire, path string) error {
	required := []struct {
		name    string
		present bool
	}{
		{"name", wire.Name != nil},
		{"description", wire.Description != nil},
		{"hexagonal_role", wire.HexagonalRole != nil},
		{"consumes", wire.Consumes != nil},
		{"produces", wire.Produces != nil},
		{"context_rel", wire.ContextRel != nil},
		{"user_invocable", wire.UserInvocable != nil},
		{"codex_override_present", wire.CodexOverridePresent != nil},
		{"references_count", wire.ReferencesCount != nil},
	}
	for _, field := range required {
		if !field.present {
			return fmt.Errorf("%s.%s is required", path, field.name)
		}
	}
	return nil
}

func normalizeContextRel(wires []contextRelWire) []ContextRel {
	out := make([]ContextRel, 0, len(wires))
	for _, wire := range wires {
		var rel ContextRel
		if wire.Kind != nil {
			rel.Kind = *wire.Kind
		}
		if wire.With != nil {
			rel.With = *wire.With
		}
		out = append(out, rel)
	}
	return out
}

func validateHexagonalRole(value, path string) error {
	switch value {
	case "domain", "driving-adapter", "driven-adapter", "supporting", "generic":
		return nil
	default:
		return fmt.Errorf("%s has unsupported value %q", path, value)
	}
}

func validDisposition(value string) bool {
	switch value {
	case "keep", "keep_off_path", "keep_strategy", "keep_optional_adapter", "keep_specialist":
		return true
	default:
		return false
	}
}

func validateUniqueStrings(values []string, path string) error {
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate value %q at index %d", path, value, index)
		}
		seen[value] = struct{}{}
	}
	return nil
}
