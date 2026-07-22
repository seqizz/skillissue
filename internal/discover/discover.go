package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is one discovered skill: a directory that directly contains a SKILL.md.
type Skill struct {
	Name    string // folder basename; the symlink name and collision key
	Path    string // absolute path to the skill directory in the cache clone
	RepoURL string // owning repo URL, for diagnostics
}

// Warning is a non-fatal issue surfaced during discovery (collisions, name
// mismatches). Callers decide how to print them.
type Warning struct {
	Message string
}

// Walk finds all skills under root. root is the clone (or subdir) directory for
// a repo. A directory qualifies iff it directly contains SKILL.md; on a match we
// do not descend further (nested skills are out of scope).
//
// checkNameMatch enables the soft warning when a skill folder name differs from
// its frontmatter name:. This is off by default because for Claude the folder
// name is the id and name: is only a title, so a mismatch is normal; only some
// agents (e.g. crush) require them equal.
func Walk(root, repoURL string, checkNameMatch bool) ([]Skill, []Warning, error) {
	var skills []Skill
	var warns []Warning

	if _, err := os.Stat(root); os.IsNotExist(err) {
		// A configured subdir that does not exist yet is a warning, not fatal.
		return nil, []Warning{{Message: fmt.Sprintf("%s: skill root %s does not exist", repoURL, root)}}, nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Skip the .git directory entirely.
		if d.Name() == ".git" {
			return fs.SkipDir
		}
		skillFile := filepath.Join(path, "SKILL.md")
		if info, statErr := os.Stat(skillFile); statErr == nil && !info.IsDir() {
			name := filepath.Base(path)
			skills = append(skills, Skill{Name: name, Path: path, RepoURL: repoURL})
			// Soft check (opt-in): crush-style agents require folder name == frontmatter name.
			if checkNameMatch {
				if fmName := frontmatterName(skillFile); fmName != "" && fmName != name {
					warns = append(warns, Warning{Message: fmt.Sprintf(
						"%s: skill folder %q has frontmatter name %q (mismatch may break some agents)", repoURL, name, fmName)})
				}
			}
			// Nested skills out of scope: stop descending into this skill.
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, warns, fmt.Errorf("discover %s: %w", root, err)
	}
	return skills, warns, nil
}

// Resolve merges skills from multiple repos in order, resolving name collisions
// with first-wins. Shadowed skills are collapsed into one warning per shadowing
// repo (a full mirror would otherwise flood the output with one line per skill).
func Resolve(all [][]Skill) (map[string]Skill, []Warning) {
	resolved := make(map[string]Skill)
	// Group shadowed skill names by their repo, preserving first-seen repo order.
	shadowed := make(map[string][]string)
	var order []string
	for _, group := range all {
		for _, s := range group {
			if _, dup := resolved[s.Name]; dup {
				if _, seen := shadowed[s.RepoURL]; !seen {
					order = append(order, s.RepoURL)
				}
				shadowed[s.RepoURL] = append(shadowed[s.RepoURL], s.Name)
				continue
			}
			resolved[s.Name] = s
		}
	}
	var warns []Warning
	for _, repo := range order {
		names := shadowed[repo]
		warns = append(warns, Warning{Message: fmt.Sprintf(
			"%d skill(s) from %s shadowed by earlier repo(s), first-wins (mirror?): %s",
			len(names), repo, strings.Join(names, ", "))})
	}
	return resolved, warns
}

// frontmatterName extracts the `name:` field from a leading YAML frontmatter
// block delimited by `---`. Returns "" if absent or unparseable (non-fatal).
func frontmatterName(skillFile string) string {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return ""
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	// Find the closing delimiter after the opening one.
	rest := content[len("---"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	var fm struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return ""
	}
	return fm.Name
}
