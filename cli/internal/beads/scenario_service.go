package beads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boshu2/agentops/cli/internal/scenarios"
)

type FetchedBead struct {
	Acceptance  string
	Description string
}

type ScenarioExtraction struct {
	BeadID        string
	Bead          FetchedBead
	Scenarios     []scenarios.Scenario
	AlreadyShaped bool
}

type ScenarioValidation struct {
	BeadID    string               `json:"bead_id"`
	Valid     bool                 `json:"valid"`
	Scenarios []scenarios.Scenario `json:"scenarios,omitempty"`
	Error     string               `json:"error,omitempty"`
}

type ScenarioRepository interface {
	Available() bool
	FetchScenarioBead(string) (FetchedBead, error)
	UpdateDescription(string, string) error
}

type ScenarioUseCases interface {
	Available() bool
	PrepareScenarios(string, bool) (ScenarioExtraction, error)
	ApplyScenarios(ScenarioExtraction) error
	ValidateScenarios(string) (ScenarioValidation, error)
}

type ScenarioService struct {
	Repository ScenarioRepository
}

func (service ScenarioService) Available() bool {
	return service.Repository != nil && service.Repository.Available()
}

func (service ScenarioService) PrepareScenarios(id string, force bool) (ScenarioExtraction, error) {
	bead, err := service.Repository.FetchScenarioBead(id)
	if err != nil {
		return ScenarioExtraction{}, fmt.Errorf("fetch acceptance for %s: %w (inspect with 'bd show %s --json')", id, err, id)
	}
	extraction := ScenarioExtraction{BeadID: id, Bead: bead}
	if !force && (scenarios.HasScenariosBlock(bead.Description) || scenarios.HasScenariosBlock(bead.Acceptance)) {
		extraction.AlreadyShaped = true
		return extraction, nil
	}
	extraction.Scenarios, err = scenarios.Extract(bead.Acceptance)
	if err != nil {
		return ScenarioExtraction{}, fmt.Errorf("extract scenarios from %s: %w; author a '## Scenarios' block manually (see CLAUDE.md acceptance doctrine)", id, err)
	}
	return extraction, nil
}

func (service ScenarioService) ApplyScenarios(extraction ScenarioExtraction) error {
	rendered := scenarios.Render(extraction.Scenarios)
	description := ComposeDescriptionWithScenarios(extraction.Bead.Description, rendered)
	if err := service.Repository.UpdateDescription(extraction.BeadID, description); err != nil {
		return fmt.Errorf("write '## Scenarios' into %s: %w (inspect with 'bd show %s')", extraction.BeadID, err, extraction.BeadID)
	}
	return nil
}

func (service ScenarioService) ValidateScenarios(id string) (ScenarioValidation, error) {
	bead, err := service.Repository.FetchScenarioBead(id)
	if err != nil {
		return ScenarioValidation{}, fmt.Errorf("fetch bead %s: %w (inspect with 'bd show %s --json')", id, err, id)
	}
	text := bead.Description
	if !scenarios.HasScenariosBlock(text) && scenarios.HasScenariosBlock(bead.Acceptance) {
		text = bead.Acceptance
	}
	parsed, err := scenarios.ParseBlock(text)
	if err != nil {
		return ScenarioValidation{BeadID: id, Error: err.Error()}, fmt.Errorf("validate scenarios for %s: %w; fix the block or regenerate it with 'ao beads scenarios extract %s --force'", id, err, id)
	}
	return ScenarioValidation{BeadID: id, Valid: true, Scenarios: parsed}, nil
}

func ParseScenarioBead(out []byte) (FetchedBead, error) {
	type bead struct {
		AcceptanceCriteria string `json:"acceptance_criteria"`
		Description        string `json:"description"`
	}
	trimmed := bytes.TrimSpace(out)
	var parsed bead
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var records []bead
		if err := json.Unmarshal(trimmed, &records); err != nil {
			return FetchedBead{}, fmt.Errorf("parse bd json array: %w", err)
		}
		if len(records) == 0 {
			return FetchedBead{}, fmt.Errorf("bead not found")
		}
		parsed = records[0]
	} else if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return FetchedBead{}, fmt.Errorf("parse bd json: %w", err)
	}
	description := strings.TrimSpace(parsed.Description)
	acceptance := strings.TrimSpace(parsed.AcceptanceCriteria)
	if acceptance == "" {
		acceptance = description
	}
	if acceptance == "" {
		return FetchedBead{}, fmt.Errorf("bead has no acceptance_criteria or description text")
	}
	return FetchedBead{Acceptance: acceptance, Description: description}, nil
}

func ComposeDescriptionWithScenarios(description, rendered string) string {
	block := strings.TrimRight(rendered, "\n")
	base := strings.TrimRight(description, "\n")
	if base == "" {
		return block + "\n"
	}
	return base + "\n\n" + block + "\n"
}

var _ ScenarioUseCases = ScenarioService{}
