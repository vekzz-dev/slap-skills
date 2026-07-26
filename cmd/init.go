package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
)

// newInitCmd creates the `slap init` command.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [<repo-url>]",
		Short: "Set up slap with a skill source",
		Long: `Set up slap by adding a skill source repository.

If run without arguments, checks whether any sources are already configured:
  - If none exist, launches the interactive 'slap source add' wizard.
  - If sources exist, lists them.

If a URL is provided, adds it as a source (equivalent to 'slap source add').`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			// No args → orchestrator mode.
			if len(args) == 0 {
				aliases, err := config.ListSources()
				if err == nil && len(aliases) > 0 {
					fmt.Println("Slap is already configured. Use 'slap source add' to add more sources.")
					return nil
				}

				// No sources — launch interactive add.
				fmt.Println("No sources configured. Starting the setup wizard...")
				sourceRoot := NewRootCmd()
				sourceRoot.SetArgs([]string{"source", "add"})
				return sourceRoot.Execute()
			}

			// URL provided — add source directly.
			repoURL := args[0]

			// Read flags.
			branch, _ := cmd.Flags().GetString("branch")
			targetDir, _ := cmd.Flags().GetString("target-dir")
			alias, _ := cmd.Flags().GetString("alias")
			if alias == "" {
				alias = "default"
			}

			// Validate the repo is reachable.
			if err := config.ValidateRepoAccess(repoURL, branch); err != nil {
				return fmt.Errorf("repo validation failed: %w", err)
			}

			// Check alias uniqueness.
			existing, err := config.ListSources()
			if err == nil {
				for _, a := range existing {
					if a == alias {
						return fmt.Errorf("source alias %q already exists. Use 'slap source add' with a different alias", alias)
					}
				}
			}

			// Create source file.
			sc := config.SourceConfig{
				Alias:  alias,
				URL:    repoURL,
				Branch: branch,
			}
			if err := config.CreateSource(alias, sc); err != nil {
				return fmt.Errorf("saving source: %w", err)
			}

			// Ensure config.yaml exists with this source in its list.
			cfg, loadErr := config.Load(config.ConfigFile)
			if loadErr != nil {
				cfg = &config.Config{TargetDir: "~/.config/opencode/skills"}
			}
			found := false
			for _, s := range cfg.Sources {
				if s == alias {
					found = true
					break
				}
			}
			if !found {
				cfg.Sources = append(cfg.Sources, alias)
			}
			if targetDir != "" {
				cfg.TargetDir = targetDir
			}
			if err := cfg.Save(config.ConfigFile); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Source %q added! Run 'slap sync' to install your skills.\n", alias)
			return nil
		},
	}

	cmd.Flags().String("alias", "", "Alias for the source (defaults to \"default\")")
	return cmd
}
