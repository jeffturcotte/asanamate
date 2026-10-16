# Provider feasibility observations

Tested on 2026-10-16 on macOS, using disposable provider homes and a disposable
working directory. Provider executables ran through `caveman`; interactive
checks used a real PTY through `rtk run` (`rtk proxy` captures streams and does
not preserve terminal input). No personal conversation history was read or
copied. Codex authentication alone was copied into its isolated home; Claude
was authenticated separately by the user.

## Gate status: failed

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

## Remaining gate work

The gate stopped at the Claude exclusion failure. Hook reentrancy, mutation
during an in-flight model turn, archived resumption, title limits, missing-title
normalization, restart persistence, representative cold-history performance,
and runtime dependency requirements remain unproven. Do not publish minimum
supported versions from these observations.

The user-approved conservative revision retains Claude discovery/linking but
refuse default Claude resumption when inactivity and exclusion cannot be
established. This changes the advertised default Claude resumption outcome.
A custom host opener still needs an
explicit native-session mapping and authoritative ownership behavior.

## References

- [Claude session APIs](https://platform.claude.com/docs/en/agent-sdk/sessions).
- [Codex app-server](https://developers.openai.com/codex/app-server).
- Exact exported Claude SDK declarations shipped in `0.3.301`.
