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

// newSyncCmd creates the `slap sync` command.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Install or update skills from the configured repo",
		Long: `Synchronise local skills with the configured git repository.

Shallow-clones the configured repo, compares each skill directory against
the local manifest, and copies new or updated skills to the target
directory.  Use --prune to also remove skills that have been deleted
from the repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ----------------------------------------------------------------
			// Phase 1 – Pre-flight: load config and manifest
			// ----------------------------------------------------------------
			cfg, err := config.Load(config.ConfigFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("slap is not configured. Run 'slap init <repo-url>' first")
				}
				return fmt.Errorf("loading config: %w", err)
			}

			// Apply flag overrides only for flags the user explicitly passed.
			if cmd.Flags().Changed("repo") {
				cfg.RepoURL = flagRepo
			}
			if cmd.Flags().Changed("branch") {
				cfg.Branch = flagBranch
			}
			if cmd.Flags().Changed("target-dir") {
				cfg.TargetDir = flagTargetDir
			}

			targetDir := expandPath(cfg.TargetDir)
			manifestPath := expandPath(config.ManifestFile)

			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			// ----------------------------------------------------------------
			// Phase 2 – Clone the repo into a temp directory
			// ----------------------------------------------------------------
			tmpDir, err := os.MkdirTemp("", "slap-sync-*")
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

			// Convert repo tree SHAs (git SHA1) to local-style SHAs
			// (ComputeLocalTreeSHA — SHA256) so that Plan can compare
			// manifest, repo, and local SHAs using the same algorithm.
			for i := range repoSkills {
				skillPath := filepath.Join(tmpDir, repoSkills[i].Name)
				if sha, computeErr := repo.ComputeLocalTreeSHA(skillPath); computeErr == nil {
					repoSkills[i].TreeSHA = sha
				}
			}

			// Compute local tree SHAs for skills tracked in the manifest.
			localSHAs := make(map[string]string, len(m.Skills))
			for name := range m.Skills {
				skillPath := filepath.Join(targetDir, name)
				sha, computeErr := repo.ComputeLocalTreeSHA(skillPath)
				if computeErr == nil {
					localSHAs[name] = sha
				}
			}

			// ----------------------------------------------------------------
			// Phase 3 – Plan the delta
			// ----------------------------------------------------------------
			prune, _ := cmd.Flags().GetBool("prune")
			actions := sync.Plan(m, repoSkills, localSHAs, prune)

			// ----------------------------------------------------------------
			// Phase 4 – Execute the plan
			// ----------------------------------------------------------------
			adds, updates, removes, skips, warnings := 0, 0, 0, 0, 0

			for _, a := range actions {
				switch a.Type {
				case sync.ActionAdd:
					adds++
					fmt.Printf("  . %s  (available — run 'slap install' to add it)\n", a.Name)

				case sync.ActionUpdate:
					updates++
					dst := filepath.Join(targetDir, a.Name)
					os.RemoveAll(dst)
					if err := copyDir(filepath.Join(tmpDir, a.Name), dst); err != nil {
						return fmt.Errorf("updating skill %s: %w", a.Name, err)
					}
					localSHA := computeLocalSHA(dst)
					m.UpsertSkill(a.Name, localSHA)
					fmt.Printf("  ~ %s  (%s... -> %s...)\n", a.Name, shortSHA(a.FromSHA), shortSHA(a.ToSHA))

				case sync.ActionRemove:
					removes++
					os.RemoveAll(filepath.Join(targetDir, a.Name))
					m.RemoveSkill(a.Name)
					fmt.Printf("  - %s\n", a.Name)

				case sync.ActionSkip:
					skips++
					fmt.Printf("  = %s  (up to date)\n", a.Name)

				case sync.ActionLocalModNoRepoChange:
					warnings++
					fmt.Printf("  ! %s  (locally modified, repo unchanged)\n", a.Name)

				case sync.ActionLocalModWithRepoUpdate:
					warnings++
					dst := filepath.Join(targetDir, a.Name)
					os.RemoveAll(dst)
					if err := copyDir(filepath.Join(tmpDir, a.Name), dst); err != nil {
						return fmt.Errorf("updating locally modified skill %s: %w", a.Name, err)
					}
					localSHA := computeLocalSHA(dst)
					m.UpsertSkill(a.Name, localSHA)
					fmt.Printf("  ! %s  (locally modified — overwritten from repo)\n", a.Name)
				}
			}

	// Update manifest metadata and persist.
	m.SourceRepo = cfg.RepoURL
	m.SourceBranch = cfg.Branch
			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("saving manifest: %w", err)
			}

			fmt.Printf("\nSummary: %d available (use 'slap install'), %d updated, %d removed, %d skipped, %d warnings\n",
				adds, updates, removes, skips, warnings)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagPrune, "prune", false, "Remove local skills that no longer exist in the repo")
	return cmd
}
