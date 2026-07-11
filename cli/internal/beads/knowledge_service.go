package beads

import (
	"context"
	"fmt"
	"path/filepath"
)

type KnowledgeRepository interface {
	VerifyCitations([]Citation) []Citation
	CreateLearning(string, []byte) (bool, error)
}

type HarvestResult struct {
	Body          string
	Target        string
	AlreadyExists bool
}

type KnowledgeUseCases interface {
	Available() bool
	Verify(context.Context, string) (*VerifyReport, error)
	Lint(context.Context, string) (*LintReport, error)
	Harvest(context.Context, string, string, bool) (HarvestResult, error)
}

type KnowledgeService struct {
	Tracker    TrackerClient
	Repository KnowledgeRepository
	Clock      Clock
}

func (service KnowledgeService) Available() bool {
	return service.Tracker != nil && service.Tracker.Available()
}

func (service KnowledgeService) Verify(ctx context.Context, beadID string) (*VerifyReport, error) {
	if !service.Available() {
		return &VerifyReport{BeadID: beadID, BDAvailable: false}, nil
	}
	raw, err := service.Tracker.Output(ctx, "show", beadID)
	if err != nil {
		return nil, fmt.Errorf("bd show %s: %w", beadID, err)
	}
	parsed, err := ParseBDShow(string(raw))
	if err != nil {
		return nil, err
	}
	citations := ExtractCitations(parsed.Body())
	if service.Repository != nil {
		citations = service.Repository.VerifyCitations(citations)
	}
	report := &VerifyReport{
		BeadID: beadID, Title: parsed.Title, Status: parsed.Status,
		Citations: citations, TotalCount: len(citations), BDAvailable: true,
	}
	for _, citation := range citations {
		switch citation.Status {
		case CitationFresh:
			report.FreshCount++
		case CitationStale:
			report.StaleCount++
		}
	}
	return report, nil
}

func (service KnowledgeService) Lint(ctx context.Context, status string) (*LintReport, error) {
	if !service.Available() {
		return &LintReport{StatusFilter: status}, nil
	}
	raw, err := service.Tracker.Output(ctx, "list", "--status", status)
	if err != nil {
		return nil, fmt.Errorf("bd list: %w", err)
	}
	ids := ParseBeadIDs(raw)
	report := &LintReport{StatusFilter: status, TotalBeads: len(ids)}
	for _, id := range ids {
		verified, err := service.Verify(ctx, id)
		if err != nil {
			report.ErrorBeads++
			continue
		}
		report.PerBead = append(report.PerBead, *verified)
		if verified.StaleCount > 0 {
			report.StaleBeads++
		} else {
			report.CleanBeads++
		}
	}
	return report, nil
}

func (service KnowledgeService) Harvest(ctx context.Context, beadID, outputDirectory string, dryRun bool) (HarvestResult, error) {
	if !service.Available() {
		return HarvestResult{}, nil
	}
	raw, err := service.Tracker.Output(ctx, "show", beadID)
	if err != nil {
		return HarvestResult{}, fmt.Errorf("bd show %s: %w", beadID, err)
	}
	parsed, err := ParseBDShow(string(raw))
	if err != nil {
		return HarvestResult{}, err
	}
	if !IsClosedStatus(parsed.Status) {
		return HarvestResult{}, fmt.Errorf("bead %s is not CLOSED (status=%q) — harvest only materialises closed beads", beadID, parsed.Status)
	}
	if service.Clock == nil {
		return HarvestResult{}, fmt.Errorf("beads harvest clock is not configured")
	}
	frontmatter := LearningFrontmatter{
		Title: parsed.Title, BeadID: beadID, Source: "bd-close",
		Date: service.Clock.Now().UTC().Format("2006-01-02"),
		Tags: []string{"bead-closure", "auto-harvested"}, Maturity: "provisional",
		Provenance: fmt.Sprintf("bd show %s (harvested via `ao beads harvest`)", beadID),
	}
	body := RenderLearningBody(frontmatter, parsed)
	result := HarvestResult{Body: body}
	if dryRun {
		return result, nil
	}
	result.Target = filepath.Join(outputDirectory, fmt.Sprintf("%s-%s-%s.md", frontmatter.Date, beadID, Slugify(parsed.Title, 40)))
	if service.Repository == nil {
		return HarvestResult{}, fmt.Errorf("beads harvest repository is not configured")
	}
	created, err := service.Repository.CreateLearning(result.Target, []byte(body))
	if err != nil {
		return HarvestResult{}, err
	}
	result.AlreadyExists = !created
	return result, nil
}

var _ KnowledgeUseCases = KnowledgeService{}
