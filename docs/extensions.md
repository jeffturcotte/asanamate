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
`~/.local/state/asanamate/tickets/<gid>/` (`ticket.json`, `ticket.md`,
`input.md`) and are rewritten on each run of an action on that ticket.
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
| `values` | `log` only: form values keyed by field ID. Select values are option IDs; `hours` is positive decimal hours as typed, such as `"1.25"` |
| `asana` | `task_gid`, `project_gid`, `title`, `url`. `summary` sends only `task_gid` and `url`; the others are empty |
| `links` | `form` only, when [project links](#project-links) knows the chosen Asana project: its links, best first. Omitted otherwise |

```json
{"version":1,"operation":"form","asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"},"links":[{"type":"harvest_project","value":"123"}]}
{"version":1,"operation":"log","values":{"hours":"1.5","project_id":"123"},"asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"}}
{"version":1,"operation":"summary","asana":{"task_gid":"42","project_gid":"","title":"","url":"https://app.asana.com/0/7/42"}}
```

**Compatibility.** Providers may ignore `version`, and must ignore request
fields they do not know: new optional fields are added without changing the
version. The version changes only for a change existing providers could not
ignore.

**Output.** One JSON object on stdout, depending on the operation:

- `form`: a form spec. Every field is required; IDs are unique snake_case
  names, and labels are not empty. Types are `select` (needs `options`, each
  with a stable, unique `id` and a `name`) and `hours`. A time form needs
  exactly one `hours` field. `remember: true` on a select saves the choice
  per Asana project. A select's optional `default` names one of its options
  to preselect, such as the project a `links` entry suggests; a remembered
  choice that is still offered wins over it.

  ```json
  {"fields":[
    {"id":"project_id","label":"Project","type":"select","remember":true,
     "options":[{"id":"123","name":"Web"}]},
    {"id":"hours","label":"Hours","type":"hours"}
  ]}
  ```

- `log`: nothing. Exit zero only after the entry exists. asanamate never
  retries a failed log, because the provider may have created the entry
  before failing.
- `summary`: `{"total_seconds":13500}`, a nonnegative integer: all-time,
  unrounded time from stopped entries linked to this exact ticket, across
  projects, visible to the account. Return `0` when there are none. Match by
  the ticket, not its title, and fetch every page first; never return a
  partial total. A provider without totals returns `{"unsupported":true}`.

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
other systems: its git repository and its time tracking project. asanamate's
Hub provider, `asanamate-hub` (`cmd/asanamate-hub`), is a separate binary that
uses only this protocol, like any other provider. asanamate only reads links;
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
