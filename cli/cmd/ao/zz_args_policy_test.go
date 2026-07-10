package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompatibleArgsPolicyPreservesUseGrammar(t *testing.T) {
	tests := []struct {
		use      string
		accepted []int
		rejected []int
	}{
		{use: "status", accepted: []int{0}, rejected: []int{1}},
		{use: "show <id>", accepted: []int{1}, rejected: []int{0, 2}},
		{use: "close <id> [paths...]", accepted: []int{1, 3}, rejected: []int{0}},
	}
	for _, test := range tests {
		policy := compatibleArgsPolicy(test.use)
		command := &cobra.Command{Use: test.use}
		for _, count := range test.accepted {
			if err := policy(command, make([]string, count)); err != nil {
				t.Errorf("%s rejected %d args: %v", test.use, count, err)
			}
		}
		for _, count := range test.rejected {
			if err := policy(command, make([]string, count)); err == nil {
				t.Errorf("%s accepted %d args", test.use, count)
			}
		}
	}
}
