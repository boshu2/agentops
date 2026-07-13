// Package done owns the verdict-backed bead-close policy.
package done

import (
	"context"
	"fmt"
	"strings"
)

const (
	DispositionConfirmed  = "CONFIRMED"
	DispositionWaived     = "waived-trivial"
	DispositionUnverified = "UNVERIFIED"
	MinimumSHAPrefix      = 7
)

type Edge struct {
	FromID, FromType, ToID, ToType, Relation, EvidenceRef string
}

type VerdictLookup struct {
	VerdictID    string
	Confirmed    bool
	Dispositions []string
	ForeignBeads []string
}

type Request struct {
	BeadID         string
	SHA            string
	Reason         string
	ForceNoVerdict bool
}

type Result struct {
	BeadID        string `json:"bead_id"`
	CommitSHA     string `json:"commit_sha"`
	Disposition   string `json:"disposition"`
	Stamp         string `json:"stamp"`
	CloseReason   string `json:"close_reason"`
	Closed        bool   `json:"closed"`
	TrackerOutput string `json:"-"`
}

type RepositoryPort interface {
	WorkingDir() (string, error)
	ResolveHead(context.Context, string) (string, error)
	CommitProvenanceOnly(context.Context, string, string) bool
	OriginEdges(context.Context, string) ([]Edge, bool)
}

type LedgerPort interface {
	Read(context.Context) ([]Edge, error)
}

type TrackerPort interface {
	Close(context.Context, string, string) (string, error)
}

type Service struct {
	repository RepositoryPort
	ledger     LedgerPort
	tracker    TrackerPort
}

func NewService(repository RepositoryPort, ledger LedgerPort, tracker TrackerPort) Service {
	return Service{repository: repository, ledger: ledger, tracker: tracker}
}

func (service Service) Execute(ctx context.Context, request Request) (Result, error) {
	cwd, err := service.repository.WorkingDir()
	if err != nil {
		return Result{}, fmt.Errorf("resolve cwd: %w", err)
	}
	sha := strings.TrimSpace(request.SHA)
	if sha == "" {
		sha, err = service.repository.ResolveHead(ctx, cwd)
		if err != nil {
			return Result{}, fmt.Errorf("resolve HEAD at %s (pass --sha to name the landed commit explicitly): %w", cwd, err)
		}
	}
	if len(sha) < MinimumSHAPrefix || !IsHexToken(sha) {
		return Result{}, fmt.Errorf("--sha %q is not a commit sha (need at least %d hex chars)", sha, MinimumSHAPrefix)
	}

	edges, err := service.ledger.Read(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read provenance ledger: %w", err)
	}
	lookup := LookupVerdicts(edges, request.BeadID, sha)
	disposition := ""
	switch {
	case lookup.Confirmed:
		disposition = DispositionConfirmed
	case len(lookup.Dispositions) == 0 && service.repository.CommitProvenanceOnly(ctx, cwd, sha):
		disposition = DispositionWaived
	default:
		if originEdges, ok := service.repository.OriginEdges(ctx, cwd); ok {
			origin := LookupVerdicts(originEdges, request.BeadID, sha)
			if origin.Confirmed {
				lookup.Confirmed = true
				lookup.VerdictID = origin.VerdictID
			} else if len(lookup.Dispositions) == 0 && len(lookup.ForeignBeads) == 0 {
				lookup = origin
			}
		}
		switch {
		case lookup.Confirmed:
			disposition = DispositionConfirmed
		case request.ForceNoVerdict:
			disposition = DispositionUnverified
		default:
			return Result{}, RefusalError(request.BeadID, sha, lookup)
		}
	}

	stamp := Stamp(sha, disposition)
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		reason = "Done"
	}
	closeReason := reason + " " + stamp
	output, err := service.tracker.Close(ctx, request.BeadID, closeReason)
	if err != nil {
		return Result{}, fmt.Errorf("br close %s: %w\n%s", request.BeadID, err, strings.TrimSpace(output))
	}
	return Result{BeadID: request.BeadID, CommitSHA: sha, Disposition: disposition, Stamp: stamp,
		CloseReason: closeReason, Closed: true, TrackerOutput: strings.TrimSpace(output)}, nil
}

func LookupVerdicts(edges []Edge, beadID, sha string) VerdictLookup {
	var lookup VerdictLookup
	for _, edge := range edges {
		if edge.Relation != "wasDerivedFrom" || edge.FromType != "verdict" || edge.ToType != "commit" || !SHABindsCommit(sha, edge.ToID) {
			continue
		}
		if verdictBead, _, ok := strings.Cut(edge.FromID, "@"); !ok || verdictBead != beadID {
			lookup.ForeignBeads = append(lookup.ForeignBeads, edge.FromID)
			continue
		}
		disposition := ParseDisposition(edge.EvidenceRef)
		lookup.Dispositions = append(lookup.Dispositions, disposition)
		if disposition == DispositionConfirmed {
			lookup.Confirmed, lookup.VerdictID = true, edge.FromID
		} else if !lookup.Confirmed {
			lookup.VerdictID = edge.FromID
		}
	}
	return lookup
}

func SHABindsCommit(query, commitID string) bool {
	query, commitID = strings.ToLower(query), strings.ToLower(commitID)
	return len(query) >= MinimumSHAPrefix && len(commitID) >= MinimumSHAPrefix && IsHexToken(query) && IsHexToken(commitID) &&
		(strings.HasPrefix(query, commitID) || strings.HasPrefix(commitID, query))
}

func IsHexToken(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9', character >= 'a' && character <= 'f', character >= 'A' && character <= 'F':
		default:
			return false
		}
	}
	return true
}

func ParseDisposition(reference string) string {
	for _, field := range strings.Fields(reference) {
		if value, ok := strings.CutPrefix(field, "disposition="); ok {
			return value
		}
	}
	return ""
}

func Stamp(sha, disposition string) string {
	return "[verdict:" + sha[:MinimumSHAPrefix] + ":" + disposition + "]"
}

func RefusalError(beadID, sha string, lookup VerdictLookup) error {
	var found string
	switch {
	case len(lookup.Dispositions) > 0:
		found = fmt.Sprintf("verdict(s) recorded for commit %s but none CONFIRMED (found: %s)", sha[:MinimumSHAPrefix], strings.Join(lookup.Dispositions, ", "))
	case len(lookup.ForeignBeads) > 0:
		found = fmt.Sprintf("no verdict for %s on commit %s — the verdict(s) there belong to OTHER bead(s): %s (a verdict certifies its own bead only)", beadID, sha[:MinimumSHAPrefix], strings.Join(lookup.ForeignBeads, ", "))
	default:
		found = fmt.Sprintf("no verdict recorded for commit %s", sha[:MinimumSHAPrefix])
	}
	return fmt.Errorf(`%s — no verdict = not done; refusing to close %s
  produce one:  ao verify %s            (front door — writes the commit-bound verdict on CONFIRMED)
  advanced:     ao pawl review %s --scope head
  waiver:       only a commit whose changed files are all under docs/provenance/ closes as waived-trivial
  stale local?  git fetch origin && git merge --ff-only origin/main   (a verdict pushed elsewhere lands on origin/main first; ao done checks it, but your local ledger may still lag it)
  escape hatch: ao done %s --force-no-verdict   (closes with an explicit UNVERIFIED stamp)`, found, beadID, beadID, beadID, beadID)
}

func ProvenanceOnlyChangedFiles(output string) bool {
	var files []string
	for _, file := range strings.Split(output, "\x00") {
		if file != "" {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return false
	}
	for _, file := range files {
		if !strings.HasPrefix(file, "docs/provenance/") {
			return false
		}
	}
	return true
}
