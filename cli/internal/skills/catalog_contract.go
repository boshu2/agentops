package skills

// ContractV3 is the typed semantic contract carried by shadow catalog v4
// entries. Legacy catalog fields remain available on CatalogEntry while the
// shadow rail is evaluated.
type ContractV3 struct {
	SchemaVersion  string                   `json:"schema_version"`
	PrimaryLayer   string                   `json:"primary_layer"`
	LifecycleSeams []string                 `json:"lifecycle_seams"`
	Authority      []string                 `json:"authority"`
	Effects        []ContractEffect         `json:"effects"`
	Artifacts      ContractArtifacts        `json:"artifacts"`
	Triggers       ContractTriggers         `json:"triggers"`
	Failure        ContractFailureSemantics `json:"failure"`
	Proof          ContractProof            `json:"proof"`
}

// ContractEffect describes one declared effect and its authorization, cleanup,
// and receipt obligations.
type ContractEffect struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Scope         string `json:"scope"`
	Authorization string `json:"authorization"`
	Cleanup       string `json:"cleanup"`
	Receipt       string `json:"receipt"`
}

// ContractArtifacts separates typed inputs from typed outputs.
type ContractArtifacts struct {
	Consumes []ContractArtifact `json:"consumes"`
	Produces []ContractArtifact `json:"produces"`
}

// ContractArtifact identifies an artifact and its meaning. SchemaRef and
// Validator are nullable in the wire contract; nil preserves an explicit null.
type ContractArtifact struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Semantics string  `json:"semantics"`
	SchemaRef *string `json:"schema_ref"`
	Validator *string `json:"validator"`
}

// ContractTriggers carries every required routing-fixture family.
type ContractTriggers struct {
	Positive         []ContractTriggerCase     `json:"positive"`
	Negative         []ContractTriggerCase     `json:"negative"`
	Ambiguity        []ContractTriggerCase     `json:"ambiguity"`
	Aliases          []ContractTriggerAlias    `json:"aliases"`
	NearestNeighbors []ContractNearestNeighbor `json:"nearest_neighbors"`
}

// ContractTriggerCase is one positive, negative, or ambiguity routing probe.
type ContractTriggerCase struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Expected string `json:"expected"`
}

// ContractTriggerAlias binds an alternate phrase to its canonical skill.
type ContractTriggerAlias struct {
	ID             string `json:"id"`
	Alias          string `json:"alias"`
	CanonicalSkill string `json:"canonical_skill"`
}

// ContractNearestNeighbor explains how a nearby skill differs.
type ContractNearestNeighbor struct {
	ID          string `json:"id"`
	Skill       string `json:"skill"`
	Distinction string `json:"distinction"`
}

// ContractFailureSemantics records the required terminal behavior for each
// hostile runtime condition.
type ContractFailureSemantics struct {
	Unavailable     ContractFailureCase `json:"unavailable"`
	Timeout         ContractFailureCase `json:"timeout"`
	PartialEvidence ContractFailureCase `json:"partial_evidence"`
	PartialMutation ContractFailureCase `json:"partial_mutation"`
	Cleanup         ContractFailureCase `json:"cleanup"`
}

// ContractFailureCase binds a closed action to human-readable detail.
type ContractFailureCase struct {
	Action string `json:"action"`
	Detail string `json:"detail"`
}

// ContractProof declares the behavioral proof class, executable command,
// content-bound harness closure, and fixture references used by the probe.
type ContractProof struct {
	Class       string   `json:"class"`
	Command     string   `json:"command"`
	HarnessRefs []string `json:"harness_refs"`
	FixtureRefs []string `json:"fixture_refs"`
}
