package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	configapp "github.com/boshu2/agentops/cli/internal/config"
)

// Native reads are bounded and fail on process errors or output truncation.
// In particular, show --include-comments is never a substitute for comments.
func contextBD(ctx context.Context, directory string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	argv := append([]string{"--readonly", "--sandbox", "-C", directory}, args...)
	command := exec.CommandContext(ctx, "bd", argv...)
	command.Dir = directory
	var out limitedContextOutput
	var diagnostic limitedContextOutput
	command.Stdout = &out
	command.Stderr = &diagnostic
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("native bd read failed: %w", err)
	}
	if out.overflow || diagnostic.overflow {
		return nil, fmt.Errorf("native bd response exceeded complete-read limit")
	}
	return out.Bytes(), nil
}

type limitedContextOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedContextOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (16 << 20) - b.Len()
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func (Gateway) NativeContext(ctx context.Context, directory string) (configapp.NativeContext, error) {
	var value configapp.NativeContext
	data, err := contextBD(ctx, directory, "context", "--json")
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}
func (Gateway) AnchorComments(ctx context.Context, directory, anchor string) ([]configapp.AnchorComment, error) {
	// Native show establishes anchor existence; its comments are never requested.
	data, err := contextBD(ctx, directory, "show", anchor, "--json")
	if err != nil {
		return nil, err
	}
	var issues []struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(data, &issues); err != nil {
		return nil, err
	}
	if len(issues) != 1 || issues[0].ID != anchor {
		return nil, fmt.Errorf("native maintenance anchor missing or ambiguous")
	}
	data, err = contextBD(ctx, directory, "comments", anchor, "--json")
	if err != nil {
		return nil, err
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, fmt.Errorf("native comments unavailable")
	}
	var value []configapp.AnchorComment
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}
