package config

import (
	"os"
	"path/filepath"

	configapp "github.com/boshu2/agentops/cli/internal/config"
)

type Gateway struct{}

func (Gateway) Resolve(output string, verbose bool) *configapp.ResolvedConfig {
	return configapp.Resolve(output, "", verbose)
}

func (Gateway) Files() (configapp.ConfigFiles, error) {
	home := filepath.Join(os.Getenv("HOME"), ".agents", "ao", "config.yaml")
	cwd, err := os.Getwd()
	if err != nil {
		return configapp.ConfigFiles{}, err
	}
	project := filepath.Join(cwd, ".agents", "ao", "config.yaml")
	return configapp.ConfigFiles{
		HomePath: home, HomeExists: exists(home), ProjectPath: project, ProjectExists: exists(project),
	}, nil
}

func (Gateway) Environment(keys []string) map[string]string {
	values := make(map[string]string)
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			values[key] = value
		}
	}
	return values
}

func (Gateway) Load() (*configapp.Config, error)   { return configapp.Load(nil) }
func (Gateway) Save(value *configapp.Config) error { return configapp.Save(value) }
func (Gateway) PreviewSave(value *configapp.Config) error {
	return configapp.PreviewSave(value)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
