package cmd

import (
	"fmt"

	"github.com/gurkan/skillissue/internal/discover"
	"github.com/gurkan/skillissue/internal/git"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show repo state, discovered skills, and collisions (read-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Println("repos:")
		var groups [][]discover.Skill
		for _, r := range c.Repos {
			dir := c.CloneDir(r)
			if !git.Exists(dir) {
				fmt.Printf("  %s: not cloned\n", r.URL)
				continue
			}
			ref, _ := git.CurrentRef(dir)
			dirty, _ := git.IsDirty(dir)
			behind, ahead, hasUp, _ := git.AheadBehind(dir)
			wr := "readonly"
			if r.Writable {
				wr = "writable"
			}
			sync := "no upstream"
			if hasUp {
				sync = fmt.Sprintf("%d behind, %d ahead", behind, ahead)
			}
			fmt.Printf("  %s [%s] ref=%s dirty=%t %s\n", r.URL, wr, ref, dirty, sync)

			skills, _, err := discover.Walk(c.SkillRoot(r), r.URL, c.WarnNameMismatch)
			if err != nil {
				return err
			}
			groups = append(groups, skills)
		}

		desired, collisions := discover.Resolve(groups)
		fmt.Printf("\nskills (%d):\n", len(desired))
		for name, s := range desired {
			fmt.Printf("  %s <- %s\n", name, s.Path)
		}
		if len(collisions) > 0 {
			fmt.Println("\ncollisions:")
			printWarnings(collisions)
		}
		return nil
	},
}
