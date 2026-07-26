package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

// parseSkillRef parses a "source:name" reference into source and bare name.
// If there is no colon, returns (name, "") where name is the full input,
// meaning the caller should resolve by name alone.
func parseSkillRef(ref string) (name, source string) {
	if idx := strings.LastIndex(ref, ":"); idx > 0 {
		return ref[idx+1:], ref[:idx]
	}
	return ref, ""
}

// findSkill finds a skill by name, optionally scoped to a source.
// Returns the manifest key, entry, and an error if not found.
func findSkill(m *manifest.Manifest, name, source string) (string, manifest.SkillEntry, error) {
	if source != "" {
		// Scoped lookup: source:name
		key := source + ":" + name
		if entry, ok := m.Skills[key]; ok {
			return key, entry, nil
		}
		return "", manifest.SkillEntry{}, fmt.Errorf("skill %q from source %q is not installed", name, source)
	}

	// Unscoped lookup: find the first skill with this bare name across all sources
	var candidates []string
	for key, entry := range m.Skills {
		// Check if key ends with ":name" or is exactly "name"
		bareName := key
		if entry.Source != "" {
			bareName = strings.TrimPrefix(key, entry.Source+":")
		}
		if bareName == name {
			candidates = append(candidates, key)
		}
	}

	if len(candidates) == 0 {
		return "", manifest.SkillEntry{}, fmt.Errorf("skill %q is not installed", name)
	}

	if len(candidates) > 1 {
		return "", manifest.SkillEntry{}, fmt.Errorf(
			"skill %q exists in multiple sources (%s). Use 'source:name' syntax to specify which one to remove",
			name, strings.Join(candidates, ", "))
	}

	key := candidates[0]
	return key, m.Skills[key], nil
}

func newRemoveCmd() *cobra.Command {
	var removeAll bool

	cmd := &cobra.Command{
		Use:   "remove [source:]skill-name",
		Short: "Remove installed skills",
		Long: `Remove one or more installed skills by name.

Use "source:name" syntax (e.g. "slap remove work:git-commit") to remove
a skill from a specific source. Without a source prefix, the skill is
matched by name (fails if the name exists in multiple sources).

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
				for key, entry := range m.Skills {
					// Derive bare directory name
					dirName := key
					if entry.Source != "" {
						dirName = strings.TrimPrefix(key, entry.Source+":")
					}
					sp := filepath.Join(targetDir, dirName)
					os.RemoveAll(sp)
					m.RemoveSkill(entry.Source, dirName)
					fmt.Printf("  - %s (%s)\n", dirName, entry.Source)
				}
				if err := m.Save(manifestPath); err != nil {
					return fmt.Errorf("saving manifest: %w", err)
				}
				fmt.Printf("\nRemoved %d skill(s). Manifest cleaned.\n", count)
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("usage: slap remove [source:]skill-name or slap remove --all")
			}

			inputName, inputSource := parseSkillRef(args[0])

			key, _, err := findSkill(m, inputName, inputSource)
			if err != nil {
				return err
			}

			entry := m.Skills[key]
			dirName := inputName
			if entry.Source != "" {
				dirName = strings.TrimPrefix(key, entry.Source+":")
			}

			sp := filepath.Join(targetDir, dirName)
			os.RemoveAll(sp)
			m.RemoveSkill(entry.Source, dirName)

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("saving manifest: %w", err)
			}

			if entry.Source != "" {
				fmt.Printf("Removed %q from source %q.\n", dirName, entry.Source)
			} else {
				fmt.Printf("Removed %q.\n", dirName)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&removeAll, "all", false, "Remove all installed skills")
	return cmd
}
