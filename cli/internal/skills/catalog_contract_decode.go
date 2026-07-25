package skills

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
)

var contractIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

type contractV3Wire struct {
	SchemaVersion  *string                       `json:"schema_version"`
	PrimaryLayer   *string                       `json:"primary_layer"`
	LifecycleSeams *[]string                     `json:"lifecycle_seams"`
	Authority      *[]string                     `json:"authority"`
	Effects        *[]contractEffectWire         `json:"effects"`
	Artifacts      *contractArtifactsWire        `json:"artifacts"`
	Triggers       *contractTriggersWire         `json:"triggers"`
	Failure        *contractFailureSemanticsWire `json:"failure"`
	Proof          *contractProofWire            `json:"proof"`
}

type contractEffectWire struct {
	ID            *string `json:"id"`
	Kind          *string `json:"kind"`
	Scope         *string `json:"scope"`
	Authorization *string `json:"authorization"`
	Cleanup       *string `json:"cleanup"`
	Receipt       *string `json:"receipt"`
}

type contractArtifactsWire struct {
	Consumes *[]contractArtifactWire `json:"consumes"`
	Produces *[]contractArtifactWire `json:"produces"`
}

type contractArtifactWire struct {
	Name      *string         `json:"name"`
	Kind      *string         `json:"kind"`
	Semantics *string         `json:"semantics"`
	SchemaRef json.RawMessage `json:"schema_ref"`
	Validator json.RawMessage `json:"validator"`
}

type contractTriggersWire struct {
	Positive         *[]contractTriggerCaseWire     `json:"positive"`
	Negative         *[]contractTriggerCaseWire     `json:"negative"`
	Ambiguity        *[]contractTriggerCaseWire     `json:"ambiguity"`
	Aliases          *[]contractTriggerAliasWire    `json:"aliases"`
	NearestNeighbors *[]contractNearestNeighborWire `json:"nearest_neighbors"`
}

type contractTriggerCaseWire struct {
	ID       *string `json:"id"`
	Prompt   *string `json:"prompt"`
	Expected *string `json:"expected"`
}

type contractTriggerAliasWire struct {
	ID             *string `json:"id"`
	Alias          *string `json:"alias"`
	CanonicalSkill *string `json:"canonical_skill"`
}

type contractNearestNeighborWire struct {
	ID          *string `json:"id"`
	Skill       *string `json:"skill"`
	Distinction *string `json:"distinction"`
}

type contractFailureSemanticsWire struct {
	Unavailable     *contractFailureCaseWire `json:"unavailable"`
	Timeout         *contractFailureCaseWire `json:"timeout"`
	PartialEvidence *contractFailureCaseWire `json:"partial_evidence"`
	PartialMutation *contractFailureCaseWire `json:"partial_mutation"`
	Cleanup         *contractFailureCaseWire `json:"cleanup"`
}

type contractFailureCaseWire struct {
	Action *string `json:"action"`
	Detail *string `json:"detail"`
}

type contractProofWire struct {
	Class       *string   `json:"class"`
	Command     *string   `json:"command"`
	HarnessRefs *[]string `json:"harness_refs"`
	FixtureRefs *[]string `json:"fixture_refs"`
}

func normalizeContractV3(
	wire *contractV3Wire,
	path string,
	skillName string,
	dependencies []string,
) (ContractV3, error) {
	if err := requireContractV3(wire, path); err != nil {
		return ContractV3{}, err
	}
	if *wire.SchemaVersion != "skill-contract.v3" {
		return ContractV3{}, fmt.Errorf("%s.schema_version must be %q", path, "skill-contract.v3")
	}
	if err := validateLayer(*wire.PrimaryLayer, path+".primary_layer"); err != nil {
		return ContractV3{}, err
	}
	if err := validateSeams(*wire.LifecycleSeams, path+".lifecycle_seams"); err != nil {
		return ContractV3{}, err
	}
	if err := validateAuthority(*wire.Authority, path+".authority"); err != nil {
		return ContractV3{}, err
	}
	effects, err := normalizeContractEffects(*wire.Effects, path+".effects")
	if err != nil {
		return ContractV3{}, err
	}
	artifacts, err := normalizeContractArtifacts(wire.Artifacts, path+".artifacts")
	if err != nil {
		return ContractV3{}, err
	}
	triggers, err := normalizeContractTriggers(wire.Triggers, path+".triggers")
	if err != nil {
		return ContractV3{}, err
	}
	failures, err := normalizeFailureSemantics(wire.Failure, path+".failure")
	if err != nil {
		return ContractV3{}, err
	}
	proof, err := normalizeContractProof(wire.Proof, path+".proof")
	if err != nil {
		return ContractV3{}, err
	}
	if err := validateSkillScopedAuthority(
		skillName,
		*wire.PrimaryLayer,
		*wire.Authority,
		path+".authority",
	); err != nil {
		return ContractV3{}, err
	}
	if err := validateEffectSemantics(*wire.Authority, effects, path+".effects"); err != nil {
		return ContractV3{}, err
	}
	if err := validateBindingArtifacts(artifacts.Produces, path+".artifacts.produces"); err != nil {
		return ContractV3{}, err
	}
	if err := validateTriggerSemantics(skillName, triggers, path+".triggers"); err != nil {
		return ContractV3{}, err
	}
	if err := validateHardDependencies(skillName, dependencies, path); err != nil {
		return ContractV3{}, err
	}
	return ContractV3{
		SchemaVersion:  *wire.SchemaVersion,
		PrimaryLayer:   *wire.PrimaryLayer,
		LifecycleSeams: *wire.LifecycleSeams,
		Authority:      *wire.Authority,
		Effects:        effects,
		Artifacts:      artifacts,
		Triggers:       triggers,
		Failure:        failures,
		Proof:          proof,
	}, nil
}

func validateSkillScopedAuthority(
	skillName string,
	primaryLayer string,
	authority []string,
	path string,
) error {
	declared := stringSet(authority)
	forbidden := []struct {
		verb     string
		rejected bool
		message  string
	}{
		{"refine_intent", skillName != "plan", "only plan may refine_intent"},
		{"dispatch_phase", skillName != "rpi", "only rpi may dispatch_phase"},
		{"write_verdict", skillName != "validate", "only validate may write_verdict"},
		{"transport", primaryLayer != "runtime", "transport requires runtime primary_layer"},
	}
	for _, check := range forbidden {
		if _, exists := declared[check.verb]; exists && check.rejected {
			return fmt.Errorf("%s: %s", path, check.message)
		}
	}
	if skillName == "rpi" {
		if _, exists := declared["dispatch_phase"]; !exists {
			return fmt.Errorf("%s: rpi must declare dispatch_phase", path)
		}
	}
	return nil
}

func validateEffectSemantics(
	authority []string,
	effects []ContractEffect,
	path string,
) error {
	mutatingKinds := stringSet([]string{
		"filesystem.write",
		"network.write",
		"environment.write",
		"credential.switch",
		"external.mutate",
		"runtime.session",
		"host.configure",
	})
	receiptRequiredKinds := stringSet([]string{
		"filesystem.write",
		"network.write",
		"environment.write",
		"credential.switch",
		"external.mutate",
		"runtime.session",
		"host.configure",
		"process.start",
	})
	cleanupRequiredKinds := stringSet([]string{
		"process.start",
		"credential.switch",
		"runtime.session",
	})

	hasMutatingEffect := false
	hasAuthorizedMutation := false
	for index, effect := range effects {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if _, required := receiptRequiredKinds[effect.Kind]; required && effect.Receipt != "required" {
			return fmt.Errorf(
				"%s effect %q (%s) requires receipt=required",
				itemPath,
				effect.ID,
				effect.Kind,
			)
		}
		if _, required := cleanupRequiredKinds[effect.Kind]; required && effect.Cleanup != "required" {
			return fmt.Errorf(
				"%s effect %q (%s) requires cleanup=required",
				itemPath,
				effect.ID,
				effect.Kind,
			)
		}
		if _, mutating := mutatingKinds[effect.Kind]; mutating {
			hasMutatingEffect = true
			if effect.Authorization == "caller" || effect.Authorization == "implement" {
				hasAuthorizedMutation = true
			}
		}
	}

	_, mayMutate := stringSet(authority)["mutate_subject"]
	switch {
	case mayMutate && !hasAuthorizedMutation:
		return fmt.Errorf(
			"%s: mutate_subject requires a mutating effect authorized by caller or implement",
			path,
		)
	case !mayMutate && hasMutatingEffect:
		return fmt.Errorf("%s: a mutating effect requires mutate_subject authority", path)
	default:
		return nil
	}
}

func validateBindingArtifacts(produces []ContractArtifact, path string) error {
	for index, artifact := range produces {
		if artifact.Semantics == "binding" &&
			(artifact.SchemaRef == nil || artifact.Validator == nil) {
			return fmt.Errorf(
				"%s[%d] binding output %q requires schema_ref and validator",
				path,
				index,
				artifact.Name,
			)
		}
	}
	return nil
}

func validateTriggerSemantics(
	skillName string,
	triggers ContractTriggers,
	path string,
) error {
	seenText := make(map[string]string)
	addText := func(value, location string) error {
		normalized := normalizeTriggerText(value)
		if previous, exists := seenText[normalized]; exists {
			return fmt.Errorf(
				"%s contains normalized trigger text collision at %s and %s",
				path,
				previous,
				location,
			)
		}
		seenText[normalized] = location
		return nil
	}
	families := []struct {
		name   string
		values []ContractTriggerCase
	}{
		{"positive", triggers.Positive},
		{"negative", triggers.Negative},
		{"ambiguity", triggers.Ambiguity},
	}
	for _, family := range families {
		for index, trigger := range family.values {
			if err := addText(
				trigger.Prompt,
				fmt.Sprintf("%s.%s[%d].prompt", path, family.name, index),
			); err != nil {
				return err
			}
		}
	}
	for index, alias := range triggers.Aliases {
		location := fmt.Sprintf("%s.aliases[%d]", path, index)
		if err := addText(alias.Alias, location+".alias"); err != nil {
			return err
		}
		if alias.CanonicalSkill != skillName {
			return fmt.Errorf(
				"%s.canonical_skill must equal owning skill %q",
				location,
				skillName,
			)
		}
	}
	for index, neighbor := range triggers.NearestNeighbors {
		if neighbor.Skill == skillName {
			return fmt.Errorf(
				"%s.nearest_neighbors[%d] nearest neighbor must differ from owning skill %q",
				path,
				index,
				skillName,
			)
		}
	}
	return nil
}

func normalizeTriggerText(value string) string {
	return cases.Fold().String(strings.Join(strings.Fields(value), " "))
}

func validateHardDependencies(skillName string, dependencies []string, path string) error {
	if skillName != "rpi" {
		if len(dependencies) != 0 {
			return fmt.Errorf("%s: %s may not declare hard skill dependencies", path, skillName)
		}
		return nil
	}
	required := stringSet([]string{"plan", "implement", "validate"})
	actual := stringSet(dependencies)
	if len(actual) != len(required) {
		return fmt.Errorf(
			"%s: rpi hard dependencies must be exactly plan, implement, and validate",
			path,
		)
	}
	for dependency := range required {
		if _, exists := actual[dependency]; !exists {
			return fmt.Errorf(
				"%s: rpi hard dependencies must be exactly plan, implement, and validate",
				path,
			)
		}
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func requireContractV3(wire *contractV3Wire, path string) error {
	required := []struct {
		name    string
		present bool
	}{
		{"schema_version", wire.SchemaVersion != nil},
		{"primary_layer", wire.PrimaryLayer != nil},
		{"lifecycle_seams", wire.LifecycleSeams != nil},
		{"authority", wire.Authority != nil},
		{"effects", wire.Effects != nil},
		{"artifacts", wire.Artifacts != nil},
		{"triggers", wire.Triggers != nil},
		{"failure", wire.Failure != nil},
		{"proof", wire.Proof != nil},
	}
	for _, field := range required {
		if !field.present {
			return fmt.Errorf("%s.%s is required", path, field.name)
		}
	}
	return nil
}

func validateLayer(value, path string) error {
	switch value {
	case "product", "campaign", "experiment", "judgment", "evidence",
		"implementation", "evolution", "runtime", "support":
		return nil
	default:
		return fmt.Errorf("%s has unsupported value %q", path, value)
	}
}

func validateSeams(values []string, path string) error {
	if err := validateUniqueStrings(values, path); err != nil {
		return err
	}
	for _, value := range values {
		switch value {
		case "product_input", "goal_design", "goal_observe", "option_shaping",
			"plan_input", "plan_review", "implement_method", "validate_evidence",
			"validate_strategy", "post_verdict", "runtime_transport",
			"cross_cutting", "standalone":
		default:
			return fmt.Errorf("%s has unsupported value %q", path, value)
		}
	}
	return nil
}

func validateAuthority(values []string, path string) error {
	if err := validateUniqueStrings(values, path); err != nil {
		return err
	}
	for _, value := range values {
		switch value {
		case "advise", "read_evidence", "refine_intent", "mutate_subject",
			"dispatch_phase", "write_verdict", "transport":
		default:
			return fmt.Errorf("%s has unsupported value %q", path, value)
		}
	}
	return nil
}

func normalizeContractEffects(wires []contractEffectWire, path string) ([]ContractEffect, error) {
	out := make([]ContractEffect, 0, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case wire.ID == nil:
			return nil, fmt.Errorf("%s.id is required", itemPath)
		case wire.Kind == nil:
			return nil, fmt.Errorf("%s.kind is required", itemPath)
		case wire.Scope == nil:
			return nil, fmt.Errorf("%s.scope is required", itemPath)
		case wire.Authorization == nil:
			return nil, fmt.Errorf("%s.authorization is required", itemPath)
		case wire.Cleanup == nil:
			return nil, fmt.Errorf("%s.cleanup is required", itemPath)
		case wire.Receipt == nil:
			return nil, fmt.Errorf("%s.receipt is required", itemPath)
		}
		if *wire.ID == "" || *wire.Scope == "" {
			return nil, fmt.Errorf("%s id and scope must not be empty", itemPath)
		}
		if err := validateContractIdentifier(*wire.ID, itemPath+".id"); err != nil {
			return nil, err
		}
		if _, exists := seen[*wire.ID]; exists {
			return nil, fmt.Errorf("%s contains duplicate effect id %q", path, *wire.ID)
		}
		seen[*wire.ID] = struct{}{}
		if err := validateEffectEnums(wire, itemPath); err != nil {
			return nil, err
		}
		out = append(out, ContractEffect{
			ID:            *wire.ID,
			Kind:          *wire.Kind,
			Scope:         *wire.Scope,
			Authorization: *wire.Authorization,
			Cleanup:       *wire.Cleanup,
			Receipt:       *wire.Receipt,
		})
	}
	return out, nil
}

func validateEffectEnums(wire contractEffectWire, path string) error {
	switch *wire.Kind {
	case "filesystem.read", "filesystem.write", "process.start", "network.read",
		"network.write", "environment.read", "environment.write", "clock.read",
		"credential.switch", "external.mutate", "runtime.session", "host.configure":
	default:
		return fmt.Errorf("%s.kind has unsupported value %q", path, *wire.Kind)
	}
	switch *wire.Authorization {
	case "caller", "intent", "implement", "validate", "goal":
	default:
		return fmt.Errorf("%s.authorization has unsupported value %q", path, *wire.Authorization)
	}
	switch *wire.Cleanup {
	case "none", "best_effort", "required":
	default:
		return fmt.Errorf("%s.cleanup has unsupported value %q", path, *wire.Cleanup)
	}
	switch *wire.Receipt {
	case "none", "optional", "required":
	default:
		return fmt.Errorf("%s.receipt has unsupported value %q", path, *wire.Receipt)
	}
	return nil
}

func normalizeContractArtifacts(wire *contractArtifactsWire, path string) (ContractArtifacts, error) {
	if wire.Consumes == nil {
		return ContractArtifacts{}, fmt.Errorf("%s.consumes is required", path)
	}
	if wire.Produces == nil {
		return ContractArtifacts{}, fmt.Errorf("%s.produces is required", path)
	}
	consumes, err := normalizeContractArtifactList(*wire.Consumes, path+".consumes")
	if err != nil {
		return ContractArtifacts{}, err
	}
	produces, err := normalizeContractArtifactList(*wire.Produces, path+".produces")
	if err != nil {
		return ContractArtifacts{}, err
	}
	seen := make(map[string]struct{}, len(consumes)+len(produces))
	for _, artifact := range append(append([]ContractArtifact{}, consumes...), produces...) {
		if _, exists := seen[artifact.Name]; exists {
			return ContractArtifacts{}, fmt.Errorf("%s contains duplicate artifact name %q", path, artifact.Name)
		}
		seen[artifact.Name] = struct{}{}
	}
	return ContractArtifacts{Consumes: consumes, Produces: produces}, nil
}

func normalizeContractArtifactList(wires []contractArtifactWire, path string) ([]ContractArtifact, error) {
	out := make([]ContractArtifact, 0, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case wire.Name == nil:
			return nil, fmt.Errorf("%s.name is required", itemPath)
		case wire.Kind == nil:
			return nil, fmt.Errorf("%s.kind is required", itemPath)
		case wire.Semantics == nil:
			return nil, fmt.Errorf("%s.semantics is required", itemPath)
		case len(wire.SchemaRef) == 0:
			return nil, fmt.Errorf("%s.schema_ref is required", itemPath)
		case len(wire.Validator) == 0:
			return nil, fmt.Errorf("%s.validator is required", itemPath)
		}
		if *wire.Name == "" {
			return nil, fmt.Errorf("%s.name must not be empty", itemPath)
		}
		if err := validateContractIdentifier(*wire.Name, itemPath+".name"); err != nil {
			return nil, err
		}
		if _, exists := seen[*wire.Name]; exists {
			return nil, fmt.Errorf("%s contains duplicate artifact name %q", path, *wire.Name)
		}
		seen[*wire.Name] = struct{}{}
		if err := validateArtifactEnums(*wire.Kind, *wire.Semantics, itemPath); err != nil {
			return nil, err
		}
		schemaRef, err := decodeNullableString(wire.SchemaRef, itemPath+".schema_ref")
		if err != nil {
			return nil, err
		}
		validator, err := decodeNullableString(wire.Validator, itemPath+".validator")
		if err != nil {
			return nil, err
		}
		out = append(out, ContractArtifact{
			Name:      *wire.Name,
			Kind:      *wire.Kind,
			Semantics: *wire.Semantics,
			SchemaRef: schemaRef,
			Validator: validator,
		})
	}
	return out, nil
}

func validateArtifactEnums(kind, semantics, path string) error {
	switch kind {
	case "intent", "plan", "evidence", "source", "candidate", "verdict",
		"receipt", "report", "projection", "runtime_state", "configuration":
	default:
		return fmt.Errorf("%s.kind has unsupported value %q", path, kind)
	}
	switch semantics {
	case "factual", "advisory", "binding":
	default:
		return fmt.Errorf("%s.semantics has unsupported value %q", path, semantics)
	}
	return nil
}

func decodeNullableString(raw json.RawMessage, path string) (*string, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%s must be a string or null: %w", path, err)
	}
	if value == "" {
		return nil, fmt.Errorf("%s must not be an empty string", path)
	}
	return &value, nil
}

func normalizeContractTriggers(wire *contractTriggersWire, path string) (ContractTriggers, error) {
	switch {
	case wire.Positive == nil:
		return ContractTriggers{}, fmt.Errorf("%s.positive is required", path)
	case wire.Negative == nil:
		return ContractTriggers{}, fmt.Errorf("%s.negative is required", path)
	case wire.Ambiguity == nil:
		return ContractTriggers{}, fmt.Errorf("%s.ambiguity is required", path)
	case wire.Aliases == nil:
		return ContractTriggers{}, fmt.Errorf("%s.aliases is required", path)
	case wire.NearestNeighbors == nil:
		return ContractTriggers{}, fmt.Errorf("%s.nearest_neighbors is required", path)
	}
	positive, err := normalizeTriggerCases(*wire.Positive, "route", path+".positive")
	if err != nil {
		return ContractTriggers{}, err
	}
	negative, err := normalizeTriggerCases(*wire.Negative, "do_not_route", path+".negative")
	if err != nil {
		return ContractTriggers{}, err
	}
	ambiguity, err := normalizeTriggerCases(*wire.Ambiguity, "clarify", path+".ambiguity")
	if err != nil {
		return ContractTriggers{}, err
	}
	aliases, err := normalizeAliases(*wire.Aliases, path+".aliases")
	if err != nil {
		return ContractTriggers{}, err
	}
	neighbors, err := normalizeNearestNeighbors(*wire.NearestNeighbors, path+".nearest_neighbors")
	if err != nil {
		return ContractTriggers{}, err
	}
	triggers := ContractTriggers{
		Positive:         positive,
		Negative:         negative,
		Ambiguity:        ambiguity,
		Aliases:          aliases,
		NearestNeighbors: neighbors,
	}
	if err := validateUniqueTriggerIDs(triggers, path); err != nil {
		return ContractTriggers{}, err
	}
	return triggers, nil
}

func normalizeTriggerCases(
	wires []contractTriggerCaseWire,
	expected string,
	path string,
) ([]ContractTriggerCase, error) {
	if len(wires) == 0 {
		return nil, fmt.Errorf("%s must not be empty", path)
	}
	out := make([]ContractTriggerCase, 0, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case wire.ID == nil:
			return nil, fmt.Errorf("%s.id is required", itemPath)
		case wire.Prompt == nil:
			return nil, fmt.Errorf("%s.prompt is required", itemPath)
		case wire.Expected == nil:
			return nil, fmt.Errorf("%s.expected is required", itemPath)
		}
		if *wire.ID == "" || *wire.Prompt == "" {
			return nil, fmt.Errorf("%s id and prompt must not be empty", itemPath)
		}
		if err := validateContractIdentifier(*wire.ID, itemPath+".id"); err != nil {
			return nil, err
		}
		if *wire.Expected != expected {
			return nil, fmt.Errorf("%s.expected must be %q", itemPath, expected)
		}
		if _, exists := seen[*wire.ID]; exists {
			return nil, fmt.Errorf("%s contains duplicate id %q", path, *wire.ID)
		}
		seen[*wire.ID] = struct{}{}
		out = append(out, ContractTriggerCase{ID: *wire.ID, Prompt: *wire.Prompt, Expected: *wire.Expected})
	}
	return out, nil
}

func normalizeAliases(wires []contractTriggerAliasWire, path string) ([]ContractTriggerAlias, error) {
	if len(wires) == 0 {
		return nil, fmt.Errorf("%s must not be empty", path)
	}
	out := make([]ContractTriggerAlias, 0, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case wire.ID == nil:
			return nil, fmt.Errorf("%s.id is required", itemPath)
		case wire.Alias == nil:
			return nil, fmt.Errorf("%s.alias is required", itemPath)
		case wire.CanonicalSkill == nil:
			return nil, fmt.Errorf("%s.canonical_skill is required", itemPath)
		}
		if err := requireUniqueNonEmptyID(seen, *wire.ID, itemPath, path); err != nil {
			return nil, err
		}
		if err := validateContractIdentifier(*wire.ID, itemPath+".id"); err != nil {
			return nil, err
		}
		if err := validateContractIdentifier(*wire.CanonicalSkill, itemPath+".canonical_skill"); err != nil {
			return nil, err
		}
		if *wire.Alias == "" || *wire.CanonicalSkill == "" {
			return nil, fmt.Errorf("%s alias and canonical_skill must not be empty", itemPath)
		}
		out = append(out, ContractTriggerAlias{
			ID:             *wire.ID,
			Alias:          *wire.Alias,
			CanonicalSkill: *wire.CanonicalSkill,
		})
	}
	return out, nil
}

func normalizeNearestNeighbors(
	wires []contractNearestNeighborWire,
	path string,
) ([]ContractNearestNeighbor, error) {
	if len(wires) == 0 {
		return nil, fmt.Errorf("%s must not be empty", path)
	}
	out := make([]ContractNearestNeighbor, 0, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case wire.ID == nil:
			return nil, fmt.Errorf("%s.id is required", itemPath)
		case wire.Skill == nil:
			return nil, fmt.Errorf("%s.skill is required", itemPath)
		case wire.Distinction == nil:
			return nil, fmt.Errorf("%s.distinction is required", itemPath)
		}
		if err := requireUniqueNonEmptyID(seen, *wire.ID, itemPath, path); err != nil {
			return nil, err
		}
		if err := validateContractIdentifier(*wire.ID, itemPath+".id"); err != nil {
			return nil, err
		}
		if err := validateContractIdentifier(*wire.Skill, itemPath+".skill"); err != nil {
			return nil, err
		}
		if *wire.Skill == "" || *wire.Distinction == "" {
			return nil, fmt.Errorf("%s skill and distinction must not be empty", itemPath)
		}
		out = append(out, ContractNearestNeighbor{
			ID:          *wire.ID,
			Skill:       *wire.Skill,
			Distinction: *wire.Distinction,
		})
	}
	return out, nil
}

func requireUniqueNonEmptyID(seen map[string]struct{}, id, itemPath, listPath string) error {
	if id == "" {
		return fmt.Errorf("%s.id must not be empty", itemPath)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("%s contains duplicate id %q", listPath, id)
	}
	seen[id] = struct{}{}
	return nil
}

func validateUniqueTriggerIDs(triggers ContractTriggers, path string) error {
	seen := make(map[string]struct{})
	add := func(id string) error {
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s contains duplicate trigger id %q", path, id)
		}
		seen[id] = struct{}{}
		return nil
	}
	for _, cases := range [][]ContractTriggerCase{triggers.Positive, triggers.Negative, triggers.Ambiguity} {
		for _, trigger := range cases {
			if err := add(trigger.ID); err != nil {
				return err
			}
		}
	}
	for _, alias := range triggers.Aliases {
		if err := add(alias.ID); err != nil {
			return err
		}
	}
	for _, neighbor := range triggers.NearestNeighbors {
		if err := add(neighbor.ID); err != nil {
			return err
		}
	}
	return nil
}

func validateContractIdentifier(value, path string) error {
	if !contractIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%s has invalid identifier %q", path, value)
	}
	return nil
}

func normalizeFailureSemantics(
	wire *contractFailureSemanticsWire,
	path string,
) (ContractFailureSemantics, error) {
	required := []struct {
		name string
		wire *contractFailureCaseWire
	}{
		{"unavailable", wire.Unavailable},
		{"timeout", wire.Timeout},
		{"partial_evidence", wire.PartialEvidence},
		{"partial_mutation", wire.PartialMutation},
		{"cleanup", wire.Cleanup},
	}
	normalized := make([]ContractFailureCase, 0, len(required))
	for _, field := range required {
		if field.wire == nil {
			return ContractFailureSemantics{}, fmt.Errorf("%s.%s is required", path, field.name)
		}
		value, err := normalizeFailureCase(field.wire, path+"."+field.name)
		if err != nil {
			return ContractFailureSemantics{}, err
		}
		normalized = append(normalized, value)
	}
	return ContractFailureSemantics{
		Unavailable:     normalized[0],
		Timeout:         normalized[1],
		PartialEvidence: normalized[2],
		PartialMutation: normalized[3],
		Cleanup:         normalized[4],
	}, nil
}

func normalizeFailureCase(wire *contractFailureCaseWire, path string) (ContractFailureCase, error) {
	if wire.Action == nil {
		return ContractFailureCase{}, fmt.Errorf("%s.action is required", path)
	}
	if wire.Detail == nil {
		return ContractFailureCase{}, fmt.Errorf("%s.detail is required", path)
	}
	switch *wire.Action {
	case "stop", "return_partial", "rollback_then_stop", "report_uncertainty":
	default:
		return ContractFailureCase{}, fmt.Errorf("%s.action has unsupported value %q", path, *wire.Action)
	}
	if *wire.Detail == "" {
		return ContractFailureCase{}, fmt.Errorf("%s.detail must not be empty", path)
	}
	return ContractFailureCase{Action: *wire.Action, Detail: *wire.Detail}, nil
}

func normalizeContractProof(wire *contractProofWire, path string) (ContractProof, error) {
	switch {
	case wire.Class == nil:
		return ContractProof{}, fmt.Errorf("%s.class is required", path)
	case wire.Command == nil:
		return ContractProof{}, fmt.Errorf("%s.command is required", path)
	case wire.HarnessRefs == nil:
		return ContractProof{}, fmt.Errorf("%s.harness_refs is required", path)
	case wire.FixtureRefs == nil:
		return ContractProof{}, fmt.Errorf("%s.fixture_refs is required", path)
	}
	switch *wire.Class {
	case "deterministic", "read_only_fixture", "mutating_isolation",
		"strategy_scenarios", "runtime_fault_injection", "routing_corpus",
		"composed_kernel":
	default:
		return ContractProof{}, fmt.Errorf("%s.class has unsupported value %q", path, *wire.Class)
	}
	if *wire.Command == "" || len(*wire.HarnessRefs) == 0 || len(*wire.FixtureRefs) == 0 {
		return ContractProof{}, fmt.Errorf(
			"%s command, harness_refs, and fixture_refs must not be empty",
			path,
		)
	}
	for index, harness := range *wire.HarnessRefs {
		if harness == "" {
			return ContractProof{}, fmt.Errorf("%s.harness_refs[%d] must not be empty", path, index)
		}
	}
	for index, fixture := range *wire.FixtureRefs {
		if fixture == "" {
			return ContractProof{}, fmt.Errorf("%s.fixture_refs[%d] must not be empty", path, index)
		}
	}
	if err := validateUniqueStrings(*wire.HarnessRefs, path+".harness_refs"); err != nil {
		return ContractProof{}, err
	}
	if err := validateUniqueStrings(*wire.FixtureRefs, path+".fixture_refs"); err != nil {
		return ContractProof{}, err
	}
	return ContractProof{
		Class:       *wire.Class,
		Command:     *wire.Command,
		HarnessRefs: *wire.HarnessRefs,
		FixtureRefs: *wire.FixtureRefs,
	}, nil
}
