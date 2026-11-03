# skillissue

Manage LLM agent skills across git repos and agent skill directories.

Upstream skill repos are cloned into a local cache. Every folder containing a
`SKILL.md` is treated as a skill and symlinked into your configured agent skill
directories (Claude, crush, etc.). Edits agents make through those symlinks land
in the local clone and can be committed and pushed back upstream.

## Install

With Go:

```
go build -o skillissue .
```

With Nix (flake). The package wraps the binary so `git` is always on `PATH`:

```
nix build            # result/bin/skillissue
nix run . -- status  # run without installing
nix profile install  # install into your profile
```

`nix develop` drops you into a shell with go, gopls, and git.

Either way skillissue shells out to the `git` binary at runtime; auth uses your
ssh-agent / git credential helpers / `~/.gitconfig`.

## Configure

Copy `config.example.yaml` to `~/.config/skillissue/config.yaml` (honors
`XDG_CONFIG_HOME`) or pass `--config <path>`. See that file for all options.

Key points:
- `writable` defaults to **false**: repos are pull-only unless you opt in to push.
- `subdir` scopes discovery (and the symlink target) to a path within a clone.
- Name collisions across repos resolve first-wins (config order); the shadowed
  skill is warned about. No prefixing, since some agents require the skill folder
  name to equal the `name:` in its frontmatter.

## Commands

```
skillissue update    # clone/pull/commit/push all repos, then relink + GC
skillissue link      # relink + GC only, no network
skillissue status    # show repo state, discovered skills, collisions (read-only)
```

Global flags: `--config <path>`, `--dry-run` (print intended actions, change
nothing).

`update` and `link` also take `--prune-cache` (or `prune_cache: true` in the
config), which deletes cache clones of repos that are no longer configured.
Without it they are kept forever: the link phase only walks `skill_paths` and
never cleans `cache_dir`. A clone with uncommitted changes, unpushed commits or
no upstream is kept and reported instead of removed, and anything in the cache
that is not a git working tree is left alone.

### `update` per-repo logic

1. Clone if missing.
2. If `writable` and the working tree is dirty, commit with a default message.
3. `git fetch`.
4. Based on ahead/behind vs upstream:
   - remote ahead only -> fast-forward (`--ff-only`),
   - local ahead only + writable -> push,
   - local ahead only + not writable -> warn, skip,
   - **both moved (diverged)** -> leave untouched, report at the end. Resolve
     manually in the cache clone (rebase/merge), then re-run.
5. Relink all skills into every `skill_path` and remove stale owned links.

## Symlink ownership

A symlink in a skill directory is considered owned by skillissue if it resolves
into `cache_dir`. Only owned links are repointed or garbage-collected. Real
files and foreign symlinks are never touched (they are reported so you notice
collisions with your own skills).

`--prune-cache` works on the other side of that boundary: it removes clone
directories under `cache_dir` itself, and only direct children that are git
working trees. It runs before the link phase, so links into a removed clone are
already dangling and get garbage-collected in the same run.

## Scope

Out of scope for now: nested skills (a skill inside another skill dir),
credential management, and automatic conflict resolution. This shit is a simple tool.
