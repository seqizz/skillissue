package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DryRun, when true, makes mutating operations (Clone/CommitAll/FastForward/Push)
// print the intended command instead of executing it.
var DryRun bool

// run executes git in dir and returns trimmed stdout. Stderr is folded into the
// error message so callers get git's own diagnostics.
func run(dir string, args ...string) (string, error) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", full...)
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func dryLog(args ...string) {
	fmt.Printf("[dry-run] git %s\n", strings.Join(args, " "))
}

// Clone clones url into dst, optionally checking out ref.
func Clone(url, ref, dst string) error {
	args := []string{"clone"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dst)
	if DryRun {
		dryLog(args...)
		return nil
	}
	_, err := run("", args...)
	return err
}

// Fetch updates remote-tracking refs and prunes deleted ones.
func Fetch(dir string) error {
	_, err := run(dir, "fetch", "--prune")
	return err
}

// IsDirty reports whether the working tree has uncommitted changes.
func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// CommitAll stages every change and commits with msg. It is a no-op on a clean
// tree. Returns whether a commit was actually made and the list of changed files.
func CommitAll(dir, msg string) (bool, []string, error) {
	dirty, err := IsDirty(dir)
	if err != nil {
		return false, nil, err
	}
	if !dirty {
		return false, nil, nil
	}

	// Get list of changed files before committing
	files, err := ChangedFiles(dir)
	if err != nil {
		return false, nil, err
	}

	if DryRun {
		dryLog("-C", dir, "add", "-A")
		dryLog("-C", dir, "commit", "-m", msg)
		return true, files, nil
	}
	if _, err := run(dir, "add", "-A"); err != nil {
		return false, nil, err
	}
	if _, err := run(dir, "commit", "-m", msg); err != nil {
		return false, nil, err
	}
	return true, files, nil
}

// ChangedFiles returns a list of paths that have changed in the working tree.
// It uses git diff --name-only to show all modified, added, or deleted files.
func ChangedFiles(dir string) ([]string, error) {
	out, err := run(dir, "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// StatusFiles returns all dirty paths reported by git status --porcelain,
// including untracked files. Each entry is the file path only (XY prefix stripped).
func StatusFiles(dir string) ([]string, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	lines := strings.Split(out, "\n")
	paths := make([]string, 0, len(lines))
	for _, l := range lines {
		if len(l) < 4 {
			continue
		}
		// porcelain format: "XY path" or "XY old -> new" for renames
		path := strings.TrimSpace(l[3:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// AheadBehind returns how many commits the local branch is behind and ahead of
// its upstream. hasUpstream is false when no upstream is configured (e.g. a
// detached ref), in which case the counts are meaningless.
func AheadBehind(dir string) (behind, ahead int, hasUpstream bool, err error) {
	// Verify an upstream exists before asking for counts.
	if _, e := run(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); e != nil {
		return 0, 0, false, nil
	}
	out, err := run(dir, "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		return 0, 0, false, err
	}
	// Output is "<behind>\t<ahead>" (left = upstream-only, right = HEAD-only).
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, false, fmt.Errorf("git rev-list: unexpected output %q", out)
	}
	behind, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, false, err
	}
	ahead, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false, err
	}
	return behind, ahead, true, nil
}

// FastForward advances the current branch to its upstream, refusing anything
// that is not a fast-forward.
func FastForward(dir string) error {
	if DryRun {
		dryLog("-C", dir, "merge", "--ff-only", "@{u}")
		return nil
	}
	_, err := run(dir, "merge", "--ff-only", "@{u}")
	return err
}

// Rebase rebases the current branch onto its upstream. Returns an error if
// the rebase fails (e.g. due to conflicts).
func Rebase(dir string) error {
	if DryRun {
		dryLog("-C", dir, "rebase", "@{u}")
		return nil
	}
	_, err := run(dir, "rebase", "@{u}")
	return err
}

// Push pushes the current branch to its upstream.
func Push(dir string) error {
	if DryRun {
		dryLog("-C", dir, "push")
		return nil
	}
	_, err := run(dir, "push")
	return err
}

// CurrentRef returns the checked-out branch name (or a short commit for a
// detached HEAD), for status display.
func CurrentRef(dir string) (string, error) {
	ref, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if ref == "HEAD" {
		// Detached: fall back to the short commit hash.
		return run(dir, "rev-parse", "--short", "HEAD")
	}
	return ref, nil
}

// Exists reports whether dir is an existing git working tree.
func Exists(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}
