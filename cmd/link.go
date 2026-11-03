package cmd

import "github.com/spf13/cobra"

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Relink skills and GC stale links (no network)",
	Long:  "Discovers skills in the existing cache and syncs symlinks into skill_paths. Does not fetch, commit, or push.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			return err
		}
		if wantPruneCache(c) {
			if err := runPruneCache(c); err != nil {
				return err
			}
		}
		return runLinkPhase(c)
	},
}
