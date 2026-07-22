package cmd

import (
	"fmt"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/discover"
	"github.com/gurkan/skillissue/internal/link"
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

func printWarnings(ws []discover.Warning) {
	for _, w := range ws {
		fmt.Println("warning:", w.Message)
	}
}
