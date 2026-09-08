package config

import (
	"fmt"
	"os"
	"strings"
)

// ContextConfig holds caller-owned locators, never source content or work status.
// No field falls back to paths.learnings_dir or a public corpus.
type ContextConfig struct {
	SourceID             string `yaml:"source_id" json:"source_id"`
	ProjectID            string `yaml:"project_id" json:"project_id"`
	OwnerScope           string `yaml:"owner_scope" json:"owner_scope"`
	BundleID             string `yaml:"bundle_id" json:"bundle_id"`
	BundleRoot           string `yaml:"bundle_root" json:"bundle_root"`
	EvidenceRoot         string `yaml:"evidence_root" json:"evidence_root"`
	StagingRoot          string `yaml:"staging_root" json:"staging_root"`
	AccessPolicyRef      string `yaml:"access_policy_ref" json:"access_policy_ref"`
	OwnerPolicyRef       string `yaml:"owner_policy_ref" json:"owner_policy_ref"`
	TaskPolicyRef        string `yaml:"task_policy_ref" json:"task_policy_ref"`
	ModelPolicyRef       string `yaml:"model_policy_ref" json:"model_policy_ref"`
	DestinationPolicyRef string `yaml:"destination_policy_ref" json:"destination_policy_ref"`
	MaintenanceWorkRef   string `yaml:"maintenance_work_ref" json:"maintenance_work_ref"`
}

func (c *ContextConfig) fields() map[string]*string {
	return map[string]*string{
		"source_id": &c.SourceID, "project_id": &c.ProjectID, "owner_scope": &c.OwnerScope,
		"bundle_id": &c.BundleID, "bundle_root": &c.BundleRoot, "evidence_root": &c.EvidenceRoot,
		"staging_root": &c.StagingRoot, "access_policy_ref": &c.AccessPolicyRef,
		"owner_policy_ref": &c.OwnerPolicyRef, "task_policy_ref": &c.TaskPolicyRef,
		"model_policy_ref": &c.ModelPolicyRef, "destination_policy_ref": &c.DestinationPolicyRef,
		"maintenance_work_ref": &c.MaintenanceWorkRef,
	}
}

func mergeContext(dst *ContextConfig, src ContextConfig) {
	for key, value := range src.fields() {
		mergeStr(dst.fields()[key], *value)
	}
}
func applyContextEnv(c *ContextConfig) {
	for key, value := range c.fields() {
		applyEnvStr(value, "AGENTOPS_CONTEXT_"+strings.ToUpper(key))
	}
}

// LoadContext uses the existing precedence and explicit-file override. Unlike
// legacy generic configuration, unreadable/malformed ambient files are errors:
// a broken owner route must not silently expose a lower-priority route.
func LoadContext(overrides ContextConfig) (ContextConfig, map[string]Source, error) {
	var value ContextConfig
	sources := map[string]Source{}
	add := func(c ContextConfig, source Source) {
		for key, field := range c.fields() {
			if *field != "" {
				*value.fields()[key] = *field
				sources[key] = source
			}
		}
	}
	paths := []struct {
		path   string
		source Source
	}{}
	explicit := strings.TrimSpace(os.Getenv("AGENTOPS_CONFIG"))
	if explicit != "" {
		paths = append(paths, struct {
			path   string
			source Source
		}{explicit, Source(explicit)})
	} else {
		hp, hl := homeConfigReadInfo()
		pp, pl := projectConfigReadInfo()
		hs, ps := SourceHome, SourceProject
		if hl {
			hs = SourceHomeLegacy
		}
		if pl {
			ps = SourceProjectLegacy
		}
		paths = append(paths, struct {
			path   string
			source Source
		}{hp, hs}, struct {
			path   string
			source Source
		}{pp, ps})
	}
	for _, p := range paths {
		cfg, err := loadFromPath(p.path)
		if err != nil {
			if explicit == "" && os.IsNotExist(err) {
				continue
			}
			return value, nil, fmt.Errorf("context config: %w", err)
		}
		if cfg != nil {
			add(cfg.Context, p.source)
		}
	}
	var environment ContextConfig
	applyContextEnv(&environment)
	add(environment, SourceEnv)
	add(overrides, SourceFlag)
	return value, sources, nil
}
