# asanamate Portability Implementation Plan

Date: 2026-10-05
Updated: 2026-10-13
Status: Tasks 1 to 5 done; v0.2.0 published 2026-10-13; Homebrew (Task 6) open

## Goal and scope

Finish the remaining portability and release work identified in the 2026-10-11
review. Follow the [portability specification](10-portability-release.md)
and preserve existing user configuration, actions, and adapter protocols.

This plan replaces the old P0/P1 checklist and branch progress notes. It covers
unfinished work; do not rebuild features already implemented. Product
implementation, package removal, live writes, commits, tags, and release secrets
are not authorized merely by recording this plan.

## Verified baseline

The review used main `ea94c81` on 2026-10-11. Recheck the starting commit before
implementation and use evidence matching the final candidate.

| Original work | Current status |
|---|---|
| Repository setup | Skippable; missing `~/code` defaults to skip |
| Repository discovery | Native `repo_source.root`, worktrees, home-relative paths; custom command support retained |
| Harvest portability | Real task names and eligible projects; separate `asanamate-hrvst` provider |
| Agent portability | Separate `asanamate-ccmux`; opt-in sources and Codex hook repair |
| Optional feature failures | Clipboard feedback, browser URL/error, missing-Git message |
| Onboarding documentation | Install/checksum/PATH, token persistence, optional dependencies, moving guide |
| Local automated proof | 906 tests and race tests, vet, staticcheck, four binaries cross-built for four target combinations |
| Earlier CI | [Successful run at ca32942](https://github.com/sadmachine/asanamate/actions/runs/37502994010): macOS/Linux checks and all four native archive smoke jobs |
| Published distribution | Public repository; v0.1.0 published on 2026-10-06 UTC; current adapter/theme changes need candidate proof |
| Homebrew | Cask generation configured; publishing disabled with `skip_upload: true` |

The isolated current-main doctor check correctly distinguished a missing token
from missing optional tools with an empty `PATH`. Current archive verification
was unavailable locally because GoReleaser was not installed. Live Asana browsing,
current packaged onboarding, and a real Homebrew install remain unverified.

## Execution rules

- Read `AGENTS.md`, trace current behavior, and match existing patterns before
  editing. This plan's baseline is not permission to overwrite newer work.
- No subagents unless explicitly requested. Prefix external commands with `rtk`;
  use the caveman proxy for model provider executables.
- Ask before writing or committing test files. The proof requirements below
  identify coverage needed after permission; they do not grant it in advance.
- Preserve released contracts. If a breaking change becomes necessary, follow
  `AGENTS.md`, including approval, migration, commit marking, and version bump.
- Do not edit personal config, credentials, shell files, or live hooks during
  routine checks. Use temporary directories and fixtures.
- Do not run user-defined actions or submit Asana/Harvest writes for readiness
  checks. Use a designated authorized account for read-only live checks.
- Keep tasks independently reviewable. Run focused checks for changed owners;
  run the full CI-equivalent gates once the candidate is assembled.
- After implementation changes action/config files, ask whether to migrate local
  config. On yes, preserve values and create the required backups before edits.

## Task 1 Reconcile the specification and implementation status

Priority: P0. Dependencies: none. Status: documentation update completed here.

Files: this plan and the linked specification.

- [x] Replace private/unreleased assumptions with the public v0.1.0 baseline.
- [x] Replace obsolete Harvest/hook paths and commands with adapter ownership.
- [x] Separate completed work from outstanding tasks and verification gaps.
- [x] Record the new-install pager target and proposed Hub distribution decision.
- [x] Remove misleading old branch status and unchecked completed checklists.
- [x] Keep candidate-specific proof separate from older CI success.

Acceptance: the docs agree on ownership, released contracts, remaining work, and
what has actually been verified. Relative links resolve. Future agents can start
at Task 2 without inferring that the release gate has passed.

## Task 2 Make the default pager action opt in

Priority: P0. Dependencies: Task 1. Status: done 2026-10-13.

Primary files: `internal/setup/setup.go`, `README.md`; existing setup tests after
permission. Review action loading and update behavior before editing.

- [x] Generate `pager.toml.example` for new installations instead of active
  `pager.toml`. Existing example-file loading rules already provide opt-in.
- [x] Explain installing/selecting a pager and renaming the example to activate it.
  Keep the built-in ticket reader as the dependency-free default.
- [x] Remove setup messaging that treats the inactive example's missing pager as
  a setup problem. Preserve warnings for existing active pager actions where
  the diagnostics already support them.
- [x] Preserve existing `pager.toml`, customized actions, action shortcuts, and
  rerun/setup update behavior. Do not silently disable users' active actions.
- [x] Update the optional-feature table and setup summary to match new defaults.

Proof: fresh setup without `less` produces valid config and no enabled pager
action; the example loads after explicit activation; existing pager files remain
byte-for-byte unchanged; browsing requires no pager.

Stop when the default dependency gap is closed. Do not add automatic package
installation, a pager detector framework, or a new config key.

## Task 3 Make diagnostic claims precise

Priority: P0. Dependencies: Task 2 for final pager wording. Status: done 2026-10-13.

Primary files: `README.md`, `docs/extensions.md` only where needed. Product code
changes are unnecessary unless the wording exposes an actual reporting defect.

- [x] Replace claims that doctor verifies every optional feature with its actual
  coverage: core authentication/config, executable presence, native repo discovery,
  and the configured agent source's read-only invocation.
- [x] State that time-tracking and project-link commands remain `unchecked` and
  that clipboard/graphics capabilities are not confirmed by doctor.
- [x] Give manual provider steps: `asanamate-hrvst check` for installed hrvst/local
  credentials and, for Hub users, `asanamate-hub check` for credential variables.
- [x] Explain that these provider checks do not prove remote authentication or
  account permissions. Define a read-only live form/link lookup as separate proof.
- [x] Clarify that an `ok` opener/editor executable can still fail because of a
  desktop session, terminal policy, or command arguments.
- [x] Include relative dates in the dependency guide: GNU date is optional;
  exact `YYYY-MM-DD` works without it. Keep existing Linux/macOS paste instructions.

Proof: every diagnostic promise maps to a current check. Disabled, unchecked,
executable-present, and remotely usable are not presented as equivalent.
No arbitrary shell parsing, provider auto-execution, or registry is added.

## Task 4 Resolve the general package scope for Hub

Priority: P0. Status: done 2026-10-13.

Decision: general release archives and the future cask ship `asanamate`,
`asanamate-hrvst`, and `asanamate-ccmux`. `asanamate-hub` moved to the private
`imarc/asanamate-hub` repository, which documents its install and linking.
v0.1.0 archives held only `asanamate`, so no released asset is removed.

- [x] Remove Hub from builds, the cask, and CI archive and smoke checks.
- [x] Remove `cmd/asanamate-hub`; the provider copies the small project-links
  protocol types it used from `internal/projectlinks`.
- [x] Keep generic project links and existing `project_links.command` settings
  unchanged, along with Hub's credential environment names and protocol.
- [x] Point imarc staff to the Hub repository from the README, the setup
  template comment, and `docs/extensions.md`; drop Hub from install,
  quarantine, and uninstall steps.
- [x] Create the `imarc/asanamate-hub` repository and push the provider;
  tagged v0.2.0 to match asanamate.

## Task 5 Verify the final packaged candidate and onboarding

Priority: P0 release gate. Dependencies: Tasks 2, 3, and 4. Status: done
2026-10-13, with the gaps recorded below.

Primary files: `.github/workflows/ci.yml`, `.github/workflows/release.yml`,
`.goreleaser.yaml`, this plan's evidence record; test changes after permission.

- [x] Identify the final candidate commit and intended tag/version. Older CI
  success and published v0.1.0 do not prove this candidate.
- [x] Run vet, staticcheck, race tests, and all target builds using the versions
  in CI. Use the existing GoReleaser CI action if local GoReleaser is unavailable;
  do not substitute a raw Go build for archive validation.
- [x] Build snapshot archives; verify checksums, binary/theme/license contents,
  version/help, and selected adapter help/check behavior on all four native
  targets. Reuse the current matrix rather than creating another release system.
- [x] Strengthen minimal-environment proof beyond help and a failing doctor:
  setup must complete without optional tools, produce loadable config/actions,
  and browse through read-only local fixtures. Prefer existing setup/API fixtures;
  add no production API override solely for tests.
- [x] Follow README archive installation verbatim on fresh macOS and Linux,
  including checksum tools, `PATH`, theme installation, and Gatekeeper behavior.
- [x] With a designated authorized Asana account, authenticate, browse My Tasks
  and a project, and read a ticket. Exercise required optional features through
  read-only lookups. No comments, ticket edits, time entries, or agent spawns.
- [x] Verify upgrade from v0.1.0 with a temporary copy of representative config
  and actions: migration preserves values, timestamps backups, and documents
  the new adapter install and hook-repair steps. Use temporary hook files.
- [x] Record date, OS/version/architecture, candidate, checksums, CI links, and
  results. Mark unavailable checks as gaps; do not call them passed.

Gaps, accepted for v0.2.0:

- Linux and Intel macOS installs were proven only by the CI smoke jobs, not by
  following the README by hand on a fresh system.
- Minimal onboarding ran against live Asana with a bare `PATH`, not against
  local fixtures.
- The v0.1.0 upgrade check used v0.1.0's config template; v0.1.0 action files
  and hook repair were not exercised.
- The release workflow's upgrade-notes step (`docs/migrations.md` to the
  release header) was added after the tag; the v0.2.0 header was applied by
  hand. Its first real run is the next release.

Release handoff: stage/commit/publish only with appropriate authorization.
Release from the verified candidate. Verify actual published checksums and
installation after publication; a successful snapshot is pre-release evidence.
Any subsequent code or packaging change requires affected checks to be repeated.

Acceptance: the specification's release conditions have matching candidate
evidence. Current archives install and browse without optional tools. Published
install instructions describe the actual released assets.

## Task 6 Finish Homebrew as a separate install milestone

Priority: P1 convenience route. Dependencies: Task 4 and a verified candidate.
This task does not block an explicitly archive-only release.

Primary files: `.goreleaser.yaml`, `.github/workflows/release.yml`, `README.md`;
the actual tap repository and release secret only when authorized.

- [ ] Confirm provisioning authorization for `sadmachine/homebrew-tap` and
  `HOMEBREW_TAP_TOKEN`. Do not create a tap or change secrets from this plan alone.
- [ ] Verify current Homebrew unsigned-cask policy and choose the supported
  approach without implying signing/notarization exists.
- [ ] Generate a cask with the agreed binary list, correct checksums, and theme
  installation that preserves existing theme files.
- [ ] Test install, upgrade, version/help, theme preservation, quarantine behavior,
  and uninstall through the real tap on supported macOS architectures.
- [ ] Remove `skip_upload` only after provisioning and verification; advertise
  Homebrew once the real route works. Keep archive/source instructions available.

Acceptance: the documented brew command works from a fresh system, does not
install optional external toolchains, and preserves user config/state on upgrade
and uninstall. Record Homebrew proof separately from archive proof.

## Candidate evidence record

Fill against the final candidate; historical evidence above is not completion.

| Gate | Required record | Current status |
|---|---|---|
| Package scope | Hub decision and agreed binary list | Hub in `imarc/asanamate-hub`; archives ship three binaries |
| Pager | Fresh inactive example; existing actions preserved | Done; setup tests cover both |
| Diagnostic docs | Precise coverage and manual provider checks | Done |
| Candidate | Commit, version, artifact checksum set | `44a4f13`, tag `v0.2.0`; checksums below |
| Automated checks | Candidate CI URL; tests, vet, staticcheck, all builds | [CI run 38491632659](https://github.com/sadmachine/asanamate/actions/runs/38491632659): tests on Ubuntu and macOS, vet, staticcheck, race, snapshot archives; 928 tests locally with `-race` |
| Native archives | macOS/Linux amd64/arm64 install and smoke results | CI smoke jobs pass on all four targets: bare `PATH`, themes install, version/help, adapters' help, doctor |
| Minimal onboarding | No optional tools; setup and fixture browsing | 2026-10-13, macOS 26.6.2 arm64, `PATH=/usr/bin:/bin`, empty home: setup completes with repos skipped, both actions written as inactive examples, doctor exits 0 |
| Live Asana | Dated read-only auth, My Tasks, project, ticket results | 2026-10-13: auth and workspace ok; `list` My Tasks; `list --project` 155 rows; `show` a ticket; TUI renders My Tasks and the reader. No writes |
| Compatibility | v0.1.0 config/action values and backups preserved | v0.1.0 config template: `config update` moves keys, keeps values, writes a timestamped backup; doctor loads it. Action files not tested |
| Publication | Actual asset checksums and README install result | [v0.2.0 release](https://github.com/sadmachine/asanamate/releases/tag/v0.2.0): README archive steps on macOS arm64, checksum OK, three binaries and 21 themes, `asanamate version` prints 0.2.0, no quarantine with `curl` |
| Homebrew | Real tap install/upgrade/uninstall | Deferred to Task 6; not advertised |

Published v0.2.0 checksums:

```text
f71f4275046b92f9c15bd377d587fb6b1145af8d458b23d7067d980ddef14bd1  asanamate_darwin_amd64.tar.gz
6bc5c3e3c39c6139b450317727c20d910538339da946d5e74f7cd59f27c29156  asanamate_darwin_arm64.tar.gz
e78adf4517b686f6f352b8ebb81e806ce30e7e02cdf7a5b8ea79211e71d4b11f  asanamate_linux_amd64.tar.gz
45c53cbb98ae51a39cb5549d36130b1eea4d9312d26bc6bd51c2b13666743532  asanamate_linux_arm64.tar.gz
```

Stop once Tasks 1 through 5 meet the archive release gate and required decisions
are recorded. Task 6 is a separately tracked install improvement. Do not expand
into Windows, OAuth, automatic dependency installation, or new package ecosystems.
