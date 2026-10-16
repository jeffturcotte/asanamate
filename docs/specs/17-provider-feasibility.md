# Provider feasibility observations

Tested on 2026-10-16 on macOS, using disposable provider homes and a disposable
working directory. Provider executables ran through `caveman`; interactive
checks used a real PTY through `rtk run` (`rtk proxy` captures streams and does
not preserve terminal input). No personal conversation history was read or
copied. Codex authentication alone was copied into its isolated home; Claude
was authenticated separately by the user.

## Gate status: hooks pending

This is partial gate evidence, not a supported-version floor or a release
approval. The user approved the conservative revision described below, allowing
generic spec 16 implementation to proceed independently of the companion's
remaining gate work. No provider integration is claimed by this evidence.

## Tested versions

- Claude Code: `2.1.301`.
- `@anthropic-ai/claude-agent-sdk`: `0.3.301`, installed in a temporary
  directory, outside the repository. Its package declares Node `>=18.0.0`,
  platform-specific optional native packages, and peer dependencies on
  `@anthropic-ai/sdk`, `@modelcontextprotocol/sdk`, and `zod`. The installed
  dependency tree contained 105 packages. This does not establish Node 18
  compatibility; only Node `24.18.0` was exercised.
- Codex CLI: `0.162.1`.
- Node: `24.18.0`.

## Claude observations

1. A native CLI print-mode conversation was created directly with an explicit
   UUID. SDK `listSessions` discovered it across projects; `getSessionInfo`
   returned the same UUID, cwd, title, and modification time. One-record local
   lookup took about 2 ms, excluding SDK startup. This is not a representative
   history benchmark.
2. SDK `renameSession` changed its title, and a separate SDK invocation read
   the new title. Interactive `--resume <uuid>` restored the same conversation.
3. While that interactive process remained open, another SDK invocation renamed
   the title again. A subsequent prompt completed in the original terminal;
   SDK lookup still returned the externally changed title. The original
   terminal retained its old cached display title.
4. While the first interactive terminal remained open, a second print-mode
   `--resume <uuid>` accepted and completed a prompt. A second interactive
   `--resume <uuid>` also opened the same conversation without refusal or a
   concurrency warning. Both interactive terminals printed the same UUID in
   their exit instructions. No additional conversation ID appeared in SDK
   discovery. These checks establish that CLI resumption does not provide the
   cross-process exclusion required by the proposed opener.
5. `SDKSessionInfo` exposes no activity or archive field. Listing cannot prove
   inactivity. Archive behavior was not established.

Claude discovery and explicit native-ID targeting work in these checks.
Safe default resumption of an independently launched conversation is **not
established**: an addon-only lock cannot exclude either native terminal.
Do not treat a confirmation about unknown activity as proof of exclusion.

## Codex observations

1. Native `codex exec` created a persisted conversation directly. An initialized
   stdio app-server connection discovered it using explicit source kinds
   (`cli`, `exec`, `appServer`). The one-record list request took about 3 ms,
   excluding process startup. This is not a representative history benchmark.
2. `thread/read` returned its exact ID, cwd, source, timestamps, and null name
   without starting a turn. `thread/name/set` set a canonical Asana suffix;
   a separate connection read that name back.
3. One app-server connection resumed the thread. A separate connection could
   rename and read it, but reported `status.type: "notLoaded"`. Thus this
   status cannot establish global inactivity.
4. Resumption through the second connection failed with code `-32600`:
   `thread <id> already has an active writer`.
5. Interactive `codex resume <id> --no-daemon` restored the same saved
   conversation. Inherited `CODEX_THREAD_ID` and `CODEX_SESSION_ID` referred to
   the parent agent; the explicit selected ID still targeted the disposable
   conversation. This is one nested-launch check, not a complete environment
   compatibility guarantee.

## Package checks (2026-10-16)

The `asana-sessions` package was then run against a new disposable Codex home,
using a conversation created directly by `codex exec`:

1. Listing found it with explicit source kinds. `thread/list` `searchTerm`
   matched the canonical marker in a name set through `thread/name/set`.
2. `link`, a repeated `link` (`changed: false`), a conflicting link, `--replace`,
   and `unlink` all verified by separate reads. `thread/name/set` cannot clear
   a name, so unlinking an otherwise untitled thread leaves `Conversation`.
3. After `thread/archive`, the archived-only query listed it as archived.
   `open` confirmed unknown activity, asked for a replacement for the missing
   cwd, rejected a nonexistent replacement, disclosed the restore, and called
   `thread/unarchive`; the thread then listed as not archived, under the same
   ID, with no additional conversation created.
4. `codex resume <id>` under `expect` exited immediately in both the package
   and a direct run, because that pseudo-terminal does not answer Codex's
   terminal queries. A Python PTY that answers them proved it (item 8).

A second disposable Codex home then covered the remaining Codex items:

5. Titles: a 5,011-character name (non-ASCII prefix plus marker) round-tripped
   unchanged, so no limit was found. `thread/name/set` rejects an empty name
   with `thread name must not be empty`.
6. Active-turn mutation: during a `codex exec resume` turn running
   `sleep 25`, a separate connection read `status.type: "notLoaded"` and
   renamed the thread. The turn completed normally; the new name persisted
   afterwards and appeared in the resumed TUI. Codex did not overwrite it.
7. Interactive `asana-sessions open` in a real PTY, with a bogus inherited
   `CODEX_THREAD_ID`, resumed the selected thread with its earlier turns and
   exited 0. Codex printed the same ID in its reconnect instructions.
8. Two concurrent `open` runs of one thread both attached to the shared
   daemon's single copy of it, with no refusal, fork, or new session file.
   Codex owns the one writer; `--no-daemon` would bypass that.
9. After `thread/archive`, interactive `open` restored and resumed the thread
   under the same ID with its earlier turns, creating no session file.
10. `codex resume` starts a background daemon under `CODEX_HOME` that keeps
   running after the TUI exits and can update itself: one disposable home's
   daemon had moved to `0.163.0` while the CLI was `0.162.1`. Stop such
   daemons before deleting a disposable home.

Cold lookup against the user's real homes (metadata only; counts and times
recorded, no titles): 282 Claude and 207 Codex sessions. The first full
lookup took 991 ms (Codex alone up to 1,076 ms); repeats took 367 ms. This fits
the 10-second deadline without an index.

## Claude checks (2026-10-16)

A disposable `CLAUDE_CONFIG_DIR`, logged in by the user, and a conversation
created directly by `claude -p --session-id`:

1. Archive: neither Claude Code `2.1.301` (`claude --help`) nor the SDK's
   session API has an archive concept, so records report `archived: false`.
2. SDK metadata reads, `link`, `unlink`, and `list` worked with no `claude`
   executable on `PATH`. The CLI is needed only to resume.
3. Missing title: the untitled conversation's `summary` equalled its first
   prompt, and `link` named it `Conversation [asana:<GID>]`.
4. Titles: a 5,013-character title round-tripped unchanged; no limit found.
5. In-flight turn: during a `claude -p --resume` turn running a 25-second
   shell command, an SDK rename persisted after the turn finished.
6. Open interactive session: an SDK rename made while `claude --resume` was
   open in a real PTY survived `/exit`; the CLI did not write its cached title
   back. A separate process read the new title afterwards (restart
   persistence). It resumed the selected ID; no conversation was created.
7. Nested launch: inherited from a parent Claude Code session,
   `CLAUDE_CODE_CHILD_SESSION` made the resumed CLI warn "Transcript saving is
   off". With only that variable removed, saving stayed on and the inherited
   `CLAUDE_CODE_SESSION_ID` did not change the resumed ID. The Claude resume
   command clears it.

## Claude activity registry (2026-10-16)

Concurrent resumption is still not excluded (Claude observation 4), but each
running CLI process writes `<CLAUDE_CONFIG_DIR>/sessions/<pid>.json` with
`pid`, `sessionId`, `procStart`, `pidDomain`, `kind`, and `status`. A second
disposable home (CLI `2.1.301`, user login) showed:

1. A fresh interactive session registers; the file is removed on `/exit`.
2. `claude --resume <id>` registers that ID.
3. `/clear` switches the entry to the new ID; `/resume <id>` to the target.
4. `claude -p` registers the ID (`busy`) while it runs, then removes it.
5. `kill -9` leaves the file behind with a dead PID; a later start removed it.
6. `procStart` equals `ps -o lstart=` under `TZ=UTC LC_ALL=C`, which guards
   against PID reuse. The user's real home (counts only) had 9 entries, all
   live, matching 9 `active` records, including the testing session.

The companion now reports `active` for a conversation held by a live entry and
`inactive` otherwise; a missing or unexpected registry gives `unknown`, which
the Claude opener still refuses. End to end: a stale entry was ignored, a
conversation open in another terminal was refused as active, and an inactive
one was resumed directly in a PTY with the inherited
`CLAUDE_CODE_CHILD_SESSION` cleared. The registry is undocumented; a small
window remains between the check and the launch.

## Remaining gate work

Only hook reentrancy remains, for both providers; it needs the spec 18 plugins.
Claude has no native exclusion; the opener relies on its live-session
registry instead and refuses when the registry cannot be read. Do not publish minimum supported versions
until the hook checks pass.

The user-approved conservative revision retains Claude discovery/linking but
refuse default Claude resumption when inactivity and exclusion cannot be
established. This changes the advertised default Claude resumption outcome.
A custom host opener still needs an
explicit native-session mapping and authoritative ownership behavior.

## References

- [Claude session APIs](https://platform.claude.com/docs/en/agent-sdk/sessions).
- [Codex app-server](https://developers.openai.com/codex/app-server).
- Exact exported Claude SDK declarations shipped in `0.3.301`.
