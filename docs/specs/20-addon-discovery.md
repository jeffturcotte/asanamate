# 20: Discover and set up optional addons

**Depends on nothing. Additive; no breaking changes.**

## Goal

A new user can find optional addons, understand what each adds, install its
requirements, configure it, and verify the result without searching the
repository. The experience works the same way for any kind of addon.

An addon is a named integration implemented through one or more existing
extension points. An installed executable, configured feature, and working
connection are different things; the UI must distinguish them.

## Context

- README introduces optional features, lists their requirements, and has
  separate setup sections. Instructions are useful but scattered.
- `internal/setup/setup.go` ends with a brief reference to README. It writes
  inactive example actions and commented provider settings.
- `internal/tui/settings.go` offers display controls and the action builder,
  but no place to discover integrations.
- `internal/tui/keys.go` hides time tracking until configured. Keep that
  behavior in normal ticket help; make discovery available elsewhere.
- Release archives contain `asanamate-hrvst` and `asanamate-ccmux`, but the
  primary README install command extracts only the main executable.
- `doctor` checks agents and feeds, but leaves time tracking and project
  links unchecked. A successful core check does not prove an addon works.
- `docs/extensions.md` documents six existing extension protocols. Keep
  them and the ability to configure arbitrary commands.

## Decisions

- Provide a small catalog shipped with asanamate, shared by CLI and TUI.
  Do not introduce a remote registry, plugin loader, or manifest protocol.
- Installation is guided. Bundle adapters with the normal install, provide
  exact instructions for external requirements, and recheck after each
  step. Do not implement a universal package manager or run downloaded
  installation scripts automatically.
- Configuration remains in the files that own it. This task adds no core
  config keys or direct integration toggles to `settings.toml`.
- Provider-specific authentication and setup stay in provider executables.
  The catalog explains or launches an explicitly supported setup command;
  it does not implement Harvest, ccmux, or Hub business logic.

## Requirements

### Entry points

- Add **Addons** to Settings (`s`). It remains available when every addon is
  off. Use the existing picker, detail/help, search, and navigation patterns.
  Do not add a global shortcut.
- Add these CLI forms and document them in usage:

  ```text
  asanamate addons                    list addons and local status
  asanamate addons <id>               show setup and removal instructions
  asanamate addons check <id>         run the documented local checks
  ```

  Listing and guides work without an Asana token or completed core setup.
  A missing core config is shown as **core setup required**, not a catalog
  failure. Invalid existing config shows its validation error without hiding
  installation guides or proposing destructive repairs.
  Unknown IDs produce an actionable error and the available IDs. A failed
  requested check exits nonzero; an unperformed remote check is reported as
  unverified, not successful.
- End `asanamate setup` with a short invitation to run `asanamate addons`
  or open Settings > Addons. Include the same entry point in README Install
  and Setup. Do not interrupt first-run Asana setup with mandatory addon
  prompts.

### Shared catalog

- Give entries a stable ID, user-facing name, benefit, extension points,
  availability, authoritative guide URL, install steps for supported
  platforms, configuration examples, verification steps, and disable/remove
  instructions. List external requirements separately from bundled adapters.
- Start with Harvest (`harvest`), ccmux (`ccmux`), and imarc Hub (`hub`). A
  single Harvest entry covers time tracking and feeds. Label Hub as private
  and requiring imarc access before sending users to its repository.
- Include an **Actions and custom commands** entry that opens the existing
  action builder and links to instructions for all six extension points.
  Custom agent, repo, project-link, tracker, and feed commands remain valid
  without registering in the catalog.
- Core/terminal features such as clipboard support and inline images are
  not addons. Keep their existing Optional features documentation.
- Store catalog content and known command descriptions in one small package,
  following repository conventions. CLI and TUI render the same content.
  Adding an entry must not require changing picker or command dispatch logic.
  Use ordinary data and functions; do not build a workflow engine.
- Do not maintain a second enabled flag, installation database, or credential
  store. Derive status from executable availability and current config.

### List and detail UX

- Each row shows name, one-line benefit, and concise status. Never hide an
  entry because it is unavailable or disabled.
- The detail view follows this order:
  1. What it adds, including each independently configurable capability.
  2. Requirements and current local status.
  3. Numbered installation, authentication, and configuration steps.
  4. Checks and a first-use example that demonstrates success.
  5. Disable, uninstall, and help links.
- Distinguish **adapter missing**, **dependency missing**, **not configured**,
  **configured**, **local check passed**, **connection unverified**, and
  **check failed**. Show status per capability where an addon supplies more
  than one. One missing feed setting must not make working time tracking
  appear broken.
- Show the next unresolved step and exact commands or TOML needed. Support
  copying commands/snippets and opening guide URLs through existing helpers.
  If clipboard or browser opening fails, leave the text/URL visible.
- Offer **Recheck** and, where supported, **Run provider setup**. The latter
  displays the command and resolved executable path, then suspends the TUI
  using the existing foreground process pattern. Provider prompts retain
  control over their own writes. Cancellation returns to addon details.
- Display configuration snippets and the actual config path. Users can open
  the existing config editor, or edit the file themselves. Do not overwrite
  custom providers or merge snippets into files automatically in this task.
  Explain that addon config edits require restarting asanamate; distinguish
  saved config from the current session's active config.
- Content wraps or scrolls in narrow terminals, works without Nerd Fonts or
  color, and leaves navigation and next actions visible.

### Installation and setup guidance

- Update normal archive installation to extract and install all bundled
  adapter executables alongside asanamate. Installing an adapter does not
  enable it, authenticate it, or install its external dependencies.
- Keep source installation equivalent, using the existing `install.sh` or
  documented `go install ./cmd/...` plus theme installation. Check archives,
  PATH guidance, macOS quarantine instructions, and uninstall instructions
  remain consistent. Publishing Homebrew is not a prerequisite.
- Catalog instructions identify the install method, supported OS, dependency
  versions, authoritative source, and final executable location. Never
  suggest fetching a different asanamate release just to obtain its adapter.
- External package installation remains a visible command for the user to
  run. Do not install Node.js, package managers, tools, hooks, or private
  repositories merely by opening the screen or selecting an addon.
- For Harvest, use the existing CLI-based setup until spec 21 lands. Spec 21
  updates that entry to native setup; this task must ship independently.

### Checks and trust boundaries

- Listing and opening details perform local inspection only. Do not launch
  commands, contact addon services, or prompt for secrets during discovery.
- Explicit checks may run only cataloged, documented read-only checks.
  Invoke known executable/argument vectors directly, with bounded execution
  and output. A check must never create a time entry or mutate remote data.
- Do not infer addon identity by searching arbitrary shell command strings,
  execute an unknown command to discover its behavior, or send credentials
  to a URL supplied by catalog output. Recognize only known command forms;
  otherwise show **custom command** and its documented verification path.
- Extend `doctor` to reuse these checks for recognized configured addons and
  report what was checked. Preserve current behavior for custom commands.
  Optional failures remain warnings and do not block browsing. If no safe
  check exists, say **unverified** and supply the next verification step.
- Authentication checks distinguish local credential availability from
  live authentication. A local success is never labeled connected. Live
  checks, when supported, require an explicit user action and identify the
  account/service being contacted.
- Never log tokens or credential file contents. Clean provider diagnostics
  before rendering them and keep arbitrary terminal controls out of output.

### Documentation and compatibility

- Add a concise README Addons section linking to the shared CLI/TUI paths
  and existing detailed guides. Keep `docs/extensions.md` as the protocol
  contract, with a link back to addon onboarding.
- Preserve all released config keys, commands, protocol versions, custom
  shell commands, and saved provider IDs. No config migration is required.
- Add no integration-specific authentication or network client to core.

## Out of scope

- Remote catalogs, automatic addon discovery from the network, third-party
  manifest loading, dependency resolution, and automatic updates.
- A general installer, automatic privilege escalation, or package-manager
  abstraction.
- Automatic core config changes, integration toggles, and hot reloading.
- Replacing providers or implementing their authentication in the TUI.

## Acceptance

- A fresh user can find Harvest, ccmux, and Hub from Settings and CLI with
  every integration off; each explains benefit, access, and next steps.
- Normal release installation puts bundled adapters on PATH without enabling
  them. Archive and source instructions reach equivalent results.
- Each catalog entry has a complete install, configure, verify, and remove
  path. Missing prerequisites and failed browser/clipboard access have clear
  fallbacks.
- Harvest time tracking and feed configuration are reported independently.
- A custom provider remains functional and is never mistaken for a known
  provider or executed during discovery.
- CLI and TUI use the same catalog and check results. Checks distinguish
  local readiness, unverified remote access, and failure.
- Cancelling provider setup leaves core config unchanged. Invalid edits are
  reported by the existing config validation path.
- Relevant existing CLI, TUI, config, and archive checks pass. During
  implementation, add focused coverage for the acceptance cases after
  obtaining the required permission to write test files; run `go test ./...`
  and `go vet ./...`. No test files are part of this specification task.
- Manually walk one bundled addon, one private/unavailable addon, and one
  custom command in a clean temporary config on macOS and Linux. Verify
  narrow-terminal navigation and disabled-feature discovery.
