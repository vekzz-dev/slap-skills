package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

func newInstallCmd() *cobra.Command {
	var installAll bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install skills from the configured repo",
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
			var available []repo.SkillDir
			for _, s := range repoSkills {
				if !m.HasSkill(s.Name) {
					available = append(available, s)
				}
			}

			if len(available) == 0 {
				fmt.Println("All skills from the repo are already installed.")
				return nil
			}

			// Sort alphabetically
			sort.Slice(available, func(i, j int) bool {
				return available[i].Name < available[j].Name
			})

			toInstall := available

			if !installAll {
				// Interactive selection
				fmt.Println("\nAvailable skills:")
				for i, s := range available {
					fmt.Printf("  %2d. %s\n", i+1, s.Name)
				}
				fmt.Print("\nEnter numbers separated by commas (or 'all'): ")
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)

				if strings.ToLower(input) == "all" {
					toInstall = available
				} else {
					selected := make(map[int]bool)
					for _, part := range strings.Split(input, ",") {
						part = strings.TrimSpace(part)
						n, err := strconv.Atoi(part)
						if err != nil || n < 1 || n > len(available) {
							fmt.Printf("Invalid selection: %s\n", part)
							continue
						}
						selected[n-1] = true
					}
					toInstall = nil
					for i, s := range available {
						if selected[i] {
							toInstall = append(toInstall, s)
						}
					}
					if len(toInstall) == 0 {
						return fmt.Errorf("no valid skills selected")
					}
				}
			}

			// Install each selected skill
			for _, s := range toInstall {
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
				fmt.Printf("\nInstalled %d skill(s).\n", len(toInstall))
			} else {
				fmt.Printf("\nInstalled %d of %d available skill(s).\n", len(toInstall), len(available))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&installAll, "all", false, "Install all skills from the repo without prompting")
	return cmd
}
