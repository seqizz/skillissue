package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Repo is a single upstream skill repository declared in the config.
type Repo struct {
	URL string `yaml:"url"`
	// Ref is an optional branch/tag to track. Empty means the remote's default HEAD.
	Ref string `yaml:"ref"`
	// Writable gates commit+push. Default false: repo is pull-only (safe default).
	Writable bool `yaml:"writable"`
	// Subdir scopes discovery (and the symlink target base) to a path within the
	// clone. Empty means the whole clone.
	Subdir string `yaml:"subdir"`
}

// Config is the top-level YAML document.
type Config struct {
	CacheDir   string   `yaml:"cache_dir"`
	SkillPaths []string `yaml:"skill_paths"`
	Repos      []Repo   `yaml:"repos"`
	// WarnNameMismatch enables a warning when a skill folder name differs from
	// its frontmatter name:. Off by default (harmless for Claude; only agents
	// like crush require them equal).
	WarnNameMismatch bool `yaml:"warn_name_mismatch"`
	// CommitMessage is used when auto-committing agent edits in writable repos.
	CommitMessage string `yaml:"commit_message"`
	// PruneCache removes cache clones of repos that are no longer configured.
	// Off by default: deleting a clone is the only destructive thing skillissue
	// can do, so it is opt-in per config or per run (--prune-cache).
	PruneCache bool `yaml:"prune_cache"`
}

const (
	defaultCacheDir      = "~/.local/cache/skillissue"
	defaultCommitMessage = "skillissue: sync agent edits"
)

// DefaultPath returns the config path honoring XDG_CONFIG_HOME, falling back to
// ~/.config/skillissue/config.yaml.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillissue", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillissue", "config.yaml")
}

// Load reads and validates the config at path, applying defaults and expanding
// all paths (~ and environment variables).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if c.CacheDir == "" {
		c.CacheDir = defaultCacheDir
	}
	if c.CommitMessage == "" {
		c.CommitMessage = defaultCommitMessage
	}
	c.CacheDir = expand(c.CacheDir)
	for i, p := range c.SkillPaths {
		c.SkillPaths[i] = expand(p)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	if len(c.SkillPaths) == 0 {
		return fmt.Errorf("config: skill_paths must list at least one directory")
	}
	if len(c.Repos) == 0 {
		return fmt.Errorf("config: repos must list at least one repository")
	}
	for i, r := range c.Repos {
		if r.URL == "" {
			return fmt.Errorf("config: repos[%d] missing url", i)
		}
	}
	return nil
}

// CloneDir returns the absolute cache path where a repo is (or will be) cloned.
// The directory name is derived from the last URL path segment minus ".git".
func (c *Config) CloneDir(r Repo) string {
	return filepath.Join(c.CacheDir, RepoName(r.URL))
}

// SkillRoot returns the directory under which skills are discovered for a repo,
// honoring the optional subdir scope.
func (c *Config) SkillRoot(r Repo) string {
	if r.Subdir != "" {
		return filepath.Join(c.CloneDir(r), r.Subdir)
	}
	return c.CloneDir(r)
}

// RepoName derives a filesystem-safe, collision-free cache directory name from a
// git URL. It qualifies the name with host and owner (not just the last path
// segment) so two repos that share a basename on different hosts/owners do not
// map to the same clone dir, e.g.
//
//	git@gitlab.innogames.de:gurkan.gur/llm-skills.git -> gitlab.innogames.de_gurkan.gur_llm-skills
//	ssh://gitea@git.gurkan.in/gurkan/llm-skills.git    -> git.gurkan.in_gurkan_llm-skills
func RepoName(url string) string {
	s := strings.TrimSuffix(url, ".git")

	// Drop the scheme (https://, ssh://, ...) if present.
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Drop userinfo (user@) that belongs to the authority, i.e. an '@' that
	// appears before the first path separator (covers scp-like git@host:path too).
	if at := strings.IndexByte(s, '@'); at >= 0 {
		if slash := strings.IndexByte(s, '/'); slash < 0 || at < slash {
			s = s[at+1:]
		}
	}
	// Flatten host/owner/repo (and the scp-like host:owner) into one segment.
	s = strings.Trim(strings.NewReplacer("/", "_", ":", "_").Replace(s), "_")
	if s == "" {
		// Degenerate URL: fall back to a sanitized slug of the whole thing.
		s = strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(url)
	}
	return s
}

// expand resolves a leading ~ and any $VAR / ${VAR} references in a path.
func expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return os.ExpandEnv(p)
}
