package cmd

import (
	"fmt"
	"os"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/discover"
	"github.com/gurkan/skillissue/internal/link"
	"github.com/gurkan/skillissue/internal/prune"
)

// runLinkPhase discovers skills across all repos, resolves collisions, then syncs
// symlinks into every skill_path and GCs stale owned links. It prints warnings
// and a summary. Used by both `update` and `link`.
func runLinkPhase(c *config.Config) error {
	var groups [][]discover.Skill
	for _, r := range c.Repos {
		skills, warns, err := discover.Walk(c.SkillRoot(r), r.URL, c.WarnNameMismatch)
		if err != nil {
			return err
		}
		printWarnings(warns)
		groups = append(groups, skills)
	}

	desired, collisions := discover.Resolve(groups)
	printWarnings(collisions)

	rep, err := link.Sync(desired, c.SkillPaths, c.CacheDir)
	if err != nil {
		return err
	}

	fmt.Printf("skills linked: %d\n", len(desired))
	for _, s := range rep.Created {
		fmt.Println("  + created ", s)
	}
	for _, s := range rep.Repointed {
		fmt.Println("  ~ repointed", s)
	}
	for _, s := range rep.Removed {
		fmt.Println("  - removed  ", s)
	}
	for _, s := range rep.Foreign {
		fmt.Println("  ! foreign (left untouched):", s)
	}
	for _, w := range rep.Warnings {
		fmt.Println("  ", w)
	}
	return nil
}

// runPruneCache drops cache clones of repos that left the config. It runs
// before the link phase on purpose: the symlinks into a removed clone are then
// already dangling, so the existing link GC reports them as removed instead of
// leaving owned links pointing at nothing.
func runPruneCache(c *config.Config) error {
	keep := make(map[string]bool, len(c.Repos))
	for _, r := range c.Repos {
		keep[config.RepoName(r.URL)] = true
	}

	rep, err := prune.Run(c.CacheDir, keep)
	if err != nil {
		return err
	}
	for _, d := range rep.Removed {
		fmt.Println("  - pruned   ", d)
	}
	for _, k := range rep.Kept {
		fmt.Fprintln(os.Stderr, "warning: kept orphaned clone", k)
	}
	return nil
}

func printWarnings(ws []discover.Warning) {
	for _, w := range ws {
		fmt.Println("warning:", w.Message)
	}
}
