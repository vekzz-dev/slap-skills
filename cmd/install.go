package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

func newInstallCmd() *cobra.Command {
	var installAll bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Select and install skills from the repo",
		Long: `List available skills from the configured repo and install the ones you choose.

Use --all to install every skill from the repo without prompting.`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cfg, err := config.Load(expandPath(config.ConfigFile))
			if err != nil {
				return fmt.Errorf("slap is not configured. Run 'slap init <repo-url>' first")
			}
			cfg.ApplyFlagOverrides(flagRepo, flagBranch, flagTargetDir)

			targetDir := expandPath(cfg.TargetDir)
			manifestPath := expandPath(config.ManifestFile)

			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			// Clone repo to temp dir
			tmpDir, err := os.MkdirTemp("", "slap-install-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			client := &repo.Client{URL: cfg.RepoURL, Branch: cfg.Branch}
			if err := client.CloneShallow(cobraCmd.Context(), tmpDir); err != nil {
				return fmt.Errorf("cloning repo: %w", err)
			}

			repoSkills, err := client.ListSkillDirs(cobraCmd.Context(), tmpDir)
			if err != nil {
				return fmt.Errorf("listing skills: %w", err)
			}

			// Convert repo tree SHAs to local format
			for i := range repoSkills {
				sp := filepath.Join(tmpDir, repoSkills[i].Name)
				if sha, computeErr := repo.ComputeLocalTreeSHA(sp); computeErr == nil {
					repoSkills[i].TreeSHA = sha
				}
			}

			// Filter: only skills not already installed
			var available []string
			for _, s := range repoSkills {
				if !m.HasSkill(s.Name) {
					available = append(available, s.Name)
				}
			}

			if len(available) == 0 {
				fmt.Println("All skills from the repo are already installed.")
				return nil
			}

			// Sort alphabetically
			sort.Strings(available)

			var selected []string

			if !installAll {
				// Interactive multi-select with arrow keys, space, enter
				prompt := &survey.MultiSelect{
					Message: "Select skills to install:",
					Options: available,
					Description: func(value string, index int) string {
						return ""
					},
				}
				if err := survey.AskOne(prompt, &selected, survey.WithPageSize(20)); err != nil {
					return err
				}
				if len(selected) == 0 {
					fmt.Println("No skills selected.")
					return nil
				}
			} else {
				selected = available
			}

			// Build a lookup by name
			skillMap := make(map[string]repo.SkillDir, len(repoSkills))
			for _, s := range repoSkills {
				skillMap[s.Name] = s
			}

			// Install each selected skill
			for _, name := range selected {
				s := skillMap[name]
				src := filepath.Join(tmpDir, s.Name)
				dst := filepath.Join(targetDir, s.Name)
				if err := copyDir(src, dst); err != nil {
					return fmt.Errorf("installing %s: %w", s.Name, err)
				}
				localSHA := computeLocalSHA(dst)
				m.UpsertSkill(s.Name, localSHA)
				fmt.Printf("  + %s\n", s.Name)
			}

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("saving manifest: %w", err)
			}

			if installAll {
				fmt.Printf("\nInstalled %d skill(s).\n", len(selected))
			} else {
				fmt.Printf("\nInstalled %d of %d available skill(s).\n", len(selected), len(available))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&installAll, "all", false, "Install all skills from the repo without prompting")
	return cmd
}
