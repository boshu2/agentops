package main

import (
	"os"

	configapp "github.com/boshu2/agentops/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	configShow      bool
	modelsSetTier   string
	modelsSetSkill  string
	configCmd       = configCommand
	configModelsCmd = func() *cobra.Command {
		command, _, err := configCommand.Find([]string{"models"})
		if err != nil {
			panic(err)
		}
		return command
	}()
)

type configModelsWriteResult = configapp.ModelsWriteResult

func runConfig(command *cobra.Command, args []string) error {
	fresh := configModule.Command()
	if configShow {
		_ = fresh.Flags().Set("show", "true")
	}
	fresh.SetOut(os.Stdout)
	return fresh.RunE(command, args)
}

func runConfigModels(command *cobra.Command, args []string) error {
	fresh := configModule.Command()
	models, _, err := fresh.Find([]string{"models"})
	if err != nil {
		return err
	}
	if modelsSetTier != "" {
		_ = models.Flags().Set("set-tier", modelsSetTier)
	}
	if modelsSetSkill != "" {
		_ = models.Flags().Set("set-skill", modelsSetSkill)
	}
	models.SetOut(os.Stdout)
	return models.RunE(command, args)
}

func handleModelsWrite() error {
	return runConfigModels(&cobra.Command{}, nil)
}
