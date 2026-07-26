package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

// list-specific flag.
var flagJSON bool

// newListCmd creates the `slap list` command.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		Long: `Display all skills currently installed from the configured repo.

By default outputs a human-readable table.  Pass --json for
machine-readable JSON output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := expandPath(config.ManifestFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest: %w", err)
			}

			if len(m.Skills) == 0 {
				return fmt.Errorf("no skills installed yet. Run 'slap sync' first")
			}

			out := cmd.OutOrStdout()
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return printManifestJSON(out, m)
			}
			return printManifestTable(out, m)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")
	return cmd
}

// printManifestJSON encodes the full manifest as indented JSON to w.
func printManifestJSON(w io.Writer, m *manifest.Manifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// printManifestTable renders a human-friendly table of installed skills to w.
func printManifestTable(w io.Writer, m *manifest.Manifest) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Skill Name\tSource\tSHA\tInstalled\tLast Synced")
	for key, entry := range m.Skills {
		sha := entry.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		// Derive bare name from key (which is "source:name" for source entries)
		bareName := key
		if entry.Source != "" {
			bareName = strings.TrimPrefix(key, entry.Source+":")
		}
		// Display "name (alias)" format
		sourceDisplay := entry.Source
		if sourceDisplay == "" {
			sourceDisplay = "-"
		}
		fmt.Fprintf(tw, "%s (%s)\t%s\t%s\t%s\t%s\n",
			bareName, sourceDisplay, sourceDisplay, sha,
			entry.InstalledAt.Format(time.RFC3339),
			entry.LastSyncedAt.Format(time.RFC3339),
		)
	}
	return tw.Flush()
}
