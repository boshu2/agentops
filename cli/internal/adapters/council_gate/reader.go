// Package council_gate contains driven adapters for council verdict artifacts.
package council_gate

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type Reader struct {
	WorkDir          string
	WorkingDirectory func() (string, error)
}

func (reader Reader) Read(ctx context.Context, path string, stdin io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		if !filepath.IsAbs(path) {
			workDir := reader.WorkDir
			if workDir == "" {
				resolve := reader.WorkingDirectory
				if resolve == nil {
					resolve = os.Getwd
				}
				workDir, err = resolve()
				if err != nil {
					return "", err
				}
			}
			path = filepath.Join(workDir, path)
		}
		data, err = os.ReadFile(path)
	}
	return string(data), err
}
