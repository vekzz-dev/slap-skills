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

// statusRow holds the drift classification for a single skill.
type statusRow struct {
	Source   string
	Status   string
	RepoSHA  string
	LocalSHA string
}

// statusSourceResult holds the status result for one source.
type statusSourceResult struct {
	Alias  string
	Err    error
	Rows   map[string]statusRow
}

// statusForSource clones one source and classifies its skills.
func statusForSource(ctx context.Context, alias string, m *manifest.Manifest, targetDir string) (*statusSourceResult, error) {
	src, err := config.ReadSource(alias)
	if err != nil {
		return &statusSourceResult{Alias: alias, Err: err}, nil
	}

	tmpDir, err := os.MkdirTemp("", "slap-status-"+alias+"-*")
	if err != nil {
		return &statusSourceResult{Alias: alias, Err: fmt.Errorf("creating temp dir: %w", err)}, nil
	}
	defer os.RemoveAll(tmpDir)

	client := &repo.Client{URL: src.URL, Branch: src.Branch}
	if err := client.CloneShallow(ctx, tmpDir); err != nil {
		return &statusSourceResult{Alias: alias, Err: fmt.Errorf("cloning: %w", err)}, nil
	}

	repoSkills, err := client.ListSkillDirs(ctx, tmpDir)
	if err != nil {
		return &statusSourceResult{Alias: alias, Err: fmt.Errorf("listing skills: %w", err)}, nil
	}

	// Convert repo tree SHAs to local-style SHAs
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

	rows := make(map[string]statusRow)

	// Classify skills from this source only.
	sourceSkills := m.SkillsBySource(alias)
	for bareName, entry := range sourceSkills {
		skillPath := filepath.Join(targetDir, bareName)
		rs, inRepo := repoMap[bareName]

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

		rows[bareName] = statusRow{
			Source:   alias,
			Status:   st,
			RepoSHA:  shortSHA(repoSHA),
			LocalSHA: shortSHA(localSHA),
		}
	}

	// Add repo skills not yet in manifest for this source.
	for _, s := range repoSkills {
		if !m.HasSkill(alias, s.Name) {
			rows[s.Name] = statusRow{
				Source:   alias,
				Status:   "new",
				RepoSHA:  shortSHA(s.TreeSHA),
				LocalSHA: "-",
			}
		}
	}

	return &statusSourceResult{Alias: alias, Rows: rows}, nil
}

// newStatusCmd creates the `slap status` command.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show drift between local skills and all configured sources",
		Long: `Compare local skills against all configured sources and report
per-skill drift for each source.

Clones each source repo (shallow) and compares tree SHAs with the local
manifest and on-disk state to classify each skill as up-to-date, behind,
new, missing, or locally modified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			// Load config.
			cfg, err := config.Load(config.ConfigFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("slap is not configured. Run 'slap source add' to add a source")
				}
				return fmt.Errorf("loading config: %w", err)
			}

			// Determine which sources to scan.
			sourceAliases := cfg.Sources
			if len(sourceAliases) == 0 {
				if cfg.RepoURL == "" {
					return fmt.Errorf("no sources configured. Run 'slap source add' to add one")
				}
				// Trigger migration and retry.
				if err := config.MigrateConfig(); err != nil {
					return fmt.Errorf("config migration: %w", err)
				}
				cfg, err = config.Load(config.ConfigFile)
				if err != nil {
					return fmt.Errorf("reloading config: %w", err)
				}
				sourceAliases = cfg.Sources
				if len(sourceAliases) == 0 {
					return fmt.Errorf("no sources configured after migration")
				}
			}

			// Load manifest.
			manifestPath := expandPath(config.ManifestFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			targetDir := expandPath(cfg.TargetDir)
			ctx := context.Background()

			// Process each source independently.
			var results []*statusSourceResult
			for _, alias := range sourceAliases {
				result, srcErr := statusForSource(ctx, alias, m, targetDir)
				if srcErr != nil {
					return fmt.Errorf("checking source %q: %w", alias, srcErr)
				}
				results = append(results, result)
			}

			// Aggregate all rows.
			totalRows := 0
			for _, r := range results {
				if r.Err != nil {
					continue
				}
				totalRows += len(r.Rows)
			}

			if totalRows == 0 {
				errMsg := "No skills found. Run 'slap sync' to install skills from configured sources."
				// Check if any source had errors.
				for _, r := range results {
					if r.Err != nil {
						errMsg = "Could not check any source. See errors above."
						break
					}
				}
				fmt.Println(errMsg)
				return nil
			}

			// Render the drift table with source column.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "Skill Name\tSource\tStatus\tRepo SHA\tLocal SHA")
			for _, r := range results {
				if r.Err != nil {
					fmt.Printf("⚠ Source %q: %v\n", r.Alias, r.Err)
					continue
				}
				for name, row := range r.Rows {
					fmt.Fprintf(w, "%s (%s)\t%s\t%s\t%s\t%s\n",
						name, row.Source, row.Source, row.Status, row.RepoSHA, row.LocalSHA)
				}
			}
			return w.Flush()
		},
	}

	return cmd
}
