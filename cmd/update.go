package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gurkan/skillissue/internal/config"
	"github.com/gurkan/skillissue/internal/git"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Clone/pull/commit/push all repos, then relink skills",
	Long: "For each repo: commit local edits (if writable), fetch, fast-forward " +
		"when the remote is ahead, push when the local is ahead, and rebase when " +
		"both have moved (only for writable repos). Diverged non-writable repos " +
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

		if wantPruneCache(c) {
			if err := runPruneCache(c); err != nil {
				return err
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
		if committed, files, err := git.CommitAll(dir, c.CommitMessage); err != nil {
			return err
		} else if committed {
			skills := extractSkillNames(files, c.SkillRoot(r))
			if len(skills) > 0 {
				fmt.Printf("%s: committed local edits (%s)\n", r.URL, formatSkills(skills))
			} else {
				fmt.Printf("%s: committed local edits\n", r.URL)
			}
			if extra := nonSkillFiles(files); len(extra) > 0 {
				fmt.Fprintf(os.Stderr, "warning: %s: committed non-SKILL.md files: %s\n", r.URL, strings.Join(extra, ", "))
			}
		}
	} else {
		// Non-writable: warn if local changes exist — skillissue won't commit them.
		if dirty, err := git.IsDirty(dir); err != nil {
			return err
		} else if dirty {
			files, _ := git.StatusFiles(dir)
			fmt.Fprintf(os.Stderr, "warning: %s: non-writable repo has local changes: %s\n", r.URL, strings.Join(files, ", "))
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
	case behind > 0 && ahead > 0 && r.Writable:
		fmt.Printf("%s: diverged (behind %d, ahead %d), rebasing\n", r.URL, behind, ahead)
		return git.Rebase(dir)
	case behind > 0 && ahead > 0 && !r.Writable:
		*diverged = append(*diverged, r.URL)
		return nil
	default:
		fmt.Printf("%s: up to date\n", r.URL)
		return nil
	}
}

// extractSkillNames extracts unique skill directory names from a list of changed file paths.
// The file paths are relative to the skill root, so we just need to extract the first component.
func extractSkillNames(files []string, skillRoot string) []string {
	skillSet := make(map[string]bool)
	for _, file := range files {
		// File paths are relative to the skill root.
		// Extract the first directory component (the skill name).
		if idx := strings.Index(file, "/"); idx >= 0 {
			skillSet[file[:idx]] = true
		}
		// If there's no "/", the file is directly in the skill root, not a skill.
	}
	// Convert to sorted slice
	skills := make([]string, 0, len(skillSet))
	for skill := range skillSet {
		skills = append(skills, skill)
	}
	// Sort for consistent output
	for i := 0; i < len(skills); i++ {
		for j := i + 1; j < len(skills); j++ {
			if skills[i] > skills[j] {
				skills[i], skills[j] = skills[j], skills[i]
			}
		}
	}
	return skills
}

// nonSkillFiles returns paths from files that are not a SKILL.md within a skill
// directory. These represent unexpected changes bundled into the auto-commit.
func nonSkillFiles(files []string) []string {
	var result []string
	for _, f := range files {
		if !strings.HasSuffix(f, "/SKILL.md") {
			result = append(result, f)
		}
	}
	return result
}

// formatSkills formats a list of skill names for display.
// With 1-2 skills: shows "skill1" or "skill1, skill2"
// With 3+ skills: shows "skill1, skill2, and 2 more"
func formatSkills(skills []string) string {
	switch len(skills) {
	case 0:
		return ""
	case 1:
		return skills[0]
	case 2:
		return skills[0] + ", " + skills[1]
	default:
		return skills[0] + ", " + skills[1] + ", and " + fmt.Sprintf("%d more", len(skills)-2)
	}
}
