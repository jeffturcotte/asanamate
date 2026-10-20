# 18: Optional conversation-linking plugins and distribution

## Goal and dependency

Ship optional Claude Code and Codex plugins that call the companion from
[spec 17](17-conversation-companion.md). Installing them automates native
ticket links outside asanamate. Disabling or removing them stops automation
without deleting history or removing existing ticket markers.

Depends on spec 17, including its provider feasibility gate. The hook
contract and native metadata format come from [spec 16](16-conversation-history.md).

## Package shape

Keep thin provider plugin roots under the companion package. Each contains
its provider's native manifest, hook declaration, and explicit-link skill.
Share behavior through `asana-sessions`; do not duplicate URL
parsing or metadata mutation in shell scripts or skill prose.

- Claude: `.claude-plugin/plugin.json` and native hook configuration.
- Codex: `.codex-plugin/plugin.json` and native hook configuration.
- Both call `asana-sessions hook <provider>` from the provider
  process's PATH. The companion is installed separately as the shared
  executable. Diagnose PATH mismatches rather than downloading a helper at
  hook execution time.
- Use provider plugin-root variables only for files inside that plugin;
  manifests must not escape their installed root to reach sibling files.
- Ship only the native hook declarations required by the tested provider
  versions. Do not rely on identical discovery rules merely because the
  payloads look compatible. Validate both manifests with native tooling.

These are `asana-sessions` plugins, useful with or without asanamate. Their
names, descriptions, skill examples, setup, and diagnostics must not imply
that asanamate is required. Both provider plugins share the standalone
package version and use its public helper commands, never its asanamate
adapter.

Provider hooks may require enabling a feature and trusting the exact hook
definition. Installation reports those requirements. It does not approve
trust prompts, bypass policy, or enable unrelated settings automatically.

## Automatic hook path

Register `UserPromptSubmit` for automatic linking. Its payload supplies the
native `session_id`, `cwd`, event name, and actual submitted `prompt`.
Dispatch with the provider named in the installed hook command. Validate the
event shape and target identity before reading or writing metadata.

For each event:

1. Extract supported Asana task URLs only from the submitted user prompt.
   Do not inspect system context, tool output, assistant text, repository
   files, or the rest of the transcript.
2. Parse unique task GIDs with the companion's shared URL rules. Repeated
   URLs for the same task count as one. No URL means no work; more than one
   distinct task means skip and provide a short ambiguity notice.
3. Resolve the exact current native session ID from the event in the
   provider home used by that process. Never use the newest session or a
   cwd match as a fallback. An unknown identity is a reported skip.
4. Invoke the same link operation as the explicit command, without
   `--replace`. An existing matching link is a no-op. Another link is a
   conflict, not permission to relink the conversation.
5. Verify the saved title. Report failure without claiming the link worked.

This rule means that even a task URL quoted in the user's prompt can trigger
linking. State that plainly during setup and in the README. Users wanting
deliberate linking instead keep hooks disabled and use the explicit skill.
The addon cannot infer intent reliably from “look at this link.”

Run locally with no Asana API or model request. Bound event input to 1 MiB,
enough for normal prompts, and use a short explicit hook timeout with a
companion deadline shorter than the native host timeout. The feasibility
gate must establish a working latency budget before choosing those timeout
values. Oversize input, unsupported payloads, and timeout all skip linking
with a concise diagnostic; they do not block the submitted prompt.

Hook stdout is a protocol channel, not a log. Emit only provider-supported
nonblocking notices when appropriate; normal no-op events are silent.
Use sanitized stderr for errors where supported. A hook wrapper exits zero
on linking failure and never returns a blocking/denial field. Cancel and
clean up child operations on its own deadline. No prompts from hooks.

### Delayed persistence and generated titles

The prompt event may precede the first saved title or even the readable
session record. Spec 17 must prove this ordering on both providers.
Prefer a verified immediate write when the native API supports it.

If a provider requires deferral, retain only pending provider/session/GID
metadata in private local addon state and complete the write on a verified
nonblocking `Stop` hook. Do not retain the prompt. Use the same conflict and
readback rules; clear pending state after success or a reported terminal
failure, and clean abandoned pending entries with bounded retention.
Document the chosen retention and cleanup policy if this path is needed.
Do not add deferred state or extra lifecycle hooks when the gate proves
immediate writes reliable. Automation cannot ship with “works after restart”
or title-overwrite races hidden as success.

## Explicit-link skill

Bundle a small skill for “link this conversation to this Asana task,”
“change this conversation's linked task,” and “unlink this conversation.”
It calls companion commands and reports verified results. It does not
instruct the model to print `/rename` and assume execution happened.

- The skill receives an explicit task URL or GID and resolves the current
  provider/session through verified host context. Use documented environment
  identity or a supported tool exposed by the integration. Verify exact
  variable/tool availability on both providers at spec 17's gate.
- If the host exposes no reliable current session ID, explain that limitation
  and require an explicit ID. Do not claim automatic current-session linking
  works there. A manually supplied ID is validated before mutation.
- A request to link a different task is not a silent overwrite: report the
  existing task and require explicit user intent to replace it. An explicit
  request to change the link authorizes `--replace`; do not ask twice.
- Preserve human title and unrelated Claude tags. Unlink removes only
  recognized Asana metadata, as spec 17 defines.
- Do not read an Asana task or require a token merely to extract its GID.
  Existing Asana skills can call the helper without installing these hooks.
- Leave the user's existing skills and AGENTS.md/CLAUDE.md files untouched.

## Installation and upgrades

Install helper and plugins only when requested. The default asanamate
install/setup remains unchanged. Standalone `asana-sessions setup` works
without asanamate installed and:

1. Checks Node, the locked SDK installation, installed provider versions,
   helper visibility from the provider environment, and compatibility.
2. Shows which provider plugins it can install and the automatic-linking
   rule. Offers Claude, Codex, both, or cancellation.
3. Registers selected plugins through supported native plugin commands or
   the supported local marketplace mechanism, using commands validated at
   the gate. Registration does not edit unrelated global hooks.
4. Prints remaining native trust/enable/restart steps and runs nonmutating
   diagnostics. It distinguishes registered, disabled, untrusted, incompatible,
   and ready; registration alone is not success.

Use a stable plugin identity and versioned package assets so repeated setup
does not duplicate hooks. Upgrade through the normal package/plugin path;
detect stale registrations and report repair steps. Never grant trust to
new hook content automatically. If a supported installer must modify a
settings file, back it up to a unique timestamped sibling, preserve unknown
keys and unrelated entries, write atomically, and remove only owned entries.

The helper works without installing either provider plugin. The core
`[conversations]` table is a separate opt-in, enabled through the normal
config-update/manual workflow. Package docs provide an optional asanamate
integration section with the adapter snippet from spec 16; ordinary setup
does not probe for, require, or rewrite asanamate configuration. Repository
builds do not install anything into personal provider homes.

## Removal

`remove` selects this addon's registered plugins and uses native removal
commands. Remove only owned registrations and, if present, owned pending
state. Preserve saved conversation titles/tags, native transcripts, and
unrelated hooks/skills. Removing hooks leaves the installed companion able
to discover and open history. Removing the helper as well requires disabling
the core command configuration; diagnose that missing command cleanly.

Unlinking every historical conversation is a separate explicit operation,
not an uninstall side effect. No bulk unlink is required in v1.

## Independent versioning, release artifacts, and CI

Keep development in this repository, but version and release `asana-sessions`
independently. Its version lives in its own `package.json` and lockfile;
its provider plugins share that version. Core retains its existing tag-only
versioning. Neither product's version must equal the other's. Asanamate
documents compatible conversation protocol/package ranges, not matching
release numbers. The standalone tool/plugins can ship before core integration.

Use namespaced standalone tags `asana-sessions/vX.Y.Z`, separate from core's
`vX.Y.Z` tags. Before implementing publication, update repository release
guidance to explicitly distinguish the two products. Existing core tag,
archive, and hub-release workflows must not trigger from a standalone tag.
An asanamate release must not republish or retag the standalone package.
Creating or pushing either product's release tags still requires an explicit
user request; writing these specs does not authorize publishing.

Publish a reproducible package tarball containing helper, provider plugin
assets, skills, README, changelog, and license information. Add checksums and
local-install instructions. Public npm publication can follow once the name
and credentials are secured; record the actual registry name in docs. Do not
make a registry account necessary to build, pack, install, or test locally.
Resolve/pin dependencies through the lockfile, not an update during release.
Verify a packed install uses only published payload/runtime dependencies,
not files from the surrounding repository.

Add package-specific CI and a namespaced release workflow. It verifies the
package version matches its tag, builds/checks the artifact, validates both
plugins, and installs/exercises the tarball in a clean directory. Test
supported Node/runtime platforms independently from the existing Go matrix.
Keep Go archives and ordinary core smoke checks unchanged: no fake Go
wrapper, bundled Node dependency, or requirement to install the package.

Before a standalone tag is created, push its commit to main, require a
successful package CI run on that exact commit, and verify local/remote
main match it with a clean tree. Keep pushed tags immutable and verify the
standalone release workflow/artifact publication before reporting completion.
Document these standalone checks alongside the existing core release rules;
do not reinterpret or weaken core's required all-environment CI gate.

Deterministic CI uses fake SDK/RPC providers and isolated homes; live
authenticated provider checks are a documented standalone release gate,
not tests that spend user tokens on every PR. Integration CI checks core's
adapter against the supported package/protocol range. A failed standalone
release does not become a successful core release, or vice versa: report
each product's status independently. No separate repository or general
monorepo release framework is needed for these two workflows.

Document installation, compatibility floor, Node/SDK dependency cost,
automatic-link semantics, trust, upgrades, PATH troubleshooting, removal,
manual naming, and local-only scope. Do not promise GUI/Paseo title sync.

## Acceptance

- Native plugins validate and load in both tested providers. Exactly one
  owned prompt hook runs per submission after repeated setup/upgrade.
- Starting either provider directly, outside asanamate and ccmux, links a
  disposable conversation after one task URL. The title persists after a
  later prompt and restart and appears in provider history.
- Duplicate links are idempotent; multiple tasks and existing different
  links cause nonblocking notices with no metadata replacement.
- Missing helper, unsupported versions, blocked trust, malformed events,
  timeout, unwritable history, and unknown session identity never block chat.
- The explicit skill links, changes, and unlinks an exact session, or reports
  lack of reliable current-session identity without guessing.
- Disabling/removing plugins preserves history and existing links. The
  installed helper still lists and opens those links without any hooks.
- Clean package installs contain required assets and pinned dependencies.
  Core Go smoke checks still pass without Node or provider tools.
- Standalone setup, plugins, skills, human CLI, and JSON output work without
  asanamate config/state or executable. Package tests/build inputs stay
  inside `packages/asana-sessions/`.
- Package/plugin versions and namespaced release tags agree; core release
  workflows ignore standalone tags and standalone release workflows ignore
  core tags. Compatibility is checked through the documented protocol range.
- No personal config is modified by fixtures or repository build steps.
  Ask before writing test files when implementing this spec.
