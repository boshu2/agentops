//go:build linux

package agentworker

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultCgroupV2Root = "/sys/fs/cgroup"

func applyCgroupV2Limits(pid int, limits CgroupLimits) CgroupStatus {
	if pid <= 0 {
		return CgroupStatus{Reason: "worker pid unavailable"}
	}
	if limits.MemoryMaxBytes <= 0 {
		return CgroupStatus{Reason: "memory cap disabled"}
	}
	root := strings.TrimSpace(limits.Root)
	if root == "" {
		root = defaultCgroupV2Root
		if _, err := os.Stat(filepath.Join(root, "cgroup.controllers")); err != nil {
			return CgroupStatus{Reason: "cgroup v2 unavailable: " + err.Error()}
		}
	}
	name := sanitizeCgroupName(limits.Name)
	if name == "" {
		name = fmt.Sprintf("agentops-worker-%d", pid)
	}
	path := filepath.Join(root, "agentops", name)
	if err := os.MkdirAll(path, 0755); err != nil {
		return CgroupStatus{Path: path, Reason: "create cgroup: " + err.Error()}
	}
	if err := os.WriteFile(filepath.Join(path, "memory.max"), []byte(strconv.FormatInt(limits.MemoryMaxBytes, 10)), 0644); err != nil {
		return CgroupStatus{Path: path, Reason: "set memory.max: " + err.Error()}
	}
	if err := os.WriteFile(filepath.Join(path, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return CgroupStatus{Path: path, Reason: "attach worker: " + err.Error()}
	}
	return CgroupStatus{Applied: true, Path: path}
}

func sanitizeCgroupName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
