package beads

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeHygieneRepository struct {
	list       map[string][]BeadRecord
	shown      map[string]BeadRecord
	commits    []AuditCommit
	closeErr   error
	closed     []string
	reparented []string
}

func (fake *fakeHygieneRepository) Available() bool { return true }
func (fake *fakeHygieneRepository) List(status string) ([]BeadRecord, error) {
	return fake.list[status], nil
}
func (fake *fakeHygieneRepository) Show(id string) (BeadRecord, error) { return fake.shown[id], nil }
func (fake *fakeHygieneRepository) Commits() []AuditCommit             { return fake.commits }
func (fake *fakeHygieneRepository) PatternExists(string) bool          { return true }
func (fake *fakeHygieneRepository) Close(id, _ string) error {
	fake.closed = append(fake.closed, id)
	return fake.closeErr
}
func (fake *fakeHygieneRepository) Reparent(id, parent string) error {
	fake.reparented = append(fake.reparented, id+"->"+parent)
	return nil
}

func TestHygieneServicePreservesMutationEvidence(t *testing.T) {
	repository := &fakeHygieneRepository{
		list:     map[string][]BeadRecord{"open": {{ID: "age-x", Title: "fixed"}}},
		commits:  []AuditCommit{{ShortSHA: "abc123", Subject: "finish age-x", CommitAt: time.Now()}},
		closeErr: errors.New("tracker refused close"),
	}
	report, err := (HygieneService{Repository: repository}).Audit(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.closed) != 1 || len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "tracker refused close") {
		t.Fatalf("closed=%v warnings=%v", repository.closed, report.Warnings)
	}
}

func TestHygieneServiceClustersAndReparents(t *testing.T) {
	repository := &fakeHygieneRepository{
		list: map[string][]BeadRecord{"open": {
			{ID: "age-e", Title: "tracker cleanup", IssueType: "epic", Labels: []string{"tracker"}},
			{ID: "age-x", Title: "tracker cleanup", Labels: []string{"tracker"}},
		}},
		shown: map[string]BeadRecord{},
	}
	report, err := (HygieneService{Repository: repository}).Cluster(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Clusters) != 1 || report.Clusters[0].Representative != "age-e" || len(repository.reparented) != 1 {
		t.Fatalf("report=%+v reparented=%v", report, repository.reparented)
	}
}
