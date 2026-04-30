//go:build !linux

package agentworker

import (
	"runtime"
	"strings"
)

func applyCgroupV2Limits(_ int, _ CgroupLimits) CgroupStatus {
	return CgroupStatus{Reason: "cgroup v2 caps unsupported on " + runtime.GOOS}
}

func sanitizeCgroupName(value string) string {
	return strings.TrimSpace(value)
}
