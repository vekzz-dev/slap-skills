package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
)

// newInitCmd creates the `slap init <repo-url>` command.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <repo-url>",
		Short: "Configure a git repo as the skill source",
		Long: `Configure a git repo as the source for AI agent skills.

Validates that the repo is reachable (ls-remote), then writes the
configuration to ~/.config/slap/config.yaml so that 'slap sync'
knows where to pull skills from.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]

			// Read flags that were passed by the user (falls back to persistent
			// flag defaults when not explicitly provided).
			branch, _ := cmd.Flags().GetString("branch")
			targetDir, _ := cmd.Flags().GetString("target-dir")

			// Validate the repo is reachable.
			if err := config.ValidateRepoAccess(repoURL, branch); err != nil {
				return fmt.Errorf("repo validation failed: %w", err)
			}

			// Build config and persist it.
			cfg := &config.Config{
				RepoURL:   repoURL,
				Branch:    branch,
				TargetDir: targetDir,
			}
			if err := cfg.Save(config.ConfigFile); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Println("Slap configured! Run 'slap sync' to install your skills.")
			return nil
		},
	}

	return cmd
}
