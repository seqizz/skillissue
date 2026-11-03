package cmd

import (
	"fmt"
	"os"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/git"
	"github.com/gurkan/skillissue/internal/link"
	"github.com/gurkan/skillissue/internal/prune"
	"github.com/spf13/cobra"
)

var (
	configPath string
	dryRun     bool
	pruneCache bool
)

var rootCmd = &cobra.Command{
	Use:   "skillissue",
	Short: "Manage LLM agent skills across git repos and agent skill dirs",
	Long: "skillissue pulls upstream skill repos into a local cache, symlinks " +
		"discovered skills into agent skill directories, and pushes agent-made " +
		"edits back upstream.",
	SilenceUsage:  true,
	SilenceErrors: true,
	// Propagate the global dry-run flag to the packages that mutate state.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		git.DryRun = dryRun
		link.DryRun = dryRun
		prune.DryRun = dryRun
	},
}

// Execute is the CLI entry point.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultPath(), "path to config file")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print intended actions without changing anything")

	// Not a persistent flag: `status` is read-only by contract and must not
	// grow a destructive switch.
	for _, c := range []*cobra.Command{updateCmd, linkCmd} {
		c.Flags().BoolVar(&pruneCache, "prune-cache", false, "also remove cache clones of repos no longer in the config")
	}
	rootCmd.AddCommand(updateCmd, linkCmd, statusCmd)
}

// loadConfig is shared by all subcommands.
func loadConfig() (*config.Config, error) {
	return config.Load(configPath)
}

// wantPruneCache reports whether this run should prune orphaned clones. The
// flag can only turn pruning on, never off: a config that opted in stays opted
// in, and the absent flag means "unspecified", not "no".
func wantPruneCache(c *config.Config) bool {
	return pruneCache || c.PruneCache
}
