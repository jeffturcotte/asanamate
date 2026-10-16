# asana-sessions

Find, link, and resume Claude Code and Codex conversations by Asana ticket.

`asana-sessions` reads each provider's own saved conversation metadata and
looks for a ticket marker at the end of the conversation title:

```text
Fix task sorting [asana:1234567890]
```

It needs no Asana token, makes no model requests, and does not need asanamate.
It reads titles and other metadata only, never transcript text.

> **Status: not released.** The provider feasibility gate has not passed
> (see [Compatibility](#compatibility)). The package is marked `private` so it
> cannot be published by accident.

## Install

The package lives in the asanamate repository for now. With Node and npm:

```sh
cd packages/asana-sessions
npm ci
npm install --global .
asana-sessions doctor
```

Install it where each provider's process can find it on `PATH`. Dependencies
install only at installation time.

## Commands

| Command | Behavior |
|---|---|
| `list <task-url-or-gid> [--json]` | List every local conversation linked to the ticket |
| `open <task-url-or-gid> --provider <name> --session <id>` | Re-check one linked conversation, then resume it in this terminal |
| `link --provider <name> --session <id> --task <task-url-or-gid>` | Add the ticket marker to one conversation's title and verify it |
| `link ... --replace` | Replace a link to another ticket |
| `unlink --provider <name> --session <id>` | Remove the ticket marker and keep the rest of the title |
| `doctor` | Check Node, the Claude Agent SDK, both CLIs, and history access |
| `help`, `version` | Work without any provider installed |
| `asanamate list`, `asanamate open` | The [asanamate](../../README.md) conversation adapter |

`<name>` is `claude` or `codex`. A task is an HTTPS `app.asana.com` task URL
(`/0/<project>/<task>`, `/1/<workspace>/.../task/<task>`, either with an
optional `/f`) or a numeric task GID. Project and profile URLs, other hosts,
credentials, ports, and numbers in prose are rejected.

### Exit status

- `0`: success, an already-correct link (`"changed": false`), or a complete
  lookup, including no matches when one provider is simply not installed.
- `1`: a failed operation, a failed verification, a cancelled open, or a
  partial or unavailable lookup. `list` still prints what it found.
- `2`: a usage error.
- `open` otherwise exits with the resumed provider's status.

### JSON

`list --json` prints the record shape of asanamate's conversation protocol,
version 1 ([docs/extensions.md](../../docs/extensions.md#conversation-history)):
`version`, `conversations` (with `provider`, `session_id`, `task_gid`,
`title`, `activity`, `archived`, and, when known, `cwd`, `branch`,
`updated_at`), and `diagnostics` (`provider`, `code`, `severity`, `message`).
Warning or error diagnostics mean the lookup is partial. The JSON version is
independent of the package version.

`link` and `unlink` print `provider`, `session_id`, `task_gid` (for `link`),
the verified `title`, and `changed`.

## Link rules

- One marker per conversation. Linking again is a no-op; duplicate identical
  markers collapse to one.
- A different ticket's marker is a conflict unless you pass `--replace`.
- An untitled conversation becomes `Conversation [asana:<GID>]`. The first
  prompt is never copied into a title.
- Renaming a conversation natively so the marker disappears removes the link.
- Claude's tag `asana:<GID>` is also recognized. A tag and title that disagree
  are a conflict; `link --replace` and `unlink` clear only that tag.
- Writes re-read the title first and verify it afterwards. A rename by the
  provider between those steps is detected and reported, not retried. A short
  local lock under `$XDG_STATE_HOME/asana-sessions/` (default
  `~/.local/state/asana-sessions/`) serializes this tool's own writes only.

## Opening

`open` re-reads the exact conversation and refuses it when it is missing or no
longer linked to the ticket. It then:

1. Refuses a conversation that is active in another process.
2. For unknown activity: Claude is refused. Codex asks for confirmation on the
   terminal; without a terminal it fails.
3. Uses the recorded directory, or asks for an existing replacement for this
   launch. It does not change Git branches or create worktrees.
4. Restores an archived Codex conversation first, and says so.
5. Resumes in the foreground: `claude --resume <id>` without an inherited
   `CLAUDE_CODE_CHILD_SESSION`, or `codex resume <id>` without the parent
   agent's `CODEX_THREAD_ID` and `CODEX_SESSION_ID`.

**Claude activity.** Claude Code lets two processes resume one conversation at
once, which can interleave its history. Every running Claude Code process
records its conversation in `$CLAUDE_CONFIG_DIR/sessions/<pid>.json` (default
`~/.claude/sessions/`), updates it on `/clear` and `/resume`, and removes it on
exit. A conversation listed there by a live process (same PID and start time)
is `active`; otherwise it is `inactive`, and `open` resumes it directly. If
the registry is missing or has an unexpected shape, activity is `unknown` and
`open` prints the `claude --resume <id>` command instead of running it. The
registry is not a documented API, so a Claude Code update could change it.
Conversations run under a different `CLAUDE_CONFIG_DIR` are not visible.

**Codex activity** is always `unknown`: a helper connection cannot see other
servers. Resuming through the shared daemon attaches to its one copy of the
conversation rather than forking it.

## Compatibility

Exercised on macOS with disposable provider homes:

| Component | Version |
|---|---|
| Node | 24.18.0 |
| `@anthropic-ai/claude-agent-sdk` | 0.3.301 (pinned) |
| Claude Code | 2.1.301 |
| Codex CLI | 0.162.1 |

These are tested versions, not a supported minimum. The SDK declares Node
`>=18` and installs 105 packages, including its peer dependencies. Whether
its metadata functions need the Claude CLI installed is not yet established.

Proven with real conversations:

- Claude: the session registry's `active`/`inactive` result for fresh,
  resumed, print-mode, `/clear`, `/resume`, and killed processes, and direct
  `open` of an inactive conversation; listing, lookup, `link`, and `unlink`
  through the SDK with no `claude` executable on `PATH`; naming an untitled conversation; a
  5,013-character title; renaming during a running turn and while the
  conversation is open in a terminal, which survives exit. Claude has no
  archive concept.
- Codex: listing with explicit sources and the title search; `link`,
  `unlink`, and `--replace`; a 5,011-character title; renaming during a
  running turn; archive listing; and interactive `open` of an active and an
  archived conversation. Two concurrent `open` runs attach to the shared
  daemon's single copy of the thread rather than forking it.
- Cold lookup over 282 Claude and 207 Codex sessions took about 1 second.

`codex resume` leaves a background daemon running under `CODEX_HOME`, and
that daemon can update itself independently of the `codex` CLI. A Claude Code
session launched from inside another one inherits `CLAUDE_CODE_CHILD_SESSION`
and stops saving its transcript; run `claude --resume` from a plain shell.

Not yet proven: hook reentrancy for both providers (spec 18). The
[feasibility notes](../../docs/specs/17-provider-feasibility.md) record the
details.

## Development

```sh
npm ci
npm test               # node:test, with fake SDK and app-server
scripts/check-pack.sh  # install the packed tarball in a clean prefix
```

`test/fixtures/task-urls.json` is shared with asanamate's Go URL parser test.

## License

[GNU GPL v3.0 or later](LICENSE).
