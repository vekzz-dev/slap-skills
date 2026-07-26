package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
	"github.com/vekzz-dev/slap-skills/internal/sync"
)

// sync-specific flag (not persistent on root).
var flagPrune bool

// syncPerSourceResult tracks the outcome of a single source's sync pass.
type syncPerSourceResult struct {
	Alias             string
	URL               string
	Available, Updated, Removed, Skipped, Warnings int
	Err               error
}

// newSyncCmd creates the `slap sync` command.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Install or update skills from all configured sources",
		Long: `Synchronise local skills with all configured git repositories.

For each source in the configuration, shallow-clones the repository,
compares each skill directory against the local manifest, and copies
new or updated skills to the target directory.  Use --prune to also
remove skills that have been deleted from a source's repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run migration first — handles old-style config upgrade.
			if err := config.MigrateConfig(); err != nil {
				return fmt.Errorf("config migration: %w", err)
			}

			// ----------------------------------------------------------------
			// Phase 1 – Pre-flight: load config, sources, and manifest
			// ----------------------------------------------------------------
			cfg, err := config.Load(config.ConfigFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("slap is not configured. Run 'slap init <repo-url>' first")
				}
				return fmt.Errorf("loading config: %w", err)
			}

			if cmd.Flags().Changed("target-dir") {
				cfg.TargetDir = flagTargetDir
			}

			// Determine which sources to process.
			sourceAliases := cfg.Sources
			if len(sourceAliases) == 0 {
				// Fallback: no sources configured but repo_url is set (pre-migration).
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

			targetDir := expandPath(cfg.TargetDir)
			manifestPath := expandPath(config.ManifestFile)

			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			prune, _ := cmd.Flags().GetBool("prune")
			ctx := context.Background()

			// ----------------------------------------------------------------
			// Phase 2 – Process each source independently
			// ----------------------------------------------------------------
			var results []syncPerSourceResult

			for _, alias := range sourceAliases {
				result := syncPerSourceResult{Alias: alias}
				src, srcErr := config.ReadSource(alias)
				if srcErr != nil {
					result.Err = fmt.Errorf("reading source %q: %w", alias, srcErr)
					results = append(results, result)
					continue
				}
				result.URL = src.URL
				fmt.Printf("\n--- Source: %s (%s) ---\n", alias, src.URL)

				// Per-source temp dir (isolated per alias) - Task 3.4
				tmpDir, tmpErr := os.MkdirTemp("", "slap-sync-"+alias+"-*")
				if tmpErr != nil {
					result.Err = fmt.Errorf("creating temp dir for %q: %w", alias, tmpErr)
					results = append(results, result)
					continue
				}

				client := &repo.Client{URL: src.URL, Branch: src.Branch}
				if cloneErr := client.CloneShallow(ctx, tmpDir); cloneErr != nil {
					os.RemoveAll(tmpDir)
					result.Err = fmt.Errorf("cloning %q: %w", alias, cloneErr)
					results = append(results, result)
					continue
				}

				repoSkills, listErr := client.ListSkillDirs(ctx, tmpDir)
				if listErr != nil {
					os.RemoveAll(tmpDir)
					result.Err = fmt.Errorf("listing skills in %q: %w", alias, listErr)
					results = append(results, result)
					continue
				}

				// Convert repo tree SHAs to local-style SHAs
				for i := range repoSkills {
					skillPath := filepath.Join(tmpDir, repoSkills[i].Name)
					if sha, computeErr := repo.ComputeLocalTreeSHA(skillPath); computeErr == nil {
						repoSkills[i].TreeSHA = sha
					}
				}

				// Compute local SHAs for skills from THIS source only
				localSHAs := make(map[string]string)
				sourceSkills := m.SkillsBySource(alias)
				for name := range sourceSkills {
					skillPath := filepath.Join(targetDir, name)
					sha, computeErr := repo.ComputeLocalTreeSHA(skillPath)
					if computeErr == nil {
						localSHAs[name] = sha
					}
				}

				// Plan the delta for this source
				actions := sync.Plan(m, repoSkills, localSHAs, prune, alias)

				// Execute the plan
				for _, a := range actions {
					switch a.Type {
					case sync.ActionAdd:
						result.Available++
						fmt.Printf("  . %s  (available — run 'slap install' to add it)\n", a.Name)

					case sync.ActionUpdate:
						result.Updated++
						dst := filepath.Join(targetDir, a.Name)
						os.RemoveAll(dst)
						if err := copyDir(filepath.Join(tmpDir, a.Name), dst); err != nil {
							result.Err = fmt.Errorf("updating skill %s from %q: %w", a.Name, alias, err)
							break
						}
						localSHA := computeLocalSHA(dst)
						m.UpsertSkill(alias, a.Name, localSHA)
						fmt.Printf("  ~ %s  (%s... -> %s...)\n", a.Name, shortSHA(a.FromSHA), shortSHA(a.ToSHA))

					case sync.ActionRemove:
						result.Removed++
						os.RemoveAll(filepath.Join(targetDir, a.Name))
						m.RemoveSkill(alias, a.Name)
						fmt.Printf("  - %s\n", a.Name)

					case sync.ActionSkip:
						result.Skipped++
						fmt.Printf("  = %s  (up to date)\n", a.Name)

					case sync.ActionLocalModNoRepoChange:
						result.Warnings++
						fmt.Printf("  ! %s  (locally modified, repo unchanged)\n", a.Name)

					case sync.ActionLocalModWithRepoUpdate:
						result.Warnings++
						dst := filepath.Join(targetDir, a.Name)
						os.RemoveAll(dst)
						if err := copyDir(filepath.Join(tmpDir, a.Name), dst); err != nil {
							result.Err = fmt.Errorf("updating locally modified skill %s from %q: %w", a.Name, alias, err)
							break
						}
						localSHA := computeLocalSHA(dst)
						m.UpsertSkill(alias, a.Name, localSHA)
						fmt.Printf("  ! %s  (locally modified — overwritten from repo)\n", a.Name)
					}
				}

				// Clean up per-source temp dir
				os.RemoveAll(tmpDir)

				if result.Err != nil {
					fmt.Printf("  ⚠ source %q finished with error: %v\n", alias, result.Err)
				}
				results = append(results, result)
			}

			// ----------------------------------------------------------------
			// Phase 3 – Save manifest once after all sources processed
			// ----------------------------------------------------------------
			// In multi-source mode, clear single-repo manifest fields
			m.SourceRepo = ""
			m.SourceBranch = ""

			if saveErr := m.Save(manifestPath); saveErr != nil {
				return fmt.Errorf("saving manifest: %w", saveErr)
			}

			// ----------------------------------------------------------------
			// Phase 4 – Per-source summary
			// ----------------------------------------------------------------
			totalAvailable, totalUpdated, totalRemoved, totalSkipped, totalWarnings := 0, 0, 0, 0, 0
			for _, res := range results {
				totalAvailable += res.Available
				totalUpdated += res.Updated
				totalRemoved += res.Removed
				totalSkipped += res.Skipped
				totalWarnings += res.Warnings
				status := "OK"
				if res.Err != nil {
					status = "ERROR"
				}
				fmt.Printf("\nSource %q: %s  (%d available, %d updated, %d removed, %d skipped, %d warnings)\n",
					res.Alias, status, res.Available, res.Updated, res.Removed, res.Skipped, res.Warnings)
			}

			fmt.Printf("\nTotal: %d available (use 'slap install'), %d updated, %d removed, %d skipped, %d warnings\n",
				totalAvailable, totalUpdated, totalRemoved, totalSkipped, totalWarnings)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagPrune, "prune", false, "Remove local skills that no longer exist in the repo")
	return cmd
}
