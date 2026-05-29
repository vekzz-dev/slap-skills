// Package cmd implements the slap CLI commands.
package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

// Root-level persistent flag values.
var (
	flagRepo      string
	flagBranch    string
	flagTargetDir string
)

// expandPath replaces a leading ~/ with the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// shortSHA returns the first 7 characters of a hex SHA, or the full string
// if it is shorter than 7.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// copyDir recursively copies a directory from src to dst.
// Both src and dst must be absolute or relative filesystem paths.
// The destination directory is created if it does not exist.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}

// computeLocalSHA returns the ComputeLocalTreeSHA for the given directory.
// Returns the string "<unknown>" if the SHA cannot be computed.
func computeLocalSHA(dir string) string {
	sha, err := repo.ComputeLocalTreeSHA(dir)
	if err != nil {
		return "<unknown>"
	}
	return sha
}

// NewRootCmd creates the slap root command with all subcommands.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slap",
		Short: "Slap Skills — manage opencode skills from a git repo",
		Long: `Slap Skills syncs opencode skills from any git repo to your local skills directory.

  init    - Configure a git repo as the skill source
  install - Select and install skills from the repo
  remove  - Remove installed skills
  sync    - Update installed skills from the repo
  list    - List installed skills
  status  - Show drift between local skills and the repo`,
	}

	cmd.PersistentFlags().StringVar(&flagRepo, "repo", "", "Override the configured repo URL")
	cmd.PersistentFlags().StringVar(&flagBranch, "branch", "main", "Git branch to sync from")
	cmd.PersistentFlags().StringVar(&flagTargetDir, "target-dir", "~/.config/opencode/skills", "Local skills directory")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

// Execute creates the root command and runs it.  os.Exit(1) on error.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
