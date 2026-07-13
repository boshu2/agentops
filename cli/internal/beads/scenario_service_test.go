package beads

import (
	"strings"
	"testing"
)

type fakeScenarioRepository struct {
	bead        FetchedBead
	description string
}

func (fake *fakeScenarioRepository) Available() bool { return true }
func (fake *fakeScenarioRepository) FetchScenarioBead(string) (FetchedBead, error) {
	return fake.bead, nil
}
func (fake *fakeScenarioRepository) UpdateDescription(_ string, description string) error {
	fake.description = description
	return nil
}

func TestScenarioServicePreservesDescriptionOnApply(t *testing.T) {
	repository := &fakeScenarioRepository{bead: FetchedBead{
		Acceptance:  "Given a state when an action then a result",
		Description: "original context",
	}}
	service := ScenarioService{Repository: repository}
	extraction, err := service.PrepareScenarios("age-x", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyScenarios(extraction); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(repository.description, "original context") || len(repository.description) <= len("original context") {
		t.Fatalf("description = %q", repository.description)
	}
}
