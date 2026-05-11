// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"
)

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		name     string
		goal     string
		expected ComplexityLevel
	}{
		{
			name:     "typo fix is fast",
			goal:     "fix typo in README",
			expected: ComplexityFast,
		},
		{
			name:     "single word is fast",
			goal:     "fix",
			expected: ComplexityFast,
		},
		{
			name:     "add small feature is fast",
			goal:     "add --verbose flag",
			expected: ComplexityFast,
		},
		{
			name:     "bump version is fast",
			goal:     "bump version to v2.5",
			expected: ComplexityFast,
		},
		{
			name:     "update config is fast",
			goal:     "update default timeout",
			expected: ComplexityFast,
		},
		{
			name:     "refactor is full",
			goal:     "refactor authentication module",
			expected: ComplexityFull,
		},
		{
			name:     "migrate is full",
			goal:     "migrate database from postgres to sqlite",
			expected: ComplexityFull,
		},
		{
			name:     "rewrite is full",
			goal:     "rewrite the CLI parser",
			expected: ComplexityFull,
		},
		{
			name:     "migration keyword is full",
			goal:     "data migration for user table schema change",
			expected: ComplexityFull,
		},
		{
			name:     "redesign is full",
			goal:     "redesign the plugin system",
			expected: ComplexityFull,
		},
		{
			name:     "add user auth is standard",
			goal:     "add user authentication to the API",
			expected: ComplexityStandard,
		},
		{
			name:     "long description without complex keywords is standard",
			goal:     "add support for custom timeout configuration in the HTTP client options struct",
			expected: ComplexityStandard,
		},
		{
			name:     "global scope keyword is standard",
			goal:     "update global config defaults",
			expected: ComplexityStandard,
		},
		{
			name:     "multi-file scope is full",
			goal:     "update logging across all modules in the system",
			expected: ComplexityFull,
		},
		{
			name:     "very long goal is full",
			goal:     "implement a comprehensive rate limiting system that handles per-user and per-IP limits with configurable windows, burst allowances, and storage backends for distributed deployments",
			expected: ComplexityFull,
		},
		{
			name:     "empty string is fast",
			goal:     "",
			expected: ComplexityFast,
		},
		{
			name:     "whitespace only is fast",
			goal:     "   ",
			expected: ComplexityFast,
		},
		{
			name:     "codebase-wide change is full",
			goal:     "fix error handling throughout the codebase",
			expected: ComplexityFull,
		},
		{
			name:     "decouple is full",
			goal:     "decouple the storage layer from business logic",
			expected: ComplexityFull,
		},
		{
			name:     "rename is fast when short",
			goal:     "rename config field",
			expected: ComplexityFast,
		},
		{
			name:     "add feature moderate length is standard",
			goal:     "add pagination to the list endpoints with cursor-based approach",
			expected: ComplexityStandard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyComplexity(tt.goal)
			if got != tt.expected {
				t.Errorf("classifyComplexity(%q) = %q, want %q", tt.goal, got, tt.expected)
			}
		})
	}
}

func TestScoreGoal(t *testing.T) {
	tests := []struct {
		name            string
		goal            string
		wantComplex     bool // complexKeywords > 0
		wantScopeGlobal bool // scopeKeywords > 0
	}{
		{
			name:            "refactor triggers complex keyword",
			goal:            "refactor the parser",
			wantComplex:     true,
			wantScopeGlobal: false,
		},
		{
			name:            "global triggers scope keyword",
			goal:            "update global settings",
			wantComplex:     false,
			wantScopeGlobal: true,
		},
		{
			name:            "plain fix triggers neither",
			goal:            "fix the bug",
			wantComplex:     false,
			wantScopeGlobal: false,
		},
		{
			name:            "migrate triggers complex",
			goal:            "migrate to new API",
			wantComplex:     true,
			wantScopeGlobal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scoreGoal(tt.goal)
			gotComplex := s.ComplexKeywords > 0
			gotScope := s.ScopeKeywords > 0
			if gotComplex != tt.wantComplex {
				t.Errorf("scoreGoal(%q).ComplexKeywords > 0 = %v, want %v", tt.goal, gotComplex, tt.wantComplex)
			}
			if gotScope != tt.wantScopeGlobal {
				t.Errorf("scoreGoal(%q).ScopeKeywords > 0 = %v, want %v", tt.goal, gotScope, tt.wantScopeGlobal)
			}
		})
	}
}

func TestComplexityLevelConstants(t *testing.T) {
	levels := []ComplexityLevel{ComplexityFast, ComplexityStandard, ComplexityFull}
	expected := []string{"fast", "standard", "full"}
	for i, l := range levels {
		if string(l) != expected[i] {
			t.Errorf("ComplexityLevel[%d] = %q, want %q", i, l, expected[i])
		}
	}
}
