package packet

import (
	"encoding/json"
	"fmt"
)

// CurrentExecutionPacketSchemaVersion is the only version emitted by current
// persistent writers. Readers continue to accept supported legacy versions.
const CurrentExecutionPacketSchemaVersion = 3

// MarshalJSON keeps explicit v1/v2 values serializable in their owned legacy
// shape for compatibility fixtures while making v3's canonical mortem names
// the default writer representation. The aggregate retains Go field names used
// by existing callers; the version boundary owns the wire names.
func (p ExecutionPacket) MarshalJSON() ([]byte, error) {
	type executionPacketAlias ExecutionPacket
	data, err := json.Marshal(executionPacketAlias(p))
	if err != nil || p.SchemaVersion < CurrentExecutionPacketSchemaVersion {
		return data, err
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("normalize canonical execution packet writer: %w", err)
	}
	moveRawJSONKey(root, "pre_mortem_verdict", "premortem_verdict")

	if rawArtifacts, ok := root["artifacts"]; ok {
		var artifacts map[string]json.RawMessage
		if err := json.Unmarshal(rawArtifacts, &artifacts); err != nil {
			return nil, fmt.Errorf("normalize canonical execution packet artifacts: %w", err)
		}
		moveRawJSONKey(artifacts, "pre_mortem_path", "premortem_path")
		normalized, err := json.Marshal(artifacts)
		if err != nil {
			return nil, fmt.Errorf("marshal canonical execution packet artifacts: %w", err)
		}
		root["artifacts"] = normalized
	}

	return json.Marshal(root)
}

func moveRawJSONKey(object map[string]json.RawMessage, legacy, canonical string) {
	if value, ok := object[legacy]; ok {
		object[canonical] = value
		delete(object, legacy)
	}
}
