# Migrations

Steps to upgrade from the previous release, newest first. The release
workflow copies a version's section to the top of its GitHub release notes,
so links here are absolute. Steps for changes not yet released wait under
`## Unreleased`, which becomes the version's heading when it is tagged.

## Unreleased

New default keys: `i` jumps to the next NEW ticket in the list, and `ctrl+d`
saves a comment draft and closes the box. If your `[keys]` already binds `i`
in `keys.main` or `ctrl+d` in `keys.input` to another binding, asanamate
names the clash and stops; move that binding to a free key, or rebind
`keys.main.next_new` or `keys.input.save_draft`.

## v0.2.0

v0.2.0 moves the color settings. The updater also migrates the earlier
`display`, `list.filter`, and `repo` settings. Run `asanamate config update` to rewrite
your config; it keeps your values and saves the old file as a timestamped
`config.toml.<time>.bak`. Until then, asanamate names the first moved key and stops.

| v0.1.0 | v0.2.0 |
|---|---|
| `theme` | `theme.name` |
| `accent_color` | `colors.accent` |
| `list.header.color` | `colors.header` |
| `list.pinned.color` | `colors.pinned` |
| `list.viewing.color` | `colors.viewing` |
| `list.selection.color` | `colors.selection` |
| `display.symbols` | `symbols` |
| `display.reduced_motion` | `reduced_motion` |
| `list.filter` | `default_filter` |
| `repo.root` | `repo_source.root` |
| `repo.command` | `repo_source.command` |
| `repo.branch_field` | `branch_field` |

The Harvest time tracking provider moved out of asanamate into its own
binary, `asanamate-hrvst`, in the same release archive (see [Install](https://github.com/sadmachine/asanamate#install)).
Install it, then run `asanamate config update`: it rewrites
`time_tracking.command = "asanamate time-provider hrvst --task-id <id>"` to
`"asanamate-hrvst --task-id <id>"`, keeping the flags. Until then, asanamate
names the old command and stops. `asanamate doctor` no longer checks hrvst;
run `asanamate-hrvst check` instead.

Default keys changed too: `enter` in the list opens the reader; `c` adds a
comment (was `C`); `y` copies the link (was `c`); `Y` copies the targeted
comment (was `y`); `@` assigns (was `A`); `A` opens the action builder (was
`ctrl+a`); `ctrl+/` opens the filter guide (was `?`), and `esc` in the filter
restores the previous filter. Pickers start in browse mode; press `/` to
search, or set `picker.type_first = true`.

The ccmux agent source moved out of asanamate into the `asanamate-ccmux`
adapter, which ships in the same archive. Install it next to `asanamate` (see
[Agents](https://github.com/sadmachine/asanamate#agents-optional)); `asanamate config update` rewrites the preset:

| v0.1.0 | v0.2.0 |
|---|---|
| `agents.preset = "ccmux"` | `agents.command = "asanamate-ccmux"` |
| `asanamate setup hooks` | `asanamate-ccmux setup` |
| `asanamate hook codex` (Codex hook) | `asanamate-ccmux hook codex` |

Codex hooks that run `asanamate hook codex` keep working in v0.2.0, and
`doctor` warns about them in its `codex_hook` row; v0.3.0 drops that command.
Run `asanamate-ccmux setup` to replace the hook.

New installs get the pager action as `actions/pager.toml.example`, inactive
until renamed. An existing `actions/pager.toml` is left as it is.
