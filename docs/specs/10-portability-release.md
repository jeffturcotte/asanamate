# asanamate Portability and Release Readiness

Date: 2026-10-05
Updated: 2026-10-13
Status: Core portability implemented; remaining release work tracked in the plan

## Purpose

Someone on a fresh supported system must be able to install asanamate,
authenticate with Asana, and browse tickets without adopting the author's
development environment. Dependencies must either be provided by the install
or belong to an explicitly enabled feature with complete setup instructions.

This specification governs portability and onboarding. Current behavior follows
the implementation and the [roadmap specs](README.md); the release
requirements below define the remaining target. New-install defaults may change,
but existing user configuration and actions must be preserved.

Implementation is tracked in the
[release readiness plan](10-portability-release-plan.md).

The repository is public. [v0.1.0](https://github.com/sadmachine/asanamate/releases/tag/v0.1.0)
was published on 2026-10-06 UTC. Its contracts require compatibility handling;
being pre-1.0 does not permit unapproved breaking changes. This work prepares
the next candidate and does not authorize publishing a tag, changing repository
visibility, or creating release secrets.

## Current implementation and remaining choices

- **Repo discovery:** a native `repo_source.root` key replaces the generated
  `find` command. Go lists the root's direct children in process; no shell,
  quoting, or external tools. `repo_source.command` remains for custom sources
  and takes precedence when set.
- **Pager:** new installations get an inactive `pager.toml.example`, satisfying
  the install-or-opt-in dependency rule. Existing active pager files remain
  unchanged, and `doctor` still warns when an active pager action's pager is
  missing.
- **Adapters:** Harvest and ccmux are separate optional binaries,
  `asanamate-hrvst` and `asanamate-ccmux`, shipped in the release archives.
  The imarc Hub provider, `asanamate-hub`, moved to the private
  `imarc/asanamate-hub` repository (decided 2026-10-13). It was never in a
  released archive, so the move breaks no released contract.
- **macOS Gatekeeper:** no paid signing or notarization. Document `curl`
  downloads, which are not quarantined, with an `xattr -d com.apple.quarantine`
  fallback for browser downloads. The Homebrew cask strips quarantine; recheck
  Homebrew's unsigned-cask policy at release time.
- **Contract:** preserve v0.1.0 configuration and actions, with documented
  migration for previously approved breaking changes. Follow `AGENTS.md` for
  any additional breaking change.
- **Homebrew:** the cask is configured with `skip_upload: true`. Archive and
  source installs are the available routes; Homebrew must not be advertised
  until real tap installation passes. It is a separate convenience milestone.

## Supported release scope

- macOS and Linux, each on amd64 and arm64.
- One core binary per target, with separately installed optional adapters. All
  shipped Go binaries use `CGO_ENABLED=0`; binary users need no Go toolchain.
  Current archive configuration includes the core, two adapters, and theme files.
- A terminal suitable for the TUI, an Asana account and personal access token,
  network access, and working HTTPS certificate trust are core prerequisites.
- Supported operating system versions and Linux environments must be stated
  from verified release results. Cross compilation alone does not establish
  runtime support.
- Release archives provide the baseline install route for both platforms.
  `go install` remains a source install route. Homebrew is planned for macOS
  after tap provisioning and verification.
- Windows, new package ecosystems, OAuth, multi-workspace support, automatic
  repository cloning, and a general dependency installer are outside this work.

## Baseline at audit

The 2026-10-05 audit found the following behavior:

| Area | Current behavior | Release gap |
|---|---|---|
| Packaging | Four target binaries configured in GoReleaser, without CGO | Actual archive and install verification |
| Setup | Authenticates and selects workspace, then requires an existing repo directory | Browsing cannot complete setup without repo configuration |
| Repo picker | Generated shell command uses `find` and `dirname` | External utilities, limited path quoting, absolute local root |
| Pager | Enabled action uses `$PAGER`, else `less` | Unchecked default dependency |
| Optional integrations | Agents and time tracking are off; Claude/tmux example is inactive | Complete install, authentication, compatibility, and disable instructions |
| Clipboard | Link copy uses OSC 52; input `Ctrl+V` uses an OS clipboard dependency | Undocumented Linux utilities and remote-session limits |
| Diagnostics | `doctor` validates config/actions and diagnoses agent linkage | No general readiness report or token check without a ticket argument |
| Harvest | Configurable numeric task ID, fixed Engineering label | Author-specific task naming and external credential-file assumptions |
| Machine relocation | Config stores a repo root; state stores absolute repo links; Codex hooks store executable paths | Clear recovery and hook repair |
| CI | Tests and vet on Ubuntu | macOS runtime checks and packaged-binary smoke checks |

No runtime path tied to the author's username was found. The existing suite
passed 486 tests, vet passed, and all four release targets built. These results
do not establish fresh-machine installation or live integration compatibility.

Current implementation references for the areas in the original audit:
[setup](../../internal/setup/setup.go),
[repo operations](../../internal/repo/repo.go),
[CLI and doctor](../../cmd/asanamate/main.go),
[Harvest provider](../../cmd/asanamate-hrvst/harvest.go),
[hook installer](../../internal/ccmux/codexhook.go),
[release configuration](../../.goreleaser.yaml), and
[CI](../../.github/workflows/ci.yml).

At the 2026-10-11 review of main `ea94c81`, setup was skippable, repo discovery
was native, Harvest used real task names, hooks supported repair, and optional
feature errors and moving instructions were implemented. All 906 tests passed,
including the race run; vet, staticcheck, and cross builds passed. Earlier CI
passed native smoke checks on all four targets at `ca32942`. That older run does
not prove the current adapter/theme archives, live onboarding, or Homebrew.

## Required behavior

### Installation

The README must provide a complete path from download to `asanamate version`.
Manual installation covers target selection, checksum verification, extraction,
executable placement in a user-writable directory, and shell `PATH` setup.
Examples must use placeholders or detected platform values, never personal paths.

Source installation states the Go version required by the current `go.mod`,
where Go installs executables, and how to add that directory to `PATH`.
Release documentation covers upgrade and uninstall without silently deleting
configuration, state, ticket exports, or external agent hooks.

Any runtime dependency required by an enabled default feature must be installed
by the supported install route or removed from that default feature. Optional
toolchains must not become mandatory package dependencies.

### First run

Setup must complete with an Asana token and workspace alone. Repository setup
must be skippable. A missing `~/code`, missing Git, missing editor, missing
pager, and missing agent tools must not prevent browsing.

Repository selection remains available later through existing configuration and
repo-link flows. Setup must explain what was configured and how to enable
optional features. It must preserve existing actions and ask before replacing
existing configuration. Token values must never appear in output or diagnostics.

Token instructions must explain session-only export, persistence across new
shells, and propagation to tmux or other launch environments. asanamate must not
write the token into its config or silently edit shell startup files.

### Repository discovery and paths

The default repo picker must not depend on `find` or `dirname`. Use native Go
directory discovery in the repo package, configured by `repo_source.root`.
Preserve custom `repo_source.command` support and the existing manual path
entry flow.

Default discovery remains limited to repositories directly inside the selected
directory. Recognize both `.git` directories and worktree `.git` files. Do not
add recursive scanning, cloning, or an index service.

Paths containing spaces, apostrophes, Unicode, and shell metacharacters must be
handled as data. The line-based custom command protocol remains unchanged;
newline-containing paths must receive an explicit unsupported-path error where
that protocol cannot represent them safely.

Absolute paths chosen by a user are legitimate local state, not author-specific
defaults. Copied config and state must have a documented recovery path after
directory relocation. Stale links must continue to prompt for reselection rather
than execute an action in the wrong repository.

### Optional features

Every shipped optional feature must document its prerequisites, installation
source, required authentication, supported versions or capabilities, activation,
verification, disable steps, and common failure modes.

| Feature | Prerequisites or capabilities | Required default behavior |
|---|---|---|
| Pager action | `$PAGER`, else `less` | Inactive example for new installs; preserve existing active actions |
| Config editing | `$VISUAL` or `$EDITOR`, otherwise available `vi`; shell | Missing editor produces actionable instructions; direct file editing remains possible |
| Repo actions and branch tracking | Compatible Git; shell for configured commands | Not required for browsing |
| Browser opening | macOS `open`, or Linux `xdg-open` and a usable desktop/browser | Failure shows the URL and a useful message |
| Clipboard link copy | Terminal OSC 52 support, including any multiplexer policy | Document capability limits; do not claim clipboard confirmation |
| Input `Ctrl+V` | macOS `pbpaste`; appropriate Linux clipboard utilities and session | Missing utility must not break input; terminal paste remains available |
| Agents and sessions | Selected provider, agent, and session tools | Off until configured; examples inactive |
| Harvest time tracking | `asanamate-hrvst`, compatible `hrvst`, Harvest login, assigned project/task | Off until configured; no assumed task name |
| Project links | User-selected provider and its credentials | Off until configured; no organization required by core |
| Relative date input | GNU `date`, or `gdate` from coreutils on macOS | Optional; exact `YYYY-MM-DD` works without it |
| Images | Supported terminal graphics and optional tmux passthrough | Automatic mode disables unsupported graphics |
| Nerd Font symbols | A Nerd Font | Opt-in; Unicode and ASCII remain usable |
| OS motion preference | Platform preference tool when available | Missing tool does not block startup |

Do not parse arbitrary shell programs to infer all their dependencies. Known
built-in integrations can identify their requirements; custom commands remain
user-managed and must be described that way.

### Diagnostics

Extend the existing `doctor` command rather than introducing a separate health
service. With no task argument, it must summarize configuration validity, file
locations, core authentication readiness, and known enabled integrations.

Reports distinguish blocking core failures, optional feature failures, and
disabled or unverified features. Blocking failures return a nonzero status.
Optional warnings do not prevent basic browsing. Authentication checks are
read-only, bounded, and do not expose credentials.

An `ok` executable check establishes presence, not browser/session usability or
provider authentication. Configured time tracking and project-link commands may
remain `unchecked`; documentation must give separate safe verification steps.
`asanamate-hrvst check` checks installed tooling and local credentials, not remote
account access. `asanamate-hub check` checks credential environment variables,
not authentication. Provider function requires a separate read-only live check.

Diagnostics must not run actions, submit time, switch branches, install tools,
or modify configuration. Retain existing agent-link diagnostics, including the
documented invocation of configured agent sources. Do not execute new arbitrary
provider commands merely to discover whether they are installed.

When Git is missing, report that fact instead of treating every path as an
invalid repository. Failure messages name the affected feature and give a
concrete recovery step or documentation link.

### Harvest portability

The Harvest form must show the actual configured task name and validate that
the task is available for the chosen project. Preserve configured task IDs,
saved project choices, Asana link metadata, and the rule against automatic retry
after potentially successful creation.

Document and verify the supported `hrvst` command output and credential-file
contract. If supported `hrvst` versions expose configuration overrides, use their
documented behavior rather than guessing locations. Do not introduce a new
credential store or require Harvest for core setup.

### Hooks and moving systems

Keep hook installation opt-in. Existing hook groups and unrelated hooks must be
preserved and backed up before modification. Installation must distinguish
complete current hooks from stale, partial, or relocated asanamate hooks.

`asanamate-ccmux setup` offers repair when the installed hook command no longer
matches the usable adapter. Detecting an old asanamate hook must not silently
suppress needed repairs. Repeated installation or repair must not duplicate hooks.
The legacy `asanamate hook codex` forwarding path remains subject to its documented
deprecation schedule; this task does not accelerate its removal.

Document what to copy between machines, what to regenerate, how to relink repos,
how to repair/remove hooks, and how to authenticate again. Config, state, exported
tickets, and logs may contain private workspace data; token instructions must
remain separate from file-copy instructions.

## Architecture and compatibility

- Keep setup orchestration in `internal/setup`, repo operations in `internal/repo`,
  provider protocol handling in `internal/timetracking`, Harvest implementation
  in `cmd/asanamate-hrvst`, ccmux/hooks in the adapter and `internal/ccmux`, and
  UI messaging in the TUI. Reuse existing config, state, and hook helpers.
- Prefer standard library implementations and existing dependencies. Do not add
  a plugin framework, shell dependency parser, installer daemon, or generic
  integration abstraction for this work.
- Preserve `repo_source.command` and existing custom actions. Default discovery
  uses the native `repo_source.root` key rather than a generated command.
- Do not automatically rewrite an existing user's custom repo command when
  changing defaults for new installations.
- Before adding config keys, follow `AGENTS.md`: choose the correct shape and
  update structs/comments, defaults, validation, template, README, and tests.
- Verify release history before any public contract change. Released keys and
  accepted values remain compatible unless the user approves a breaking change.
- Local configuration migration is a separate user decision after implementation;
  follow the backup and `doctor` requirements in `AGENTS.md`.

## General distribution and organization adapters

Core setup, browsing, default actions, and general install instructions must
require no imarc account, Hub credentials, or organization-specific filesystem
layout. Keep the generic project-links protocol independent of any provider.

Decision (2026-10-13): `asanamate-hub` lives in the private
`imarc/asanamate-hub` repository, with its own install and linking
instructions, and is not part of general archives or the future cask. The
generic project-links protocol and existing `project_links.command` settings
are unchanged. Do not create a configurable Hub URL, a second release pipeline
in this repository, or a new credential system for it.

## Release acceptance

The release is ready only when all of the following are satisfied:

1. A fresh supported system installs a release binary and completes setup without
   Git, a repo directory, a pager, tmux, agent tools, or Harvest.
2. That system browses My Tasks and a project and reads a ticket with only core
   prerequisites installed.
3. Optional features activate through documented steps and fail locally with
   actionable messages when a dependency or capability is missing.
4. General executable defaults, templates, and install instructions embed no
   author usernames, real workspace/task IDs, repo paths, or required organization
   settings. Organization adapters are separated.
5. Alternate home/XDG paths and supported unusual path characters work. Stale
   links and relocated hooks recover without losing unrelated settings.
6. All four target archives contain runnable binaries and valid checksums.
   Runtime coverage and any architecture verification gaps are recorded honestly.
7. Automated checks pass and live install/authentication/integration checks have
   dated evidence against the release candidate. Build success is not a substitute
   for those checks.
8. Documentation describes the exact published candidate and the limits of each
   diagnostic. Optional distribution exceptions are decided and recorded, not
   silently treated as meeting the general-package requirement.

Evidence must identify the candidate commit and artifact checksums. Any code or
packaging change after verification invalidates the affected evidence. A release
must use the verified artifacts or rebuild from the same candidate and recheck
the published artifacts before claiming the install route works. Homebrew is not
part of the archive release gate while it remains explicitly unavailable.

Stop implementation when these conditions are met. Defer unrelated product
features and convenience installers to separate specifications.
