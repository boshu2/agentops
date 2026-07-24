package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func decodeCatalog(data []byte) (*Catalog, error) {
	if err := validateJSONTokens(data); err != nil {
		return nil, err
	}

	var probe struct {
		SchemaVersion json.RawMessage `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("decode catalog envelope: %w", err)
	}
	if len(probe.SchemaVersion) == 0 {
		return nil, errors.New("schema_version is required")
	}
	var version string
	if err := json.Unmarshal(probe.SchemaVersion, &version); err != nil {
		return nil, fmt.Errorf("schema_version must be a string: %w", err)
	}

	var (
		cat *Catalog
		err error
	)
	switch version {
	case "1":
		cat, err = decodeCatalogV1(data)
	case "2":
		cat, err = decodeCatalogV2(data)
	case "3":
		cat, err = decodeCatalogV3(data)
	case "4":
		cat, err = decodeCatalogV4(data)
	default:
		return nil, fmt.Errorf("unsupported schema_version %q", version)
	}
	if err != nil {
		return nil, fmt.Errorf("catalog v%s: %w", version, err)
	}
	if err := validateCatalogIdentity(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func decodeStrict(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errors.New("trailing JSON value")
	default:
		return fmt.Errorf("trailing JSON: %w", err)
	}
}

// validateJSONTokens rejects duplicate object keys before unmarshalling can
// silently retain only the last value. It rejects null everywhere except the
// two nullable v4 artifact fields and proves one complete JSON value with no
// trailing content.
func validateJSONTokens(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSONValue(decoder, "$"); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("invalid JSON at %s: %w", path, err)
	}
	if token == nil {
		if strings.HasSuffix(path, ".schema_ref") || strings.HasSuffix(path, ".validator") {
			return nil
		}
		return fmt.Errorf("null value is not permitted at %s", path)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		return walkJSONObject(decoder, path)
	case '[':
		return walkJSONArray(decoder, path)
	default:
		return fmt.Errorf("unexpected JSON delimiter %q at %s", delim, path)
	}
}

func walkJSONObject(decoder *json.Decoder, path string) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid object key at %s: %w", path, err)
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("non-string object key at %s", path)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate object key %q at %s", key, path)
		}
		seen[key] = struct{}{}
		if err := walkJSONValue(decoder, path+"."+key); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("unterminated object at %s: %w", path, err)
	}
	if end != json.Delim('}') {
		return fmt.Errorf("unexpected object terminator %q at %s", end, path)
	}
	return nil
}

func walkJSONArray(decoder *json.Decoder, path string) error {
	index := 0
	for decoder.More() {
		if err := walkJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
		index++
	}
	end, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("unterminated array at %s: %w", path, err)
	}
	if end != json.Delim(']') {
		return fmt.Errorf("unexpected array terminator %q at %s", end, path)
	}
	return nil
}

func validateCatalogIdentity(cat *Catalog) error {
	if cat.SkillCount < 0 {
		return fmt.Errorf("skill_count must be non-negative, got %d", cat.SkillCount)
	}
	if cat.SkillCount != len(cat.Skills) {
		return fmt.Errorf("skill_count %d does not match skills length %d", cat.SkillCount, len(cat.Skills))
	}
	seen := make(map[string]struct{}, len(cat.Skills))
	for index, entry := range cat.Skills {
		if _, exists := seen[entry.Name]; exists {
			return fmt.Errorf("duplicate skill name %q at skills[%d]", entry.Name, index)
		}
		seen[entry.Name] = struct{}{}
	}
	return nil
}
