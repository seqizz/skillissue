package link

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gurkan/skillissue/internal/discover"
)

// DryRun, when true, makes Sync report intended filesystem changes without
// performing them.
var DryRun bool

// Report accumulates what Sync did, for a human-readable summary.
type Report struct {
	Created   []string // "skillpath/name -> target"
	Repointed []string
	Removed   []string // stale owned links GC'd
	Foreign   []string // untouched non-owned entries that collide with a skill name
	Warnings  []string
}

// Sync makes every skill_path contain an up-to-date symlink for each skill in
// the desired set, and garbage-collects owned links that are stale. cacheDir is
// used as the ownership boundary: a symlink is owned iff it resolves inside it.
func Sync(desired map[string]discover.Skill, skillPaths []string, cacheDir string) (*Report, error) {
	rep := &Report{}
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, err
	}

	for _, sp := range skillPaths {
		if err := ensureDir(sp, rep); err != nil {
			return nil, err
		}
		if err := linkInto(sp, desired, absCache, rep); err != nil {
			return nil, err
		}
		if err := gc(sp, desired, absCache, rep); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

func ensureDir(dir string, rep *Report) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if DryRun {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("[dry-run] mkdir -p %s", dir))
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// linkInto creates or repoints one symlink per desired skill in skill_path sp.
func linkInto(sp string, desired map[string]discover.Skill, absCache string, rep *Report) error {
	for name, skill := range desired {
		linkPath := filepath.Join(sp, name)
		target, err := filepath.Abs(skill.Path)
		if err != nil {
			return err
		}

		info, err := os.Lstat(linkPath)
		if os.IsNotExist(err) {
			if err := createLink(target, linkPath, rep); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			// A real file/dir: user's own skill, leave it, warn.
			rep.Foreign = append(rep.Foreign, linkPath)
			continue
		}

		// It is a symlink. Only touch it if we own it (resolves into cache).
		current, _ := os.Readlink(linkPath)
		currentAbs := current
		if !filepath.IsAbs(currentAbs) {
			currentAbs = filepath.Join(sp, current)
		}
		if !owned(currentAbs, absCache) {
			rep.Foreign = append(rep.Foreign, linkPath)
			continue
		}
		if currentAbs == target {
			continue // already correct
		}
		// Owned but pointing elsewhere: repoint.
		if DryRun {
			rep.Repointed = append(rep.Repointed, fmt.Sprintf("[dry-run] %s -> %s", linkPath, target))
			continue
		}
		if err := os.Remove(linkPath); err != nil {
			return err
		}
		if err := os.Symlink(target, linkPath); err != nil {
			return err
		}
		rep.Repointed = append(rep.Repointed, fmt.Sprintf("%s -> %s", linkPath, target))
	}
	return nil
}

func createLink(target, linkPath string, rep *Report) error {
	if DryRun {
		rep.Created = append(rep.Created, fmt.Sprintf("[dry-run] %s -> %s", linkPath, target))
		return nil
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return err
	}
	rep.Created = append(rep.Created, fmt.Sprintf("%s -> %s", linkPath, target))
	return nil
}

// gc removes owned symlinks in sp that are dangling or no longer in the desired
// set. Foreign entries are never removed.
func gc(sp string, desired map[string]discover.Skill, absCache string, rep *Report) error {
	entries, err := os.ReadDir(sp)
	if err != nil {
		return err
	}
	for _, e := range entries {
		linkPath := filepath.Join(sp, e.Name())
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue // not a symlink, foreign, skip
		}
		current, _ := os.Readlink(linkPath)
		currentAbs := current
		if !filepath.IsAbs(currentAbs) {
			currentAbs = filepath.Join(sp, current)
		}
		if !owned(currentAbs, absCache) {
			continue // foreign symlink, leave it
		}
		// Owned. Keep only if it matches a live desired skill by name AND target.
		if s, ok := desired[e.Name()]; ok {
			if wantAbs, _ := filepath.Abs(s.Path); wantAbs == currentAbs {
				continue
			}
		}
		// Stale (orphaned name or dangling target): remove.
		if DryRun {
			rep.Removed = append(rep.Removed, fmt.Sprintf("[dry-run] rm %s", linkPath))
			continue
		}
		if err := os.Remove(linkPath); err != nil {
			return err
		}
		rep.Removed = append(rep.Removed, linkPath)
	}
	return nil
}

// owned reports whether an absolute symlink target lies inside the cache dir.
func owned(targetAbs, absCache string) bool {
	cleaned := filepath.Clean(targetAbs)
	return cleaned == absCache || strings.HasPrefix(cleaned, absCache+string(os.PathSeparator))
}
