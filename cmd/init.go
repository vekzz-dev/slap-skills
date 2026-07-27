package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
)

// newInitCmd creates the `slap init` command.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up slap with your first skill source",
		Long: `Run the initial setup wizard to add your first skill source.

If sources are already configured, shows a message directing you
to 'slap source add' for additional sources.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

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
		},
	}

	return cmd
}
