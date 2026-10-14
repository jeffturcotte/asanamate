# Extensions

asanamate is extended by external commands: any program or shell snippet you
name in the config. This page is the contract between asanamate and those
commands, one section per extension point:

| Extension point | Config | Command gets | Command returns |
|---|---|---|---|
| [Actions](#actions) | `actions/*.toml` | `ASANAMATE_*` variables and ticket files | exit status |
| [Agent sources](#agent-sources) | `agents.command`, `agents.states` | nothing | one tab-separated line per agent |
| [Repo sources](#repo-sources) | `repo_source.command` | nothing | one repo path per line |
| [Time tracking](#time-tracking) | `time_tracking.command` | one JSON request on stdin | one JSON reply |
| [Project links](#project-links) | `project_links.command` | one JSON request on stdin | one JSON reply |
| [Feeds](#feeds) | `feed.commands` | one JSON request on stdin | one JSON reply |

Each format is covered by a test that runs a small fixture script through the
real code path (`internal/*/testdata/`), so a format change cannot slip in
unnoticed. Changes are called out in the release notes.

## Rules for every command

- **Shell.** asanamate runs the configured string with `/bin/sh -c`, so it can
  be a script path, a pipeline, or a one-liner. Quote paths with spaces.
- **Data never enters the command string.** asanamate passes ticket and
  provider data only through environment variables, files, and stdin, so
  ticket text cannot inject shell syntax. Treat that data as untrusted.
- **Environment.** Commands inherit asanamate's environment. Actions drop
  inherited `ASANAMATE_*` variables first, so values from an outer run cannot
  leak in.
- **Working directory.** asanamate's own, except actions with `repo = true`,
  which run in the ticket's repo.
- **No terminal.** Except foreground and exit actions, commands run without
  a terminal: stdin is empty (or the JSON request), and prompts fail.
- **State directory.** Paths below under `~/.local/state/asanamate` use
  `$XDG_STATE_HOME/asanamate` when `XDG_STATE_HOME` is set.

## Actions

An action is one TOML file in `~/.config/asanamate/actions/`. See the
README's [Actions](../README.md#actions) section for its keys (`name`, `key`,
`mode`, `repo`, `agent`, `context`, `form`, `input`, `confirm_writes`) and
for the action builder.

**When it runs.** When you pick the action in the TUI's action menu, after
asanamate resolves its repo (`repo = true`), its agent (`agent = true`), its
form, and its input box. Cancelling any of those skips the run.

**Input.** The ticket arrives in environment variables and files:

| Variable | Value |
|---|---|
| `ASANAMATE_GID`, `ASANAMATE_TITLE`, `ASANAMATE_URL` | task gid, name, permalink |
| `ASANAMATE_SLUG` | branch-friendly title, e.g. `fix-login-bug` |
| `ASANAMATE_COMPLETED`, `ASANAMATE_ASSIGNEE`, `ASANAMATE_DUE`, `ASANAMATE_TAGS` | task details |
| `ASANAMATE_MY_SECTION` | My Tasks section |
| `ASANAMATE_PROJECT`, `ASANAMATE_PROJECT_GID`, `ASANAMATE_SECTION` | active project and the ticket's section in it |
| `ASANAMATE_REPO` | resolved repo (`repo = true` actions) |
| `ASANAMATE_WORKTREE` | the repo's worktree with `$ASANAMATE_BRANCH` checked out, the main one included; empty when none has it |
| `ASANAMATE_BRANCH` | the ticket's branch: the one saved for it, else `branch_field`'s value, else the ID field, else the title slug |
| `ASANAMATE_AGENT_STATE`, `ASANAMATE_AGENT_STATUS`, `ASANAMATE_AGENT_PATH`, `ASANAMATE_AGENT_TARGET`, `ASANAMATE_AGENT_TITLE` | the agent the action is about (the chosen one for `agent = true`, else the most urgent): normalized state, raw status, directory, jump id, title. Empty without agents |
| `ASANAMATE_AGENT_TARGETS` | every linked agent, one `target<TAB>state<TAB>path<TAB>title` line each |
| `ASANAMATE_COMMENT_GID`, `ASANAMATE_COMMENT_AUTHOR`, `ASANAMATE_COMMENT_AUTHOR_GID`, `ASANAMATE_COMMENT_DATE`, `ASANAMATE_COMMENT_TEXT` | the highlighted comment when the menu opened: story gid, author name and gid, `created_at` timestamp, and body as Markdown. Empty when no comment is highlighted |
| `ASANAMATE_TICKET_JSON`, `ASANAMATE_TICKET_MD` | full ticket as JSON / Markdown |
| `ASANAMATE_INPUT_FILE` | text typed into the `input` box, possibly empty. Empty path for actions without `input` |
| `ASANAMATE_PARAM_<ID>` | selected action form value, with its field ID uppercased |
| `ASANAMATE_FIELD_<NAME>` | custom field display values, e.g. `ASANAMATE_FIELD_BRANCH_NAME` |
| `ASANAMATE_CONFIRM_WRITES` | `1`/`0`, read by the write-back subcommands |

The variables above are set on every run, empty when they do not apply,
except `ASANAMATE_PARAM_*` and `ASANAMATE_FIELD_*`, which exist only for the
action's form fields and the ticket's custom fields. Control characters other
than newline and tab are removed from values. The files live in
`~/.local/state/asanamate/tickets/<gid>/run-<unique>/` (`ticket.json`,
`ticket.md`, `input.md`). Each run keeps its own private snapshot, so later
actions on the same ticket cannot overwrite a running action's context.
Snapshots remain until you remove them; commands must use the exported paths.
Foreground and exit actions read stdin from the terminal; background actions
get empty stdin.

`mode` decides how the command runs: `foreground` suspends the TUI and
resumes it afterwards, `background` runs detached, and `exit` quits
asanamate, then runs the command in its place. `asanamate --exit-on-action`
turns background actions into exit actions.

**Output.** asanamate does not read it. Foreground and exit actions write to
the terminal. Background actions append to
`~/.local/state/asanamate/actions.log`, under a `--- <time> <command>` line.
To change the ticket, call `asanamate comment`, `move`, or `field`; they
ask before writing unless `$ASANAMATE_CONFIRM_WRITES` is `0` or `--yes` is
given.

**Exit status.** For foreground and background actions, zero shows
`<name>: done` in the status line, and non-zero shows `<name> failed: exit
status <n>` (with the log path for background actions). An exit action
replaces asanamate, so its exit status becomes asanamate's.

**Timeouts.** None. A background action runs in its own session, without a
terminal, and keeps running after asanamate quits.

**Example.** A shorter `internal/action/testdata/action.sh`:

```sh
#!/bin/sh
printf 'gid=%s\n' "$ASANAMATE_GID"
printf 'branch=%s\n' "$ASANAMATE_BRANCH"
head -n 1 "$ASANAMATE_TICKET_MD"
```

```toml
# actions/show.toml
name = "Show ticket basics"
key = "i"
mode = "foreground"
command = '~/bin/show-ticket.sh | less'
```

## Agent sources

Set `agents.command` to list running coding agents; asanamate links each one
to the ticket whose branch it has checked out. asanamate has no built-in
sources: `asanamate-ccmux` (`cmd/asanamate-ccmux`), the ccmux adapter, is an
agents command like any other.

**When it runs.** When the TUI starts, then 5 seconds after each run
finishes, while the TUI is open. `asanamate doctor` runs it once.

**Input.** None: no arguments, empty stdin.

**Output.** One line per agent on stdout:

```
<path>\t<status>[\t<target>[\t<title>[\t<branch>\t<repo>]]]
```

- `path`: the agent's working directory. asanamate asks git for its branch
  and repository (worktrees resolve to their repository); a path outside git
  is listed but has no branch to link by.
- `status`: the tool's raw status. A status equal to a state name (`waiting`,
  `working`, `completed`, `idle`) maps to it, ignoring case;
  `[agents.states]` maps more statuses to each state; anything else is
  `unknown`.
- `target`: optional id your tool uses to jump to the agent; actions get it
  as `$ASANAMATE_AGENT_TARGET`.
- `title`: optional short description; control characters become spaces,
  spaces collapse, and it is cut to 60 characters.
- `branch`, `repo`: optional, as a pair: the branch and main repository to
  link the agent by, for a tool that knows them better than git does at
  `path`. When both are set, asanamate uses them and skips git; otherwise git
  decides, and a set `branch` is used only when git finds none.

Fields are trimmed, and a trailing `\r` is dropped. Lines with no tab or an
empty path are skipped; columns past the sixth are ignored. An empty output
means no agents. Leave a column empty to skip it, such as `title` before
`branch`.

**Exit status.** Non-zero fails the whole refresh: the status line shows
`agents: agents command failed: exit status <n>`, and the previous list
stays. stderr is not shown.

**Timeouts.** 10 seconds per run in the TUI; the command is killed after
that, and the run counts as failed.

**Example.** `internal/agents/testdata/agents.sh` prints this shape:

```sh
#!/bin/sh
printf '%s\t%s\t%s\t%s\n' "$HOME/code/web" busy web-1 'Fix login'
printf '%s\t%s\n' "$HOME/code/api" permission
printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$HOME/code/web/vendor/lib" idle web-2 '' fix-login "$HOME/code/web"
```

```toml
[agents]
command = '~/bin/my-agents.sh'

[agents.states]
working = ["busy"]
waiting = ["permission"]
```

## Repo sources

Set `repo_source.command` to fill the repo picker from your own list, such
as `sesh list -z`. It replaces `repo_source.root`.

**When it runs.** Each time asanamate opens the repo picker, such as for the
first `repo = true` action on a project with no linked repo, or when you link
a repo with `L`.

**Input.** None: no arguments, empty stdin.

**Output.** One repo path per line on stdout. asanamate trims each line,
expands a leading `~` to your home directory, and skips blank lines and
repeats, keeping the order. Paths are not checked until you pick one; then
asanamate needs a git working tree there.

**Exit status.** Non-zero opens the picker with no listed repos and the error
`repo_source command failed: exit status <n>`; you can still type a path.
stderr is not shown.

**Timeouts.** 1 minute; the command is killed after that, and the run counts
as failed.

**Example.** `internal/repo/testdata/repos.sh` prints this shape:

```sh
#!/bin/sh
printf '%s\n' '~/code/web' '~/code/api'
```

```toml
[repo_source]
command = '~/bin/repos.sh'
```

## Time tracking

Set `time_tracking.command` to log time to a tracker, and `time_tracking.id`
to a name for it. asanamate's Harvest provider, `asanamate-hrvst`
(`cmd/asanamate-hrvst`), is a separate binary that uses only this protocol,
like any other provider. Use a different `id` when changing trackers so saved
choices stay separate.

**When it runs.** Once per request, as a new process each time:

| Operation | When |
|---|---|
| `form` | `t` on a ticket, after choosing an Asana project when it has several |
| `log` | submitting the form |
| `summary` | loading a ticket's details, and after a successful log |

**Input.** One JSON object on stdin, then end of input:

| Field | Value |
|---|---|
| `version` | protocol version, `1` |
| `operation` | `form`, `log`, or `summary` |
| `values` | `log` only: form values keyed by field ID. Select values are option IDs; `hours` is positive decimal hours as typed, such as `"1.25"`; `date` is `YYYY-MM-DD` |
| `asana` | `task_gid`, `project_gid`, `title`, `url`. `summary` sends only `task_gid` and `url`; the others are empty |
| `links` | `form` only, when [project links](#project-links) knows the chosen Asana project: its links, best first. Omitted otherwise |

```json
{"version":1,"operation":"form","asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"},"links":[{"type":"harvest_project","value":"123"}]}
{"version":1,"operation":"log","values":{"hours":"1.5","project_id":"123","spent_date":"2026-10-05"},"asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"}}
{"version":1,"operation":"summary","asana":{"task_gid":"42","project_gid":"","title":"","url":"https://app.asana.com/0/7/42"}}
```

**Compatibility.** Providers may ignore `version`, and must ignore request
fields they do not know: new optional fields are added without changing the
version. The version changes only for a change existing providers could not
ignore.

**Output.** One JSON object on stdout, depending on the operation:

- `form`: a form spec. IDs are unique snake_case names, and labels are not
  empty. Types are `select` (needs `options`, each with a stable, unique `id`
  and a `name`), `bool` (`"true"` or `"false"`), `number` (any decimal),
  `string` (any text, may be empty), `hours` (positive decimal hours), and
  `date` (`YYYY-MM-DD`; the user may type `date -d` text such as `+1 day`,
  which asanamate resolves first). Every field except `string` needs a value. A time form needs exactly one
  `hours` field. `remember: true` on a select saves the choice per Asana
  project. The optional string `default` sets a field's initial value; hours
  cannot have one. A select's `default` names one of its options, such as the
  project a `links` entry suggests; a remembered choice that is still offered
  wins over it. A `bool` without a default starts as `"false"`, and a `date`
  as today.

  ```json
  {"fields":[
    {"id":"project_id","label":"Project","type":"select","remember":true,
     "options":[{"id":"123","name":"Web"}]},
    {"id":"hours","label":"Hours","type":"hours"},
    {"id":"spent_date","label":"Date","type":"date"}
  ]}
  ```

- `log`: nothing. Exit zero only after the entry exists. asanamate never
  retries a failed log, because the provider may have created the entry
  before failing.
- `summary`: `{"total_seconds":13500,"today_seconds":4500}`. `total_seconds`
  is a required nonnegative integer: all-time,
  unrounded time from stopped entries linked to this exact ticket, across
  projects, visible to the account. Return `0` when there are none. Match by
  the ticket, not its title, and fetch every page first; never return a
  partial total. Optional `today_seconds` is a nonnegative integer no greater
  than `total_seconds`: the subset logged on today's local calendar date,
  using the entry's logged date rather than its creation time. Return `0`
  when none were logged today; omit it or return `null` when daily totals
  are unavailable. The header shows the total followed by today's value,
  when supplied. Harvest calculates both values from the same pages.
  A provider without totals returns `{"unsupported":true}`.

**Exit status.** Non-zero is a failure; asanamate shows the provider's stderr
with the error. A failed `form` or `log` shows in the status line or the
form. A failed or invalid `summary` shows the ticket's time as unavailable
(`{"unsupported":true}` shows unsupported) and blocks neither reading the
ticket nor logging time, so providers that reject `summary` as an unknown
operation keep working.

**Timeouts.** 1 minute per request; the command is killed after that, and the
request counts as failed.

**Example.** `internal/timetracking/testdata/provider.sh`, without its
request log:

```sh
#!/bin/sh
request=$(cat)
case $request in
*'"operation":"form"'*)
	printf '%s\n' '{"fields":[{"id":"project_id","label":"Project","type":"select","remember":true,"options":[{"id":"123","name":"Web"}]},{"id":"hours","label":"Hours","type":"hours"}]}'
	;;
*'"operation":"log"'*)
	;; # create the entry here; exit non-zero if that fails
*'"operation":"summary"'*)
	printf '%s\n' '{"total_seconds":13500}'
	;;
*)
	echo "unknown operation" >&2
	exit 1
	;;
esac
```

Matching the raw request text keeps the example free of dependencies; a real
provider should parse it with a JSON tool such as `jq`.

```toml
[time_tracking]
id = "my-tracker"
command = '~/bin/my-tracker.sh'
```

## Project links

Set `project_links.command` to suggest where each Asana project lives in
other systems: its git repository and its time tracking project. imarc's
Hub provider, `asanamate-hub`, lives in its own repository and uses only this
protocol, like any other provider. asanamate only reads links;
providers never receive anything to write.

**When it runs.** Once per lookup, as a new process each time:

- opening the repo picker for a project, such as for a `repo = true` action
  on a project with no linked repo, or when you link a project's repo with `L`
- `t` on a ticket, after choosing an Asana project, before the time form's
  `form` request

**Input.** One JSON object on stdin, then end of input:

| Field | Value |
|---|---|
| `version` | protocol version, `1` |
| `operation` | `links` |
| `asana` | `project_gid`, the Asana project's GID |

```json
{"version":1,"operation":"links","asana":{"project_gid":"7"}}
```

**Compatibility.** As for [time tracking](#time-tracking): ignore `version`
and unknown fields if you like; the version changes only for a change
existing providers could not ignore.

**Output.** `{"links":[...]}` on stdout: the project's links, best first, each
with a nonempty `type` and `value`. Return `{"links":[]}` when the project is
unknown.

| Type | Value | Used by |
|---|---|---|
| `git` | a git remote URL, such as `https://github.com/acme/web` or `git@github.com:acme/web.git` | the repo picker: local repos with a remote that matches, ignoring scheme, user, port, case, and a trailing `.git`, come first, marked `suggested`, and the cursor starts on the first. Clones come before their worktrees |
| `harvest_project` | a Harvest project ID | `asanamate-hrvst`, which preselects it in the time form |

asanamate reads only `git` itself. Every link, of any type, goes unchanged to
the time provider's `form` request as `links`, so other trackers can define
their own types.

```json
{"links":[
  {"type":"git","value":"https://github.com/acme/web"},
  {"type":"harvest_project","value":"123"}
]}
```

**Exit status.** Non-zero is a failure; asanamate shows `no suggestions:`
with the provider's stderr in the picker or status line, and the picker and
time form work without suggestions.

**Timeouts.** 1 minute per lookup, counted with the rest of the request it
belongs to; the command is killed after that, and the lookup counts as failed.

**Example.** `internal/projectlinks/testdata/provider.sh`, without its
request log:

```sh
#!/bin/sh
request=$(cat)
case $request in
*'"project_gid":"7"'*)
	printf '%s\n' '{"links":[{"type":"git","value":"https://github.com/acme/web"},{"type":"harvest_project","value":"123"}]}'
	;;
*)
	printf '%s\n' '{"links":[]}'
	;;
esac
```

```toml
[project_links]
command = '~/bin/project-links.sh'
```

## Feeds

Add commands to `feed.commands` to show their lines in the Feed panel under
the ticket list, such as time entries, CI runs, deploys, or meetings. A feed
only reports; asanamate never sends it anything to write. asanamate's Harvest
provider answers this protocol too: add `asanamate-hrvst` to list the day's
Harvest time entries.

**When it runs.** As a new process for each feed when the TUI starts, every
`feed.refresh_interval` while it is open, and on reload (`r`).
`asanamate doctor` runs each once.

**Input.** One JSON object on stdin, then end of input:

| Field | Value |
|---|---|
| `version` | protocol version, `1` |
| `operation` | `feed` |
| `date` | today's local date, `YYYY-MM-DD` |

```json
{"version":1,"operation":"feed","date":"2026-10-12"}
```

**Compatibility.** As for [time tracking](#time-tracking): ignore `version`
and unknown fields if you like; the version changes only for a change
existing providers could not ignore.

**Output.** One JSON object on stdout:

| Field | Value |
|---|---|
| `source` | required, 1 to 8 characters, shown at each item's right edge, such as `ci` |
| `summary` | optional; joined into the panel's top edge, cut to 24 characters, such as `1 failed` |
| `icon` | optional: the glyph before `summary`, one of `clock`, `due`, `branch`, `repo`, `clip`, `folder`, `refresh`, `bell`, or `warning`. Glyphs follow `symbols`: `nerd` draws them all, `unicode` and `ascii` only `clock`; an unknown name draws none |
| `items` | required, may be empty: the feed's lines |

Each item has:

| Field | Value |
|---|---|
| `time` | required, RFC 3339. A time after now is an upcoming item, listed first under `Next` and shown as `in 25m` |
| `text` | required, one line; control characters become spaces, and it is cut to fit |
| `tone` | optional: `ok`, `warn`, `error`, or `muted`, colored by the theme; empty is the plain text color |
| `ticket_gid` | optional: the Asana task the item belongs to |
| `url` | optional: an Asana task URL links the item to that task; any other URL is what `enter` opens |
| `branch`, `repo` | optional: link the item like an agent, to the loaded ticket on `branch`, limited to tickets linked to `repo` when it is set |

asanamate links an item by `ticket_gid`, then `url`, then `branch`: the first
that names a loaded ticket wins, and with several tickets on one branch, the
first in the list. A linked item shows its ticket's title after its text,
unless its text already ends with " · " and the title, and `enter` opens
the ticket. An item that links to nothing still shows.

```json
{"source":"ci","summary":"1 failed","icon":"warning","items":[
  {"time":"2026-10-12T14:05:00-04:00","text":"build failed","branch":"feature/x","tone":"error"},
  {"time":"2026-10-12T15:30:00-04:00","text":"deploy window","url":"https://example.com/deploys"}
]}
```

**Exit status.** Non-zero or invalid output fails that run: the panel shows
`<source> failed` in its summary and keeps the feed's previous items. stderr
is not shown in the panel; `asanamate doctor` shows it. Other feeds are
unaffected.

**Timeouts.** 1 minute per run; the command is killed after that, and the run
counts as failed.

**Example.** `internal/feed/testdata/feed.sh`, without its request log:

```sh
#!/bin/sh
request=$(cat)
case $request in
*'"operation":"feed"'*)
	printf '%s\n' '{"source":"ci","summary":"1 failed","icon":"warning","items":[
	  {"time":"2026-10-12T14:05:00-04:00","text":"build failed","branch":"feature/x","repo":"/code/web","tone":"error"},
	  {"time":"2026-10-12T15:30:00-04:00","text":"deploy window","url":"https://example.com/deploys"}]}'
	;;
*)
	echo "unknown operation" >&2
	exit 1
	;;
esac
```

```toml
[feed]
commands = ["asanamate-hrvst", "~/bin/ci-feed.sh"]
```
