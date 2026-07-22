package delivery

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const nativePRListLimit = 100

type nativePRRecord struct {
	NodeID      string `json:"id"`
	Number      int    `json:"number"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Draft       bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	HeadRefOID  string `json:"headRefOid"`
	BaseRefOID  string `json:"baseRefOid"`
	Body        string `json:"body"`
}

func prEffectMarker(effectID string) string {
	return "<!-- agentops-gc-delivery-effect:" + effectID + " -->"
}

func nativePRBody(intent PRIntent) string {
	return prEffectMarker(intent.EffectID) + "\n\nManaged by the AgentOps GC delivery reducer."
}

func (p *NativeProviders) ObservePR(ctx context.Context, intent PRIntent) (PRObservation, error) {
	if err := validateNativePRIntent(intent); err != nil {
		return PRObservation{}, err
	}
	output, err := p.gh(ctx,
		"pr", "list", "--repo", intent.Repository, "--state", "all",
		"--head", intent.Branch, "--limit", strconv.Itoa(nativePRListLimit),
		"--json", "id,number,url,state,isDraft,headRefName,baseRefName,headRefOid,baseRefOid,body",
	)
	if err != nil {
		return PRObservation{}, fmt.Errorf("observe native PR: %w", err)
	}
	var records []nativePRRecord
	if err := decodeStrict(output, &records); err != nil {
		return PRObservation{}, errors.New("native PR observation is not strict JSON")
	}
	if len(records) >= nativePRListLimit {
		return PRObservation{State: "ambiguous"}, nil
	}
	exact := make([]nativePRRecord, 0, 1)
	conflicting := false
	marker := prEffectMarker(intent.EffectID)
	for _, record := range records {
		if record.NodeID == "" || record.Number < 1 || record.URL == "" || record.HeadRefName == "" || record.BaseRefName == "" {
			return PRObservation{}, errors.New("native PR observation has incomplete identity")
		}
		if record.HeadRefName != intent.Branch || record.BaseRefName != intent.BaseRef || !bodyHasExactMarker(record.Body, marker) {
			conflicting = true
			continue
		}
		exact = append(exact, record)
	}
	if len(exact) > 1 || (len(exact) == 1 && conflicting) {
		return PRObservation{State: "ambiguous"}, nil
	}
	if len(exact) == 0 {
		if conflicting {
			return PRObservation{State: "conflicting"}, nil
		}
		return PRObservation{State: "absent"}, nil
	}
	record := exact[0]
	state, err := nativePRState(record.State)
	if err != nil {
		return PRObservation{}, err
	}
	return PRObservation{
		State: state, Draft: record.Draft, BaseOID: record.BaseRefOID, Head: record.HeadRefOID,
		PR: PullRequest{
			ID: intent.PRID, EffectID: intent.EffectID, Repository: intent.Repository,
			BaseRef: record.BaseRefName, Branch: record.HeadRefName, NodeID: record.NodeID,
			Number: strconv.Itoa(record.Number), URL: record.URL,
		},
	}, nil
}

func (p *NativeProviders) CreatePR(ctx context.Context, intent PRIntent) (PRObservation, error) {
	if err := validateNativePRIntent(intent); err != nil {
		return PRObservation{}, err
	}
	if intent.NodeID != "" || intent.Number != "" || intent.URL != "" {
		return PRObservation{}, errors.New("native PR create cannot replace a known actual PR")
	}
	base, err := p.ObserveBase(ctx, intent.BaseRef)
	if err != nil || base != intent.BaseOID {
		return PRObservation{}, errors.New("native PR create base moved before effect")
	}
	observed, err := p.ObservePR(ctx, intent)
	if err != nil {
		return PRObservation{}, err
	}
	if observed.State == "open" {
		return observed, nil
	}
	if observed.State != "absent" {
		return PRObservation{}, errors.New("native PR create found conflicting prior PR state")
	}
	title := "AgentOps delivery " + intent.HandoffID[:12]
	if _, err := p.gh(ctx,
		"pr", "create", "--repo", intent.Repository, "--base", intent.BaseRef,
		"--head", intent.Branch, "--title", title, "--body", nativePRBody(intent),
	); err != nil {
		return PRObservation{}, fmt.Errorf("create native PR: %w", err)
	}
	created, err := p.ObservePR(ctx, intent)
	if err != nil {
		return PRObservation{}, err
	}
	if created.State != "open" || !matchesPRObservation(created, intent) {
		return PRObservation{}, errors.New("native PR create was not observed with exact identity")
	}
	return created, nil
}

func validateNativePRIntent(intent PRIntent) error {
	if intent.SchemaVersion != "pr-intent.v1" || intent.HandoffID == "" || intent.Epoch < 1 || intent.Repository == "" || intent.BaseRef == "" || !isHex(intent.BaseOID, 40) || intent.Branch == "" || !isHex(intent.ExpectedHead, 40) || intent.PRID == "" || !isHex(intent.EffectID, 64) {
		return errors.New("native PR intent has incomplete exact identity")
	}
	if !strings.HasPrefix(intent.Branch, "gc/delivery/") {
		return errors.New("native PR intent branch is outside delivery namespace")
	}
	return nil
}

func bodyHasExactMarker(body, marker string) bool {
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if line == marker {
			count++
		}
	}
	return count == 1
}

func nativePRState(value string) (string, error) {
	switch strings.ToUpper(value) {
	case "OPEN":
		return "open", nil
	case "CLOSED":
		return "closed", nil
	case "MERGED":
		return "merged", nil
	default:
		return "", errors.New("native PR observation has unsupported state")
	}
}
