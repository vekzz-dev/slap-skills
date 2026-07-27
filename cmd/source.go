package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

// newSourceCmd creates the `slap source` command and its subcommands.
func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage skill source repositories",
		Long: `Configure and manage multiple git repositories as skill sources.

Each source is a git repository containing AI agent skills. Sources are
stored in ~/.config/slap/sources/ and identified by a unique alias.`,
	}

	cmd.AddCommand(newSourceAddCmd())
	cmd.AddCommand(newSourceListCmd())
	cmd.AddCommand(newSourceRemoveCmd())

	return cmd
}

// newSourceAddCmd creates the `slap source add` command.
func newSourceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new skill source",
		Long: `Add a new git repository as a skill source.

Prompts for a Git URL and a unique alias. Validates that the repository
is reachable and that the alias does not conflict with existing sources.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			// Step 1: Prompt for Git URL
			var url string
			urlPrompt := &survey.Input{
				Message: "Git URL:",
				Help:    "URL of a git repository that contains AI agent skills",
			}
			if err := survey.AskOne(urlPrompt, &url, survey.WithValidator(survey.Required)); err != nil {
				return interruptOrError(err)
			}
			url = strings.TrimSpace(url)

			// Step 2: Load existing sources for alias uniqueness check
			existingSources, err := config.ListSources()
			if err != nil {
				return fmt.Errorf("listing existing sources: %w", err)
			}

			var alias string
			aliasPrompt := &survey.Input{
				Message: "Alias:",
				Help:    "A short, unique name for this source (e.g. \"community\", \"work\")",
			}
			if err := survey.AskOne(aliasPrompt, &alias,
				survey.WithValidator(survey.Required),
				survey.WithValidator(func(val interface{}) error {
					input := val.(string)
					for _, existing := range existingSources {
						if input == existing {
							return fmt.Errorf("alias %q is already in use", input)
						}
					}
					return nil
				}),
			); err != nil {
				return interruptOrError(err)
			}
			alias = strings.TrimSpace(alias)

			// Step 3: Validate URL reachable
			if err := config.ValidateRepoAccess(url, "main"); err != nil {
				return fmt.Errorf("source not reachable: %w", err)
			}

			// Step 4: Write source file
			sc := config.SourceConfig{
				Alias:  alias,
				URL:    url,
				Branch: "main",
			}
			if err := config.CreateSource(alias, sc); err != nil {
				return fmt.Errorf("saving source: %w", err)
			}

			// Step 5: Ensure config.yaml exists with this source in its Sources list.
			// This is needed so sync/install can find configured sources.
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
			if err := cfg.Save(config.ConfigFile); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Source %q added.\n", alias)
			return nil
		},
	}
}

// newSourceListCmd creates the `slap source list` command.
func newSourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured skill sources",
		Long:  `Display all configured skill sources with their alias, URL, and branch.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			aliases, err := config.ListSources()
			if err != nil {
				return fmt.Errorf("listing sources: %w", err)
			}
			if aliases == nil {
				aliases = []string{}
			}

			if len(aliases) == 0 {
				fmt.Println("No sources configured.")
				fmt.Println("Use 'slap source add' to add one.")
				return nil
			}

			// Table header
			fmt.Println("Alias\tURL\tBranch")
			for _, alias := range aliases {
				sc, err := config.ReadSource(alias)
				if err != nil {
					fmt.Printf("%s\t<error reading>\t—\n", alias)
					continue
				}
				fmt.Printf("%s\t%s\t%s\n", sc.Alias, sc.URL, sc.Branch)
			}
			return nil
		},
	}
}

// newSourceRemoveCmd creates the `slap source remove <alias>` command.
func newSourceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <alias>",
		Short: "Remove a skill source",
		Long: `Remove a configured source and optionally uninstall its installed skills.

If the source has installed skills, you will be prompted to confirm
uninstallation before the source is removed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			// Check alias exists.
			aliases, err := config.ListSources()
			if err != nil {
				return fmt.Errorf("listing sources: %w", err)
			}
			found := false
			for _, a := range aliases {
				if a == alias {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("source %q not found", alias)
			}

			// Read source config (validates it's well-formed).
			_, err = config.ReadSource(alias)
			if err != nil {
				return fmt.Errorf("reading source: %w", err)
			}

			// Check for installed skills from this source.
			manifestPath := expandPath(config.ManifestFile)
			m, manifestErr := manifest.Load(manifestPath)
			var skills map[string]manifest.SkillEntry
			if manifestErr == nil {
				skills = m.SkillsBySource(alias)
			} else {
				skills = nil
			}

			if len(skills) > 0 {
				// Prompt to uninstall.
				var uninstall bool
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("Source %q has %d installed skill(s). Uninstall them?", alias, len(skills)),
					Default: false,
				}
				if err := survey.AskOne(prompt, &uninstall); err != nil {
					return interruptOrError(err)
				}
				if uninstall {
					// Load config to get target directory.
					cfg, cfgErr := config.Load(config.ConfigFile)
					targetDir := expandPath(cfg.TargetDir)
					if cfgErr == nil && targetDir != "" {
						for name := range skills {
							sp := filepath.Join(targetDir, name)
							os.RemoveAll(sp)
						}
					}
					// Remove from manifest regardless of config load outcome.
					for name := range skills {
						m.RemoveSkill(alias, name)
						fmt.Printf("  - %s\n", name)
					}
				}
			}

			// Delete source file.
			if err := config.DeleteSource(alias); err != nil {
				return fmt.Errorf("deleting source: %w", err)
			}

			// Save manifest if we removed entries.
			if len(skills) > 0 && manifestErr == nil {
				if saveErr := m.Save(manifestPath); saveErr != nil {
					return fmt.Errorf("saving manifest: %w", saveErr)
				}
			}

			fmt.Printf("Source %q removed.\n", alias)
			return nil
		},
	}
}

// interruptOrError converts a survey interrupt error to a clean cancellation
// message, or returns the original error otherwise.
func interruptOrError(err error) error {
	if err != nil && strings.Contains(err.Error(), "interrupt") {
		fmt.Println("Cancelled.")
		return nil
	}
	return err
}
