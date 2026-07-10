// practices: [hexagonal-architecture, design-by-contract]
package orchestration

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ToolsContractRelPath    = "docs/contracts/orchestration-tools.yaml"
	ProfilesContractRelPath = "docs/contracts/orchestration-profiles.yaml"
)

// ToolsContract is the parsed orchestration-tools.yaml document.
type ToolsContract struct {
	VersionFloors map[string]VersionFloor `yaml:"version_floors"`
	Tools         []ToolSpec              `yaml:"tools"`
}

// VersionFloor is a minimum version requirement for a binary.
type VersionFloor struct {
	MinVersion string `yaml:"min_version"`
	Note       string `yaml:"note,omitempty"`
}

// ToolSpec describes one tool in the matrix.
type ToolSpec struct {
	ID          string   `yaml:"id"`
	Binary      string   `yaml:"binary"`
	ProbeArgs   []string `yaml:"probe_args"`
	RequiredFor []string `yaml:"required_for"`
	Optional    bool     `yaml:"optional"`
}

// ProfilesContract is the parsed orchestration-profiles.yaml document.
type ProfilesContract struct {
	Profiles map[string]ProfileSpec `yaml:"profiles"`
}

// ProfileSpec is one orchestration profile (may extend another).
type ProfileSpec struct {
	ProfileID        string         `yaml:"profile_id"`
	Extends          string         `yaml:"extends,omitempty"`
	Shape            string         `yaml:"shape,omitempty"`
	OwnerSkill       string         `yaml:"owner_skill,omitempty"`
	RequiredTools    []string       `yaml:"required_tools"`
	OptionalTools    []string       `yaml:"optional_tools,omitempty"`
	Roles            []string       `yaml:"roles,omitempty"`
	Panes            []ProfilePane  `yaml:"panes"`
	SpawnArgv        [][]string     `yaml:"spawn_argv"`
	SpawnArgvAdd     [][]string     `yaml:"spawn_argv_add"`
	LifecycleMarkers map[string]any `yaml:"lifecycle_markers,omitempty"`
	RequestContract  string         `yaml:"request_contract,omitempty"`
	ResultContract   string         `yaml:"result_contract,omitempty"`
	ChecklistMarkers map[string]any `yaml:"checklist_markers"`
	SendContracts    map[string]any `yaml:"send_contracts,omitempty"`
}

// ProfilePane is one pane slot in a profile.
type ProfilePane struct {
	Slot     int    `yaml:"slot" json:"slot"`
	Runtime  string `yaml:"runtime" json:"runtime"`
	Model    string `yaml:"model,omitempty" json:"model,omitempty"`
	Role     string `yaml:"role,omitempty" json:"role,omitempty"`
	ReadOnly bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
}

// LoadToolsContract reads orchestration-tools.yaml under repoRoot.
func LoadToolsContract(repoRoot string) (ToolsContract, error) {
	path := filepath.Join(repoRoot, ToolsContractRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolsContract{}, fmt.Errorf("read %s: %w", ToolsContractRelPath, err)
	}
	var doc ToolsContract
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ToolsContract{}, fmt.Errorf("parse %s: %w", ToolsContractRelPath, err)
	}
	return doc, nil
}

// LoadProfilesContract reads and resolves orchestration-profiles.yaml.
func LoadProfilesContract(repoRoot string) (ProfilesContract, error) {
	path := filepath.Join(repoRoot, ProfilesContractRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return ProfilesContract{}, fmt.Errorf("read %s: %w", ProfilesContractRelPath, err)
	}
	var raw struct {
		Profiles map[string]ProfileSpec `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ProfilesContract{}, fmt.Errorf("parse %s: %w", ProfilesContractRelPath, err)
	}
	resolved := make(map[string]ProfileSpec, len(raw.Profiles))
	for id, spec := range raw.Profiles {
		if spec.ProfileID == "" {
			spec.ProfileID = id
		}
		if spec.Extends != "" {
			base, ok := raw.Profiles[spec.Extends]
			if !ok {
				return ProfilesContract{}, fmt.Errorf("profile %q extends unknown %q", id, spec.Extends)
			}
			spec = mergeProfile(base, spec)
			spec.ProfileID = id
		}
		resolved[id] = spec
	}
	return ProfilesContract{Profiles: resolved}, nil
}

// ProfileByID returns a resolved profile or an error.
func (c ProfilesContract) ProfileByID(id string) (ProfileSpec, error) {
	p, ok := c.Profiles[id]
	if !ok {
		return ProfileSpec{}, fmt.Errorf("unknown profile %q", id)
	}
	return p, nil
}

func mergeProfile(base, overlay ProfileSpec) ProfileSpec {
	out := base
	if overlay.Shape != "" {
		out.Shape = overlay.Shape
	}
	if overlay.OwnerSkill != "" {
		out.OwnerSkill = overlay.OwnerSkill
	}
	if len(overlay.RequiredTools) > 0 {
		out.RequiredTools = overlay.RequiredTools
	}
	if len(overlay.OptionalTools) > 0 {
		out.OptionalTools = overlay.OptionalTools
	}
	if len(overlay.Roles) > 0 {
		out.Roles = overlay.Roles
	}
	if len(overlay.Panes) > 0 {
		out.Panes = overlay.Panes
	}
	if len(overlay.SpawnArgv) > 0 {
		out.SpawnArgv = overlay.SpawnArgv
	}
	if len(overlay.SpawnArgvAdd) > 0 {
		out.SpawnArgv = append(append([][]string{}, out.SpawnArgv...), overlay.SpawnArgvAdd...)
	}
	if overlay.ChecklistMarkers != nil {
		if out.ChecklistMarkers == nil {
			out.ChecklistMarkers = map[string]any{}
		}
		for k, v := range overlay.ChecklistMarkers {
			out.ChecklistMarkers[k] = v
		}
	}
	if overlay.SendContracts != nil {
		out.SendContracts = overlay.SendContracts
	}
	if overlay.LifecycleMarkers != nil {
		out.LifecycleMarkers = overlay.LifecycleMarkers
	}
	if overlay.RequestContract != "" {
		out.RequestContract = overlay.RequestContract
	}
	if overlay.ResultContract != "" {
		out.ResultContract = overlay.ResultContract
	}
	return out
}

// SpawnArgvFlat returns spawn argv tokens for drift-gate comparisons.
func (p ProfileSpec) SpawnArgvFlat() []string {
	var flat []string
	for _, chunk := range p.SpawnArgv {
		flat = append(flat, chunk...)
	}
	return flat
}
