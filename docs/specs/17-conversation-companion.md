# 17: Standalone asana-sessions package

## Goal and dependency

Implement `asana-sessions`, a standalone package and CLI for finding and
resuming Claude Code and Codex conversations by Asana ticket. It discovers
native histories and writes portable ticket metadata without requiring
asanamate. The optional adapter implements [spec 16](16-conversation-history.md);
it depends on that contract, not a built or installed core application.

## Feasibility gate: do this first

The [2026-10-16 live observations](17-provider-feasibility.md) found that
Claude Code `2.1.301` permits concurrent native resumption without writer
exclusion. The user approved a conservative revision: retain Claude discovery
and linking, but refuse default Claude resumption with unknown activity until
safe resumption is established. Generic spec 16 implementation may proceed;
the companion's remaining feasibility and release gates have not passed.

Use disposable provider homes and conversations, never the user's current
chat or personal history. Record versions and observations in a short
compatibility section beside the companion's installation instructions.

Prove each operation with actual installed provider versions:

1. List a conversation created directly by the provider, not by the addon.
2. Read its ID, current title, cwd, archive state, and available activity
   evidence. Record fields that cannot be established; return `unknown`.
3. Rename it while it is active; read the name back from a separate history
   lookup. Submit another prompt, exit, restart, and verify persistence.
   Check whether the running client's cached or generated title overwrites
   the change. Exercise this while a prompt hook is executing too.
4. Resume the same native ID in an interactive terminal. Confirm no new
   conversation was silently created. Test archived sessions separately.
5. Test another process already using the same ID. Establish native
   exclusion/lock behavior and which activity signals are authoritative.
6. Verify title-length limits, missing titles, custom provider-home paths,
   and supported API/CLI arguments.
7. Measure cold metadata-only lookup against representative histories and
   verify it fits spec 16's deadline. Do not add a database speculatively.

Publish the tested minimum Claude CLI, Claude SDK, Codex CLI, and Node
versions from these observations. Do not invent minimum versions from docs
or trust the latest development API as a released contract.

If active-session mutation, safe identity targeting, archive recovery, or
hook reentrancy cannot be established, report the specific unsupported
behavior and revise the design before proceeding. Do not replace the API
with an undocumented transcript/SQLite writer. A provider cannot pass the
release gate with only mocked tests.

## Packaging and ownership

Keep the Node package at `packages/asana-sessions/` in this repository for
now, exposing the `asana-sessions` executable. Use JavaScript ESM and the
native Node test runner; no TypeScript compilation or bundler is required for this
small command. Include `package.json`, a dependency lockfile, package-owned
README, changelog, tests, license information, and local installation
instructions. Target the public name `asana-sessions`, subject to registry
availability; a scoped registry fallback must preserve the executable name.
No repository move is part of this task.

Rationale: use the official Claude SDK's supported local session functions
instead of maintaining a second parser/writer for its transcript format in
Go. This adds Node and the pinned SDK only to the optional companion. Core
build, startup, and unrelated Go tests never require Node. Record the SDK's
actual transitive/runtime requirements at the gate, including whether its
bundled CLI is needed for metadata-only operations. Install dependencies at
addon installation time, never inside a prompt hook.

Separate public command dispatch, shared ticket-marker rules, and the two
concrete provider modules. Keep native SDK/RPC details inside those modules.
Add a thin asanamate adapter that calls the same engine functions. Only that
adapter reads `ASANAMATE_*` variables or interprets asanamate snapshots.
The engine uses plain validated arguments and objects; it does not spawn
asanamate or read its config/state. All package runtime/build/test inputs
live inside the package so a packed install is independent of this checkout.
Avoid a new service, daemon, dynamic provider registry, or single-use interface.

Any required private lock/pending state belongs under
`$XDG_STATE_HOME/asana-sessions/` (else `~/.local/state/asana-sessions/`),
not asanamate's state directory. Normal discovery still reads native provider
metadata directly; the package does not create its own history database.

## Commands

| Command | Behavior |
|---|---|
| `list <URL-or-GID> [--json]` | Find all linked local sessions; print human rows or versioned JSON |
| `open <URL-or-GID> --provider <name> --session <id>` | Revalidate an exact linked session and perform terminal-resume flow |
| `link --provider <name> --session <id> --task <URL-or-GID>` | Link one explicit native session and verify the saved title |
| `link ... --replace` | Explicitly replace a conflicting existing ticket link |
| `unlink --provider <name> --session <id>` | Remove only the canonical ticket suffix; preserve title and unrelated metadata |
| `hook <claude-or-codex>` | Read one native hook event on stdin; apply spec 18 |
| `doctor` | Check runtime, versions, histories, plugin/helper availability, and API support without starting a model turn |
| `setup` / `remove` | Register/remove only this addon's provider plugins, per spec 18 |
| `help` / `version` | Work without provider credentials or network access |
| `asanamate list` | Adapt spec 16's stdin JSON request to the shared lookup engine and emit its protocol response |
| `asanamate open` | Adapt `ASANAMATE_CONVERSATION_JSON` to the shared open engine, preserving terminal stdin |

Public commands never require an asanamate environment. Human list output
includes provider, full native ID, title, last activity, archive state, and
diagnostics. `--json` returns a documented versioned object with normalized
records and diagnostics; reuse spec 16's response shape in v1 rather than
inventing a second record schema. The package owns its public JSON contract;
the adapter owns compatibility with asanamate's protocol if they later
diverge. Protocol versions are independent of package SemVer.

Complete results, including zero matches and informational absence of an
unused provider, exit zero. Partial/unavailable public lookups emit their
structured results and exit nonzero. The asanamate adapter instead returns
a valid partial protocol response with exit zero so core can show usable
rows and diagnostics; malformed requests and whole-operation failures exit
nonzero. Keep that transport distinction in the adapter, not the engine.

Mutation commands print a structured success result with provider, native
ID, task GID when applicable, and verified title. Exit nonzero on failed
verification. Exit zero with `changed: false` for an already-correct link.
Explicit unlink also clears a recognized `asana:<GID>` Claude tag when that
tag supplies the link; preserve any unrelated tag. Repairing a tag/title
conflict through `--replace` reconciles only recognized Asana metadata.

## Link rules

Use spec 16's canonical suffix. Preserve the title's human-readable prefix.
With no usable title, use `Conversation [asana:<GID>]`; do not fetch a task
title or generate one with a model. If the provider imposes a limit, trim
only the prefix at a Unicode boundary to retain the entire marker.

- Same GID: no duplicate marker. Normalize duplicate identical trailing
  markers to one when explicitly linking.
- Another GID or conflicting recognized tag: fail with a precise conflict
  unless explicit `--replace` was supplied. Hooks never supply it.
- A user-initiated native rename that removes the marker removes the
  portable link. No hidden core association resurrects it later.
- Re-read before mutation and verify after mutation. Use a provider's
  conditional-write facility if it has one. Otherwise derive the new title
  from the latest read, stop on an observed conflict, and document the
  remaining race with a simultaneous native rename. Do not promise atomic
  compare-and-swap where the API supplies none or retry indefinitely.
- Serialize overlapping addon writes per provider/session with a bounded
  local lock. A lock does not claim to protect against the provider's own
  concurrent title writer; the feasibility gate must cover that race.
- Unlink/relink targets one explicit ID. Never choose a session by recency,
  cwd, title, or a guessed parent process.

Parse URLs with a URL parser, accepting HTTPS task links on exactly
`app.asana.com`, without credentials or an alternate port. Support the
legacy `/0/<project>/<task>` path, its `/f` suffix, and modern paths with
`/task/<gid>`. Query strings/fragments do not change identity. Reject project
and profile URLs, spoofed hosts, arbitrary digit strings extracted from
prose, and ambiguous paths. Match the existing
`internal/asana/types.go:TaskGIDFromURL` fixtures where applicable; keep a
small package-owned JSON fixture corpus usable by Go and Node to prevent
drift.
Go integration tests may consume those fixtures from the checkout; the
standalone package's tests must not reach outside its package directory.
Accept a raw validated GID only in explicit commands, not automatic hooks.

## Claude adapter

- Use supported SDK local history functions: `listSessions`,
  `getSessionInfo`, and `renameSession`; use `tagSession` only for explicit
  recognized-tag cleanup/repair. Verify exact signatures and semantics at
  the gate and pin the tested SDK dependency.
- Read existing CLI-created and SDK-created histories without starting an
  agent. Honor `CLAUDE_CONFIG_DIR` and the SDK's supported home behavior.
- Discover beyond the current repo/worktree; a ticket lookup must not miss
  sessions solely because asanamate runs elsewhere.
- Enumerate all metadata pages where the SDK paginates. Handle archive or
  hidden-session metadata explicitly, as established by the gate. Do not
  repurpose a user's unrelated tag or hidden marker during listing.
- Resume with the installed Claude CLI's verified `--resume <native-id>`
  behavior. Listing and renaming are local operations; they require no
  Asana token and must not call the model API.

## Codex adapter

- Use a short-lived app-server connection with its required initialization
  handshake and bounded JSON-RPC request handling. Use `thread/list`,
  `thread/read`, and `thread/name/set`; verify exact schemas at the gate.
- Honor `CODEX_HOME`. Include interactive, noninteractive, and app-server
  sources explicitly; defaults can omit SDK/Paseo-created conversations.
  Include archived and non-archived histories through separate queries,
  exhaust cursors, and deduplicate by native ID.
- Read and rename metadata without `thread/resume`, `turn/start`, or a model
  request. Isolate protocol stdout from diagnostics and clean up the
  app-server subprocess on timeout, cancellation, and ordinary exit.
- A stopped helper connection's `notLoaded` status says nothing about
  another server's ownership. Return unknown activity without authoritative
  evidence of global activity/inactivity.
- Resume with the installed Codex CLI's verified `resume <native-id>`
  behavior. Archived sessions must be restorable without changing their ID;
  perform documented unarchive only as part of an explicit open and disclose
  that side effect. Listing never unarchives.

## Opening and filesystem recovery

Both public `open` and the asanamate adapter re-read the exact selected
native record before opening. Reject missing or changed ticket links, an
unsupported provider, malformed selection data, or a session that no longer
exists. The adapter validates the private snapshot before calling the engine;
the public command supplies the same selection through explicit arguments.
Selection data is not executable instructions. Provider names map to the
two supported implementations, not arbitrary executable paths. Spawn with
argument arrays and inherit the
terminal and signals; report the provider's exit status correctly.
Check inherited provider/host session-identity variables at the feasibility
gate: a resumed process must target the selected native ID even when
asanamate itself was launched inside another agent. Clear only variables
known to carry the parent's session identity; preserve credentials and home
overrides. Include this nested-launch case in the real terminal proof.

Use the recorded cwd when it exists. Otherwise, show the missing path and
let the user enter an existing replacement directory or cancel. If cwd is
unknown, require the same choice. The replacement applies to this launch;
do not silently rewrite native metadata. Do not substitute the current
ticket's repo, switch branches, restore deleted files, or create a worktree.
Display the recorded branch as context; native resume does not restore Git.

Handle activity honestly:

- `active`: default terminal opener refuses duplicate resumption and tells
  the user to use the owning host or stop the existing session. The first
  release does not claim generic focus support.
- `inactive`: proceed, honoring native provider exclusion/lock failures.
- `unknown`: before starting a provider, require an explicit terminal choice
  to resume despite unknown activity or cancel. In noninteractive use,
  fail with recovery instructions rather than hanging. This choice is not
  evidence that no other process exists; native safeguards still apply.
  Claude is the conservative exception: the default opener refuses unknown
  activity until safe resumption is established, because the tested CLI
  supplies no cross-process writer exclusion. Report a precise recovery path;
  do not offer confirmation as a substitute for missing exclusion.
  Claude Code's live-session registry establishes `active` and `inactive`
  ([observations](17-provider-feasibility.md#claude-activity-registry-2026-10-16));
  only an unreadable registry leaves Claude activity `unknown`.

Do not forcibly take over a locked session. If the provider cannot safely
resume an active conversation, refuse rather than forking silently. Host
focus is supported only by a user-configured custom opener with an exact
native-session-to-host-agent mapping. No ccmux or Paseo dependency is added.

## Privacy and failures

Read metadata only. Do not search transcript bodies for GIDs, copy message
text into an index, persist prompt payloads, or send histories to a service.
Respect provider-home overrides and OS permissions. Treat provider data as
untrusted and keep shell metacharacters inert. Any addon lock/state files
use private directories and atomic replacement. No persistent history
database or activity registry is required in v1.

Listing supports partial per-provider failure as spec 16 defines. Metadata
writes fail explicitly; hook wrappers handle those errors without blocking
the conversation. Distinguish unsupported versions from missing histories
and inaccessible files. Dependency/runtime failures include actionable
installation guidance without dumping secrets or raw protocol output.

## Acceptance

- A clean package tarball installs and runs `help`, `version`, `list`,
  `link`, and `open` without an asanamate executable, config directory,
  state directory, source checkout, or Asana token. Provider-dependent
  operations still require their documented local history/runtime.
- Public human/JSON commands and the thin asanamate adapter call the same
  lookup/link/open engine; only the adapter knows asanamate's environment
  and snapshot contract. JSON and partial-result exit behavior are covered.
- The feasibility gate has actual results and a tested compatibility floor
  for both providers, including archived and active-session cases.
- Metadata operations make no model requests and work without an Asana
  token. Stubs assert no turn-start API was invoked.
- Fixtures cover legacy/modern URLs, exact suffix matching, duplicate and
  conflicting markers, Unicode limits, and hostile text/paths.
- Discovery includes manually named sessions, other worktrees, SDK/app-server
  sessions, archives, pagination, and one-provider failures.
- Fake RPC/SDK adapters exercise timeout, malformed replies, process cleanup,
  rename races, and failed readback. Real disposable sessions prove native
  persistence and resumption on the compatibility floor.
- Terminal checks cover normal return, interruption, locked/active/unknown
  activity, missing cwd, replacement cwd, and failed resume without a new ID.
- Obtain permission before creating test files. Run focused Node checks and
  the package's clean-install verification after implementation.
