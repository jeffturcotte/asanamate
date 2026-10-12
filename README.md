# asanamate

An Asana client for the terminal, built for the Asana work you do every day:
read your tickets, update them, and create new ones without leaving the
keyboard. It fits the way you already use your terminal, full screen or in a
tmux pane, sidebar, or popup ([Layout](#layout), [tmux](#tmux)). Your own
commands extend it, and its CLI offers every write the TUI makes, so scripts
and coding agents can do what you can. macOS and Linux only.

## What it does

- **Browse** My Tasks or any project: filter, group, sort, pin tickets, and
  save views. See [Filtering](#filtering).
- **Read** a ticket's fields, description, subtasks, comments, reactions, and
  attachments, with images inline in kitty-graphics terminals. See
  [Layout](#layout).
- **Edit** a ticket: comment (with images), move it to a section, assign or
  unassign it, set its due date or a custom field, add or remove projects, and
  mark it complete or reopen it. React to a comment with an emoji. See
  [Keys](#keys).
- **Create** a ticket in a project or My Tasks, or a subtask. See
  [Keys](#keys).
- **Script** every edit and create from the CLI: `asanamate comment`, `move`,
  `field`, `assign`, `due`, `project`, `complete`, `react`, and `create`. See
  [Writing back to Asana](#writing-back-to-asana).
- **Act** on a ticket: run your own commands with the ticket as context, such
  as starting a coding agent in its repo. See [Actions](#actions).
- **Show agents** (optional): coding agents running in ccmux or any other tool
  appear next to their tickets. See [Agents](#agents-optional).
- **Track time** (optional): log time to Harvest from a ticket. See
  [Time tracking](#time-tracking-optional).
- **Suggest links** (optional): preselect each project's repo and Harvest
  project from imarc's Hub or another source. See
  [Project links](#project-links-optional).
- **Follow a feed** at a glance: a panel under the list shows today's related
  Asana activity, agent state changes, unpushed git work on ticket branches,
  and lines from any feed command, such as Harvest time entries. See
  [Feed panel](#feed-panel).

**Extending asanamate.** External commands plug in at six points:
[actions](docs/extensions.md#actions),
[agent sources](docs/extensions.md#agent-sources),
[time tracking providers](docs/extensions.md#time-tracking),
[repo sources](docs/extensions.md#repo-sources),
[project links](docs/extensions.md#project-links), and
[feeds](docs/extensions.md#feeds).
[docs/extensions.md](docs/extensions.md) documents each protocol.

**Left to Asana.** asanamate does not rename or delete tickets, change a
description after creating a ticket, edit or delete comments, set tags,
followers, or dependencies, or create or change projects and sections.

**Early release.** asanamate is pre-1.0: expect breaking changes, including to
config keys, before 1.0. Release notes call out each one.

## Install

asanamate needs macOS or Linux on amd64 or arm64, a terminal, an Asana account
with a personal access token, and network access. Everything else is optional;
see [Optional features](#optional-features).

### Release archive

Each [GitHub release](https://github.com/sadmachine/asanamate/releases) has an
archive per system, `asanamate_<os>_<arch>.tar.gz`, and `checksums.txt`. Set
`version` to the release you want; the rest detects your system:

```sh
version=vX.Y.Z
os=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin or linux
arch=$(uname -m); case $arch in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; esac
archive=asanamate_${os}_${arch}.tar.gz
base=https://github.com/sadmachine/asanamate/releases/download/$version
curl -fL -O "$base/$archive" -O "$base/checksums.txt"
grep " $archive\$" checksums.txt | sha256sum -c   # macOS: shasum -a 256 -c
tar -xzf "$archive" asanamate themes install-themes.sh
mkdir -p ~/.local/bin && mv asanamate ~/.local/bin/
sh ./install-themes.sh
asanamate version
```

The archive also holds three optional binaries: `asanamate-hrvst`, the Harvest
provider for [Time tracking](#time-tracking-optional), `asanamate-ccmux`,
the ccmux adapter for [Agents](#agents-optional), and `asanamate-hub`, the Hub
provider for [Project links](#project-links-optional). Extract and move the
ones you use the same way:

```sh
tar -xzf "$archive" asanamate-hrvst && mv asanamate-hrvst ~/.local/bin/
tar -xzf "$archive" asanamate-ccmux && mv asanamate-ccmux ~/.local/bin/
tar -xzf "$archive" asanamate-hub && mv asanamate-hub ~/.local/bin/
```

If `asanamate version` is not found, add `~/.local/bin` to your `PATH` in your
shell's startup file (`~/.zshrc`, `~/.bashrc`), then open a new shell:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

On macOS, files downloaded with `curl` run as is. The binary is not notarized,
so if you downloaded the archive with a browser, macOS blocks it; clear the
quarantine flag once:

```sh
xattr -d com.apple.quarantine ~/.local/bin/asanamate
xattr -d com.apple.quarantine ~/.local/bin/asanamate-hrvst   # if you use it
xattr -d com.apple.quarantine ~/.local/bin/asanamate-ccmux   # if you use it
xattr -d com.apple.quarantine ~/.local/bin/asanamate-hub     # if you use it
```

### From source

Needs Go 1.26 or later. Install from a checkout so the standard theme files
are available alongside the binaries:

```sh
git clone https://github.com/sadmachine/asanamate.git
cd asanamate
go install ./cmd/asanamate
sh ./install-themes.sh
go install ./cmd/asanamate-hrvst   # Harvest time tracking
go install ./cmd/asanamate-ccmux   # ccmux agents
go install ./cmd/asanamate-hub     # Hub project links
```

This installs to `$(go env GOBIN)`, else `$(go env GOPATH)/bin` (usually
`~/go/bin`); add that directory to your `PATH` as above.

### Upgrade and uninstall

Upgrade the way you installed: replace the binary with a newer archive, or
rerun `go install` from an updated checkout. Run `sh ./install-themes.sh`
to add any new themes; existing theme files are never overwritten. Upgrades
never touch your config, actions, or state.

To uninstall, delete the binaries (`asanamate`, plus `asanamate-hrvst`,
`asanamate-ccmux`, and `asanamate-hub` if you installed them). Your
files stay until you delete them yourself: the config and actions in
`~/.config/asanamate`, the state, ticket exports, and logs in
`~/.local/state/asanamate`, and the Codex hook, if you added it, in
`~/.codex/hooks.json`.

## Setup

1. Create a personal access token at <https://app.asana.com/0/my-apps> and
   put it in `ASANA_ACCESS_TOKEN` (see [Token](#token)).
2. Run `asanamate setup`. It checks the token, asks for your workspace and, optionally, the
   directory that holds your git repositories (`-` skips it), and writes
   `~/.config/asanamate/config.toml` with every default explained, plus a
   pager action and a Claude example in `~/.config/asanamate/actions/`. It
   warns when the pager action's pager (`$PAGER`, else `less`) is not
   installed.
3. Run `asanamate`. `asanamate doctor` checks the setup if something fails.

### Token

asanamate reads the token only from `ASANA_ACCESS_TOKEN` and never writes it
to a file or edits your shell files.

- For the current shell only: `export ASANA_ACCESS_TOKEN=...`
- For every new shell: add that line to your shell's startup file
  (`~/.zshrc`, `~/.bashrc`), or load the token there from a password manager.
- tmux keeps the environment it started with, so panes and popups of a tmux
  server started before you set the token lack it. Copy it in with
  `tmux set-environment -g ASANA_ACCESS_TOKEN "$ASANA_ACCESS_TOKEN"`, or restart
  tmux. Other launchers, such as desktop shortcuts, need it in their own
  environment the same way.

### Optional features

Browsing needs none of these. Each one affects only its own feature, and
`asanamate doctor` reports which are usable.

| Feature | Needs | Turn on | Turn off | `doctor` row | Without it |
|---|---|---|---|---|---|
| Pager action (`v`) | `$PAGER`, else `less` (your package manager) | on by default | delete `actions/pager.toml` | `pager` | the action fails; setup and `doctor` warn |
| `asanamate config` | `$VISUAL` or `$EDITOR`, else `vi` | set `$EDITOR` | not needed | `editor` | edit `config.toml` directly |
| Repo actions, agent branch matching | [git](https://git-scm.com/downloads) | install git | not needed | `git` | browsing works; repo actions report that git is missing |
| Repo picker | a directory of clones | `repo_source.root` or `repo_source.command` | leave both empty | `repo_source` | type a repo path when an action asks |
| Open in browser (`o`, `f`) | macOS `open`; Linux [xdg-utils](https://www.freedesktop.org/wiki/Software/xdg-utils/) and a desktop browser | always on | not needed | `browser` | the status line shows the error and the URL to copy |
| Copy link or comment (`c`, `y`) | terminal OSC 52 support; in tmux, `set -g set-clipboard on` | always on | not needed | not checked | nothing is copied; asanamate cannot detect this |
| Paste image into comment (`alt+v`) | macOS `osascript` (built in); Linux `wl-clipboard` (Wayland) or `xclip` (X11). On macOS the terminal must send Option as Alt (kitty `macos_option_as_alt`, Ghostty `macos-option-as-alt`, iTerm2 "Esc+") | always on | not needed | not checked | the comment box shows why; type the image's Markdown yourself |
| Paste into input (`Ctrl+V`) | macOS `pbpaste`; Linux `xclip` or `xsel` (X11) or `wl-clipboard` (Wayland) | always on | not needed | not checked | the input shows why, and your terminal's paste still works |
| Agent status | an agents command, such as `asanamate-ccmux` for ccmux; see [Agents](#agents-optional) | `agents.command` | remove `[agents]` keys | `agents` | off |
| Time tracking | `asanamate-hrvst` and [hrvst](https://github.com/kgajera/hrvst-cli) 3.x, or your own provider; see [Time tracking](#time-tracking-optional) | `[time_tracking]` | remove `[time_tracking]` keys | `time_tracking` | off |
| Project link suggestions | a project links command, such as `asanamate-hub` for imarc's Hub; see [Project links](#project-links-optional) | `project_links.command` | remove `project_links.command` | `project_links` | no suggestions |
| Inline images | a kitty-graphics terminal ([kitty](https://sw.kovidgoyal.net/kitty/), [Ghostty](https://ghostty.org)); in tmux, `set -g allow-passthrough on` | `images.mode = "auto"` (default) | `images.mode = "off"` | not checked | links shown instead |
| Nerd Font symbols | a [Nerd Font](https://www.nerdfonts.com) | `symbols = "nerd"` | `symbols = "unicode"` or `"ascii"` | not checked | unicode or ascii symbols |
| Follow OS reduced motion | macOS `defaults`; GNOME `gsettings` | leave `reduced_motion` unset | set `reduced_motion` | not checked | set `reduced_motion` yourself |

Run `asanamate doctor` after turning a feature on: its row reads `ok` when the
tools it needs are present, and the `warning` text names what is missing.

Actions you write run your own commands; installing what they use is up to you.

`asanamate config` opens the config file in `$VISUAL` or `$EDITOR` (falling
back to `vi`) and reports any errors in it after you save.

`asanamate config update` rewrites the config from the current template so it
picks up new settings and comments, keeping every value you set. The previous
file is saved beside it as a timestamped backup, such as
`config.toml.2026-10-06--14-30-05.bak`; copy any comments of your own back
from it.
Actions live in their own files, so updates never touch them.

## Layout

At 100 columns and wider, the ticket list and the reader sit side by side in
bordered panels; the focused one has an accent-colored border. The list widens
to fit full ticket titles, but leaves the reader at least 80 columns and cuts
titles that no longer fit. At 160 columns
and wider, a views panel on the left lists My Tasks, recent projects, the
groupings, sorting, and running agents: press `0`, move with `j`/`k`, and open one
with `enter`. Press `Z` to show or hide this panel independently of the reader.
It hides automatically below 160 columns. In a reader at least 90 columns wide,
the cards view puts the agents and subtasks beside the details card. In narrower terminals the list
fills the screen and `enter` opens the reader in its place.

Fit asanamate to its spot with the `[layout]` table in `config.toml`:

- `reader = false` starts with the reader hidden, such as for a tmux
  sidebar. Press `z` to show or hide the reader for the session; hiding it
  moves reader focus to the list. `--hide-preview` also starts with the reader
  hidden; `z` still toggles it.
- `list_width = 60` fixes the list at 60 columns, borders included, and gives
  the reader the rest of the width. `=` and the automatic fitting leave it
  alone. When fewer than 30 columns would be left for the reader beside the
  list and the views panel, the reader hides, as in narrow terminals.

```toml
[layout]
reader = true
list_width = 60
```

Select `+ Add project` below the ticket's project rows and press `enter` to
add it to another project. The picker excludes projects already on the ticket.

The bottom bar shows the mode (`NORMAL`, `READ`, `EDIT`, `FILTER`, `VIEWS`,
or `FEED`), the view, filter, grouping, sorting, ticket count, and agents, with the keys
for what has focus on the right. Messages take the keys' place until the next
key press.

## Keys

| Key | Action |
|---|---|
| `j`/`k` | move in the list; in cards view, select the next or previous field, section, or comment; in markdown view, scroll |
| up/down arrows | move in the list or scroll the reader |
| `g`/`G` | first/last ticket; in the reader, jump to its top or bottom (in cards view, selecting the first or last field, section, or comment) |
| `ctrl+d`/`ctrl+u` | half page down/up in the list, or scroll the reader half a page |
| `ctrl+f`/`ctrl+b` (or pgdown/pgup) | page down/up in the list, or scroll the reader a page |
| `tab`/`shift+tab`, `l`/`h` | focus the next/previous pane: the views panel (wide screens), the list, and the reader; in the cards view, entering the reader selects its first editable row |
| `enter` | in the list, open the ticket in the reader; in the reader, edit the selected row (Add comment opens the comment editor; Show all comments lists the older ones, since the cards view shows the newest `reader.comments.latest`), show a comment cut to its preview in full, or open the selected comment's menu: `r` reacts with an emoji (`1`-`9` pick one; picking one you reacted with removes it) |
| `esc` | return focus to the list |
| `0` / `1` / `2` / `3` | focus the views panel (wide screens) / the list / the reader / the Feed panel (expanding it) |
| `F` | expand or collapse the Feed panel until the view changes |
| `/` | edit the filter (available fields appear while editing; `ctrl+/` opens the filter guide; `enter` keeps the filter, `esc` restores the previous one) |
| `p` | switch project (recent first) |
| `I` | open Activity: your actions and changes directly related to you |
| `H` | ticket history: the last 10 tickets focused in the reader, most recent first; picking one selects it (keeping it in the Viewing section when the view doesn't show it) and focuses the reader |
| `L` | repo links: link, relink, or unlink each project's repo (starts on the viewed project), or give the selected ticket its own repo that overrides its projects' links without changing them |
| `space`, `a` | run an action on the selected ticket |
| `A` | create or edit action files (also available in Settings) |
| `P` | pin / unpin the selected ticket; pins persist locally per project or My Tasks, above filtered tickets |
| `e` | edit the selected ticket: add a comment, move it to a section (of a project, or of My Tasks when it is yours), add or remove a project, set a custom field (text, number, date, single- or multi-select, people), assign it, mark it complete or reopen it, or set its branch (saved locally, overrides `branch_field`; empty removes it) |
| `c` | add a comment to the selected ticket, from the list or the reader; in the comment box, `alt+v` pastes a clipboard image (see [Writing back to Asana](#writing-back-to-asana)) |
| `d` / `m` / `@` | set the due date / move to a section / assign, skipping the `e` menu |
| `x` | mark the selected ticket complete, or reopen it when it is complete; a ticket the filter now hides stays in Viewing until you move away, so `x` again undoes it |
| `n` | create a ticket from a title: in the viewed project (in the selected ticket's section when the list is grouped by section), or in My Tasks assigned to you; then select it in the reader (under Viewing when the filter hides it). Set other fields with the edit keys |
| `N` | create a subtask of the selected ticket from a title, then show the ticket in the reader |
| `.` | run the last picked action again on the selected ticket |
| `t` | log completed time for the selected ticket (only when time tracking is configured) |
| `f` | attachments: view images inline (`j`/`k` step between them) or open in the browser |
| `b` | group the list by a field (built-ins, `list.fields`, or a custom field on the loaded tickets) |
| `B` | sort by field and direction, or restore Asana order |
| `=` | fit the list pane to its content (also on project, grouping, sorting, and filter changes); does nothing while `layout.list_width` is set |
| `v` | switch the reader between the cards and markdown views |
| `z` | show or hide the reader for the session (see [Layout](#layout)) |
| `Z` | show or hide the views sidebar; automatically hidden below 160 columns |
| `V` | recall or manage saved views |
| `]` / `[` | apply the next/previous saved view, in name order |
| `ctrl+s` | save the current filter, grouping, and sorting under a name |
| `s` | settings: separators, header spacing, reader view, auto-update interval, selection style, and project list columns; changes persist between sessions |
| `o` | open the ticket in the browser |
| `y` | copy the selected ticket's link to the clipboard (requires terminal OSC 52 support) |
| `Y` | copy the targeted comment as Markdown in the cards reader (requires terminal OSC 52 support); does nothing on other targets |
| `r` | reload tickets, rerun the Feed panel's feeds and git check, and clear the inline image cache (including failed loads) |
| `R` | set the automatic reload interval (persists between sessions) |
| `?` | show every key |
| `q` | quit |

Pickers start in browse mode: `j`/`k` move, `/` searches, `enter` picks, and
`esc` stops searching or closes. Multi-select pickers check items with `space`
or `tab` and confirm with `enter`. In the repo picker, `ctrl+s` (or the
`Use "<path>"` row) uses the typed path. Set `picker.type_first = true` to
open pickers in search mode.

The action menu, the `e` edit menu, and the settings menu work differently:
each item has its own key, shown in brackets, which picks it at once. They
have no search; arrows and `ctrl+n`/`ctrl+p` move, and `esc` closes. Action
keys come from each action file; edit and settings menu keys are fixed.

Every key above can be changed in the `[keys]` tables of `config.toml`; see
[Configuration](#configuration).

In text editors (comments, field values, action input, and action builder text),
press `ctrl+e` to edit the current draft in `$VISUAL`, then `$EDITOR`, or `vim`
if neither is set. Remap it with `keys.input.external_editor`,
`keys.form.external_editor`, and `keys.form_field.external_editor`.
For example, launch with `VISUAL=nvim asanamate`.
Editor settings may include flags, such as `VISUAL='vim -f'`.
Save and quit (`:wq` in Vim) to return to the same draft; `ctrl+s` still submits.
Quit without saving (`:q!`) to keep the previous draft. Editor failures also keep
the previous draft and show an error. The private temporary buffer is deleted
when the editor exits. Search and filter inputs keep their inline controls.

Inline images are cached by attachment for the current session. Manual reload
downloads the displayed ticket's images again; automatic reload keeps the cache.
The attachment viewer downloads its image each time it opens.

Manually pinned tickets appear in **Pinned**, regardless of the current filter.
Pins stay local to asanamate, persist across restarts, and belong to the project
or My Tasks where you pinned them. Pinning does not change the task in Asana.
Pinned tickets appear only once, even when they match the filter. Completed
pins remain until you unpin them.

**Viewing** temporarily keeps an edited or opened ticket visible when the
current view excludes it. Moving to another ticket clears it. Unpinning a
ticket outside the filter keeps it in Viewing until you move away.

In My Tasks, a `NEW` badge after the title marks tickets that changed since
you last viewed them, including tickets newly assigned to you. The `new` color
role styles it; the selected row shows it reversed. Asana counts new comments, assignment, completion, and edits
to the name, due date, description, or custom fields as changes; moving a
ticket between projects is not. The mark clears once the ticket loads in the
reader. The first My Tasks load marks nothing.

## Filtering

Space-separated terms, all of which must match:

- words match the title
- `section:`, `project:`, `assignee:`, `tag:` match names
- `project:<name>[<section>]` matches a section within that project, such as
  `project:web[backlog]` or `project:web["in progress"]`
- `is:open`, `is:done`
- `due:overdue`, `due:today`, `due:week` (the next 7 days, today included),
  `due:none`
- `agent:any`, `agent:none`, `agent:<state>` match linked [agents](#agents-optional)
- `-term` negates a term; `"double quotes"` group words

Example: `is:open section:"in progress" -tag:blocked`. Set the startup filter
with `default_filter`.

### Saved views

Press `ctrl+s`, type a friendly name, then press `ctrl+s` again to save the
current filter, grouping, and sorting. Press `V` to search saved views by name and press
`enter` to apply one to the current project or My Tasks. Saved views are reusable
across projects and persist between sessions; they do not switch projects.
Changing a filter, grouping, or sorting after recall leaves the saved view unchanged.

On wide terminals, the Views panel shows the current saved view and up to four
other recently used views. Focus it with `0`, select a view with `j`/`k`, then
press `enter` to apply it to the current project. `V` offers the full list;
`[` and `]` still cycle in name order. Views that have never been used appear
alphabetically after recent views.

The ticket list's title shows the current view name, or `Custom` when no saved
view is associated with the current project. After edits it shows `(modified)`;
the Views panel marks the name with `*`. Reverting those edits clears the mark.
In narrow and list-only layouts, the name appears in the status bar instead.
The view name and its settings are remembered separately for My Tasks and each
project, while recently used view names are shared across projects.

Save under the same name to replace a view, with confirmation. The `V` picker
also offers saving and deletion; deletion requires confirmation. These keys
work in narrow terminals and list-only mode too.

### Activity

Press `I`, or choose Activity in the wide Views panel. The view shows Asana's
stories from recently changed tasks across the workspace, including tasks you
reassigned to someone else, newest first.

Activity scans in the background at launch and every
`activity.refresh_interval` (default 2 minutes), so the view opens ready. The
first scan covers 30 days; later scans fetch only what changed since the last
one. Results are cached in `~/.cache/asanamate/activity.json`, so a restart
keeps them. The cache holds comment and task text; delete the file to clear
it. In the Views panel, the Activity row counts new related events (not your
own actions or automated reminders) found since you last opened Activity.
Opening it clears the count and badges those events `NEW` (styled by
`colors.new`, like changed My Tasks tickets) until you leave.
Set `activity.refresh_interval = "0"` to turn background scans off; Activity
then scans when you open it.

- `[` / `]` switches between **Both**, **Mine** (your actions), and **Related**
  (actions by others on tasks currently assigned to you, assignments to you,
  explicit user mentions in comments, or tasks whose current description
  mentions you). Each event explains why it appears.
- `D` chooses the last 24 hours, 7 days (the default), or 30 days, from the
  cache.
- `/` searches ticket names, actors, and activity text. Words combine;
  Activity search is separate from the saved ticket filter.
- `j` / `k` selects an event. `Tab` or `2` focuses its reader to scroll it.
- Events in a row on the same ticket share one entry, its newest event, with
  a `+N more` badge (styled by `colors.more`). `Space` shows the earlier
  events indented under it, where each can be selected, and hides them again.
- `Enter` opens the existing ticket reader, where the usual edit and action
  keys work; `Esc` returns to Activity. On narrow screens, the first `Enter`
  shows the event, and the next opens the ticket. In list-only mode, opening
  a ticket selects it in the ticket list instead.
- `!` includes or hides automated updates and reminders. These never count
  as your actions, even when Asana attributes a reminder to your user ID.
- `r` rescans the selected period from the API, replacing its cached events,
  so deleted comments disappear. Events appear as each task loads, and the
  statusline counts scanned tasks. `I` or `Esc` from the event list returns
  to tickets; `?` shows all Activity keys.

Activity requires paid access to Asana's workspace search API. The statusline
shows scan coverage, and failed or incomplete requests show at the top of the
event list. Search indexing
can lag behind changes, and Asana consolidates stories, so this is not a
complete audit log. Current ownership and description mentions are labelled
as current context, not proof of who owned a task when an older event occurred.

### Feed panel

A `[3] Feed` panel sits under the ticket list whenever the list has a
bordered panel. Collapsed, it is one border line with each source's summary,
such as `3h 10m logged · 4 new · 2 unpushed`. Expanded, it lists:

- **Upcoming** items from feeds, such as meetings, soonest first under a
  `Next` header, shown as `in 25m`.
- **Asana:** today's related Activity events (not your own actions or
  reminders); the summary counts new ones, as the Views panel does.
- **Agents:** each running agent once, from when it entered its state, such
  as `mocpa · completed`. Agents linked to no loaded ticket show as
  `no ticket`, so agents on other branches are visible somewhere. Agents
  already running when asanamate starts show no time.
- **Git:** uncommitted files and unpushed commits in the worktree checked out
  on each loaded ticket's branch, in the ticket's linked repos. A branch with
  no upstream shows `not pushed`.
- **Feeds:** lines from each `feed.commands` command; see
  [Feeds](docs/extensions.md#feeds). For Harvest time entries, add
  `asanamate-hrvst`.

Items linked to a ticket show its title. With `feed.expand = "auto"`, the
panel opens when the list leaves `feed.height` empty rows; it decides when a
view loads or the terminal resizes, never on a background refresh, so the
list does not jump. `F` expands or collapses it until the view changes. `3`
focuses it: `j`/`k` select an item, `enter` opens its ticket (or its link),
and `esc` returns to the list. Feeds and the git check run at start, every
`feed.refresh_interval`, and on reload (`r`).

```toml
[feed]
expand = "auto"
height = 12
refresh_interval = "1m"
commands = ["asanamate-hrvst"]
```

## List-only mode and scripting

`--no-preview` remains a deprecated alias for `--hide-preview`. See the
[deprecated features list](docs/deprecated.md) for compatibility and removal plans.

- `asanamate --hide-preview` starts with the preview hidden, whatever
  `[layout]` says; `z` shows or hides it for the session. It is handy in a small
  popup or a tmux sidebar: pick a ticket, press `space` or `a`, and run an
  action. The views sidebar is independent: `Z` toggles it, and it hides
  automatically below 160 columns (see [Layout](#layout)).

  ```tmux
  # Sidebar: the list only, 50 columns down the left of the window
  bind-key S split-window -hbf -l 50 "asanamate --hide-preview"
  # Popup: pick a ticket, run an action, and close
  bind-key T display-popup -E -w 60% -h 60% "asanamate --hide-preview --exit-on-action"
  ```

- `--exit-on-action` runs `background` actions like `exit` ones: asanamate
  quits, then runs the action, so a popup closes once the action finishes.
  `foreground` actions still return to the list.
- `asanamate list` prints tickets for other tools. The default is My Tasks with
  `default_filter` applied.
  - `--project <gid>` lists a project instead. The gid is the number after
    `/project/` in the project's URL.
  - `--filter "<query>"` overrides the filter. `--filter ""` shows everything.
  - `--format tsv` (default) prints `gid, section, due, title, url` separated
    by tabs. `--format jsonl` prints one JSON object per task.
- `asanamate show <gid>` prints one ticket as Markdown. `--format json` prints
  JSON instead.
- `asanamate doctor [<gid>]` checks that asanamate is ready to browse and
  which optional features can work, and, for a ticket, why agents link to it
  or not. See [Troubleshooting](#troubleshooting).

Pick a ticket with fzf:

```sh
asanamate list | fzf --delimiter '\t' --with-nth 2,4 --preview 'asanamate show {1}' | cut -f1
```

## Upgrading to v0.2.0

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
binary, `asanamate-hrvst`, in the same release archive (see [Install](#install)).
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
[Agents](#agents-optional)); `asanamate config update` rewrites the preset:

| v0.1.0 | v0.2.0 |
|---|---|
| `agents.preset = "ccmux"` | `agents.command = "asanamate-ccmux"` |
| `asanamate setup hooks` | `asanamate-ccmux setup` |
| `asanamate hook codex` (Codex hook) | `asanamate-ccmux hook codex` |

Codex hooks that run `asanamate hook codex` keep working in v0.2.0, and
`doctor` warns about them in its `codex_hook` row; v0.3.0 drops that command.
Run `asanamate-ccmux setup` to replace the hook.

## Configuration

`~/.config/asanamate/config.toml` (or `$XDG_CONFIG_HOME/asanamate/config.toml`).
Changes to `config.toml`, theme files, and action files take effect the next
time asanamate starts; actions saved with the action builder (`A`) apply at
once. Run `asanamate doctor` to check a change without starting the TUI.

| Key | Default | Meaning |
|---|---|---|
| `workspace` | set by setup | Asana workspace gid |
| `theme.name` | `"auto"` | color theme: `auto` (follow the terminal background), a built-in preset, or a file in `themes/`; see [Themes](#themes) |
| `theme.dark` / `theme.light` | `"dark"` / `"light"` | themes `auto` uses on dark and light backgrounds |
| `colors.*` | the theme's | per-role overrides on top of the theme: a color (`0`–`255`, `#rgb`, or `#rrggbb`) or a style table `{ fg, bg, bold, italic, underline, faint, reverse }`; see [Themes](#themes) |
| `default_filter` | `"is:open"` | filter applied at startup |
| `confirm_writes` | `true` | write-back subcommands ask before writing |
| `layout.reader` | `true` | start with the reader beside the list; `z` shows or hides it for the session; `--hide-preview` starts with it hidden. See [Layout](#layout) |
| `layout.list_width` | `0` | fix the list pane at this many columns (at least 30), borders included, and give the reader the rest; the reader hides when fewer than 30 columns remain for it; `0` sizes the list automatically. See [Layout](#layout) |
| `list.layout` | `"single"` | `single` (one line per ticket) or `multi` (title, then fields on a second line) |
| `list.refresh_interval` | `"30s"` | automatic list reload interval, at least `1s`; durations such as `"15s"` or `"1m"`; `R` or the `s` settings menu overrides it and the override persists, `ctrl+s` applies it; the statusline shows the interval, and an automatic reload marks the list border yellow instead of opening the loading modal |
| `list.fields` | `["section", "due"]` | default values shown with the title, in aligned columns (`single` layout); `s`, then `c` overrides columns per project; due dates show relative to today and are colored by urgency |
| `branch_field` | `""` | custom field holding the ticket's git branch (`$ASANAMATE_BRANCH`); empty uses the ticket's ID field, else the title slug |
| `agents.command` | unset (off) | opt-in agent tracking, such as `"asanamate-ccmux"`; see [Agents](#agents-optional) |
| `symbols` | `unicode` on UTF-8, else `ascii` | `unicode`, `nerd`, or `ascii` for ticket markers and agent states |
| `reduced_motion` | OS setting | `true` shows static agent symbols instead of the spinner |
| `list.separator` | `false` | frame each ticket with lines above and below; neighbours share one; the `s` settings menu toggles it and the choice persists |
| `list.group_by` | `""` (ungrouped) | starting grouping: any `list.fields` name; `b` picks another |
| `list.sort.by` | `""` (Asana order) | starting sort: any list field or `title`; `B` picks a field and direction; applies within groups or across an ungrouped list |
| `list.sort.direction` | `"asc"` | `asc` (earliest dates / A–Z) or `desc` (latest dates / Z–A); missing values always last |
| `list.header.style` | `"rule"` | group headers: `rule` (`── Label (n) ───`) or `bar` (reversed bar) |
| `list.header.spacing` | `false` | `true` adds a blank line above and below each group header; the `s` settings menu toggles it and the choice persists |
| `list.selection.style` | `"marker"` | selected ticket: `marker` (bold title with a left `▌`) or `bar` (reversed row; agent badges swap colors); press `s`, then `l` to toggle it and persist the choice |
| `reader.view` | `"cards"` | reader's starting view: `cards` (details card, titled sections, one box per comment) or `markdown` (the rendered ticket Markdown); `v` switches and the choice persists |
| `images.mode` | `"auto"` | kitty graphics: `auto`, `kitty` (force on), or `off` |
| `images.inline` | `false` | cards view: draw images in descriptions and comments in place of their links (needs `images.mode` on and a terminal with kitty Unicode placeholders, such as kitty or Ghostty) |
| `activity.refresh_interval` | `"2m"` | background Activity scan interval, at least `1s`; `"0"` turns background scans off, and Activity then scans when opened. See [Activity](#activity) |
| `feed.expand` | `"auto"` | Feed panel under the list: `auto` (expanded when the list leaves `feed.height` empty rows), `collapsed`, or `expanded`; `F` toggles it. See [Feed panel](#feed-panel) |
| `feed.height` | `12` | Feed panel rows when expanded, borders included; at least `3` |
| `feed.refresh_interval` | `"1m"` | how often feeds and the git check run, at least `1s`; `"0"` runs them only at start and on reload (`r`) |
| `feed.commands` | `[]` | commands that answer the [feed protocol](docs/extensions.md#feeds), such as `["asanamate-hrvst"]` for today's Harvest time entries |
| `picker.type_first` | `false` | open pickers in search mode, so typing filters at once; `esc` leaves search for browse mode |
| `keys.<scope>.<name>` | see the template | key bindings: a list of keys per binding, such as `[keys.main] quit = ["q", "ctrl+q"]`; `[]` unbinds. Scopes: `main`, `activity`, `views`, `filter`, `picker`, `form`, `form_field`, `input`, `builder`, `notice`. Keys are a character (`"j"`, `"?"`) or a key name with modifiers (`"ctrl+d"`, `"shift+tab"`, `"enter"`, `"space"`, `"pgdown"`); a letter with `ctrl` or `alt` is lower case, plus `shift` for the capital (`"ctrl+shift+q"`, not `"ctrl+Q"`); `ctrl+c` always quits and can't be bound. `asanamate setup` and `asanamate config update` write every binding, commented, with its default |
| `reader.max_text_width` | `0` | cards view: wrap description and comment text at this many columns (words are kept whole); `0` wraps at the pane width |
| `reader.comments.latest` | `3` | cards view: list only this many of the newest comments, under a Show all comments row; `0` lists them all |
| `reader.comments.preview_lines` | `2` | cards view: cut each listed comment but the newest to this many lines of text, ending in `see more …` (`enter` shows it in full); `0` shows comments in full |
| `repo_source.root` | your setup directory, or empty | directory whose direct children with a `.git` entry (clones and worktrees) fill the repo picker; absolute or starting with `~`; empty: type a path |
| `repo_source.command` | unset | prints one repo path per line; replaces `repo_source.root` when set; see [Repo sources](docs/extensions.md#repo-sources) |
| `time_tracking.id` / `time_tracking.command` | unset (off) | provider ID for saved choices and command implementing the [time tracking JSON protocol](docs/extensions.md#time-tracking) |
| `project_links.command` | unset (off) | command implementing the [project links JSON protocol](docs/extensions.md#project-links); its links preselect the repo picker and time form |

The title is always shown. `list.fields` picks the other columns and their
order: the built-ins `section`, `due`, `assignee`, `initials` (the assignee's
initials as a colored badge, handy in shared projects), `project`, `tags`, and
`completed` (open/done). Any other name is matched to a custom field, ignoring
case and surrounding spaces. A column shows only when some visible ticket has
a value for it.

Open settings with `s`, then press `c` to choose columns for the current
project (or My Tasks). Press `space` or `tab` to toggle a column,
`ctrl+up`/`ctrl+down` to change its order, and `enter` to save. Press `/` to
search; clear the search before reordering. `esc` clears search or cancels.
`ctrl+r` removes the project's override and restores the `list.fields` defaults. The title and agent badge remain independent of these
choices. Column choices persist across restarts and stay with the project
when applying named views.

The picker offers common fields and the custom fields attached to the project,
including fields with no values and fields on empty projects. My Tasks offers
custom fields from its loaded tasks. Selected custom fields are remembered by
Asana ID, so renames and duplicate names do not change which value is shown.
An unavailable selected field stays in the picker so it can be removed; its
column has no value on tasks where the field is absent.

`list.group_by` takes the same names and puts a `Value (count)` header above
each group. Groups keep Asana's order (first seen), with tickets missing
the value last. `due` groups into Overdue, Today, Tomorrow, Next 7 days, Later,
and No due date.

`list.sort.by` sorts tickets within each group without changing group order,
or the whole list when ungrouped. Press `B` to pick a field and direction,
or choose one from the wide terminal's Sort by section. Due dates sort
chronologically; other fields sort alphabetically, ignoring case. Equal
values keep Asana order. Missing or invalid due dates sort last in both
directions. Pinned tickets keep their own order at the top. Sorting persists
per project and is included in saved views.

Custom fields are separate Asana objects, so two can share a name (for example
one per project). When that happens, asanamate uses the one attached to the
active project; otherwise the first with a value. Writes never guess:
`asanamate field` needs `--project` (actions pass it automatically). For example:

```toml
[list]
layout = "multi"
fields = ["section", "Status", "Branch Name", "due"]
```

Configs written by older versions of setup list repos with a generated
`find` command. It keeps working; to switch to the built-in listing, replace it
with the directory it searched:

```toml
[repo_source]
root = "~/code"
```

To pick repos from sesh or zoxide instead:

```toml
[repo_source]
command = "sesh list -z"
```

### Themes

`[theme] name` picks a built-in theme, a theme file, or
`auto`, which asks the terminal for its background color and uses
`theme.dark` or `theme.light`. The defaults `dark` and `light` use ANSI color
numbers, so they follow your terminal's color scheme. These 20 additional
presets ship as separate TOML files in release archives and the source
checkout. The installation steps run `install-themes.sh`, which copies them
to `themes/<name>.toml` next to `config.toml`. It respects `XDG_CONFIG_HOME`
and keeps existing files, including any edits you made:

| Dark presets | Light presets |
|---|---|
| `one-dark` | `one-light` |
| `monokai` | `tokyo-day` |
| `tokyo-night` | `gruvbox-light` |
| `sonokai` | `catppuccin-latte` |
| `dracula` | `solarized-light` |
| `nord` | |
| `gruvbox-dark` | |
| `catppuccin-mocha` | |
| `solarized-dark` | |
| `rose-pine` | |
| `sonokai-andromeda` | |
| `sonokai-atlantis` | |
| `sonokai-maia` | |
| `sonokai-shusia` | |
| `sonokai-espresso` | |

`sonokai` uses Sonokai's default style. The five `sonokai-*` files use the
corresponding [upstream Sonokai styles](https://github.com/sainnhe/sonokai).
For example, set `name = "sonokai-espresso"` in `[theme]` for Espresso.
Sonokai's MIT license ships alongside the theme files as `LICENSE.sonokai`.

For a fixed preset, set `name = "tokyo-night"` in `[theme]`. To switch with
the terminal background:

```toml
[theme]
name = "auto"
dark = "one-dark"
light = "one-light"
```

Presets adapt their palettes to the UI roles and reading pane Markdown.
They do not change your terminal's background; use a matching terminal
theme for the closest appearance. Palette references live in each standard
theme file under the repository's `themes/` directory. Standard themes are
not embedded in the binary; selecting one requires its installed file.

A theme file is `themes/<name>.toml` next to `config.toml`; the names `dark`,
`light`, and `auto` are reserved. Edit a standard file directly or copy it
under another name. Setup and `asanamate config update` write
`themes/example.toml.example` to start from:

```toml
base = "dark"            # built-in theme for every color this file leaves out

[colors]
accent = "#7aa2f7"
selection = { fg = "#bb9af7", bold = true }

[colors.mode]
normal = { fg = "#1a1b26", bg = "#7aa2f7", bold = true }

[colors.markdown]
link = { fg = "#7dcfff", underline = true }
```

Colors layer in order: the built-in base, the theme file, then `[colors]` in
`config.toml`. Each style field (fg, bg, bold, ...) layers on its own, so
`accent = "#ffffff"` keeps the theme's bold. Unset `header`, `selection`, and
`mode.normal` use `accent`; unset `mode.filter` uses `warn`. A mode pill
without a background is drawn reversed.

Project markers in the sidebar, ticket header, and Details, and enum custom
field values use `colors.asana` hue families. Asana dark and light shades
share the same family. Unset red, orange, yellow, green, teal, blue, purple,
pink, and gray use the foreground of `error`, `pinned`, `warn`, `ok`,
`working`, `accent`, `viewing`, `viewing`, and `muted`, respectively. Unknown
or unset Asana colors have no marker.

Leading bracket tags such as `[MOCPA]` use a stable color from `title_tags`
(case-insensitive), which defaults to `authors`. Set `title_tags = []` to
disable tag colors. Branch custom fields use `branch`, defaulting to the
`viewing` foreground. Existing themes need no new keys. Selected bars retain
their selection colors.

For example, in either a theme file or `config.toml`:

```toml
[colors]
branch = "#baa0f8"
title_tags = ["#78cee9", "#baa0f8", "#e3d367"]

[colors.asana]
orange = "#f3a96a"
pink = "#f76c7c"
```

| Role | Used for |
|---|---|
| `accent` | reader headings, focused panel border, NORMAL and VIEWS pills |
| `border` | unfocused panel and card borders |
| `muted` | secondary text: distant due dates, list fields, and tickets in the Later and No due date groups |
| `highlight` | bar selection and the views panel cursor |
| `ok`, `warn`, `error` | status messages, due-date urgency, Today and Overdue group headers, positive time logged today (`ok`), and applied filter text (`warn`) |
| `working` | working coding agents |
| `header`, `selection` | group headers, selection marker |
| `pinned`, `viewing` | Pinned and Viewing section headers |
| `reacted` | comment reactions you made, drawn as pills |
| `new` | `NEW` badge on My Tasks tickets changed since you last viewed them |
| `more` | `+N more` badge on Activity events in a row on one ticket |
| `branch` | branch column, ticket header chip, and Details branch field |
| `title_tags` | stable colors for leading bracket tags in ticket titles; defaults to `authors` |
| `asana.red`, `.orange`, `.yellow`, `.green`, `.teal`, `.blue`, `.purple`, `.pink`, `.gray` | Asana project markers and enum option colors |
| `authors` | comment authors' dots and gutters and `initials` badges, cycled |
| `mode.normal`, `mode.filter`, `mode.read`, `mode.edit` | statusline mode pills |
| `markdown.text`, `.heading`, `.h1`, `.link`, `.code`, `.code_block`, `.quote`, `.rule` | reading pane Markdown, on top of glamour's dark or light style |

### Time tracking (optional)

Harvest support is a separate provider binary, `asanamate-hrvst`, shipped
with asanamate (see [Install](#install)). It uses
[hrvst](https://github.com/kgajera/hrvst-cli) 3.x
(`npm install -g hrvst-cli`), which needs Node.js 20 or later. Set it up:

1. Install `asanamate-hrvst` on your `PATH`.
2. Run `hrvst login` and authorize your Harvest account. hrvst stores the
   credentials in `~/.hrvst/config.json`; `asanamate-hrvst` reads them from
   there to create entries with the Asana link, and never stores them itself.
   `asanamate-hrvst check` confirms hrvst is installed and logged in.
3. Find the numeric ID of the Harvest task you log time to. Run
   `hrvst users project-assignments me --output=json` and pick the `task.id`
   under `task_assignments`.
4. Add to the config:

```toml
[time_tracking]
id = "hrvst"
command = "asanamate-hrvst --task-id YOUR_TASK_ID"
```

To turn time tracking off, remove the `[time_tracking]` keys.

Press `t` on a ticket. If ticket belongs to several Asana projects, choose one.
The form shows the Harvest projects where your task is active, the task under
its real name, decimal hours, and the date; an error naming the task ID means
it is not active on any of your projects. Press
Enter to edit a field, Tab to move, and Ctrl+S to log. Project and task choices
are remembered per Asana project after a successful log; hours starts empty,
and the date starts as today. Type the date as `YYYY-MM-DD` or `date -d` text
such as `yesterday` (relative dates need GNU date; see the CLI notes in
[Writing back to Asana](#writing-back-to-asana)).
Harvest notes contain ticket title; external reference links to Asana ticket
and chosen Asana project.

The ticket's header shows **Time tracked** with a clock icon below the local
repo information; the Markdown reader shows an equivalent summary. It shows
total time first, then today's time: **3h 45m (1h 15m today)**. Today uses your
local calendar date and the entry's logged date, so backdated entries count
only toward the total. Providers without daily totals show only the total.
Both values use unrounded time from stopped entries linked to that exact ticket,
across projects and visible to your authenticated
account. Harvest permissions may limit this to your own entries. Running timers
and older entries without the ticket's external reference are excluded. The
summary refreshes with ticket details and after a successful log. Loading,
zero logged time, unsupported retrieval, and unavailable retrieval are distinct;
a lookup failure does not prevent reading the ticket or logging time.

Other trackers can implement the same protocol: the command reads one JSON
request (`form`, `log`, or an optional `summary`) on stdin and prints a JSON
reply. See [Time tracking](docs/extensions.md#time-tracking) for the request
and reply formats. Use a different provider `id` when changing trackers so
saved choices stay separate.

### Project links (optional)

A project links command tells asanamate where an Asana project lives in other
systems: its git repository and its time tracking project. When the repo
picker opens for a project without a linked repo, local repos whose remote
matches come first, marked `suggested`, with the cursor on the first; Enter
links it as if you had picked it. The time form preselects the suggested
tracker project unless you chose another one before. asanamate only reads
these links; it never writes them back. A lookup failure shows in the picker
or status line, and the picker and form work without suggestions.

imarc's [Hub](https://hub.imarc.net) support is a separate binary,
`asanamate-hub`, shipped with asanamate (see [Install](#install)). Set it up:

1. Install `asanamate-hub` on your `PATH`.
2. Create a Hub service account at
   `https://hub.imarc.net/admin/service-accounts`; a read-only one is enough.
   Export its client ID and secret in your shell's startup file, so
   asanamate's child processes see them. `asanamate-hub check` confirms both
   are set.

   ```sh
   export HUB_CLIENT_ID=...
   export HUB_CLIENT_SECRET=...
   ```

3. Add to the config:

```toml
[project_links]
command = "asanamate-hub"
```

Hub projects whose `Asana Project` link equals the Asana project's GID supply
their `Git Repository` links and `Harvest Project` links (the current
contract's first), skipping links outside their dates; `asanamate-hrvst`
preselects that Harvest project. `--url` points at another Hub.

Other systems can implement the same protocol: the command reads one JSON
request on stdin and prints the project's links. See
[Project links](docs/extensions.md#project-links) for the formats.

## Actions

Press `A`, or choose **Build / edit actions** in Settings (`s`, then `a`).
Choose **Create action** or an existing action. The builder provides every
standard action setting, including ordered form fields and select options.
Use j/k or arrows to move, Enter to edit, and Ctrl+S to accept a text
edit. Press `/` to search the current menu; Escape clears search and returns
to navigation. Outside search, Escape returns or discards the draft.
Help below the list starts with `nf-fa-circle_info` (`\uf05a`) for Nerd Font
symbols, or `(i)` for Unicode/ASCII, and describes the selected setting. It
stays visible in text editors. In particular, leaving **Input title** blank
disables free-text input.
The command editor supports multiple lines. Escape cancels a text edit or
returns from a nested form; changes remain in the draft. At the action form,
Ctrl+S or **Save action** validates and saves; Escape discards the draft.

New filenames must end in `.toml` and stay inside the actions directory.
Existing filenames stay fixed. Saved actions become available immediately.
The builder rejects duplicate shortcuts within a context and files changed
externally since opening. Saves from multiple builder sessions are serialized;
retry if another session is saving. Edits rewrite TOML formatting and comments,
but preserve the complete original in a unique `<filename>.*.bak` file beside
it. Backups are ignored by action loading. Opening and accepting an unchanged
command preserves its original text. Editing commands with tabs or control
characters requires acknowledging the text editor's whitespace conversion.

Each action is its own file in `~/.config/asanamate/actions/`, next to
`config.toml`. Every `*.toml` file there loads in file name order, so prefixes
such as `10-` and `20-` set the menu order. Other files, such as
`claude-tmux.toml.example`, are ignored until renamed. A file holds one action,
with its keys at the top level:

```toml
# actions/claude.toml
name = "Start Claude"   # shown in the menu
key = "c"               # one character
mode = "background"     # foreground | background | exit
repo = true             # resolve the ticket's repo and run inside it
context = ""            # "" (everywhere) | "comment"; see below
command = '''git switch "$ASANAMATE_BRANCH" 2>/dev/null || git switch -c "$ASANAMATE_BRANCH" &&
tmux new-window -c "$ASANAMATE_REPO" -n "$ASANAMATE_SLUG" -e "ASANAMATE_TICKET_MD=$ASANAMATE_TICKET_MD" -e "ASANAMATE_GID=$ASANAMATE_GID" 'claude "$(cat "$ASANAMATE_TICKET_MD")"' '''
confirm_writes = false  # optional per-action override
```

Commands run with `/bin/sh -c`. Ticket data is available only through
environment variables and files. Always reference the variables, and never
paste ticket text into the command, because ticket content is untrusted.

Common variables are `$ASANAMATE_GID`, `$ASANAMATE_TITLE`,
`$ASANAMATE_BRANCH`, `$ASANAMATE_REPO`, and `$ASANAMATE_TICKET_MD` (the full
ticket as Markdown). [Actions](docs/extensions.md#actions) lists every
variable, the ticket files, and how output and exit status are handled.

Modes:

- `foreground`: suspends the TUI, runs in the same terminal, and resumes.
- `background`: runs detached. Output goes to `~/.local/state/asanamate/actions.log`.
- `exit`: quits asanamate, then runs the command. Best for popups.

Context: an action with `context = "comment"` appears only when a comment is
highlighted in the cards view, listed before the other actions. Press `space`
or `a` on a comment to open the menu. Its key may repeat a key of an
everywhere action; the comment action wins while a comment is highlighted.

```toml
# actions/reply.toml
name = "Reply to comment"
key = "r"
context = "comment"
input = "Reply"
command = '''{ printf '> %s wrote:\n\n' "$ASANAMATE_COMMENT_AUTHOR"; printf '%s\n' "$ASANAMATE_COMMENT_TEXT" | sed 's/^/> /'; printf '\n'; cat "$ASANAMATE_INPUT_FILE"; } |
asanamate comment "$ASANAMATE_GID" -'''
```

Input: set `input` to a title, and asanamate asks for free-form text right
before running the action. Enter adds a newline, ctrl+s runs, and esc cancels.
Leaving it empty is fine. Each action run has its own input file and ticket
snapshot, so another run on the same ticket cannot overwrite its context.
For example, to put notes above the ticket in
Claude's first prompt:

```toml
# actions/claude-notes.toml
name = "Start Claude with notes"
key = "C"
mode = "background"
repo = true
input = "Notes for Claude"
command = '''git switch "$ASANAMATE_BRANCH" 2>/dev/null || git switch -c "$ASANAMATE_BRANCH" &&
p="$ASANAMATE_INPUT_FILE.prompt" &&
{ [ -s "$ASANAMATE_INPUT_FILE" ] && { cat "$ASANAMATE_INPUT_FILE"; printf '\n\nTicket information below.\n\n'; }; cat "$ASANAMATE_TICKET_MD"; } > "$p" &&
tmux new-window -c "$ASANAMATE_REPO" -n "$ASANAMATE_SLUG" -e "P=$p" -e "ASANAMATE_GID=$ASANAMATE_GID" 'claude "$(cat "$P")"' '''
```

Actions can use the same form spec with static fields in TOML. Answers reach
the command through `ASANAMATE_PARAM_<ID>`:

```toml
# actions/deploy.toml
name = "Deploy"
key = "D"
command = '''deploy --target "$ASANAMATE_PARAM_TARGET" --replicas "$ASANAMATE_PARAM_REPLICAS" \
  --dry-run="$ASANAMATE_PARAM_DRY_RUN" --note "$ASANAMATE_PARAM_NOTE"'''
[[form.fields]]
id = "target"
label = "Target"
type = "select"
remember = true
default = "stage"
[[form.fields.options]]
id = "stage"
name = "Staging"
[[form.fields.options]]
id = "prod"
name = "Production"
[[form.fields]]
id = "replicas"
label = "Replicas"
type = "number"
default = "2"
[[form.fields]]
id = "dry_run"
label = "Dry run"
type = "bool"
default = "true"
[[form.fields]]
id = "note"
label = "Note"
type = "string"
```

Each field has an `id`, a `label`, a `type`, and an optional `default`:

| Type | Value passed to the command | `default` |
|---|---|---|
| `select` | the chosen option's `id`; needs `options` | an option `id` |
| `bool` | `true` or `false`; enter toggles it | `"true"` or `"false"`; `"false"` when omitted |
| `number` | any decimal, such as `-1` or `2.5` | a decimal |
| `string` | any single-line text; may be empty | any text |
| `hours` | positive decimal hours | not allowed |
| `date` | `YYYY-MM-DD`; typed as a date or `date -d` text such as `+1 day` | a `YYYY-MM-DD` date; today when omitted |

Write `default` as a TOML string, even for `bool`, `number`, and `date`
(`default = "true"`). Every field except `string` needs a value before the
form submits. Only `select` takes `options` and `remember`.

Remembered action choices use the chosen Asana project and action key. They
are saved when the form is submitted. Existing `input` actions keep their text
box; an action can use both.

**tmux note:** `tmux new-window` and `split-window` run their command in the
tmux server's environment, so `ASANAMATE_*` variables do not reach them. Pass
the ones you need with `-e NAME="$NAME"`, as in the example above.

The active project is the one picked for the repo when `repo = true`.
Otherwise it is the project you are viewing (if the ticket is in it), or the
ticket's only project.

Repo resolution: the first time a project's ticket runs a `repo = true` action,
you pick a repo from `repo_source` or type any path (`/`, the path, then
`ctrl+s`). asanamate
remembers it. Tickets in several projects always ask which project to use.
A ticket given its own repo with `L` (**This ticket only**) uses that repo
instead, skips the project question, and leaves every project link as it is.
The reading pane's header shows the local repo such an action will run in,
when it will not ask: the ticket's own repo, else its only project's link.

More examples:

```toml
# actions/branch.toml
name = "Create branch and record it"
key = "b"
mode = "foreground"
repo = true
command = '''git switch -c "feature/$ASANAMATE_SLUG" && asanamate field "$ASANAMATE_GID" "Branch Name" "feature/$ASANAMATE_SLUG"'''
```

```toml
# actions/session.toml
name = "Jump to repo session"
key = "s"
mode = "exit"
repo = true
command = 'sesh connect "$ASANAMATE_REPO"'
```

## Agents (optional)

asanamate can show coding agents that another tool is running, such as
[ccmux](https://github.com/motherskitchenblr2/ccmux), next to their tickets.
It is off until you set a command that lists them. For ccmux, use the
`asanamate-ccmux` adapter:

```toml
branch_field = "Branch Name"

[agents]
command = "asanamate-ccmux"   # runs `ccmux show --json`
```

`asanamate-ccmux` ships in the release archive next to `asanamate`; extract it
with `tar -xzf "$archive" asanamate-ccmux` and move it to `~/.local/bin/` too
(see [Install](#install)). It must be on the `PATH` asanamate runs with.
It does not install ccmux; without ccmux, the status line and `doctor` show
`ccmux not installed` with a link to it.

Or use any tool with a command that prints `<path>\t<status>[\t<target>[\t<title>]]`
per agent ([Agent sources](docs/extensions.md#agent-sources) has the details),
plus a mapping from its statuses to asanamate's states:

```toml
[agents]
command = '''my-agents --tsv'''

[agents.states]        # a status equal to a state name maps to it already
working   = ["running", "busy"]
waiting   = ["permission", "input"]
completed = ["done", "finished"]
idle      = ["sleeping"]   # anything unmapped is "unknown"
```

The command runs every 5 seconds. With `asanamate-ccmux`, a session that
finished a turn you have not looked at yet counts as `completed`, and an
agent's title is ccmux's session summary, else its first prompt (one line, cut
to 60 characters), else its pane title. Agents link by the branch and repo
ccmux reports, so a session in a nested repo links to the project around it.

### Agent status

ccmux can report a busy session as `idle`, Codex sessions most of all.
`asanamate-ccmux` therefore gives an `idle` session the state its agent last
reported itself, when that report is newer than ccmux's:

- **Claude Code** keeps a file per running session in
  `$CLAUDE_CONFIG_DIR/sessions/` (else `~/.claude/sessions/`). The adapter
  reads it as is; nothing to install.
- **Codex** reports through a hook. `asanamate-ccmux setup` adds
  `asanamate-ccmux hook codex` to `$CODEX_HOME/hooks.json` (else
  `~/.codex/hooks.json`), after asking, saving the old file as a timestamped
  `hooks.json.<time>.bak`. Codex asks you to trust the new
  hooks on its next start, and needs hooks on (`[features] hooks = true`). The
  hook writes one file per session in `$XDG_STATE_HOME/asanamate/agents/`
  (else `~/.local/state/asanamate/agents/`).
  Without it, Codex states fall back to the Codex pane title, and are
  inaccurate or wrong when the title format changes.

  The hook runs `asanamate-ccmux` by its full path. After you move or
  reinstall it somewhere else, run `asanamate-ccmux setup` again: it finds the
  old hook, including one that runs `asanamate hook codex` from before
  v0.2.0, and offers to replace it with the current path. Your other hooks
  stay, and the old file is saved as a timestamped `hooks.json.<time>.bak`
  again. To remove the hook, delete the entries whose command ends in
  `hook codex` from `hooks.json`.

Any status other than `idle` from ccmux is passed on as is, so
`[agents.states]` can map it.

**Linking.** A ticket matches every agent whose working directory has
`$ASANAMATE_BRANCH` checked out, in the ticket's own repo when set, else a
repo linked to one of the ticket's projects (worktrees count as their repo). A project shared across repos that
is linked to just one of them pulls that repo's agents into every ticket it
holds; press `L` to unlink it.

Agents missing from a ticket usually come down to one of these:

- `branch_field` is unset, or empty for the ticket, so the branch is the title
  slug rather than the branch you work on. When a set `branch_field` is
  empty, the reading pane warns, and actions whose command uses
  `$ASANAMATE_BRANCH` ask before running. A branch typed there, other than
  the fallback, is saved for the ticket and used from then on in place of
  `branch_field`. Set branch in the `e` menu edits or removes it.
- The repo is on another branch or a detached HEAD. When a ticket has no
  agents, the reading pane lists the agents in its repos and the branch each
  one is on. Start agents with an action that switches to `$ASANAMATE_BRANCH`
  first, like the Start Claude example in [Actions](#actions).

`asanamate doctor [<gid>]` prints the agents asanamate sees, with their branches
and repos, and for a ticket, its branch, linked repos, and why each agent in
them does or does not match.

**Display.** Linked agents appear right-aligned on the ticket's title line,
most urgent first; more than four show as grouped counts (`⚠1 ◐3 ●2`). The
header sums them up, and tickets with a waiting agent get a yellow title. The
reading pane lists the ticket's agents with their titles and directories.

| State | unicode | ascii | Meaning |
|---|---|---|---|
| waiting | `⚠` | `(!)` | needs you (question or permission prompt) |
| working | spinner (`◐` with reduced motion) | `(~)` | busy |
| completed | `●` | `(+)` | finished, not yet looked at |
| idle | `○` | `(-)` | nothing happening |
| unknown | `?` | `(?)` | status not in the mapping |

`symbols = "unicode"` (default on UTF-8 locales), `"nerd"` (needs a Nerd Font),
or `"ascii"` (default otherwise) also sets the ticket markers (`□`/`✓`,
`[ ]`/`[x]`). Override single states with `[agents.symbols]`, e.g.
`waiting = "!"`. `reduced_motion = true` stops the spinner; unset, asanamate
follows the OS setting (macOS Reduce Motion, GNOME animations).

**Filtering.** `agent:any`, `agent:none`, and `agent:<state>`, for example
`agent:waiting` for "what needs me".

**Actions.** Every action gets the most urgent linked agent in
`ASANAMATE_AGENT_*` and all of them in `$ASANAMATE_AGENT_TARGETS`. An action
with `agent = true` needs one: it runs directly for a single agent, asks which
one (by title) when there are several, and refuses when there are none.

```toml
# actions/agent.toml
name = "Start agent in a worktree"
key = "c"
mode = "background"
repo = true
command = '''ccmux spawn claude --cwd "$ASANAMATE_REPO" --worktree "$ASANAMATE_BRANCH" --detach --prompt "$(cat "$ASANAMATE_TICKET_MD")" &&
asanamate field --yes "$ASANAMATE_GID" "Branch Name" "$ASANAMATE_BRANCH"'''
```

```toml
# actions/jump.toml
name = "Jump to agent"
key = "j"
mode = "exit"
agent = true
command = 'ccmux switch "$ASANAMATE_AGENT_TARGET"'
```

Ticket text becomes the agent's prompt, so anyone who can edit a ticket can
steer the agent. Be careful combining this with auto-approved tools.

## Writing back to Asana

```sh
asanamate comment [--yes] <gid> "Opened PR https://..."   # "-" reads stdin
asanamate move    [--yes] [--project <gid>] <gid> "In Review"
asanamate move    [--yes] --my-tasks <gid> "Later"          # a section of your My Tasks
asanamate field   [--yes] [--project <gid>] <gid> "Branch Name" feature/fix-login
asanamate assign  [--yes] <gid> "Ann Lee"                   # or me; "" unassigns
asanamate due     [--yes] <gid> 2026-10-01                  # "" clears the date
asanamate project add    [--yes] <gid> <project-gid>
asanamate project remove [--yes] <gid> <project-gid>
asanamate complete [--yes] [--reopen] <gid>                 # --reopen marks it incomplete
asanamate react   [--yes] [--remove] <comment-gid> 👍       # --remove takes it back
asanamate create  [--yes] [--project <gid> [--section "To Do"] | --parent <gid>] \
                  [--assignee "Ann Lee"] [--due 2026-10-01] [--notes <markdown | ->] "Fix login"
```

The TUI's edits run the same code, so each one is also a command here.

`asanamate create` prints only the new ticket's gid, so scripts can chain it
(`gid=$(asanamate create --yes "Fix login")`). Without `--project` or
`--parent`, the ticket goes to your My Tasks, assigned to you. `--parent`
makes a subtask and cannot be combined with `--project`. `--assignee` takes a
name or `me` like `asanamate assign`, and `--notes` takes the description in
Markdown like a comment (`-` reads stdin), without image uploads.

`asanamate assign` takes a workspace user's full name (case-insensitive), or
`me` for yourself, and refuses a name two users share. Dates, in `asanamate
due`, `create --due`, `field`, and the TUI, take `YYYY-MM-DD` or anything
`date -d` accepts, such as `tomorrow` or `+1 day`, and are saved as
`YYYY-MM-DD`. Relative dates need GNU date; on macOS, `brew install coreutils`
provides `gdate`, which asanamate uses when installed. `asanamate move
--my-tasks` works only on tickets assigned to you, since Asana shows a
ticket's My Tasks section only to its assignee. Wherever a command takes a
name (a section, field, option, or user), a gid works too.

`asanamate react` takes a comment's gid, which `asanamate show --format json
<gid>` lists under `comments`. Asana does not document writing reactions, so
asanamate reads the comment back after each one and fails if the reaction did
not change.

`asanamate field` takes the value as typed for text fields, a number, a date,
or an option name (case-insensitive). Multi-select and people
fields take a comma-separated list of option names or users' full names, such
as `"Frontend, Backend"`; the list replaces the field's current value. `""`
clears any field.

Each command asks on the terminal before writing unless `--yes` is given,
`ASANAMATE_CONFIRM_WRITES=0` is set, or `confirm_writes = false`. With no
terminal available (for example, a background action), a write that needs
confirmation is refused.

Comments from the CLI or TUI accept Markdown: `**bold**`, `*italic*`,
`~~strikethrough~~`, inline code, fenced code blocks, links, lists, and blockquotes.
Put code fences on their own lines:

````markdown
Here are the values:

```json
{"enabled": true}
```
````

For multiline CLI comments, use stdin so the shell does not interpret backticks:

```sh
asanamate comment <gid> - <<'COMMENT'
**Example**

```json
{"enabled": true}
```
COMMENT
```

Code and link text do not expand `@Full Name` mentions. Raw HTML stays literal.
Asana comments do not support headings or tables: headings become bold and
table syntax stays text. Lists inside quotes and code blocks or quotes inside
lists are flattened to supported formatting.

A local image, `![](~/Desktop/shot.png)`, is attached to the task and shown
inline in the comment; a comment can be only an image. Paths are relative to
the working directory, `~/` is your home directory, and a path with spaces
needs angle brackets: `![](<~/My Shots/a.png>)`. Only image files qualify, up
to 100 MB each. The confirmation lists the files to upload, and each upload
also adds an "attached" entry to the task's activity. Images on the web
(`https://...`) keep only their alt text. In the TUI comment box, `alt+v`
saves the clipboard image to a temporary file and inserts its Markdown.

User mentions display their names without profile links. Other links stay intact.
Fenced code blocks preserve whitespace but do not add syntax highlighting.

In a comment (from the CLI or the `e` menu), `@Full Name` mentions a
workspace user and notifies them, the same as in Asana. Matching ignores case
and picks the longest name that fits, so `@victoria andersen` tags Victoria
Andersen even if a Victoria also exists. An `@` inside a word, or one that
matches no one or two people with the same name, stays as plain text.

## tmux

```tmux
# Popup (pair with exit-mode actions)
bind-key A display-popup -E -w 90% -h 90% asanamate
# Quick-pick popup: list only
bind-key T display-popup -E -w 60% -h 60% "asanamate --hide-preview --exit-on-action"
# Pane (pair with background actions)
bind-key a split-window -h asanamate
# Needed for inline images inside tmux
set -g allow-passthrough on
```

## Troubleshooting

`asanamate doctor` checks everything browsing needs and every optional
feature, without running actions or writing anything:

- `error` lines block browsing: an invalid config, a missing or rejected
  token, or an inaccessible workspace. `doctor` exits nonzero when any appear.
- `warning` lines name a missing tool and the feature that needs it.
- `off` features are not configured; `unchecked` ones run your own commands,
  which `doctor` does not run.
- An `agents` warning ending in `exit status 127` means the shell cannot
  find the agents command, such as `asanamate-ccmux`: install it on your
  `PATH` (see [Agents](#agents-optional)).
- A `codex_hook` warning means Codex still runs `asanamate hook codex`, which
  v0.3.0 drops; run `asanamate-ccmux setup` to replace it.

With a ticket gid, it also explains why each agent links to that ticket or not.

## Moving to another machine

1. Install asanamate on the new machine (see [Install](#install)).
2. Set `ASANA_ACCESS_TOKEN` there the same way as before (see [Token](#token)).
   Never copy the token inside a file with the others; create a new token at
   <https://app.asana.com/0/my-apps> if you prefer.
3. Copy `~/.config/asanamate/` (config, actions, and themes). If a path in it differs
   on the new machine, such as `repo_source.root`, edit it; paths starting
   with `~` carry over.
4. Optionally copy `~/.local/state/asanamate/state.toml` to keep repo links,
   saved branches, saved views, recent projects, and Harvest project choices.
   Repo links are absolute paths: an action whose linked repo is missing asks
   you to pick it again, and `L` relinks or unlinks any of them.
5. Install the tools your own actions use, and the optional tools you had on
   (see [Optional features](#optional-features)). Run `hrvst login` again for
   time tracking.
6. Run `asanamate-ccmux setup` if you use the Codex hook; it adds or repairs
   the hook for the new path.
7. Run `asanamate doctor` and fix what it reports.

The state file, ticket exports (`tickets/`), and the action log hold project
names, ticket text, and comments from your workspace; treat them as private.
Exports and the agent status files are rebuilt as you use asanamate, so there
is no need to copy them.

## Files

- Config: `~/.config/asanamate/config.toml`
- Actions: `~/.config/asanamate/actions/*.toml`
- Themes: `~/.config/asanamate/themes/*.toml`
- State (repo links, saved branches, saved views, recent projects, the cached project list, ticket history, seen My Tasks tickets): `~/.local/state/asanamate/state.toml`
- Ticket exports: `~/.local/state/asanamate/tickets/<gid>/`
- Background action log: `~/.local/state/asanamate/actions.log`
- Activity cache (the last 30 days of Activity events): `~/.cache/asanamate/activity.json`

`$XDG_CONFIG_HOME`, `$XDG_STATE_HOME`, and `$XDG_CACHE_HOME` are honored.
