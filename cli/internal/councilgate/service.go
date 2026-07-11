// Package councilgate owns fail-closed verdict aggregation policy.
package councilgate

import (
	"context"
	"io"

	"github.com/boshu2/agentops/cli/internal/liveness"
	"github.com/boshu2/agentops/cli/internal/verdict"
)

const (
	ExitCouncil  = 6
	ExitDisagree = 8
)

type Outcome uint8

const (
	OutcomePass Outcome = iota
	OutcomeUnverified
	OutcomeDuplicateContext
	OutcomeDuplicateJudge
	OutcomeCrossFamily
	OutcomeAllFail
	OutcomeDisagreement
)

type Request struct {
	Paths []string
	Stdin io.Reader
}

type Result struct {
	Outcome    Outcome
	Total      int
	Pass       int
	Fail       int
	Unverified int
	Contexts   int
	Families   int
	Duplicate  string
}

type Policy struct {
	RequireCrossFamily bool
}

type Reader interface {
	Read(context.Context, string, io.Reader) (string, error)
}

type Service struct {
	reader Reader
	policy Policy
}

func NewService(reader Reader, policy Policy) Service {
	return Service{reader: reader, policy: policy}
}

func (service Service) Evaluate(ctx context.Context, request Request) Result {
	result := Result{Total: len(request.Paths)}
	families := map[string]bool{}
	judges := map[string]bool{}
	contexts := map[string]bool{}
	for _, path := range request.Paths {
		text, err := service.reader.Read(ctx, path, request.Stdin)
		identity, gaps := verdict.Identity(text)
		if err != nil || !verdict.HasCommandsRun(text) || len(gaps) > 0 {
			result.Unverified++
			continue
		}
		canonicalContext := liveness.CanonicalizeContextID(identity.ContextID)
		if contexts[canonicalContext] {
			result.Outcome = OutcomeDuplicateContext
			result.Duplicate = identity.ContextID
			return result
		}
		contexts[canonicalContext] = true
		if judges[identity.JudgeName] {
			result.Outcome = OutcomeDuplicateJudge
			result.Duplicate = identity.JudgeName
			return result
		}
		judges[identity.JudgeName] = true
		pass, fail := verdict.TokenCounts(text)
		switch {
		case pass == 1 && fail == 0:
			result.Pass++
			families[identity.JudgeModelFamily] = true
		case fail == 1 && pass == 0:
			result.Fail++
		default:
			result.Unverified++
		}
	}
	result.Contexts = len(contexts)
	result.Families = len(families)
	switch {
	case result.Unverified > 0:
		result.Outcome = OutcomeUnverified
	case result.Pass == result.Total && result.Contexts < 2:
		result.Outcome = OutcomeDuplicateContext
	case result.Pass == result.Total && service.policy.RequireCrossFamily && result.Families < 2:
		result.Outcome = OutcomeCrossFamily
	case result.Pass == result.Total:
		result.Outcome = OutcomePass
	case result.Fail == result.Total:
		result.Outcome = OutcomeAllFail
	default:
		result.Outcome = OutcomeDisagreement
	}
	return result
}
