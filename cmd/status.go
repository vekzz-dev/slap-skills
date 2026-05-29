package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

// newStatusCmd creates the `slap status` command.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show drift between local skills and the repo",
		Long: `Compare local skills against the configured repo and report
per-skill drift.

Clones the repo (shallow) and compares tree SHAs with the local manifest
and on-disk state to classify each skill as up-to-date, behind, new,
missing, or locally modified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config.
			cfg, err := config.Load(config.ConfigFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("slap is not configured. Run 'slap init <repo-url>' first")
				}
				return fmt.Errorf("loading config: %w", err)
			}

			// Apply flag overrides for explicitly passed flags.
			if cmd.Flags().Changed("repo") {
				cfg.RepoURL = flagRepo
			}
			if cmd.Flags().Changed("branch") {
				cfg.Branch = flagBranch
			}

			// Load manifest.
			manifestPath := expandPath(config.ManifestFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			// Clone the repo to a temp dir.
			tmpDir, err := os.MkdirTemp("", "slap-status-*")
			if err != nil {
				return fmt.Errorf("creating temp directory: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			ctx := context.Background()
			client := &repo.Client{
				URL:    cfg.RepoURL,
				Branch: cfg.Branch,
			}
			if err := client.CloneShallow(ctx, tmpDir); err != nil {
				return fmt.Errorf("cloning repo: %w", err)
			}

			repoSkills, err := client.ListSkillDirs(ctx, tmpDir)
			if err != nil {
				return fmt.Errorf("listing skill directories: %w", err)
			}

			// Convert repo tree SHAs to local-style SHAs so comparisons
			// against the manifest (which stores ComputeLocalTreeSHA values)
			// are consistent.
			for i := range repoSkills {
				skillPath := filepath.Join(tmpDir, repoSkills[i].Name)
				if sha, computeErr := repo.ComputeLocalTreeSHA(skillPath); computeErr == nil {
					repoSkills[i].TreeSHA = sha
				}
			}

			// Build repo skill map for O(1) lookups.
			repoMap := make(map[string]repo.SkillDir, len(repoSkills))
			for _, s := range repoSkills {
				repoMap[s.Name] = s
			}

			// Classify each skill.
			type statusRow struct {
				Status   string
				RepoSHA  string
				LocalSHA string
			}
			rows := make(map[string]statusRow)

			targetDir := expandPath(cfg.TargetDir)

			for name, entry := range m.Skills {
				skillPath := filepath.Join(targetDir, name)
				rs, inRepo := repoMap[name]

				var st string
				var repoSHA string
				var localSHA string

				if inRepo {
					repoSHA = rs.TreeSHA
				}

				switch {
				case !inRepo:
					st = "removed from repo"
				default:
					// Check if the directory exists on disk.
					_, statErr := os.Stat(skillPath)
					if statErr != nil {
						st = "missing"
					} else {
						if sha, err := repo.ComputeLocalTreeSHA(skillPath); err == nil {
							localSHA = sha
						}
						switch {
						case localSHA != entry.SHA:
							st = "locally-modified"
						case rs.TreeSHA != entry.SHA:
							st = "behind"
						default:
							st = "up-to-date"
						}
					}
				}

				rows[name] = statusRow{
					Status:   st,
					RepoSHA:  shortSHA(repoSHA),
					LocalSHA: shortSHA(localSHA),
				}
			}

			// Add repo skills not yet in the manifest.
			for _, s := range repoSkills {
				if _, inManifest := m.Skills[s.Name]; !inManifest {
					rows[s.Name] = statusRow{
						Status:   "new",
						RepoSHA:  shortSHA(s.TreeSHA),
						LocalSHA: "-",
					}
				}
			}

			// Render the drift table.
			if len(rows) == 0 {
				fmt.Println("No skills found. Run 'slap sync' to install skills from the configured repo.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "Skill Name\tStatus\tRepo SHA\tLocal SHA")
			for name, row := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, row.Status, row.RepoSHA, row.LocalSHA)
			}
			return w.Flush()
		},
	}

	return cmd
}
