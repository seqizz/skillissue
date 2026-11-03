// Package prune removes cache clones of repos that are no longer configured.
//
// Dropping a repo from the config leaves its clone behind forever: the link
// phase only ever walks skill_paths and treats cache_dir as an ownership
// boundary, never as something to clean. That is mostly harmless, but it keeps
// a full copy of a repo's content on disk after the config stopped admitting
// it, which matters when repo membership is a profile switch rather than an
// edit (see the work specialisation in the nixos flake).
//
// Deleting a clone can destroy work that exists nowhere else, so every orphan
// is checked for unpublished state first and kept when in doubt.
package prune

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gurkan/skillissue/internal/git"
)

// DryRun, when true, makes Run report intended removals without performing them.
var DryRun bool

// Report accumulates what Run did, for a human-readable summary.
type Report struct {
	Removed []string // absolute paths of removed clone dirs
	Kept    []string // "<path>: <reason>" for orphans deliberately left alone
}

// Run removes direct children of cacheDir whose name is not in keep. keep holds
// the cache directory names of the currently configured repos, i.e. the output
// of config.RepoName for each repo URL.
//
// An orphan is kept, not removed, when it holds state that exists only there:
// a dirty working tree, commits ahead of upstream, or no upstream at all (which
// makes "is this published?" unanswerable). Entries that are not git working
// trees are kept too, so an unrelated file dropped into the cache survives.
func Run(cacheDir string, keep map[string]bool) (*Report, error) {
	rep := &Report{}

	entries, err := os.ReadDir(cacheDir)
	if os.IsNotExist(err) {
		return rep, nil // nothing cloned yet
	}
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if keep[e.Name()] {
			continue
		}
		// IsDir is false for a symlink, so a symlinked clone is never followed
		// and never removed. Only plain directories are candidates.
		if !e.IsDir() {
			continue
		}

		dir := filepath.Join(cacheDir, e.Name())
		if reason := keepReason(dir); reason != "" {
			rep.Kept = append(rep.Kept, fmt.Sprintf("%s: %s", dir, reason))
			continue
		}

		if DryRun {
			rep.Removed = append(rep.Removed, fmt.Sprintf("[dry-run] rm -r %s", dir))
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return nil, err
		}
		rep.Removed = append(rep.Removed, dir)
	}

	// ReadDir is already sorted, but keep the output stable regardless.
	sort.Strings(rep.Removed)
	sort.Strings(rep.Kept)
	return rep, nil
}

// keepReason returns a human-readable reason to spare dir, or "" when dir is a
// published git clone that can be re-created by a plain `skillissue update`.
func keepReason(dir string) string {
	if !git.Exists(dir) {
		return "not a git working tree"
	}
	dirty, err := git.IsDirty(dir)
	if err != nil {
		return fmt.Sprintf("cannot read git status (%v)", err)
	}
	if dirty {
		return "uncommitted changes"
	}
	behind, ahead, hasUpstream, err := git.AheadBehind(dir)
	if err != nil {
		return fmt.Sprintf("cannot compare with upstream (%v)", err)
	}
	if !hasUpstream {
		return "no upstream, cannot tell if commits are published"
	}
	if ahead > 0 {
		return fmt.Sprintf("%d unpushed commit(s)", ahead)
	}
	_ = behind // being behind is fine: a re-clone picks the remote state up
	return ""
}
