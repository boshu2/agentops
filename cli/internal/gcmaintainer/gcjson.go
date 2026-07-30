package gcmaintainer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// gcOutput runs the gc binary with args and returns its stdout, failing with
// the caller's description when the command exits non-zero. gc's own stderr
// passes through to the operation's stderr.
func (o *ops) gcOutput(description string, args ...string) ([]byte, error) {
	cmd := exec.Command(o.gcBin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = o.stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", description)
	}
	return out.Bytes(), nil
}

// gcJSON runs gc and decodes its stdout into payload. A command failure fails
// with the description; undecodable or trivially false output (null/false,
// matching the shell port's `jq -e .` acceptance) is reported as malformed.
func (o *ops) gcJSON(payload any, description string, args ...string) error {
	out, err := o.gcOutput(description, args...)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "null" || trimmed == "false" || !json.Valid([]byte(trimmed)) {
		return fmt.Errorf("%s returned malformed JSON", description)
	}
	if err := json.Unmarshal([]byte(trimmed), payload); err != nil {
		return fmt.Errorf("%s returned malformed JSON", description)
	}
	return nil
}
