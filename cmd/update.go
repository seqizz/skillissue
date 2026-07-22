package cmd

import (
	"fmt"
	"os"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/git"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Clone/pull/commit/push all repos, then relink skills",
	Long: "For each repo: commit local edits (if writable), fetch, fast-forward " +
		"when the remote is ahead, push when the local is ahead. Diverged repos " +
		"are left untouched and reported. Finally relinks all skills.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			return err
		}

		var diverged []string
		for _, r := range c.Repos {
			if err := syncRepo(c, r, &diverged); err != nil {
				// A single repo failure should not abort the whole run.
				fmt.Fprintf(os.Stderr, "warning: %s: %v\n", r.URL, err)
			}
		}

		if err := runLinkPhase(c); err != nil {
			return err
		}

		if len(diverged) > 0 {
			fmt.Println("\ndiverged repos (resolve manually, e.g. rebase/merge in the cache clone):")
			for _, d := range diverged {
				fmt.Println("  ", d)
			}
			return fmt.Errorf("%d repo(s) diverged", len(diverged))
		}
		return nil
	},
}

// syncRepo runs the per-repo pull/push algorithm from the plan.
func syncRepo(c *config.Config, r config.Repo, diverged *[]string) error {
	dir := c.CloneDir(r)

	if !git.Exists(dir) {
		fmt.Printf("%s: cloning into %s\n", r.URL, dir)
		if err := git.Clone(r.URL, r.Ref, dir); err != nil {
			return err
		}
		if dryRun {
			return nil // nothing more to do for a not-yet-cloned repo
		}
	}

	if r.Writable {
		if committed, err := git.CommitAll(dir, c.CommitMessage); err != nil {
			return err
		} else if committed {
			fmt.Printf("%s: committed local edits\n", r.URL)
		}
	}

	if err := git.Fetch(dir); err != nil {
		return err
	}

	behind, ahead, hasUpstream, err := git.AheadBehind(dir)
	if err != nil {
		return err
	}
	if !hasUpstream {
		fmt.Printf("%s: no upstream configured, skipping pull/push\n", r.URL)
		return nil
	}

	switch {
	case behind > 0 && ahead == 0:
		fmt.Printf("%s: fast-forwarding (%d behind)\n", r.URL, behind)
		return git.FastForward(dir)
	case ahead > 0 && behind == 0 && r.Writable:
		fmt.Printf("%s: pushing (%d ahead)\n", r.URL, ahead)
		return git.Push(dir)
	case ahead > 0 && behind == 0 && !r.Writable:
		fmt.Printf("%s: local ahead by %d but repo is not writable, not pushing\n", r.URL, ahead)
		return nil
	case behind > 0 && ahead > 0:
		*diverged = append(*diverged, r.URL)
		return nil
	default:
		fmt.Printf("%s: up to date\n", r.URL)
		return nil
	}
}
