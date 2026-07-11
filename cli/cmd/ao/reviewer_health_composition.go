package main

import (
	"time"

	revieweradapter "github.com/boshu2/agentops/cli/internal/adapters/reviewerhealth"
	"github.com/boshu2/agentops/cli/internal/reviewerhealth"
)

const reviewerProbeTimeout = 10 * time.Second

var reviewerHealthService = reviewerhealth.NewService(reviewerhealth.DefaultCatalog(), revieweradapter.SystemProbe())
