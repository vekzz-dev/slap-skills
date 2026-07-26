package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

// availableSkill bundles a skill from a specific source for display.
type availableSkill struct {
	Name    string
	Source  string
	SHA     string
	TempDir string // path to cloned repo temp dir for this source
}

// installAvailableForSource clones a single source and returns available skills
// (not yet installed for that source), with the source alias attached.
// The caller is responsible for cleaning up the returned TempDir for each skill
// (they all share the same dir for a given source).
func installAvailableForSource(ctx context.Context, alias string, m *manifest.Manifest) ([]availableSkill, string, error) {
	src, err := config.ReadSource(alias)
	if err != nil {
		return nil, "", fmt.Errorf("reading source %q: %w", alias, err)
	}

	tmpDir, err := os.MkdirTemp("", "slap-install-"+alias+"-*")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp dir for %q: %w", alias, err)
	}

	client := &repo.Client{URL: src.URL, Branch: src.Branch}
	if err := client.CloneShallow(ctx, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("cloning %q: %w", alias, err)
	}

	repoSkills, err := client.ListSkillDirs(ctx, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("listing skills in %q: %w", alias, err)
	}

	// Convert repo tree SHAs to local format
	for i := range repoSkills {
		sp := filepath.Join(tmpDir, repoSkills[i].Name)
		if sha, computeErr := repo.ComputeLocalTreeSHA(sp); computeErr == nil {
			repoSkills[i].TreeSHA = sha
		}
	}

	var result []availableSkill
	for _, s := range repoSkills {
		if !m.HasSkill(alias, s.Name) {
			result = append(result, availableSkill{
				Name:    s.Name,
				Source:  alias,
				SHA:     s.TreeSHA,
				TempDir: tmpDir,
			})
		}
	}
	return result, tmpDir, nil
}

func newInstallCmd() *cobra.Command {
	var installAll bool
	var installSource string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Select and install skills from sources",
		Long: `List available skills from configured sources and install the ones you choose.

Use --all to install every skill without prompting.
Use --source to limit to a specific source alias.`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Run migration first.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			cfg, err := config.Load(expandPath(config.ConfigFile))
			if err != nil {
				return fmt.Errorf("slap is not configured. Run 'slap source add' to add a source")
			}
			if cobraCmd.Flags().Changed("target-dir") {
				cfg.TargetDir = flagTargetDir
			}

			targetDir := expandPath(cfg.TargetDir)
			manifestPath := expandPath(config.ManifestFile)

			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			// Determine which sources to scan
			sourceAliases := cfg.Sources
			if installSource != "" {
				// Validate the requested source exists
				found := false
				for _, a := range sourceAliases {
					if a == installSource {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("source %q is not configured", installSource)
				}
				sourceAliases = []string{installSource}
			}

			if len(sourceAliases) == 0 {
				return fmt.Errorf("no sources configured. Run 'slap source add' to add one")
			}

			// Collect available skills from all relevant sources
			ctx := context.Background()
			var allAvailable []availableSkill
			var tmpDirs []string

			for _, alias := range sourceAliases {
				avail, tmpDir, srcErr := installAvailableForSource(ctx, alias, m)
				if srcErr != nil {
					// Clean up any temp dirs from previous sources
					for _, d := range tmpDirs {
						os.RemoveAll(d)
					}
					return fmt.Errorf("scanning source %q: %w", alias, srcErr)
				}
				tmpDirs = append(tmpDirs, tmpDir)
				allAvailable = append(allAvailable, avail...)
			}

			// Ensure cleanup of all temp dirs
			defer func() {
				for _, d := range tmpDirs {
					os.RemoveAll(d)
				}
			}()

			if len(allAvailable) == 0 {
				fmt.Println("All skills from the configured sources are already installed.")
				return nil
			}

			// Build display options and lookup maps
			var displayOptions []string
			optionToSkill := make(map[string]availableSkill)
			bareNameToSkill := make(map[string]availableSkill)

			for _, as := range allAvailable {
				// Keep the first occurrence for bare name dedup
				if _, exists := bareNameToSkill[as.Name]; !exists {
					bareNameToSkill[as.Name] = as
				}

				var label string
				if installSource != "" {
					label = as.Name
				} else {
					label = fmt.Sprintf("%s (%s)", as.Name, as.Source)
				}
				// Deduplicate label
				if _, exists := optionToSkill[label]; !exists {
					optionToSkill[label] = as
					displayOptions = append(displayOptions, label)
				}
			}
			sort.Strings(displayOptions)

			var selected []string

			if !installAll {
				prompt := &survey.MultiSelect{
					Message: "Select skills to install:",
					Options: displayOptions,
					Description: func(value string, index int) string {
						return ""
					},
				}
				if err := survey.AskOne(prompt, &selected, survey.WithPageSize(20)); err != nil {
					if strings.Contains(err.Error(), "interrupt") {
						fmt.Println("Cancelled.")
						return nil
					}
					return err
				}
				if len(selected) == 0 {
					fmt.Println("No skills selected.")
					return nil
				}
			} else {
				selected = displayOptions
			}

			// Install each selected skill
			for _, label := range selected {
				skill, ok := optionToSkill[label]
				if !ok {
					// Fallback to bare name for --all mode
					skill, ok = bareNameToSkill[label]
					if !ok {
						return fmt.Errorf("internal error: unknown skill %q", label)
					}
				}

				src := filepath.Join(skill.TempDir, skill.Name)
				dst := filepath.Join(targetDir, skill.Name)
				if err := copyDir(src, dst); err != nil {
					return fmt.Errorf("installing %s from %q: %w", skill.Name, skill.Source, err)
				}
				localSHA := computeLocalSHA(dst)
				m.UpsertSkill(skill.Source, skill.Name, localSHA)
				fmt.Printf("  + %s (%s)\n", skill.Name, skill.Source)
			}

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("saving manifest: %w", err)
			}

			if installAll {
				fmt.Printf("\nInstalled %d skill(s).\n", len(selected))
			} else {
				fmt.Printf("\nInstalled %d of %d available skill(s).\n", len(selected), len(allAvailable))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&installAll, "all", false, "Install all skills without prompting")
	cmd.Flags().StringVar(&installSource, "source", "", "Only show skills from the given source alias")
	return cmd
}
