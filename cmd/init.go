package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
)

// newInitCmd creates the `slap init <repo-url>` command.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <repo-url> [--alias name]",
		Short: "Add a git repo as a skill source",
		Long: `Configure a git repo as a skill source using the multi-source system.

Validates that the repo is reachable (ls-remote), then creates a source
configuration under ~/.config/slap/sources/<alias>.yaml so that 'slap sync'
knows where to pull skills from. This is equivalent to 'slap source add'.

If --alias is not provided, defaults to "default".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]

			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

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
