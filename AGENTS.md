# AGENTS.md

## Config keys

The config is TOML, decoded into the structs in `internal/config/config.go`.

- **Shape:** before adding a key, judge whether its concept will grow more
  settings. A growing concept gets its own sub-table, even with one key today
  (`[list.header]` holds `style`, `spacing`, `color`). A standalone setting
  stays a snake_case key in its parent table (`list.group_by`). When unsure,
  ask the user.
- **Every key change touches:** the struct field and its doc comment,
  `Default()`, `validate()` (errors name the dotted key, such as
  `list.header.style`), the template in `internal/setup/setup.go`, the README
  config table, and the config tests.
- **Released keys are a public contract:** `Load` rejects unknown keys, and
  `asanamate config update` (`Merge` in `internal/setup/update.go`) fails on
  keys the template lacks, so renaming, moving, or removing a released key, or
  narrowing its accepted values, breaks existing configs. Before the first
  release, change keys freely. After it, add keys and values rather than
  changing them, and make a breaking change only with the user's approval.
- **Approved breaking changes:** teach `Merge` to move the old key to the new
  one, mark the commit breaking (`feat!:` plus a `BREAKING CHANGE:` footer
  naming the old and new keys), add the upgrade steps under `## Unreleased` in
  `docs/migrations.md` (it becomes the release notes' opening section; use
  absolute links), and tell the user the next release tag needs a breaking
  version bump (see Version bumps).

## Version bumps

Releases come only from `v*` tags; there is no version file. Everything on
`main` since the latest tag (`git describe --tags --abbrev=0 main`) ships in
the next release, so the pending bump is the largest one any of those commits
needs:

| Change | Before `v1.0.0` | From `v1.0.0` |
|---|---|---|
| Breaking (`!` or `BREAKING CHANGE:`; see Config keys) | minor | major |
| `feat` | patch | minor |
| `fix`, `perf`, or another change users see | patch | patch |
| `docs`, `test`, `ci`, `chore`, `refactor` only | none | none |

A `v2` or later tag also requires the module path in `go.mod` to end in `/v2`.

After merging a branch into local `main`, read `git log --oneline <tag>..main`
and tell the user the pending bump and the version it gives, such as "patch:
v0.2.0 to v0.2.1". The user often holds several merges for one release, so
recommend a tag, never create or push one. Say whether this merge raised the
pending bump; a merge that adds only `none` commits, or no more than the bump
already pending, needs no new recommendation beyond naming it. Before a tag,
rename `## Unreleased` in `docs/migrations.md` to the tag's `## vX.Y.Z` (the
release workflow matches the exact tag); drop the heading if it is empty.

## Local config migration

When a task changes what lives in `~/.config/asanamate` (a config key or
value, an action key, a new file, or a new layout), finish the task, then ask
the user whether to migrate their local config to it. On yes:

- Copy each file you will change to a timestamped sibling first, named
  `<file>.yyyy-mm-dd--hh-mm-ss.bak` (`date +%Y-%m-%d--%H-%M-%S`), such as
  `config.toml.2026-10-06--14-30-05.bak`. Never overwrite or delete an
  existing backup. `asanamate config update` names its own backups this way.
- Bring `config.toml` and `actions/` onto the new shape, keeping every value
  the user set. `asanamate config update` handles new template keys in
  `config.toml`; everything else is a hand edit.
- Validate with the branch's build: `go run ./cmd/asanamate doctor` loads the
  config and actions and names the first bad file or key. Done when it runs
  clean; then report what changed and where the backups are.
- Check whether `~/.config/asanamate` sits in a git repo
  (`git -C ~/.config/asanamate rev-parse --show-toplevel`). If it does, ask
  the user whether to commit the changes. On yes, stage and commit only paths
  under `~/.config/asanamate`, never unrelated changes elsewhere in that repo.

## Asana writes

Every write to Asana goes through `writeback.Service`
(`internal/writeback/writeback.go`), so the TUI and the CLI share one
implementation. Add a new write there first, with tests, then expose it in
both: a TUI edit that calls the `Service` method (with no `Confirm`), and a
CLI subcommand in `cmd/asanamate/main.go` with `--yes`, its `usage` line, and
the README "Writing back to Asana" section. The TUI never calls the Asana
client's write methods directly.
