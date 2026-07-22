package cmd

import (
	"fmt"
	"os"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/git"
	"github.com/gurkan/skillissue/internal/link"
	"github.com/spf13/cobra"
)

var (
	configPath string
	dryRun     bool
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
	rootCmd.AddCommand(updateCmd, linkCmd, statusCmd)
}

// loadConfig is shared by all subcommands.
func loadConfig() (*config.Config, error) {
	return config.Load(configPath)
}
