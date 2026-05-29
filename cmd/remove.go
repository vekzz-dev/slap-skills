package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

func newRemoveCmd() *cobra.Command {
	var removeAll bool

	cmd := &cobra.Command{
		Use:   "remove [skill-name]",
		Short: "Remove installed skills",
		Long: `Remove one or more installed skills by name.

Use --all to remove every skill installed by Slap and clean up the manifest.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			manifestPath := expandPath(config.ManifestFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			cfg, err := config.Load(expandPath(config.ConfigFile))
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			targetDir := expandPath(cfg.TargetDir)

			if removeAll {
				count := len(m.Skills)
				if count == 0 {
					fmt.Println("No skills installed.")
					return nil
				}
				for name := range m.Skills {
					sp := filepath.Join(targetDir, name)
					os.RemoveAll(sp)
					m.RemoveSkill(name)
					fmt.Printf("  - %s\n", name)
				}
				if err := m.Save(manifestPath); err != nil {
					return fmt.Errorf("saving manifest: %w", err)
				}
				fmt.Printf("\nRemoved %d skill(s). Manifest cleaned.\n", count)
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("usage: slap remove <skill-name> or slap remove --all")
			}

			name := args[0]
			if !m.HasSkill(name) {
				return fmt.Errorf("skill %q is not installed", name)
			}

			sp := filepath.Join(targetDir, name)
			os.RemoveAll(sp)
			m.RemoveSkill(name)

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("saving manifest: %w", err)
			}
			fmt.Printf("Removed %q.\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&removeAll, "all", false, "Remove all installed skills")
	return cmd
}
