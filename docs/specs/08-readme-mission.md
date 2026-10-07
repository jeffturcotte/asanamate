# 08: README mission and scope

**Depends on 02, 03, and 05.**

## Goal

The README's first screen states what asanamate is for, what it covers today,
and what it leaves to Asana, so a reader knows at once whether it fits their
work.

## Context

- `README.md` lines 1-30: the tagline ("A terminal UI for reading Asana
  tickets and running your own commands on them") and the "What it does"
  list.
- The tagline describes an earlier, narrower goal. The project's goal is in
  `docs/specs/README.md`.

## Requirements

- Rewrite the tagline paragraph around three ideas: daily Asana work in the
  terminal, layouts that fit any terminal workflow (full screen, tmux pane,
  sidebar, popup), and extension through your own commands.
- Keep "What it does" accurate to the code on `main` when you merge. Name each
  feature once and link to its README section.
- Add a short "Extending asanamate" line or list that names the extension
  points: actions, agent sources, time tracking providers, and repo sources.
  Link to `docs/extensions.md` (05).
- Keep the "does not do" line current. 02 and 03 add completing and creating
  tickets, so remove those from it and name what is still left to Asana.

## Out of scope

- Restructuring the rest of the README.
- Documenting features that are not merged.

## Acceptance

- The first screen of the README (about 40 lines) gives the goal, the feature
  list, the extension points, and the limits.
- Every link in the new text resolves to a heading in the README.
- No claim in the new text contradicts the code on `main`.
