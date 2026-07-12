package main

import (
	"context"

	evaladapter "github.com/boshu2/agentops/cli/internal/adapters/eval"
)

const judgeOutputSchema = evaladapter.JudgeOutputSchema

func runCodexExec(ctx context.Context, prompt, outputSchemaPath string) (string, int, error) {
	return evaladapter.RunCodexExec(ctx, prompt, outputSchemaPath)
}
